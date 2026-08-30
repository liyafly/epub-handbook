package editset

import (
	"errors"
	"testing"
)

func TestApplyReplacesRanges(t *testing.T) {
	t.Parallel()
	content := []byte("Hello, 世界！ This is a test.")
	got, err := Apply("f.xhtml", content, []Edit{
		Replace("f.xhtml", 7, 9, []byte("世界")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "Hello, 世界 This is a test." {
		t.Fatalf("got %q", got)
	}
}

func TestApplyInsertAndRemove(t *testing.T) {
	t.Parallel()
	content := []byte("abcdef")
	got, err := Apply("f", content, []Edit{
		Insert("f", 3, []byte("XY")),
		Replace("f", 0, 1, []byte{}), // 清空区间 [0,1)；nil 才是「删除整个 entry」
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "bcXYdef" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyIgnoresDeleteEdits(t *testing.T) {
	t.Parallel()
	got, err := Apply("f", []byte("abc"), []Edit{Delete("f"), Replace("f", 0, 1, []byte("z"))})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "zbc" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyRejectsForeignBatch(t *testing.T) {
	t.Parallel()
	if _, err := Apply("a", []byte("x"), []Edit{Replace("b", 0, 0, []byte("y"))}); err == nil {
		t.Fatal("want error for foreign edit in batch")
	}
}

func TestApplyRejectsOverlapAndBounds(t *testing.T) {
	t.Parallel()
	if _, err := Apply("f", []byte("abcdef"), []Edit{
		Replace("f", 0, 3, []byte("x")),
		Replace("f", 2, 2, []byte("y")),
	}); !errors.Is(err, ErrOverlap) {
		t.Fatalf("want ErrOverlap, got %v", err)
	}
	if _, err := Apply("f", []byte("abc"), []Edit{Replace("f", 2, 2, []byte("y"))}); err == nil {
		t.Fatal("want out-of-bounds error")
	}
}

func TestSortStableForSameOffsetInserts(t *testing.T) {
	t.Parallel()
	edits := []Edit{
		Insert("f", 1, []byte("B")),
		Insert("a", 5, []byte("A")),
		Insert("f", 1, []byte("A")),
	}
	Sort(edits)
	want := []string{"a@5", "f@1", "f@1"}
	for i, w := range want {
		if edits[i].Path != w[:1] {
			t.Fatalf("edits[%d].Path = %q, want %q", i, edits[i].Path, w[:1])
		}
	}
	// 同一位置的插入保持原相对次序。
	if string(edits[1].Replacement) != "B" || string(edits[2].Replacement) != "A" {
		t.Fatalf("stable order broken: %q %q", edits[1].Replacement, edits[2].Replacement)
	}
}

func TestValidateDetectsOverlapAcrossBatch(t *testing.T) {
	t.Parallel()
	if err := Validate([]Edit{
		Replace("b", 0, 4, []byte("x")),
		Replace("a", 0, 4, []byte("x")),
		Replace("a", 3, 2, []byte("y")),
	}); !errors.Is(err, ErrOverlap) {
		t.Fatalf("want ErrOverlap, got %v", err)
	}
	if err := Validate([]Edit{
		Replace("a", 0, 4, []byte("x")),
		Replace("a", 4, 0, []byte("y")),
		Replace("b", 0, 1, []byte("z")),
	}); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}
