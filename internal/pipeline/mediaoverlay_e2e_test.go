package pipeline

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/report"
)

// 两章、每章一个音频；时间轴按「每个词 0.5 秒、句间 0.3 秒」合成，第二章把一个数字念成词、
// 漏念章题，用来覆盖插值与数字归一。
var moChapters = []struct {
	file, title string
	paras       []string
}{
	{"one.xhtml", "Chapter One", []string{
		"The farm was quiet at night. The animals slept in the barn while the old dog kept watch by the gate.",
		"In the morning the hens woke first. They walked across the yard and looked for grain near the kitchen door. The horses followed them slowly.",
		"Nobody spoke about the strange dream. Everyone knew that something would change before the summer ended, and they waited for a sign.",
	}},
	{"two.xhtml", "Chapter Two", []string{
		"Three nights later the old pig called a meeting. He stood on the platform and asked the animals to listen with care.",
		"He told them about 7 rules that every animal should keep. The young pigs wrote the rules on the wall in large white letters.",
		"The meeting ended late. The animals went back to their places and thought about the rules for a long time before they slept.",
	}},
}

func moChapterXHTML(title string, paras []string, figure bool) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en" xml:lang="en"><head><title>` + title + `</title><link rel="stylesheet" type="text/css" href="styles.css"/></head><body>` + "\n")
	b.WriteString(`<section epub:type="chapter">` + "\n<h1>" + title + "</h1>\n")
	for _, p := range paras {
		b.WriteString("<p>" + p + "</p>\n")
	}
	if figure {
		b.WriteString(`<figure id="fig-1"><img src="cover.png" alt="Handwritten page"/><figcaption><a epub:type="noteref" role="doc-noteref" href="#note-1">Transcription</a></figcaption></figure>` + "\n")
		b.WriteString(`<aside epub:type="footnotes"><aside id="note-1" epub:type="footnote" role="doc-footnote"><p><a epub:type="backlink" role="doc-backlink" href="#fig-1">◎</a>All animals are equal.</p></aside></aside>` + "\n")
	}
	b.WriteString("</section></body></html>\n")
	return b.String()
}

