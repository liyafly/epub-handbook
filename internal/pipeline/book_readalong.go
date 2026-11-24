package pipeline

// 有声同步的书级部分（docs/pipeline/media-overlay.md）：
//   - book.json 的 readAlong 段（旧布局写在制作区 variants.json 里）；
//   - epub book transcribe：经 extern 调 whisper-cli，把词级时间轴缓存到 <状态目录>/asr/；
//   - 构建 mediaOverlay 变体时调用 epub.media.overlay.build。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/caps/mediaoverlay"
	"github.com/liyafly/epub-handbook/internal/extern"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

const (
	whisperTool       = "whisper-cli"
	readAlongAsrDir   = "asr"
	maxWhisperJSON    = 256 << 20
	maxAudioFileBytes = 256 << 20
)

// readAlongConfig 是 book.json 的 readAlong 段。相对路径以书根（旧布局：制作区）为基准，
// model 允许 ~/ 开头。
type readAlongConfig struct {
	Audio       string `json:"audio"`
	Model       string `json:"model,omitempty"`
	Language    string `json:"language,omitempty"`
	Narrator    string `json:"narrator,omitempty"`
	Granularity string `json:"granularity,omitempty"`
	AudioMap    string `json:"audioMap,omitempty"`
}

// readAlongSpec 是解析成绝对路径后的配置。
type readAlongSpec struct {
	AudioDir    string
	Model       string
	Language    string
	Narrator    string
	Granularity string
	AudioMap    string
}

func resolveReadAlong(cfg *readAlongConfig, baseDir string) (*readAlongSpec, error) {
	if cfg == nil {
		return nil, nil
	}
	if cfg.Audio == "" {
		return nil, errors.New("readAlong.audio is required")
	}
	abs := func(p string) string {
		if p == "" {
			return ""
		}
		if rest, ok := strings.CutPrefix(p, "~/"); ok {
			if home, err := os.UserHomeDir(); err == nil {
				return filepath.Join(home, filepath.FromSlash(rest))
			}
		}
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(baseDir, filepath.FromSlash(p))
	}
	switch cfg.Granularity {
	case "", "sentence", "paragraph":
	default:
		return nil, fmt.Errorf("readAlong.granularity must be sentence or paragraph, got %q", cfg.Granularity)
	}
	return &readAlongSpec{
		AudioDir: abs(cfg.Audio), Model: abs(cfg.Model), Language: cmpOr(cfg.Language, "auto"),
		Narrator: cfg.Narrator, Granularity: cfg.Granularity, AudioMap: abs(cfg.AudioMap),
	}, nil
}

func cmpOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// checkMediaOverlayVariants 要求 mediaOverlay 变体有 readAlong 配置。
func checkMediaOverlayVariants(layout bookLayout) error {
	for _, v := range layout.Variants {
		if v.MediaOverlay && layout.ReadAlong == nil {
			return fmt.Errorf("variant %q sets mediaOverlay but there is no readAlong section", v.Name)
		}
	}
	return nil
}

func (l bookLayout) asrDir() string { return filepath.Join(l.StateDir, readAlongAsrDir) }

// audioFiles 列出音频目录里的 .mp3 / .m4a（按文件名排序）。
func audioFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read audio directory: %w", err)
	}
	var out []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.Type().IsRegular() && !strings.HasPrefix(e.Name(), ".") && (ext == ".mp3" || ext == ".m4a") {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no .mp3 or .m4a files in %s", dir)
	}
	return out, nil
}

// cachedTimings 读缓存；不存在或与当前模型不一致时返回 ok=false。
func cachedTimings(path, modelSHA string) bool {
	data, err := readSmallFileLimit(path, maxWhisperJSON)
	if err != nil {
		return false
	}
	var doc struct {
		SchemaVersion string `json:"schemaVersion"`
		Engine        struct {
			ModelSHA string `json:"modelSha256"`
		} `json:"engine"`
	}
	if json.Unmarshal(data, &doc) != nil || doc.SchemaVersion != "1" {
		return false
	}
	return modelSHA == "" || doc.Engine.ModelSHA == modelSHA
}

func readSmallFileLimit(path string, limit int64) ([]byte, error) {
	return zipfs.ReadFileContext(context.Background(), path, limit)
}

// missingTimings 返回还没有时间轴缓存的音频文件名（不校验模型，构建时只要有就用）。
func missingTimings(ctx context.Context, layout bookLayout) ([]string, error) {
	ra := layout.ReadAlong
	files, err := audioFiles(ra.AudioDir)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, name := range files {
		sum, err := zipfs.FileSHA256ContextLimit(ctx, filepath.Join(ra.AudioDir, name), maxAudioFileBytes)
		if err != nil {
			return nil, err
		}
		if !cachedTimings(filepath.Join(layout.asrDir(), sum+".json"), "") {
			missing = append(missing, name)
		}
	}
	return missing, nil
}

