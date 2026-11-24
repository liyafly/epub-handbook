package pipeline

import (
	"context"
	"encoding/json"
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
