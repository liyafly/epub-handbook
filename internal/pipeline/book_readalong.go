package pipeline

// 有声同步的书级部分（docs/pipeline/media-overlay.md）：
//   - book.json 的 readAlong 段（旧布局写在制作区 variants.json 里）；
//   - epub book transcribe：readAlong.audio 是单个 m4b 时先经 ffmpeg 按章节标记无损切成
//     <状态目录>/audio/<sha>/NNN.m4a；再经 extern 调 whisper-cli，把词级时间轴缓存到 <状态目录>/asr/；
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
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/caps/mediaoverlay"
	"github.com/liyafly/epub-handbook/internal/extern"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

const (
	whisperTool         = "whisper-cli"
	ffmpegTool          = "ffmpeg"
	ffprobeTool         = "ffprobe"
	readAlongAsrDir     = "asr"
	readAlongSplitDir   = "audio"
	whisperModelsDir    = "whisper-models"
	maxWhisperJSON      = 256 << 20
	maxProbeJSON        = 16 << 20
	maxAudioFileBytes   = 256 << 20
	maxAudioBundleBytes = 8 << 30
)

// readAlongConfig 是 book.json 的 readAlong 段。相对路径以书根（旧布局：制作区）为基准，
// model 允许 ~/ 开头；只写文件名（如 ggml-small.en.bin）或不写时到共享模型目录找（findWhisperModel）。
type readAlongConfig struct {
	Audio       string `json:"audio"`
	Model       string `json:"model,omitempty"`
	Language    string `json:"language,omitempty"`
	Narrator    string `json:"narrator,omitempty"`
	Granularity string `json:"granularity,omitempty"`
	AudioMap    string `json:"audioMap,omitempty"`
}

// readAlongSpec 是解析成绝对路径后的配置。AudioDir 是每章一个文件的目录，
// 或带章节标记的单个 m4b / m4a / mp4 文件（见 bundle）。
type readAlongSpec struct {
	AudioDir    string
	Model       string // 模型路径；只写了文件名时为空，见 ModelName
	ModelName   string
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
	spec := &readAlongSpec{
		AudioDir: abs(cfg.Audio), Model: abs(cfg.Model), Language: cmpOr(cfg.Language, "auto"),
		Narrator: cfg.Narrator, Granularity: cfg.Granularity, AudioMap: abs(cfg.AudioMap),
	}
	if cfg.Model != "" && !strings.ContainsAny(cfg.Model, `/\`) {
		spec.Model, spec.ModelName = "", cfg.Model
	}
	return spec, nil
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

// bundle 报告 readAlong.audio 是否是单个整本音频文件（m4b 等），而不是每章一个文件的目录。
func (r *readAlongSpec) bundle() bool {
	if isDir(r.AudioDir) {
		return false
	}
	switch strings.ToLower(filepath.Ext(r.AudioDir)) {
	case ".m4b", ".m4a", ".mp4":
		return true
	}
	return false
}

// chapterAudioDir 返回每章音频所在目录。整本音频返回 <状态目录>/audio/<sha 前 16 位>，
// ready=false 表示还没切分（transcribe 会切）。
func (l bookLayout) chapterAudioDir(ctx context.Context) (dir string, ready bool, err error) {
	ra := l.ReadAlong
	if !ra.bundle() {
		return ra.AudioDir, true, nil
	}
	if !isFile(ra.AudioDir) {
		return "", false, fmt.Errorf("readAlong.audio not found: %s", ra.AudioDir)
	}
	sum, err := zipfs.FileSHA256ContextLimit(ctx, ra.AudioDir, maxAudioBundleBytes)
	if err != nil {
		return "", false, err
	}
	dir = filepath.Join(l.StateDir, readAlongSplitDir, sum[:16])
	return dir, isDir(dir), nil
}

func toolBin(env, name string) string { return cmpOr(os.Getenv(env), name) }

func requireFFmpeg() (ffmpeg, ffprobe string, err error) {
	ffmpeg, ffprobe = toolBin("EPUB_FFMPEG_BIN", ffmpegTool), toolBin("EPUB_FFPROBE_BIN", ffprobeTool)
	for _, bin := range []string{ffmpeg, ffprobe} {
		if err := extern.Require(bin); err != nil {
			return "", "", fmt.Errorf("%s was not found on PATH; m4b / AAC audio needs FFmpeg (macOS: brew install ffmpeg; Windows: winget install ffmpeg) or set EPUB_FFMPEG_BIN / EPUB_FFPROBE_BIN", bin)
		}
	}
	return ffmpeg, ffprobe, nil
}

func runTool(ctx context.Context, name string, argv []string) ([]byte, error) {
	res, err := extern.RunLong(ctx, "", argv)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if res.ExitCode != 0 {
		tail := strings.TrimSpace(string(res.Stderr))
		if len(tail) > 600 {
			tail = tail[len(tail)-600:]
		}
		return nil, fmt.Errorf("%s exited %d: %s", name, res.ExitCode, tail)
	}
	return res.Stdout, nil
}

type audioChapter struct{ start, end string }

// probeChapters 读整本音频的章节标记（ffprobe -show_chapters）。
func probeChapters(ctx context.Context, ffprobe, src string) ([]audioChapter, error) {
	out, err := runTool(ctx, "ffprobe", []string{ffprobe, "-v", "error", "-show_chapters", "-of", "json", src})
	if err != nil {
		return nil, err
	}
	if len(out) > maxProbeJSON {
		return nil, errors.New("ffprobe output too large")
	}
	var doc struct {
		Chapters []struct {
			Start string `json:"start_time"`
			End   string `json:"end_time"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("ffprobe output is not valid JSON: %w", err)
	}
	var chapters []audioChapter
	for _, c := range doc.Chapters {
		chapters = append(chapters, audioChapter{c.Start, c.End})
	}
	return chapters, nil
}

