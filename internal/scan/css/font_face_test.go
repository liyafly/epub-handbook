package css

import "testing"

func TestDeclarationAtRulesRemainInProjection(t *testing.T) {
	sheet, err := Parse([]byte(`@font-face{font-family:Demo;src:url(demo.woff2)}@page{margin:1em}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Rules) != 2 || sheet.Rules[0].AtRuleName != "font-face" || len(sheet.Rules[0].Declarations) != 2 || sheet.Rules[1].AtRuleName != "page" {
		t.Fatalf("lost declaration rules: %+v", sheet.Rules)
	}
}

func TestUnquotedURLSpanExcludesClosingParenthesis(t *testing.T) {
	for _, value := range []string{"demo.woff2", "demo.woff2   ", `demo\)file.woff2`} {
		data := []byte("@font-face{src:url(" + value + ")}")
		sheet, err := Parse(data)
		if err != nil || len(sheet.References) != 1 {
			t.Fatalf("parse=%v %v", sheet, err)
		}
		want := value
		if value == "demo.woff2   " {
			want = "demo.woff2"
		}
		ref := sheet.References[0]
		if ref.Value != want || string(data[ref.ValueSpan.Start:ref.ValueSpan.End]) != want {
			t.Fatalf("reference=%+v, want %q", ref, want)
		}
	}
}
