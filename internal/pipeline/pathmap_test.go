package pipeline

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liyafly/epub-handbook/internal/report"
)

func TestNormalizePreviewMatchesAppliedCandidate(t *testing.T) {
	for _, mode := range []string{"format", "deobfuscate", "normalize"} {
		t.Run(mode, func(t *testing.T) {
			before := buildSampleEpub(t)
			original, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			after := filepath.Join(t.TempDir(), "candidate.epub")
			opts := Options{CapabilityID: "epub.structure.normalize", InputPath: before, OutputPath: after, DryRun: true, Args: Args{"mode": mode}}
			preview, err := Run(t.Context(), opts)
			if err != nil || preview.ExitCode != ExitOK || preview.Envelope.Status != report.StatusPlanned {
				t.Fatalf("preview exit=%d err=%v findings=%v", preview.ExitCode, err, preview.Envelope.Findings)
			}
			if _, err := os.Stat(after); !os.IsNotExist(err) {
				t.Fatalf("preview wrote output: %v", err)
			}
			opts.DryRun = false
			applied, err := Run(t.Context(), opts)
			if err != nil || applied.ExitCode != ExitOK {
				t.Fatalf("apply exit=%d err=%v findings=%v", applied.ExitCode, err, applied.Envelope.Findings)
			}
			for _, key := range []string{"mappings", "rewrittenFiles", "movedResources", "renamedResources"} {
				key = "epub.structure.normalize." + key
				if !reflect.DeepEqual(preview.Envelope.Facts[key], applied.Envelope.Facts[key]) {
					t.Fatalf("preview/apply differ for %s", key)
				}
			}
			current, err := os.ReadFile(before)
			if err != nil || !bytes.Equal(original, current) {
				t.Fatal("source archive changed")
			}
		})
	}
}

// TestNormalizeEnvelopeFeedsRedlinePathMap 锁定 AGENTS.md 步骤 4–6 的工作流：
// `epub run epub.structure.normalize --json` 的信封原样保存后，直接作为
// `epub redline --path-map <envelope.json>` 的映射来源，改名后的资源不再被
// 判为缺失/新增。fixture 的 c1.xhtml 位于 OPF 根目录，format 阶段会把它
// 移入 Text/，因此必然产生至少一条 mapping。
func TestNormalizeEnvelopeFeedsRedlinePathMap(t *testing.T) {
	before := buildSampleEpub(t)
	dir := t.TempDir()
	after := filepath.Join(dir, "normalized.epub")
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.structure.normalize",
		InputPath:    before,
		OutputPath:   after,
		Args:         Args{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(after); err != nil {
		t.Fatalf("normalized EPUB 未写出: %v (exit=%d findings=%#v)", err, outcome.ExitCode, outcome.Envelope.Findings)
	}
	mappings, ok := outcome.Envelope.Facts["epub.structure.normalize.mappings"]
	if !ok {
		t.Fatalf("信封缺少 facts[epub.structure.normalize.mappings]: %v", keysOf(outcome.Envelope.Facts))
	}
	rawMappings, err := json.Marshal(mappings)
	if err != nil {
		t.Fatal(err)
	}
	var pairs []struct{ From, To string }
	if err := json.Unmarshal(rawMappings, &pairs); err != nil || len(pairs) == 0 {
		t.Fatalf("mappings 应为非空 {from,to} 数组，got %s (err=%v)", rawMappings, err)
	}

	// 与 CLI --json 一致：整份信封落盘。
	envelopePath := filepath.Join(dir, "normalize-envelope.json")
	data, err := json.MarshalIndent(outcome.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envelopePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var reparsed report.Envelope
	if err := json.Unmarshal(data, &reparsed); err != nil || reparsed.SchemaVersion != "2" {
		t.Fatalf("落盘信封应为 schemaVersion=2 的 JSON: %v", err)
	}

	code, err := RedlineCompareWith(before, after, "all", nil, []string{envelopePath}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if code != ExitOK {
		t.Fatalf("以信封为 --path-map 的全量红线应通过，exit=%d", code)
	}
	code, err = RedlineCompareWith(before, after, "all", nil, nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if code == ExitOK {
		t.Fatal("不带 --path-map 时资源改名应被红线判为差异（对照）")
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