// probeDuration 读音频容器记录的时长（秒）。
func probeDuration(ctx context.Context, ffprobe, src string) (float64, error) {
	out, err := runTool(ctx, "ffprobe", []string{ffprobe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", src})
	if err != nil {
		return 0, err
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("ffprobe reported no duration for %s", filepath.Base(src))
	}
	return d, nil
}

// splitBundle 把整本音频按章节标记切成 NNN.m4a（只复制 AAC 数据，不重新编码），
// 先写进临时目录，全部成功后改名到位；已切过则直接返回。
func splitBundle(ctx context.Context, layout bookLayout, bio BookIO) (string, error) {
	dir, ready, err := layout.chapterAudioDir(ctx)
	if err != nil || ready {
		return dir, err
	}
	src := layout.ReadAlong.AudioDir
	ffmpeg, ffprobe, err := requireFFmpeg()
	if err != nil {
		return "", err
	}
	chapters, err := probeChapters(ctx, ffprobe, src)
	if err != nil {
		return "", err
	}
	if len(chapters) == 0 {
		return "", fmt.Errorf("%s has no chapter markers; split it into one file per chapter and point readAlong.audio at that folder", filepath.Base(src))
	}
	if err := zipfs.MkdirAll(filepath.Dir(dir)); err != nil {
		return "", err
	}
	tmp, err := zipfs.MkdirTemp(filepath.Dir(dir), ".split-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = zipfs.RemoveAll(tmp) }()
	fmt.Fprintf(bio.out(), "SPLIT %s: %d chapters\n", filepath.Base(src), len(chapters))
	for i, c := range chapters {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		out := filepath.Join(tmp, fmt.Sprintf("%03d.m4a", i+1))
		argv := []string{ffmpeg, "-nostdin", "-v", "error", "-i", src, "-ss", c.start, "-to", c.end,
			"-map", "0:a:0", "-c", "copy", "-map_metadata", "-1", "-map_chapters", "-1",
			"-fflags", "+bitexact", "-movflags", "+faststart", out}
		if _, err := runTool(ctx, "ffmpeg", argv); err != nil {
			return "", fmt.Errorf("split chapter %d: %w", i+1, err)
		}
	}
	if err := zipfs.Chmod(tmp, 0o755); err != nil {
		return "", err
	}
	if err := zipfs.RenameNoReplace(tmp, dir); err != nil {
		return "", err
	}
	fmt.Fprintf(bio.out(), "PASS split -> %s\n", layout.displayPath(dir))
	return dir, nil
}

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

// cachedTimings 读缓存；不存在、与当前模型或词时间算法不一致时返回 ok=false。
// timing 为空表示只要有缓存就行（构建时）。
func cachedTimings(path, modelSHA, timing string) bool {
	data, err := readSmallFileLimit(path, maxWhisperJSON)
	if err != nil {
		return false
	}
	var doc struct {
		SchemaVersion string `json:"schemaVersion"`
		Engine        struct {
			ModelSHA   string `json:"modelSha256"`
			WordTiming string `json:"wordTiming"`
		} `json:"engine"`
	}
	if json.Unmarshal(data, &doc) != nil || doc.SchemaVersion != "1" {
		return false
	}
	return (modelSHA == "" || doc.Engine.ModelSHA == modelSHA) && (timing == "" || doc.Engine.WordTiming == timing)
}

func readSmallFileLimit(path string, limit int64) ([]byte, error) {
	return zipfs.ReadFileContext(context.Background(), path, limit)
}

// missingTimings 返回还没有时间轴缓存的音频文件名（不校验模型，构建时只要有就用）。
func missingTimings(ctx context.Context, layout bookLayout) ([]string, error) {
	dir, ready, err := layout.chapterAudioDir(ctx)
	if err != nil {
		return nil, err
	}
	if !ready {
		return []string{filepath.Base(layout.ReadAlong.AudioDir) + " (not split yet)"}, nil
	}
	files, err := audioFiles(dir)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, name := range files {
		sum, err := zipfs.FileSHA256ContextLimit(ctx, filepath.Join(dir, name), maxAudioFileBytes)
		if err != nil {
			return nil, err
		}
		if !cachedTimings(filepath.Join(layout.asrDir(), sum+".json"), "", "") {
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
	model, err := findWhisperModel(layout, opts.Model)
	if err != nil {
		return err
	}
	bin := cmpOr(os.Getenv("EPUB_WHISPER_BIN"), whisperTool)
	if err := extern.Require(bin); err != nil {
		return fmt.Errorf("%s was not found on PATH; install whisper.cpp (macOS: brew install whisper-cpp; Windows: whisper-cli.exe from the whisper.cpp release) or set EPUB_WHISPER_BIN", bin)
	}
	version := whisperVersion(ctx, bin)
	audioDir, err := splitBundle(ctx, layout, bio)
	if err != nil {
		return err
	}
	files, err := audioFiles(audioDir)
	if err != nil {
		return err
	}
	var ffmpeg, ffprobe string
	for _, name := range files {
		if isAAC(name) {
			if ffmpeg, ffprobe, err = requireFFmpeg(); err != nil {
				return err
			}
			break
		}
	}
	modelSHA, err := zipfs.FileSHA256ContextLimit(ctx, model, 8<<30)
	if err != nil {
		return err
	}
	// whisper.cpp 的逐 token 时间估计常把整段词都压到段首；能认出模型尺寸时用 DTW 对齐
	// （要关 flash attention）。认不出就退回 token 时间并提示。
	preset := dtwPreset(model)
	timing := "dtw"
	if preset == "" {
		timing = "token"
		fmt.Fprintf(bio.err(), "WARN %s: model size not recognized from the file name; word times use whisper's token timestamps, which can be coarse (name the model like ggml-small.en.bin)\n", filepath.Base(model))
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
		src := filepath.Join(audioDir, name)
		data, err := zipfs.ReadFileContext(ctx, src, maxAudioFileBytes)
		if err != nil {
			return err
		}
		sum := sha256Hex(data)
		dst := filepath.Join(layout.asrDir(), sum+".json")
		if !opts.Force && cachedTimings(dst, modelSHA, timing) {
			fmt.Fprintf(bio.out(), "SKIP %s (cached)\n", name)
			skipped++
			continue
		}
		outBase := filepath.Join(scratch, fmt.Sprintf("%03d", i+1))
		input := src
		var duration float64
		if isAAC(name) {
			// whisper-cli 读不了 AAC：先解码成 16 kHz 单声道 WAV（时间轴不变）。
			if duration, err = probeDuration(ctx, ffprobe, src); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			input = outBase + ".wav"
			if _, err := runTool(ctx, "ffmpeg", []string{ffmpeg, "-nostdin", "-v", "error", "-i", src, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", input}); err != nil {
				return fmt.Errorf("%s: decode: %w", name, err)
			}
		} else if duration, err = mediaoverlay.MP3Duration(data); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		fmt.Fprintf(bio.out(), "[%d/%d] %s (%s) …\n", i+1, len(files), name, clockString(duration))
		argv := []string{bin, "-m", model, "-f", input, "-l", ra.Language, "-np", "-ojf", "-of", outBase}
		if preset != "" {
			argv = append(argv, "--dtw", preset, "-nfa")
		}
		if _, err := runTool(ctx, "whisper-cli", argv); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if input != src {
			_ = zipfs.Remove(input)
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
			"engine":        map[string]any{"name": "whisper.cpp", "version": version, "model": filepath.Base(model), "modelSha256": modelSHA, "wordTiming": timing},
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

// findWhisperModel 依次取：--model；readAlong.model 写的路径；EPUB_WHISPER_MODEL（文件或目录）；
// 从书目录往上每一级的 whisper-models/（和字体库一样，放在书旁边，所有书共用）。
// 在目录里找时，readAlong.model 只写了文件名就找这个名字，没写就要求目录里只有一个 ggml-*.bin。
func findWhisperModel(layout bookLayout, override string) (string, error) {
	ra := layout.ReadAlong
	if override != "" {
		p, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		if !isFile(p) {
			return "", usageErrorf("Whisper model not found: %s", p)
		}
		return p, nil
	}
	if ra.Model != "" {
		if !isFile(ra.Model) {
			return "", usageErrorf("Whisper model not found: %s (readAlong.model)", ra.Model)
		}
		return ra.Model, nil
	}
	var dirs []string
	if env := os.Getenv("EPUB_WHISPER_MODEL"); env != "" {
		if rest, ok := strings.CutPrefix(env, "~/"); ok {
			if home, err := os.UserHomeDir(); err == nil {
				env = filepath.Join(home, rest)
			}
		}
		if isFile(env) && ra.ModelName == "" {
			return filepath.Abs(env)
		}
		if isDir(env) {
			dirs = append(dirs, env)
		}
	}
	for dir := layout.BookDir; ; dir = filepath.Dir(dir) {
		if candidate := filepath.Join(dir, whisperModelsDir); isDir(candidate) {
			dirs = append(dirs, candidate)
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	for _, dir := range dirs {
		if ra.ModelName != "" {
			if p := filepath.Join(dir, ra.ModelName); isFile(p) {
				return p, nil
			}
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var found []string
		for _, e := range entries {
			if n := e.Name(); e.Type().IsRegular() && strings.HasPrefix(n, "ggml-") && strings.HasSuffix(n, ".bin") {
				found = append(found, n)
			}
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			return filepath.Join(dir, found[0]), nil
		default:
			return "", usageErrorf("several Whisper models in %s (%s); set readAlong.model to one of the file names", dir, strings.Join(found, ", "))
		}
	}
	want := "a ggml-*.bin model (e.g. ggml-small.en.bin)"
	if ra.ModelName != "" {
		want = ra.ModelName
	}
	return "", usageErrorf("no Whisper model found: put %s in a %s/ folder next to your books (e.g. %s), or set readAlong.model, EPUB_WHISPER_MODEL or --model",
		want, whisperModelsDir, filepath.Join(filepath.Dir(layout.BookDir), whisperModelsDir))
}

// dtwPreset 从 ggml 模型文件名认出 whisper.cpp 的 DTW 预设名（ggml-small.en-q5_1.bin → small.en）。
func dtwPreset(model string) string {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(model)), ".bin")
	name = strings.TrimPrefix(name, "ggml-")
	if i := strings.Index(name, "-q"); i > 0 {
		name = name[:i]
	}
	name = strings.ReplaceAll(name, "large-v", "large.v")
	name = strings.ReplaceAll(name, "-turbo", ".turbo")
	switch name {
	case "tiny", "tiny.en", "base", "base.en", "small", "small.en", "medium", "medium.en",
		"large.v1", "large.v2", "large.v3", "large.v3.turbo":
		return name
	}
	return ""
}

func isAAC(name string) bool { return strings.EqualFold(filepath.Ext(name), ".m4a") }

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
// 特殊 token（[_BEG_] 等）跳过。有 DTW 时间（t_dtw，厘秒）时词起点取词首 token 的 t_dtw，
// 终点取同段下一个词的起点、段末词取段末；没有时退回 offsets（毫秒）。
func whisperWords(raw []byte) ([][]any, error) {
	var doc struct {
		Transcription []struct {
			Tokens []struct {
				Text    string `json:"text"`
				Offsets struct {
					From int64 `json:"from"`
					To   int64 `json:"to"`
				} `json:"offsets"`
				DTW *int64 `json:"t_dtw"`
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
		first := len(words)
		useDTW := true
		var segEnd float64
		for _, tk := range seg.Tokens {
			if !strings.HasPrefix(tk.Text, "[_") && !strings.HasPrefix(tk.Text, "<|") {
				useDTW = useDTW && tk.DTW != nil && *tk.DTW >= 0
				segEnd = max(segEnd, float64(tk.Offsets.To)/1000)
			}
		}
		for _, tk := range seg.Tokens {
			t := tk.Text
			if strings.HasPrefix(t, "[_") || strings.HasPrefix(t, "<|") {
				continue
			}
			s, e := float64(tk.Offsets.From)/1000, float64(tk.Offsets.To)/1000
			if useDTW {
				s = float64(*tk.DTW) / 100
			}
			if strings.HasPrefix(t, " ") || len(words) == first {
				words = append(words, word{strings.TrimSpace(t), s, e})
				continue
			}
			last := &words[len(words)-1]
			last.text += t
			if !useDTW {
				last.end = max(last.end, e)
			}
		}
		if useDTW {
			for i := first; i < len(words); i++ {
				end := segEnd
				if i+1 < len(words) {
					end = words[i+1].start
				}
				words[i].end = max(words[i].start, end)
			}
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
	audioDir, _, err := layout.chapterAudioDir(r.ctx)
	if err != nil {
		return err
	}
	args := Args{"audio_dir": audioDir, "timings_dir": layout.asrDir()}
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
