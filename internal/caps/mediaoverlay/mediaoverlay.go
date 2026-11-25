// Package mediaoverlay builds EPUB3 Media Overlays (read-along) from per-chapter
// audio and recognizer word timings (SPEC-实现约束 §5.14).
//
// The capability never runs a recognizer: timings come from `epub book
// transcribe` or any tool writing contracts/schemas/v2/word-timings.schema.json.
package mediaoverlay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
)

const (
	CapabilityID = "epub.media.overlay.build"
	ActiveClass  = "mo-active"
	idPrefix     = "mo-"
	minMatchRate = 0.90
	weakRate     = 0.5
	edgeInfo     = 5.0 // 片头/片尾超过这么多秒没覆盖就报 info
	// 语速体检：对上至少 rateMinWords 个词的句子，语速快于 rateFast 或慢于 rateSlow 词/秒
	// 多半是时间轴坏了（识别器把一段词压到同一时刻，或时间拉散）；一章有 rateMinSuspects 句就报 warn。
	rateMinWords    = 5
	rateFast        = 9.0
	rateSlow        = 0.5
	rateMinSuspects = 3

	minChapterWords = 50 // 默认配对时，少于这么多词的 chapter 文档（封面、扉页）不算

	maxTimingsBytes = 64 << 20
	maxMapBytes     = 1 << 20
)

// Params 是 epub.media.overlay.build 的参数。
type Params struct {
	AudioDir    string // 每章一个音频
	AudioMap    string // 可选 JSON：{"<xhtml href 或容器路径>": "<音频文件名>"}
	TimingsDir  string // word-timings JSON 目录
	Granularity string // sentence | paragraph
	Narrator    string
}

type chapterFact struct {
	XHTML     string   `json:"xhtml"`
	Audio     string   `json:"audio"`
	SMIL      string   `json:"smil"`
	Segments  int      `json:"segments"`
	BookWords int      `json:"bookWords"`
	Heard     int      `json:"heardWords"`
	Matched   int      `json:"matchedWords"`
	MatchRate float64  `json:"matchRate"`
	Weak      []string `json:"weakSegments"`
	Suspect   []string `json:"timingSuspects"`
	Begin     float64  `json:"begin"`
	End       float64  `json:"end"`
	Duration  float64  `json:"audioDuration"`
}

type timingsFile struct {
	SchemaVersion string `json:"schemaVersion"`
	Audio         struct {
		Path     string  `json:"path"`
		SHA256   string  `json:"sha256"`
		Duration float64 `json:"duration"`
	} `json:"audio"`
	Words [][]json.RawMessage `json:"words"`
}

type chapterPlan struct {
	item      opf.ManifestItem
	audioDisk string
	audioName string // 容器内路径
	smilName  string
	smilID    string
	audioID   string
	doc       string
	plan      xhtml.ReadAlongPlan
	align     ChapterAlignment
	duration  float64
}

// Run 执行本 capability：扫描（只读）→ 应用（唯一写点）→ 报告。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if b == nil {
		return report.Result{}, errors.New("media overlay requires an EPUB book")
	}
	edits, files, facts, findings, err := scanPhase(ctx, b, p)
	if err != nil {
		return report.Result{}, err
	}
	failed := slices.ContainsFunc(findings, func(f report.Finding) bool { return f.Level == "error" })
	if failed {
		edits, files = nil, nil
	}
	if err := b.Apply(edits); err != nil {
		return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
	}
	for _, f := range files {
		if err := b.AddFile(f[0], f[1]); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
	}
	status := report.StatusComplete
	if failed {
		status = report.StatusFailed
	}
	if facts == nil {
		facts = []chapterFact{}
	}
	return report.Result{
		Capability: CapabilityID,
		Status:     status,
		Facts:      map[string]any{"chapters": facts},
		Findings:   findings,
	}, nil
}

func errFinding(id, title, detail, loc string) report.Finding {
	return report.Finding{Level: "error", ID: "media-overlay." + id, Title: title, Detail: detail, Location: loc}
}

