package pipeline

import (
	"testing"
)

// TestChainNeedsWriteFollowsContract verifies that --output requirements follow
// the execution and permission fields in the capability contract.
func TestChainNeedsWriteFollowsContract(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id   string
		want bool
	}{
		{"epub.structure.normalize", true}, // 写单产物
		{"epub.package.migrate.epub3", true},
		{"epub.package.split", true},      // 多产物仍是写出型，只是走 output_dir
		{"epub.package.nav.audit", false}, // 只读
		{"epub.notes.popup.normalize", false},
		{"epub.style.demo.maintain", false},
		{"epub.source.intake", false},
	}
	for _, tc := range cases {
		chain, err := ResolveChain(root, tc.id)
		if err != nil {
			t.Errorf("%s: ResolveChain: %v", tc.id, err)
			continue
		}
		if got := chainNeedsWrite(chain); got != tc.want {
			t.Errorf("%s: chainNeedsWrite = %v, want %v", tc.id, got, tc.want)
		}
	}
}
