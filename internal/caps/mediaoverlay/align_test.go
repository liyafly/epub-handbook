package mediaoverlay

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func lcsLen(a, b []string) int {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}
	return dp[0][0]
}

func TestLCSPairsIsLongestAndMonotonic(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	alphabet := []string{"a", "b", "c", "d"}
	for range 300 {
		a := make([]string, r.IntN(30))
		b := make([]string, r.IntN(30))
		for i := range a {
			a[i] = alphabet[r.IntN(len(alphabet))]
		}
		for i := range b {
			b[i] = alphabet[r.IntN(len(alphabet))]
		}
		pairs := lcsPairs(a, b)
		if len(pairs) != lcsLen(a, b) {
			t.Fatalf("a=%v b=%v: %d pairs, LCS %d", a, b, len(pairs), lcsLen(a, b))
		}
		keys := make([]int, 0, len(pairs))
		for k := range pairs {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for i, k := range keys {
			if a[k] != b[pairs[k]] || (i > 0 && pairs[k] <= pairs[keys[i-1]]) {
				t.Fatalf("bad pairing %v for a=%v b=%v", pairs, a, b)
			}
		}
	}
}

func words(spec string) []Word {
	var out []Word
	for i, w := range strings.Fields(spec) {
		out = append(out, Word{Text: w, Start: float64(i), End: float64(i) + 0.8})
	}
	return out
}

func TestAlignClips(t *testing.T) {
	segs := []string{"Chapter One", "Mr Jones locked the hen-houses.", "‘Comrades!’", "He sang 4 songs."}
	// 片头两词、"Chapter One" 没念、"Comrades" 没识别出来、数字写成词
	heard := words("This is Mr. Jones locked the hen houses. He sang four songs.")
	got := Align(segs, heard, 20)
	if got.BookWords != 13 || got.Matched != 10 {
		t.Fatalf("words %d matched %d", got.BookWords, got.Matched)
	}
	want := []Clip{
		{Begin: 0, End: 2, Words: 2, Matched: 0},
		{Begin: 2, End: 7.8, Words: 6, Matched: 6},
		{Begin: 7.8, End: 8, Words: 1, Matched: 0},
		{Begin: 8, End: 12.8, Words: 4, Matched: 4},
	}
	for i, c := range got.Clips {
		w := want[i]
		if c.Words != w.Words || c.Matched != w.Matched || abs(c.Begin-w.Begin) > 1e-9 || abs(c.End-w.End) > 1e-9 {
			t.Errorf("clip %d = %+v, want %+v", i, c, w)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