func scanPhase(ctx context.Context, b *book.Book, p Params) ([]editset.Edit, [][2]string, []chapterFact, []report.Finding, error) {
	var findings []report.Finding
	fail := func(f report.Finding) ([]editset.Edit, [][2]string, []chapterFact, []report.Finding, error) {
		return nil, nil, nil, append(findings, f), nil
	}
	paragraphs := false
	switch p.Granularity {
	case "", "sentence":
	case "paragraph":
		paragraphs = true
	default:
		return fail(errFinding("bad-granularity", "granularity must be sentence or paragraph", p.Granularity, ""))
	}
	if p.AudioDir == "" || p.TimingsDir == "" {
		return fail(errFinding("missing-input", "audio_dir and timings_dir are required", "", ""))
	}

	container, err := b.CurrentContext(ctx, opf.ContainerPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read %s: %w", opf.ContainerPath, err)
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	opfData, err := b.CurrentContext(ctx, opfPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read %s: %w", opfPath, err)
	}
	pkg, err := opf.Parse(opfPath, opfData)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	tree, err := opf.ScanSpanTreeContext(ctx, opfData)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: %w", opfPath, err)
	}
	items := opf.ManifestNodes(tree)
	usedIDs := map[string]bool{}
	for _, n := range items {
		if id, _ := n.AttrByLocal("", "id"); id != "" {
			usedIDs[id] = true
		}
		if _, ok := n.AttrByLocal("", "media-overlay"); ok {
			return fail(errFinding("already-present", "the book already has media overlays", "remove them before rebuilding", opfPath))
		}
	}
	for _, it := range pkg.Manifest {
		if it.MediaType == "application/smil+xml" {
			return fail(errFinding("already-present", "the book already has media overlays", it.Href, opfPath))
		}
	}

	chapters, audio, fs := pickChapters(ctx, b, pkg, p)
	if len(fs) > 0 {
		return nil, nil, nil, fs, nil
	}
	byHash, fs := loadTimings(ctx, p.TimingsDir)
	if len(fs) > 0 {
		return nil, nil, nil, fs, nil
	}

	opfDir := path.Dir(opfPath)
	var plans []chapterPlan
	for i, item := range chapters {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, nil, err
		}
		disk := filepath.Join(p.AudioDir, audio[i])
		sum, err := book.FileSHA256Context(ctx, disk)
		if err != nil {
			findings = append(findings, errFinding("audio-unreadable", "cannot read audio file", err.Error(), disk))
			continue
		}
		tf, ok := byHash[sum]
		if !ok {
			findings = append(findings, errFinding("timings-missing", "no word timings for this audio file",
				"run `epub book transcribe`, or write timings for sha256 "+sum, disk))
			continue
		}
		data, err := b.CurrentContext(ctx, item.ArchivePath)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("read %s: %w", item.ArchivePath, err)
		}
		if err := opf.EditableUTF8(data); err != nil {
			findings = append(findings, errFinding("unsupported-encoding", "XHTML encoding cannot be edited safely", err.Error(), item.ArchivePath))
			continue
		}
		stem := strings.TrimSuffix(path.Base(item.ArchivePath), path.Ext(item.ArchivePath))
		plan, err := xhtml.PlanReadAlong(string(data), idPrefix+safeToken(stem)+"-", paragraphs)
		if err != nil {
			findings = append(findings, errFinding("segment-failed", "cannot plan read-along segments", err.Error(), item.ArchivePath))
			continue
		}
		words, err := tf.words()
		if err != nil {
			findings = append(findings, errFinding("timings-invalid", "word timings are malformed", err.Error(), disk))
			continue
		}
		texts := make([]string, len(plan.Segments))
		for k, s := range plan.Segments {
			texts[k] = s.Text
		}
		al := Align(texts, words, tf.Audio.Duration)
		ext := strings.ToLower(path.Ext(audio[i]))
		cp := chapterPlan{
			item: item, audioDisk: disk, doc: string(data), plan: plan, align: al, duration: tf.Audio.Duration,
			audioName: path.Join(opfDir, "Audio", safeToken(stem)+ext),
			smilName:  path.Join(opfDir, "Smil", safeToken(stem)+".smil"),
			smilID:    uniqueID(usedIDs, "mo-smil-"+safeToken(stem)),
			audioID:   uniqueID(usedIDs, "mo-audio-"+safeToken(stem)),
		}
		for _, name := range []string{cp.audioName, cp.smilName} {
			if b.Has(name) {
				findings = append(findings, errFinding("entry-exists", "target entry already exists", name, name))
			}
		}
		plans = append(plans, cp)
	}

	var facts []chapterFact
	for _, cp := range plans {
		f := chapterFact{
			XHTML: cp.item.ArchivePath, Audio: cp.audioName, SMIL: cp.smilName,
			Segments: len(cp.plan.Segments), BookWords: cp.align.BookWords, Heard: cp.align.Heard,
			Matched: cp.align.Matched, MatchRate: round3(cp.align.MatchRate()), Weak: []string{}, Suspect: []string{},
			Duration: round3(cp.duration),
		}
		if n := len(cp.align.Clips); n > 0 {
			f.Begin, f.End = round3(cp.align.Clips[0].Begin), round3(cp.align.Clips[n-1].End)
		}
		for k, c := range cp.align.Clips {
			if c.Words > 0 && float64(c.Matched)/float64(c.Words) < weakRate {
				f.Weak = append(f.Weak, cp.plan.Segments[k].ID)
			}
			if c.Matched >= rateMinWords && c.Spoken >= 0 {
				if rate := float64(c.Matched) / max(c.Spoken, 1e-3); rate > rateFast || rate < rateSlow {
					f.Suspect = append(f.Suspect, fmt.Sprintf("%s (%d words in %.1fs)", cp.plan.Segments[k].ID, c.Matched, c.Spoken))
				}
			}
		}
		facts = append(facts, f)
		loc := cp.item.ArchivePath
		switch {
		case len(cp.plan.Segments) == 0:
			findings = append(findings, errFinding("no-segments", "chapter has no read-along text", "", loc))
		case cp.align.MatchRate() < minMatchRate:
			findings = append(findings, errFinding("low-match",
				"too few words match the audio; the audio is probably for another chapter",
				fmt.Sprintf("matched %d of %d words (%.1f%%), audio %s", cp.align.Matched, cp.align.BookWords, 100*cp.align.MatchRate(), filepath.Base(cp.audioDisk)), loc))
		}
		if len(f.Weak) > 0 {
			findings = append(findings, report.Finding{Level: "warn", ID: "media-overlay.weak-segments",
				Title: "some segments barely match the audio; their timing is interpolated", Detail: strings.Join(f.Weak, " "), Location: loc})
		}
		if len(f.Suspect) >= rateMinSuspects {
			shown := f.Suspect[:min(len(f.Suspect), 5)]
			findings = append(findings, report.Finding{Level: "warn", ID: "media-overlay.timing-suspect",
				Title:  "word times look broken: some sentences are spoken impossibly fast or slow, so highlights will lag or stall",
				Detail: fmt.Sprintf("%d sentences, e.g. %s; re-run epub book transcribe with a whisper.cpp that supports --dtw", len(f.Suspect), strings.Join(shown, ", ")), Location: loc})
		}
		if f.Begin > edgeInfo || (cp.duration > 0 && cp.duration-f.End > edgeInfo) {
			findings = append(findings, report.Finding{Level: "info", ID: "media-overlay.uncovered-edges",
				Title:  "audio before the first or after the last segment is not played",
				Detail: fmt.Sprintf("first %.1fs, last %.1fs of %.1fs", f.Begin, f.End, cp.duration), Location: loc})
		}
	}
	if slices.ContainsFunc(findings, func(f report.Finding) bool { return f.Level == "error" }) {
		return nil, nil, facts, findings, nil
	}

	edits, files, err := buildEdits(ctx, b, opfPath, opfData, tree, plans, p.Narrator)
	if err != nil {
		var f *editFailure
		if errors.As(err, &f) {
			return nil, nil, facts, append(findings, errFinding("edit-failed", "cannot place edits safely", f.msg, f.loc)), nil
		}
		return nil, nil, nil, nil, err
	}
	return edits, files, facts, findings, nil
}