func writeMediaOverlayFixture(t *testing.T) (input, audioDir, timingsDir string) {
	t.Helper()
	dir := t.TempDir()
	entries := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf": `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid">
<metadata>
<dc:title>Read-along fixture</dc:title>
<dc:identifier id="uid">urn:uuid:read-along-fixture</dc:identifier>
<dc:language>en</dc:language>
<meta property="dcterms:modified">2026-11-24T00:00:00Z</meta>
</metadata>
<manifest>
<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/>
<item id="one" href="one.xhtml" media-type="application/xhtml+xml"/>
<item id="two" href="two.xhtml" media-type="application/xhtml+xml"/>
<item id="style" href="styles.css" media-type="text/css"/>
</manifest>
<spine><itemref idref="nav"/><itemref idref="one"/><itemref idref="two"/></spine>
</package>
`,
		"OEBPS/nav.xhtml":  `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en" xml:lang="en"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="one.xhtml">One</a></li><li><a href="two.xhtml">Two</a></li></ol></nav></body></html>`,
		"OEBPS/cover.png":  "PNG fixture bytes",
		"OEBPS/styles.css": "p { margin: 0; }",
		"OEBPS/one.xhtml":  moChapterXHTML(moChapters[0].title, moChapters[0].paras, true),
		"OEBPS/two.xhtml":  moChapterXHTML(moChapters[1].title, moChapters[1].paras, false),
	}
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	mt, _ := w.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	mt.Write([]byte("application/epub+zip"))
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		e, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		e.Write([]byte(entries[n]))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	input = filepath.Join(dir, "fixture.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	audioDir, timingsDir = filepath.Join(dir, "audio"), filepath.Join(dir, "timings")
	os.MkdirAll(audioDir, 0o755)
	os.MkdirAll(timingsDir, 0o755)
	for i, ch := range moChapters {
		audio := []byte(fmt.Sprintf("ID3 fake audio for chapter %d", i+1))
		name := fmt.Sprintf("%02d.mp3", i+1)
		if err := os.WriteFile(filepath.Join(audioDir, name), audio, 0o644); err != nil {
			t.Fatal(err)
		}
		spoken := ch.paras
		if i == 0 {
			spoken = append([]string{ch.title}, append(slices.Clone(ch.paras), "All animals are equal.")...)
		} else {
			spoken = slices.Clone(ch.paras)
			spoken[1] = strings.Replace(spoken[1], "7", "seven", 1)
		}
		var words [][]any
		at := 1.0
		for _, s := range spoken {
			for _, sent := range strings.SplitAfter(s, ". ") {
				for _, wd := range strings.Fields(sent) {
					words = append(words, []any{wd, at, at + 0.4})
					at += 0.5
				}
				at += 0.3
			}
		}
		sum := sha256.Sum256(audio)
		tf := map[string]any{
			"schemaVersion": "1",
			"audio":         map[string]any{"path": name, "sha256": hex.EncodeToString(sum[:]), "duration": at + 2},
			"engine":        map[string]any{"name": "fixture", "version": "1"},
			"words":         words,
		}
		data, _ := json.Marshal(tf)
		if err := os.WriteFile(filepath.Join(timingsDir, name+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return input, audioDir, timingsDir
}

func runMediaOverlay(t *testing.T, input, output string, args Args) Outcome {
	t.Helper()
	t.Setenv("PATH", t.TempDir()) // nav.audit 的 epubcheck 探测保持稳定
	outcome, err := Run(t.Context(), Options{CapabilityID: "epub.media.overlay.build", InputPath: input, OutputPath: output, Args: args})
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func TestMediaOverlayEndToEndGolden(t *testing.T) {
	input, audioDir, timingsDir := writeMediaOverlayFixture(t)
	output := filepath.Join(t.TempDir(), "out.epub")
	outcome := runMediaOverlay(t, input, output, Args{"audio_dir": audioDir, "timings_dir": timingsDir, "narrator": "A Reader"})
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusComplete {
		t.Fatalf("status=%q exit=%d findings=%+v", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	root := repoRootForTest(t)
	golden := filepath.Join(root, "testdata", "media_overlay")
	for _, name := range []string{"OEBPS/content.opf", "OEBPS/one.xhtml", "OEBPS/two.xhtml", "OEBPS/Smil/one.smil", "OEBPS/Smil/two.smil", "OEBPS/styles.css"} {
		got := readPipelineEPUBEntry(t, output, name)
		compareGolden(t, filepath.Join(golden, strings.ReplaceAll(name, "/", "_")), got)
	}
	if got := readPipelineEPUBEntry(t, output, "OEBPS/Audio/one.mp3"); string(got) != "ID3 fake audio for chapter 1" {
		t.Fatalf("audio entry = %q", got)
	}
	zr, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".mp3") && f.Method != zip.Store {
			t.Fatalf("%s must be stored, method %d", f.Name, f.Method)
		}
	}
	env := outcome.Envelope
	env.Input.Path, env.Input.SHA256 = "<fixture.epub>", ""
	env.Output.Path, env.Output.SHA256 = "<out.epub>", ""
	data, _ := json.MarshalIndent(env, "", "  ")
	data = bytes.ReplaceAll(data, []byte(audioDir), []byte("<audio>"))
	data = bytes.ReplaceAll(data, []byte(output), []byte("<out.epub>"))
	compareGolden(t, filepath.Join(golden, "envelope.json"), append(data, '\n'))
}

func compareGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from golden\n--- got ---\n%s", filepath.Base(path), got)
	}
}

func TestMediaOverlayFailures(t *testing.T) {
	input, audioDir, timingsDir := writeMediaOverlayFixture(t)
	cases := []struct {
		name  string
		setup func(t *testing.T) Args
		want  string
	}{
		{"count mismatch", func(t *testing.T) Args {
			os.WriteFile(filepath.Join(audioDir, "03.mp3"), []byte("extra"), 0o644)
			t.Cleanup(func() { os.Remove(filepath.Join(audioDir, "03.mp3")) })
			return Args{"audio_dir": audioDir, "timings_dir": timingsDir}
		}, "media-overlay.chapter-count-mismatch"},
		{"missing timings", func(t *testing.T) Args {
			return Args{"audio_dir": audioDir, "timings_dir": t.TempDir()}
		}, "media-overlay.timings-missing"},
		{"swapped audio", func(t *testing.T) Args {
			m := filepath.Join(t.TempDir(), "map.json")
			os.WriteFile(m, []byte(`{"one.xhtml": "02.mp3", "two.xhtml": "01.mp3"}`), 0o644)
			return Args{"audio_dir": audioDir, "timings_dir": timingsDir, "audio_map": m}
		}, "media-overlay.low-match"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "out.epub")
			outcome := runMediaOverlay(t, input, output, c.setup(t))
			if outcome.ExitCode == ExitOK {
				t.Fatalf("exit 0, findings=%+v", outcome.Envelope.Findings)
			}
			if c.want != "" && !slices.ContainsFunc(outcome.Envelope.Findings, func(f report.Finding) bool { return f.ID == c.want }) {
				t.Fatalf("findings=%+v, want %s", outcome.Envelope.Findings, c.want)
			}
			if _, err := os.Stat(output); err == nil && c.want != "" {
				if got := readPipelineEPUBEntryOptional(t, output, "OEBPS/Smil/one.smil"); got != nil {
					t.Fatal("failed run must not add SMIL")
				}
			}
		})
	}
}

func TestMediaOverlayRejectsBadGranularity(t *testing.T) {
	input, audioDir, timingsDir := writeMediaOverlayFixture(t)
	outcome, err := Run(t.Context(), Options{CapabilityID: "epub.media.overlay.build", InputPath: input,
		OutputPath: filepath.Join(t.TempDir(), "out.epub"),
		Args:       Args{"audio_dir": audioDir, "timings_dir": timingsDir, "granularity": "word"}})
	if err == nil && outcome.ExitCode != ExitUsage {
		t.Fatalf("bad granularity accepted: exit %d", outcome.ExitCode)
	}
}

func readPipelineEPUBEntryOptional(t *testing.T, epubPath, target string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == target {
			return readPipelineEPUBEntry(t, epubPath, target)
		}
	}
	return nil
}