// BookTranscribeOptions 是 epub book transcribe 的输入。
type BookTranscribeOptions struct {
	Dir   string
	Model string // 覆盖 readAlong.model
	Force bool   // 忽略缓存重新识别
}

// BookTranscribe 为 readAlong.audio 里的每个音频生成词级时间轴缓存。
func BookTranscribe(ctx context.Context, opts BookTranscribeOptions, bio BookIO) int {
	return bookExit(bio, bookTranscribe(ctx, opts, bio))
}

func bookTranscribe(ctx context.Context, opts BookTranscribeOptions, bio BookIO) error {
	layout, err := resolveBookLayout(opts.Dir, readBookEnv())
	if err != nil {
		return err
	}
	ra := layout.ReadAlong
	if ra == nil {
		return usageErrorf("no readAlong section in %s (legacy layout: %s)", bookConfigName, legacyVariantsName)
	}
	model := ra.Model
	if opts.Model != "" {
		model, err = filepath.Abs(opts.Model)
		if err != nil {
			return err
		}
	}
	if model == "" {
		return usageErrorf("no Whisper model: set readAlong.model or pass --model <ggml-*.bin>")
	}
	if !isFile(model) {
		return usageErrorf("Whisper model not found: %s (download a ggml model such as ggml-small.en.bin)", model)
	}
	bin := cmpOr(os.Getenv("EPUB_WHISPER_BIN"), whisperTool)
	if err := extern.Require(bin); err != nil {
		return fmt.Errorf("%s was not found on PATH; install whisper.cpp (macOS: brew install whisper-cpp; Windows: whisper-cli.exe from the whisper.cpp release) or set EPUB_WHISPER_BIN", bin)
	}
	version := whisperVersion(ctx, bin)
	files, err := audioFiles(ra.AudioDir)
	if err != nil {
		return err
	}
	modelSHA, err := zipfs.FileSHA256ContextLimit(ctx, model, 8<<30)
	if err != nil {
		return err
	}
	if err := zipfs.MkdirAll(layout.asrDir()); err != nil {
		return err
	}
	scratch, cleanup, err := extern.TempDir("", "epub-transcribe-*")
	if err != nil {
		return err
	}
	defer cleanup()

	done, skipped := 0, 0
	for i, name := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		src := filepath.Join(ra.AudioDir, name)
		if strings.EqualFold(filepath.Ext(name), ".m4a") {
			return fmt.Errorf("%s: whisper-cli cannot read AAC; convert the chapter to MP3 first", name)
		}
		data, err := zipfs.ReadFileContext(ctx, src, maxAudioFileBytes)
		if err != nil {
			return err
		}
		sum := sha256Hex(data)
		dst := filepath.Join(layout.asrDir(), sum+".json")
		if !opts.Force && cachedTimings(dst, modelSHA) {
			fmt.Fprintf(bio.out(), "SKIP %s (cached)\n", name)
			skipped++
			continue
		}
		duration, err := mediaoverlay.MP3Duration(data)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		fmt.Fprintf(bio.out(), "[%d/%d] %s (%s) …\n", i+1, len(files), name, clockString(duration))
		outBase := filepath.Join(scratch, fmt.Sprintf("%03d", i+1))
		argv := []string{bin, "-m", model, "-f", src, "-l", ra.Language, "-np", "-ojf", "-of", outBase}
		res, err := extern.RunLong(ctx, "", argv)
		if err != nil {
			return fmt.Errorf("%s: whisper-cli: %w", name, err)
		}
		if res.ExitCode != 0 {
			tail := strings.TrimSpace(string(res.Stderr))
			if len(tail) > 600 {
				tail = tail[len(tail)-600:]
			}
			return fmt.Errorf("%s: whisper-cli exited %d: %s", name, res.ExitCode, tail)
		}
		raw, err := zipfs.ReadFileContext(ctx, outBase+".json", maxWhisperJSON)
		if err != nil {
			return fmt.Errorf("%s: read whisper output: %w", name, err)
		}
		words, err := whisperWords(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		doc := map[string]any{
			"schemaVersion": "1",
			"audio":         map[string]any{"path": name, "sha256": sum, "duration": roundMillis(duration)},
			"engine":        map[string]any{"name": "whisper.cpp", "version": version, "model": filepath.Base(model), "modelSha256": modelSHA},
			"words":         words,
		}
		out, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		if err := zipfs.WriteFileAtomic(dst, append(out, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(bio.out(), "PASS %s: %d words -> %s\n", name, len(words), layout.displayPath(dst))
		done++
	}
	fmt.Fprintf(bio.out(), "Transcribed %d, cached %d. Next: epub book build%s\n", done, skipped, readAlongVariantHint(layout))
	return nil
}

func readAlongVariantHint(layout bookLayout) string {
	for _, v := range layout.Variants {
		if v.MediaOverlay {
			return " (variant " + v.Name + ")"
		}
	}
	return " (add a variant with \"mediaOverlay\": true)"
}

func whisperVersion(ctx context.Context, bin string) string {
	res, err := extern.Run(ctx, "", []string{bin, "--version"})
	if err == nil {
		for _, line := range strings.Split(string(res.Stdout)+"\n"+string(res.Stderr), "\n") {
			if _, v, ok := strings.Cut(line, "version:"); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return "unknown"
}

// whisperWords 把 whisper.cpp -ojf 的 token 合成词：以空格开头的 token 开新词，
// 特殊 token（[_BEG_] 等）跳过。时间取 offsets（毫秒）。
func whisperWords(raw []byte) ([][]any, error) {
	var doc struct {
		Transcription []struct {
			Tokens []struct {
				Text    string `json:"text"`
				Offsets struct {
					From int64 `json:"from"`
					To   int64 `json:"to"`
				} `json:"offsets"`
			} `json:"tokens"`
		} `json:"transcription"`
	}
	if err := json.Unmarshal([]byte(strings.ToValidUTF8(string(raw), "�")), &doc); err != nil {
		return nil, fmt.Errorf("whisper output is not valid JSON: %w", err)
	}
	type word struct {
		text       string
		start, end float64
	}
	var words []word
	for _, seg := range doc.Transcription {
		for _, tk := range seg.Tokens {
			t := tk.Text
			if strings.HasPrefix(t, "[_") || strings.HasPrefix(t, "<|") {
				continue
			}
			s, e := float64(tk.Offsets.From)/1000, float64(tk.Offsets.To)/1000
			if strings.HasPrefix(t, " ") || len(words) == 0 {
				words = append(words, word{strings.TrimSpace(t), s, e})
				continue
			}
			last := &words[len(words)-1]
			last.text += t
			last.end = max(last.end, e)
		}
	}
	out := make([][]any, 0, len(words))
	for _, w := range words {
		if w.text != "" && !strings.ContainsRune(w.text, '�') {
			out = append(out, []any{w.text, w.start, w.end})
		}
	}
	return out, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func roundMillis(x float64) float64 { return float64(int64(x*1000+0.5)) / 1000 }

func clockString(sec float64) string {
	s := int(sec + 0.5)
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}

// readAlongCandidate 对 mediaOverlay 变体的候选跑 epub.media.overlay.build，并把候选换成结果。
func (r *bookRunner) readAlongCandidate(layout bookLayout, c *candidate, tag string) error {
	ra := layout.ReadAlong
	out := filepath.Join(c.dir, "readalong.epub")
	args := Args{"audio_dir": ra.AudioDir, "timings_dir": layout.asrDir()}
	if ra.Narrator != "" {
		args["narrator"] = ra.Narrator
	}
	if ra.Granularity != "" {
		args["granularity"] = ra.Granularity
	}
	if ra.AudioMap != "" {
		args["audio_map"] = ra.AudioMap
	}
	if err := r.capabilityCheck(layout, filepath.Join(layout.StateDir, "media-overlay"+c.out.suffix()+".json"),
		"epub.media.overlay.build"+tag, Options{CapabilityID: "epub.media.overlay.build", InputPath: c.final, OutputPath: out, Args: args}); err != nil {
		return err
	}
	c.final = out
	return nil
}

// dropUntranscribed 去掉缺时间轴缓存的 mediaOverlay 变体（报 WARN）；只构建该变体时直接报错。
func (r *bookRunner) dropUntranscribed(layout bookLayout, targets []outputSpec) ([]outputSpec, error) {
	kept := targets[:0:0]
	for _, out := range targets {
		if !out.MediaOverlay {
			kept = append(kept, out)
			continue
		}
		missing, err := missingTimings(r.ctx, layout)
		if err != nil {
			return nil, fmt.Errorf("variant %q: %w", out.Name, err)
		}
		if len(missing) == 0 {
			kept = append(kept, out)
			continue
		}
		msg := fmt.Sprintf("variant %s: %d audio file(s) have no timings (%s); run: epub book transcribe", out.Name, len(missing), strings.Join(missing, ", "))
		if r.variant != "" {
			return nil, errors.New(msg)
		}
		fmt.Fprintln(r.bio.err(), "WARN skip", msg)
		r.addStep(bookStep{Status: "WARN", Label: "read-along [" + out.Name + "]", Lines: []string{msg}})
	}
	return kept, nil
}