var chapterTypeRe = regexp.MustCompile(`epub:type\s*=\s*["'][^"']*\bchapter\b`)

// pickChapters 返回要配音频的章节与对应音频文件名（AudioDir 内）。
func pickChapters(ctx context.Context, b *book.Book, pkg *opf.Package, p Params) ([]opf.ManifestItem, []string, []report.Finding) {
	spine := opf.SpineXHTMLItems(pkg)
	if p.AudioMap != "" {
		raw, err := book.ReadFileContext(ctx, p.AudioMap, maxMapBytes)
		if err != nil {
			return nil, nil, []report.Finding{errFinding("audio-map-unreadable", "cannot read audio map", err.Error(), p.AudioMap)}
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, nil, []report.Finding{errFinding("audio-map-invalid", "audio map must be a JSON object of strings", err.Error(), p.AudioMap)}
		}
		var items []opf.ManifestItem
		var audio []string
		used := map[string]bool{}
		for _, it := range spine {
			for _, key := range []string{it.Href, it.ArchivePath} {
				if a, ok := m[key]; ok {
					items, audio = append(items, it), append(audio, a)
					used[key] = true
					break
				}
			}
		}
		var unknown []string
		for k := range m {
			if !used[k] {
				unknown = append(unknown, k)
			}
		}
		if len(unknown) > 0 {
			slices.Sort(unknown)
			return nil, nil, []report.Finding{errFinding("audio-map-unknown", "audio map names documents that are not in the spine", strings.Join(unknown, ", "), p.AudioMap)}
		}
		return items, audio, nil
	}
	entries, err := os.ReadDir(p.AudioDir)
	if err != nil {
		return nil, nil, []report.Finding{errFinding("audio-dir-unreadable", "cannot list audio directory", err.Error(), p.AudioDir)}
	}
	var audio []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.Type().IsRegular() && (ext == ".mp3" || ext == ".m4a") && !strings.HasPrefix(e.Name(), ".") {
			audio = append(audio, e.Name())
		}
	}
	slices.Sort(audio)
	var items []opf.ManifestItem
	for _, it := range spine {
		data, err := b.CurrentContext(ctx, it.ArchivePath)
		if err != nil {
			continue
		}
		if chapterTypeRe.Match(data) && readableWords(string(data)) >= minChapterWords {
			items = append(items, it)
		}
	}
	if len(items) != len(audio) {
		names := make([]string, len(items))
		for i, it := range items {
			names[i] = it.Href
		}
		return nil, nil, []report.Finding{errFinding("chapter-count-mismatch",
			"number of chapters and audio files differ; give an audio_map",
			fmt.Sprintf("%d chapter documents (epub:type=chapter, at least %d words: %s), %d audio files (.mp3/.m4a)",
				len(items), minChapterWords, strings.Join(names, " "), len(audio)), p.AudioDir)}
	}
	return items, audio, nil
}

