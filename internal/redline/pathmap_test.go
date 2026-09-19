package redline

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestLoadPathMapAcceptsEnvelope 锁定 --path-map 直接吃 `epub run
// epub.structure.normalize --json` 信封：facts 中 `*.mappings` 数组即映射源。
func TestLoadPathMapAcceptsEnvelope(t *testing.T) {
	envelope := `{
	  "schemaVersion": "2",
	  "capability": "epub.structure.normalize",
	  "status": "complete",
	  "facts": {
	    "epub.structure.normalize.dryRun": false,
	    "epub.structure.normalize.stages": [{"operation": "format", "mappings": [{"from": "x", "to": "y"}]}],
	    "epub.structure.normalize.mappings": [
	      {"from": "OEBPS/a.xhtml", "to": "OEBPS/Text/a.xhtml"},
	      {"from": "OEBPS/Text/a.xhtml", "to": "OEBPS/Text/chapter1.xhtml"}
	    ]
	  },
	  "findings": [], "events": [], "nextCommands": []
	}`
	got, err := LoadPathMap([]byte(envelope))
	if err != nil {
		t.Fatal(err)
	}
	// 链式传递：a.xhtml 最终指向 chapter1.xhtml；stages 内嵌的 mappings 不重复读取。
	want := map[string]string{
		"OEBPS/a.xhtml":      "OEBPS/Text/chapter1.xhtml",
		"OEBPS/Text/a.xhtml": "OEBPS/Text/chapter1.xhtml",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("path map = %v, want %v", got, want)
	}
}

// TestLoadPathMapEnvelopeMultipleMappingKeys 锁定多个 `.mappings` 键按键名
// 排序后依次链式处理（链式调用信封聚合多个 stage 的场景）。
func TestLoadPathMapEnvelopeMultipleMappingKeys(t *testing.T) {
	envelope := `{"schemaVersion":"2","facts":{
	  "b.stage.mappings":[{"from":"mid.xhtml","to":"final.xhtml"}],
	  "a.stage.mappings":[{"from":"orig.xhtml","to":"mid.xhtml"}],
	  "mappings":[{"from":"other.css","to":"Styles/other.css"}],
	  "a.stage.notmappings":[{"from":"ignored","to":"ignored2"}]
	}}`
	got, err := LoadPathMap([]byte(envelope))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"orig.xhtml": "final.xhtml",
		"mid.xhtml":  "final.xhtml",
		"other.css":  "Styles/other.css",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("path map = %v, want %v", got, want)
	}
}

