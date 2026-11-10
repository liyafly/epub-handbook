package navaudit

import (
	"bytes"
	"path/filepath"
	"testing"
)

func withMetadata(t *testing.T, metas string) string {
	t.Helper()
	base := writeEPUB3WithImageFixture(t, true)
	out := filepath.Join(t.TempDir(), "metadata.epub")
	rewriteZipEntry(t, base, out, "OEBPS/content.opf", func(data []byte) []byte {
		return bytes.Replace(data, []byte("</metadata>"), []byte(metas+"</metadata>"), 1)
	})
	return out
}

func findingKinds(t *testing.T, path string) map[string]string {
	t.Helper()
	kinds := map[string]string{}
	for _, f := range runNativeAudit(t, path).Findings {
		kinds[f.Detail] = f.Level
	}
	return kinds
}

func TestMetadataRefinesAndCollections(t *testing.T) {
	good := withMetadata(t, `<meta property="belongs-to-collection" id="series">丛书</meta>`+
		`<meta refines="#series" property="collection-type">series</meta>`+
		`<meta refines="#series" property="group-position">2</meta>`+
		`<meta refines="#cover" property="file-as">x</meta>`+
		`<meta property="schema:accessMode">textual</meta>`+
		`<meta property="schema:accessMode">visual</meta>`+
		`<meta property="schema:accessibilityHazard">none</meta>`)
	kinds := findingKinds(t, good)
	for _, k := range []string{"metadata-refines-missing", "collection-meta-without-collection", "access-mode-vocabulary", "accessibility-hazard-vocabulary"} {
		if _, ok := kinds[k]; ok {
			t.Errorf("valid metadata reported %s", k)
		}
	}

	cases := []struct {
		name, metas, kind, level string
	}{
		{"missing refines target", `<meta refines="#nope" property="file-as">x</meta>`, "metadata-refines-missing", "error"},
		{"group-position without collection", `<meta property="group-position">2</meta>`, "collection-meta-without-collection", "warn"},
		{"collection-type on a non-collection", `<meta property="dcterms:alternative" id="alt">x</meta><meta refines="#alt" property="collection-type">series</meta>`, "collection-meta-without-collection", "warn"},
		{"unknown access mode", `<meta property="schema:accessMode">text</meta>`, "access-mode-vocabulary", "warn"},
		{"unknown hazard", `<meta property="schema:accessibilityHazard">noFlashing</meta>`, "accessibility-hazard-vocabulary", "warn"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := findingKinds(t, withMetadata(t, c.metas))[c.kind]; got != c.level {
				t.Errorf("%s level = %q, want %q", c.kind, got, c.level)
			}
		})
	}
}