// readableWords 数一个文档里会参与对齐的书面词，用来排除只有图片的封面、扉页。
func readableWords(doc string) int {
	plan, err := xhtml.PlanReadAlong(doc, "\x00", false)
	if err != nil {
		return 0
	}
	n := 0
	for _, s := range plan.Segments {
		n += len(tokens(s.Text))
	}
	return n
}

func loadTimings(ctx context.Context, dir string) (map[string]timingsFile, []report.Finding) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []report.Finding{errFinding("timings-dir-unreadable", "cannot list timings directory", err.Error(), dir)}
	}
	out := map[string]timingsFile{}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		file := filepath.Join(dir, e.Name())
		raw, err := book.ReadFileContext(ctx, file, maxTimingsBytes)
		if err != nil {
			return nil, []report.Finding{errFinding("timings-unreadable", "cannot read timings file", err.Error(), file)}
		}
		var tf timingsFile
		if err := json.Unmarshal(raw, &tf); err != nil || tf.SchemaVersion != "1" || len(tf.Audio.SHA256) != 64 {
			return nil, []report.Finding{errFinding("timings-invalid", "timings file does not follow word-timings schema v1", e.Name(), file)}
		}
		out[strings.ToLower(tf.Audio.SHA256)] = tf
	}
	return out, nil
}

func (tf timingsFile) words() ([]Word, error) {
	out := make([]Word, 0, len(tf.Words))
	for i, w := range tf.Words {
		if len(w) != 3 {
			return nil, fmt.Errorf("word %d: want [text, start, end]", i)
		}
		var x Word
		if json.Unmarshal(w[0], &x.Text) != nil || json.Unmarshal(w[1], &x.Start) != nil || json.Unmarshal(w[2], &x.End) != nil {
			return nil, fmt.Errorf("word %d: want [text, start, end]", i)
		}
		out = append(out, x)
	}
	return out, nil
}

func safeToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "x"
	}
	return b.String()
}

func uniqueID(used map[string]bool, base string) string {
	id := base
	for i := 2; used[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	used[id] = true
	return id
}

func round3(x float64) float64 {
	return float64(int64(x*1000+0.5)) / 1000
}

// relHref 返回从 fromDir 指向容器路径 to 的相对 URL（逐段转义）。
func relHref(fromDir, to string) string {
	from := strings.Split(strings.Trim(fromDir, "/"), "/")
	if fromDir == "." || fromDir == "" {
		from = nil
	}
	parts := strings.Split(to, "/")
	i := 0
	for i < len(from) && i < len(parts)-1 && from[i] == parts[i] {
		i++
	}
	var segs []string
	for range len(from) - i {
		segs = append(segs, "..")
	}
	for _, s := range parts[i:] {
		segs = append(segs, url.PathEscape(s))
	}
	return strings.Join(segs, "/")
}
