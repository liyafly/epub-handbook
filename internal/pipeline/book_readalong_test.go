package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeMP3 是 100 帧 MPEG-1 Layer III（128 kbps、44.1 kHz）的静音帧，约 2.6 秒。
func fakeMP3() []byte {
	frame := make([]byte, 417)
	copy(frame, []byte{0xff, 0xfb, 0x90, 0x00})
	var data []byte
	for range 100 {
		data = append(data, frame...)
	}
	return data
}

// fakeWhisperJSON 按「每词 0.1 秒」给出 whisper.cpp -ojf 形状的 token。
func fakeWhisperJSON(text string) string {
	type tok struct {
		Text    string `json:"text"`
		Offsets struct {
			From int `json:"from"`
			To   int `json:"to"`
		} `json:"offsets"`
	}
	toks := []tok{{Text: "[_BEG_]"}}
	at := 100
	for _, w := range strings.Fields(text) {
		t := tok{Text: " " + w}
		t.Offsets.From, t.Offsets.To = at, at+80
		toks = append(toks, t)
		at += 100
	}
	doc := map[string]any{"transcription": []any{map[string]any{"tokens": toks}}}
	data, _ := json.Marshal(doc)
	return string(data)
}

func setupReadAlongBook(t *testing.T, h *bookHarness) (dir string) {
	t.Helper()
	dir = h.newBook("")
	var paras []string
	for _, p := range moChapters[0].paras {
		paras = append(paras, "<p>"+p+"</p>")
	}
	chapter := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="en" lang="en">
<head><title>One</title><link rel="stylesheet" type="text/css" href="../Styles/reader.css"/></head>
<body><section epub:type="chapter"><h1>Chapter One</h1>
` + strings.Join(paras, "\n") + `
</section></body></html>
`
	mustWrite(t, filepath.Join(dir, "epub/OEBPS/Text/01-chapter.xhtml"), chapter)
	mustWrite(t, filepath.Join(dir, "素材/audio/01.mp3"), string(fakeMP3()))
	mustWrite(t, filepath.Join(dir, "models/ggml-test.bin"), "model bytes")
	path := filepath.Join(dir, "book.json")
	var cfg bookConfig
	if err := json.Unmarshal([]byte(mustRead(t, path)), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.ReadAlong = &readAlongConfig{Audio: "素材/audio", Model: "models/ggml-test.bin", Language: "en", Narrator: "A Reader"}
	cfg.Variants = map[string]variantConfig{"readalong": {MediaOverlay: true}}
	if err := writeBookConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	// 假 whisper-cli：--version 报版本；识别时把预置 JSON 复制到 -of 指定的位置。
	bin := filepath.Join(t.TempDir(), "whisper-cli")
	mustWrite(t, filepath.Join(filepath.Dir(bin), "out.json"), fakeWhisperJSON("Chapter One "+strings.Join(moChapters[0].paras, " ")))
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then echo "whisper.cpp version: 9.9.9"; exit 0; fi
of=""
while [ $# -gt 0 ]; do
  if [ "$1" = "-of" ]; then of="$2"; fi
  shift
done
cp "` + filepath.Join(filepath.Dir(bin), "out.json") + `" "$of.json"
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EPUB_WHISPER_BIN", bin)
	return dir
}

func TestBookReadAlongTranscribeAndBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake whisper-cli is a shell script")
	}
	h := newBookHarness(t)
	dir := setupReadAlongBook(t, h)

	// 没有时间轴：通用版照常构建，跟读变体跳过并报 WARN。
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build without timings = %d: %s", code, h.err)
	}
	if !strings.Contains(h.err.String(), "WARN skip variant readalong") || isFile(filepath.Join(dir, "out", "book-readalong.epub")) {
		t.Fatalf("read-along variant must be skipped before transcribe: %s", h.err)
	}
	if code := h.runVariant(dir, "build", "readalong"); code == 0 {
		t.Fatal("building only the read-along variant without timings must fail")
	}

	if code := BookTranscribe(context.Background(), BookTranscribeOptions{Dir: dir}, h.io()); code != 0 {
		t.Fatalf("transcribe = %d: %s %s", code, h.out, h.err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, ".epub", "asr"))
	if len(entries) != 1 {
		t.Fatalf("asr cache entries = %d", len(entries))
	}
	var cache struct {
		Audio struct {
			Path     string  `json:"path"`
			Duration float64 `json:"duration"`
		} `json:"audio"`
		Engine struct {
			Version string `json:"version"`
			Model   string `json:"model"`
		} `json:"engine"`
		Words [][]any `json:"words"`
	}
	if err := json.Unmarshal([]byte(mustRead(t, filepath.Join(dir, ".epub", "asr", entries[0].Name()))), &cache); err != nil {
		t.Fatal(err)
	}
	if cache.Audio.Path != "01.mp3" || cache.Audio.Duration != 2.612 || cache.Engine.Version != "9.9.9" || cache.Engine.Model != "ggml-test.bin" || len(cache.Words) < 60 {
		t.Fatalf("cache = %+v (%d words)", cache, len(cache.Words))
	}
	if code := BookTranscribe(context.Background(), BookTranscribeOptions{Dir: dir}, h.io()); code != 0 || !strings.Contains(h.out.String(), "SKIP 01.mp3 (cached)") {
		t.Fatalf("second transcribe must reuse the cache: %d %s", code, h.out)
	}

	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	readalong := filepath.Join(dir, "out", "book-readalong.epub")
	names := memberNames(t, readalong)
	if !names["OEBPS/Smil/01-chapter.smil"] || !names["OEBPS/Audio/01-chapter.mp3"] {
		t.Fatalf("read-along output members: %v", names)
	}
	if generic := memberNames(t, filepath.Join(dir, "out", "book.epub")); generic["OEBPS/Smil/01-chapter.smil"] {
		t.Fatal("generic output must not carry media overlays")
	}
	if !strings.Contains(readMember(t, readalong, "OEBPS/Text/01-chapter.xhtml"), `<span id="mo-01-chapter-s2">`) {
		t.Fatal("chapter lacks sentence spans")
	}
	if !strings.Contains(h.out.String(), "PASS epub.media.overlay.build [readalong]") {
		t.Fatalf("build output: %s", h.out)
	}
}

// TestBookReadAlongM4B 用假 ffprobe / ffmpeg 走整本 m4b：transcribe 先按章节标记切分，
// 再解码成 WAV 交给 whisper；build 从切分目录取音频。
func TestBookReadAlongM4B(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tools are shell scripts")
	}
	h := newBookHarness(t)
	dir := setupReadAlongBook(t, h)
	mustWrite(t, filepath.Join(dir, "素材/book.m4b"), "M4B bytes")
	path := filepath.Join(dir, "book.json")
	var cfg bookConfig
	if err := json.Unmarshal([]byte(mustRead(t, path)), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.ReadAlong.Audio = "素材/book.m4b"
	if err := writeBookConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	probe := filepath.Join(tools, "ffprobe")
	ffmpeg := filepath.Join(tools, "ffmpeg")
	log := filepath.Join(tools, "ffmpeg.log")
	writeScript := func(p, body string) {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeScript(probe, `case "$*" in
*-show_chapters*) echo '{"chapters":[{"start_time":"0.000000","end_time":"2.612000"}]}';;
*) echo 2.612;;
esac
`)
	writeScript(ffmpeg, `for last; do :; done
