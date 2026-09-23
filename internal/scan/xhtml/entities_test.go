package xhtml

import (
	"reflect"
	"testing"
)

func TestDecodeAttrWithMap(t *testing.T) {
	plain, plainOffsets, err := DecodeAttrWithMap("图.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if plain != "图.jpg" || !reflect.DeepEqual(plainOffsets, []int{0, 0, 0, 3, 4, 5, 6, 7}) {
		t.Fatalf("plain UTF-8 mapping = %q, %v", plain, plainOffsets)
	}

	raw := "A&amp;B&#169;&#x1F600;"
	decoded, rawOff, err := DecodeAttrWithMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	if want := "A&B©😀"; decoded != want {
		t.Fatalf("decoded = %q, want %q", decoded, want)
	}
	wantOffsets := []int{0, 1, 6, 7, 7, 13, 13, 13, 13, len(raw)}
	if !reflect.DeepEqual(rawOff, wantOffsets) {
		t.Fatalf("rawOff = %v, want %v", rawOff, wantOffsets)
	}

	decoded, rawOff, err = DecodeAttrWithMap(`&lt;&gt;&quot;&apos;&#10;&#x9;`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "<>\"'\n\t"; decoded != want {
		t.Fatalf("predefined refs decoded = %q, want %q", decoded, want)
	}
	if rawOff[len(rawOff)-1] != len(`&lt;&gt;&quot;&apos;&#10;&#x9;`) {
		t.Fatalf("final raw offset = %d", rawOff[len(rawOff)-1])
	}
}

func TestDecodeAttrWithMapRejectsUnsupportedReferences(t *testing.T) {
	for _, raw := range []string{"bare&", "&nbsp;", "&bogus;", "&#;", "&#xZZ;", "&#0;", "&#xD800;", "&#x110000;"} {
		if _, _, err := DecodeAttrWithMap(raw); err == nil {
			t.Errorf("DecodeAttrWithMap(%q) unexpectedly succeeded", raw)
		}
	}
}