// TestLoadPathMapLegacyShapesStillWork 保证 stages / 单对象两种旧形状不变。
func TestLoadPathMapLegacyShapesStillWork(t *testing.T) {
	cases := map[string]string{
		"stages": `{"stages":[{"mappings":[{"from":"a","to":"b"}]},{"mappings":[{"from":"b","to":"c"}]}]}`,
		"single": `{"mappings":[{"from":"a","to":"b"},{"from":"b","to":"c"}]}`,
	}
	want := map[string]string{"a": "c", "b": "c"}
	for name, src := range cases {
		got, err := LoadPathMap([]byte(src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: path map = %v, want %v", name, got, want)
		}
	}
}

func TestLoadPathMapRejectsMalformedMappings(t *testing.T) {
	for name, src := range map[string]string{
		"envelope": `{"schemaVersion":"2","facts":{"x.mappings":[{"from":1,"to":"b"}]}}`,
		"legacy":   `{"mappings":["not-an-object"]}`,
		"badjson":  `{`,
	} {
		if _, err := LoadPathMap([]byte(src)); err == nil || !isErrInput(err) {
			t.Errorf("%s: err = %v, want input error", name, err)
		}
	}
}

// TestCompareFilesWithEnvelopePathMap 端到端：信封映射喂给 CompareFiles，
// 改名后的正文不再被判为缺失/新增。
func TestCompareFilesWithEnvelopePathMap(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	buildEpub(t, before, baseEntries())
	renamed := baseEntries()
	for i := range renamed {
		if renamed[i].name == "OEBPS/Text/c1.xhtml" {
			renamed[i].name = "OEBPS/Text/chapter1.xhtml"
		}
	}
	buildEpub(t, after, renamed)
	envelope := `{"schemaVersion":"2","capability":"epub.structure.normalize","status":"complete",
	  "facts":{"epub.structure.normalize.mappings":[{"from":"OEBPS/Text/c1.xhtml","to":"OEBPS/Text/chapter1.xhtml"}]}}`
	mapPath := filepath.Join(dir, "normalize-envelope.json")
	if err := os.WriteFile(mapPath, []byte(envelope), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	pm, err := LoadPathMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	withMap, err := CompareFiles(before, after, "text", Options{PathMap: pm})
	if err != nil {
		t.Fatal(err)
	}
	if withMap.Code != 0 {
		t.Fatalf("带信封映射的 text 红线应通过，code=%d lines=%v", withMap.Code, withMap.Lines)
	}
	without, err := CompareFiles(before, after, "text", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if without.Code == 0 {
		t.Fatal("不带映射时改名应被判为差异，用作对照")
	}
}

// TestLoadPathMapRejectsEnvelopesWithoutMappings 锁定：用户显式传了 --path-map，
// 却拿到零条映射时必须报输入错误，而不是静默返回空 map。
// 静默空 map 的后果是改名后的正文被逐一判成"文件缺失 / 新增文件"，
// 真正的红线问题被噪声淹没（典型触发：把 FAILED 的 normalize 信封喂进来）。
func TestLoadPathMapRejectsEnvelopesWithoutMappings(t *testing.T) {
	for name, src := range map[string]string{
		"facts-null":             `{"schemaVersion":"2","capability":"epub.structure.normalize","status":"failed","facts":null}`,
		"facts-absent":           `{"schemaVersion":"2","capability":"epub.structure.normalize","status":"failed"}`,
		"facts-empty":            `{"schemaVersion":"2","facts":{}}`,
		"facts-no-mappings-key":  `{"schemaVersion":"2","facts":{"epub.structure.normalize.dryRun":true}}`,
		"mappings-object":        `{"schemaVersion":"2","facts":{"epub.structure.normalize.mappings":{"OEBPS/a.xhtml":"OEBPS/b.xhtml"}}}`,
		"mappings-string":        `{"schemaVersion":"2","facts":{"epub.structure.normalize.mappings":"OEBPS/a.xhtml"}}`,
		"mappings-null":          `{"schemaVersion":"2","facts":{"epub.structure.normalize.mappings":null}}`,
		"toplevel-array":         `[{"mappings":[{"from":"a","to":"b"}]}]`,
		"empty-object":           `{}`,
		"legacy-no-mappings":     `{"stages":[{"operation":"format","moved_resources":0}]}`,
		"legacy-mappings-object": `{"mappings":{"from":"a","to":"b"}}`,
		"legacy-mappings-string": `{"mappings":"OEBPS/a.xhtml"}`,
	} {
		got, err := LoadPathMap([]byte(src))
		if err == nil || !isErrInput(err) {
			t.Errorf("%s: err = %v (map=%v), want input error", name, err, got)
			continue
		}
		if got != nil {
			t.Errorf("%s: 出错时应返回 nil map，实际 %v", name, got)
		}
	}
}

// TestLoadPathMapAcceptsEmptyMappingList 锁定反向边界：未改名的**成功**
// normalize 会输出 `"mappings": []`（structure_normalize 的 nonNilMappings），
// 空数组必须继续被接受为"没有映射"，只有缺键才是错误。
func TestLoadPathMapAcceptsEmptyMappingList(t *testing.T) {
	for name, src := range map[string]string{
		"envelope": `{"schemaVersion":"2","capability":"epub.structure.normalize","status":"complete",` +
			`"facts":{"epub.structure.normalize.mappings":[],"epub.structure.normalize.warnings":[]}}`,
		"legacy-single": `{"mappings":[]}`,
		"legacy-stages": `{"stages":[{"operation":"format","mappings":[]}]}`,
	} {
		got, err := LoadPathMap([]byte(src))
		if err != nil {
			t.Errorf("%s: err = %v, want nil", name, err)
			continue
		}
		if len(got) != 0 {
			t.Errorf("%s: path map = %v, want 空 map", name, got)
		}
	}
}

// TestLoadPathMapErrorMentionsFailedRun 保证错误信息足以让人自查，
// 而不是只说"无效输入"。
func TestLoadPathMapErrorMentionsFailedRun(t *testing.T) {
	_, err := LoadPathMap([]byte(`{"schemaVersion":"2","status":"failed","facts":null}`))
	if err == nil {
		t.Fatal("want error")
	}
	for _, want := range []string{"mappings", "epub.structure.normalize"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息缺少 %q: %v", want, err)
		}
	}
}