echo "$*" >> "`+log+`"
echo "audio $last" > "$last"
`)
	t.Setenv("EPUB_FFPROBE_BIN", probe)
	t.Setenv("EPUB_FFMPEG_BIN", ffmpeg)

	if code := h.run(dir, "build"); code != 0 || !strings.Contains(h.err.String(), "book.m4b (not split yet)") {
		t.Fatalf("build before split = %d: %s", code, h.err)
	}
	if code := BookTranscribe(context.Background(), BookTranscribeOptions{Dir: dir}, h.io()); code != 0 {
		t.Fatalf("transcribe = %d: %s %s", code, h.out, h.err)
	}
	calls := mustRead(t, log)
	if !strings.Contains(calls, "-ss 0.000000 -to 2.612000 -map 0:a:0 -c copy") || !strings.Contains(calls, "-ar 16000 -ac 1") {
		t.Fatalf("ffmpeg calls:\n%s", calls)
	}
	if !strings.Contains(h.out.String(), "SPLIT book.m4b: 1 chapters") || !strings.Contains(h.out.String(), "PASS 001.m4a") {
		t.Fatalf("transcribe output: %s", h.out)
	}
	if code := BookTranscribe(context.Background(), BookTranscribeOptions{Dir: dir}, h.io()); code != 0 || strings.Contains(h.out.String(), "SPLIT") {
		t.Fatalf("second transcribe must reuse the split: %d %s", code, h.out)
	}
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	readalong := filepath.Join(dir, "out", "book-readalong.epub")
	names := memberNames(t, readalong)
	if !names["OEBPS/Audio/01-chapter.m4a"] || !strings.Contains(readMember(t, readalong, "OEBPS/package.opf"), `media-type="audio/mp4"`) {
		t.Fatalf("read-along output members: %v", names)
	}
}

func TestReadAlongConfigErrors(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	setVariants(t, dir, map[string]variantConfig{"readalong": {MediaOverlay: true}})
	if code := h.run(dir, "build"); code == 0 || !strings.Contains(h.err.String(), "no readAlong section") {
		t.Fatalf("mediaOverlay without readAlong: %d %s", code, h.err)
	}
	if code := BookTranscribe(context.Background(), BookTranscribeOptions{Dir: h.newBook("")}, h.io()); code != ExitUsage {
		t.Fatalf("transcribe without readAlong = %d", code)
	}
}

func TestWhisperWordsDTW(t *testing.T) {
	raw := `{"transcription":[{"tokens":[
{"text":"[_BEG_]","offsets":{"from":0,"to":0},"t_dtw":-1},
{"text":" Mr","offsets":{"from":1000,"to":1000},"t_dtw":110},
{"text":".","offsets":{"from":1000,"to":1000},"t_dtw":130},
{"text":" Jones","offsets":{"from":1000,"to":1000},"t_dtw":150},
{"text":" slept.","offsets":{"from":1000,"to":3000},"t_dtw":220}]}]}`
	words, err := whisperWords([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := [][]any{{"Mr.", 1.1, 1.5}, {"Jones", 1.5, 2.2}, {"slept.", 2.2, 3.0}}
	if fmt.Sprint(words) != fmt.Sprint(want) {
		t.Fatalf("words = %v, want %v", words, want)
	}
	// 没有 t_dtw 时退回 offsets。
	words, _ = whisperWords([]byte(strings.ReplaceAll(raw, `"t_dtw"`, `"x"`)))
	if fmt.Sprint(words[0]) != "[Mr. 1 1]" {
		t.Fatalf("token fallback = %v", words)
	}
}

func TestDTWPreset(t *testing.T) {
	for in, want := range map[string]string{
		"/m/ggml-small.en.bin": "small.en", "ggml-base.bin": "base", "ggml-large-v3-turbo.bin": "large.v3.turbo",
		"ggml-medium.en-q5_0.bin": "medium.en", "ggml-test.bin": "", "model.gguf": "",
	} {
		if got := dtwPreset(in); got != want {
			t.Errorf("dtwPreset(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindWhisperModel(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	shared := filepath.Join(filepath.Dir(dir), whisperModelsDir)
	layout := bookLayout{BookDir: dir, ReadAlong: &readAlongSpec{}}
	t.Setenv("EPUB_WHISPER_MODEL", "")
	if _, err := findWhisperModel(layout, ""); err == nil || !strings.Contains(err.Error(), whisperModelsDir) {
		t.Fatalf("no model: %v", err)
	}
	mustWrite(t, filepath.Join(shared, "ggml-small.en.bin"), "small")
	if got, err := findWhisperModel(layout, ""); err != nil || got != filepath.Join(shared, "ggml-small.en.bin") {
		t.Fatalf("single shared model = %q, %v", got, err)
	}
	mustWrite(t, filepath.Join(shared, "ggml-base.en.bin"), "base")
	if _, err := findWhisperModel(layout, ""); err == nil || !strings.Contains(err.Error(), "several Whisper models") {
		t.Fatalf("ambiguous: %v", err)
	}
	spec, err := resolveReadAlong(&readAlongConfig{Audio: "a", Model: "ggml-base.en.bin"}, dir)
	if err != nil || spec.ModelName != "ggml-base.en.bin" || spec.Model != "" {
		t.Fatalf("bare model name: %+v %v", spec, err)
	}
	layout.ReadAlong = spec
	if got, err := findWhisperModel(layout, ""); err != nil || got != filepath.Join(shared, "ggml-base.en.bin") {
		t.Fatalf("named model = %q, %v", got, err)
	}
	t.Setenv("EPUB_WHISPER_MODEL", filepath.Join(shared, "ggml-small.en.bin"))
	layout.ReadAlong = &readAlongSpec{}
	if got, _ := findWhisperModel(layout, ""); got != filepath.Join(shared, "ggml-small.en.bin") {
		t.Fatalf("env model = %q", got)
	}
}

// 变体自己的书名和标识符：打包后改写，算作变体的源，红线照常通过；NCX 的 dtb:uid 跟着改。
func TestBookVariantMetadata(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	setVariants(t, dir, map[string]variantConfig{"alt": {Metadata: map[string]string{"title": "Alt Edition", "identifier": "urn:uuid:alt-edition"}}})
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	alt := filepath.Join(dir, "out", "book-alt.epub")
	opfText := readMember(t, alt, "OEBPS/package.opf")
	if !strings.Contains(opfText, "Alt Edition") || !strings.Contains(opfText, "urn:uuid:alt-edition") {
		t.Fatalf("variant OPF:\n%s", opfText)
	}
	if !strings.Contains(readMember(t, alt, "OEBPS/toc.ncx"), `content="urn:uuid:alt-edition"`) {
		t.Fatal("NCX dtb:uid not synced")
	}
	if strings.Contains(readMember(t, filepath.Join(dir, "out", "book.epub"), "OEBPS/package.opf"), "Alt Edition") {
		t.Fatal("generic output must keep its own metadata")
	}
	if !strings.Contains(h.out.String(), "PASS epub.metadata.edit [alt]") {
		t.Fatalf("build output: %s", h.out)
	}
	setVariants(t, dir, map[string]variantConfig{"alt": {Metadata: map[string]string{"cover": "x"}}})
	if code := h.run(dir, "build"); code == 0 || !strings.Contains(h.err.String(), "metadata.cover is not editable") {
		t.Fatalf("bad metadata key: %d %s", code, h.err)
	}
}
