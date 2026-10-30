package pipeline

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypographyReplaceAndAppendEndToEnd(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, mode := range []string{"replace", "append"} {
		t.Run(mode, func(t *testing.T) {
			input := writeNewCapabilityEPUB(t)
			dir := t.TempDir()
			styles := filepath.Join(dir, "sample", "Styles")
			if err := os.MkdirAll(styles, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "sample", "preset.json"), []byte(`{"name":"sample","version":"1","layers":["media.css"]}`), 0644); err != nil {
				t.Fatal(err)
			}
			style := filepath.Join(styles, "media.css")
			if err := os.WriteFile(style, []byte("p { margin: 0; }"), 0644); err != nil {
				t.Fatal(err)
			}
			args := Args{"preset": "sample", "preset_dir": dir, "scope_paths": `["OEBPS/chapter.xhtml"]`}
			first, err := Run(t.Context(), Options{CapabilityID: "epub.typography.optimize", InputPath: input, CaptureOutput: true, Args: args})
			if err != nil || first.ExitCode != ExitOK {
				t.Fatalf("first=%+v err=%v", first.Envelope, err)
			}
			if err := os.WriteFile(style, []byte("p { margin: 1em; }"), 0644); err != nil {
				t.Fatal(err)
			}
			// Omission verifies the public default; append must pass through registry.
			if mode == "append" {
				args["preset_mode"] = mode
			}
			second, err := Run(t.Context(), Options{CapabilityID: "epub.typography.optimize", InputPath: input, InputBytes: first.OutputBytes, CaptureOutput: true, Args: args})
			if err != nil || second.ExitCode != ExitOK {
				t.Fatalf("second=%+v err=%v", second.Envelope, err)
			}
			page := readPipelineEPUBEntryBytes(t, second.OutputBytes, "OEBPS/chapter.xhtml")
			want := 1
			if mode == "append" {
				want = 2
			}
			if strings.Count(string(page), "epub-preset-") != want {
				t.Fatalf("mode=%s page=%s", mode, page)
			}
			if second.Envelope.Facts["epub.typography.optimize.presetMode"] != mode {
				t.Fatal(second.Envelope.Facts)
			}
			if mode == "append" {
				return
			}
			second.Envelope.Input.Path = "<fixture.epub>"
			second.Envelope.Input.SHA256 = ""
			if second.Envelope.Output != nil {
				second.Envelope.Output.SHA256 = ""
			}
			var actions []map[string]any
			rawActions, err := json.Marshal(second.Envelope.Facts["epub.typography.optimize.stylesheetActions"])
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(rawActions, &actions); err != nil {
				t.Fatal(err)
			}
			for _, action := range actions {
				action["source"] = "<presets>/sample/Styles/media.css"
			}
			second.Envelope.Facts["epub.typography.optimize.stylesheetActions"] = actions
			normalizePipelineNextCommands(second.Envelope.NextCommands, input)
			data, err := json.MarshalIndent(second.Envelope, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			data = bytes.ReplaceAll(data, []byte(dir), []byte("<presets>"))
			data = append(data, '\n')
			golden := filepath.Join(repoRootForTest(t), "testdata/typography/scoped-replace.report.json")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, data, 0644); err != nil {
					t.Fatal(err)
				}
				return
			}
			expected, err := os.ReadFile(golden)
			if err != nil || !bytes.Equal(expected, data) {
				t.Fatalf("golden mismatch err=%v\n%s", err, data)
			}
		})
	}
}
