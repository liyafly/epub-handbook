package mediaoverlay

import (
	"strings"
	"unicode"
)

// Word 是识别器给出的一个词及其起止秒数。
type Word struct {
	Text       string
	Start, End float64
}

// Clip 是一个分段在音频里的区间。
type Clip struct {
	Begin, End float64
	Words      int // 分段的书面词数
	Matched    int // 与识别词对上的书面词数
}

// ChapterAlignment 汇总一章的对齐结果。
type ChapterAlignment struct {
	Clips     []Clip
	BookWords int
	Heard     int
	Matched   int
}

// MatchRate 是对上的书面词比例。
func (a ChapterAlignment) MatchRate() float64 {
	if a.BookWords == 0 {
		return 0
	}
	return float64(a.Matched) / float64(a.BookWords)
}

const (
	maxJoinGap = 2.0 // 相邻句之间的空白超过这个秒数就截断，不连到下一句
	endPad     = 0.6 // 截断时在最后一个词后留的余量
	tailPad    = 1.0 // 一章最后一句之后留的余量
	minClip    = 0.05
)

// tokens 把文字规范成对齐用的词：小写、去撇号、拆成字母数字串，个位数字转英文词，
// 汉字和假名逐字拆开。
func tokens(s string) []string {
	s = strings.ToLower(strings.NewReplacer("’", "", "'", "").Replace(s))
	// 汉字、假名逐字成词：中日文没有空格，识别器的分词也和书面不同
	var spaced strings.Builder
	for _, r := range s {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) {
			spaced.WriteByte(' ')
			spaced.WriteRune(r)
			spaced.WriteByte(' ')
		} else {
			spaced.WriteRune(r)
		}
	}
	words := strings.FieldsFunc(spaced.String(), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	digits := [...]string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	for i, w := range words {
		if len(w) == 1 && w[0] >= '1' && w[0] <= '9' {
			words[i] = digits[w[0]-'1']
		}
	}
	return words
}

// Align 把各分段的文字与识别词逐词对上，给出每段的音频区间。duration 用于封顶。
func Align(segments []string, heard []Word, duration float64) ChapterAlignment {
	var bookTok []string
	var owner []int
	segStart := make([]int, len(segments))
	for k, s := range segments {
		segStart[k] = len(bookTok)
		for _, t := range tokens(s) {
			bookTok = append(bookTok, t)
			owner = append(owner, k)
		}
	}
	var heardTok []string
	var heardAt []Word
	for _, w := range heard {
		for _, t := range tokens(w.Text) {
			heardTok = append(heardTok, t)
			heardAt = append(heardAt, w)
		}
	}
	b2w := lcsPairs(bookTok, heardTok)

	n := len(segments)
	first := make([]float64, n)
	last := make([]float64, n)
	has := make([]bool, n)
	clips := make([]Clip, n)
	for i, k := range owner {
		clips[k].Words++
		j, ok := b2w[i]
		if !ok {
			continue
		}
		clips[k].Matched++
		if !has[k] {
			lead := i - segStart[k] // 本段开头没对上的词：往前借同样多的识别词
			first[k] = heardAt[max(0, j-lead)].Start
			has[k] = true
		}
		last[k] = heardAt[j].End
	}
	// 没对上的分段：夹在前后已对上的分段之间
	for k := range n {
		if has[k] {
			continue
		}
		p := 0.0
		for x := k - 1; x >= 0; x-- {
			if has[x] {
				p = last[x]
				break
			}
		}
		q := p
		for x := k + 1; x < n; x++ {
			if has[x] {
				q = first[x]
				break
			}
		}
		first[k], last[k] = p, max(p, q)
	}
	for k := 1; k < n; k++ {
		first[k] = max(first[k], first[k-1])
	}
	for k := range n {
		var e float64
		if k+1 < n {
			nb := first[k+1]
			if nb-last[k] < maxJoinGap {
				e = nb
			} else {
				e = last[k] + endPad
			}
		} else {
			e = last[k] + tailPad
		}
		if duration > 0 {
			e = min(e, duration)
		}
		e = max(e, first[k]+minClip)
		clips[k].Begin, clips[k].End = first[k], e
	}
	return ChapterAlignment{Clips: clips, BookWords: len(bookTok), Heard: len(heardTok), Matched: len(b2w)}
}

// maxEditDistance 封顶差分轮数。正常有声书一章的差异只有几百；超过这个数基本是
// 音频和章节对错了，直接按「没对上」处理，避免回溯表占用过多内存。
const maxEditDistance = 3000

// lcsPairs 用 Myers 差分求最长公共子序列，返回 a 下标 → b 下标的配对。
func lcsPairs(a, b []string) map[int]int {
	n, m := len(a), len(b)
	pairs := make(map[int]int)
	if n == 0 || m == 0 {
		return pairs
	}
	maxD := min(n+m, maxEditDistance)
	offset := maxD + 1
	v := make([]int, 2*maxD+3)
	// trace[d] 保存第 d 轮开始前 v 在 k∈[-(d-1), d-1] 的值
	var trace [][]int
	found := false
	for d := 0; d <= maxD && !found; d++ {
		if d == 0 {
			trace = append(trace, nil)
		} else {
			trace = append(trace, append([]int(nil), v[offset-(d-1):offset+d]...))
		}
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				found = true
				break
			}
		}
	}
	if !found {
		return pairs
	}
	x, y := n, m
	for d := len(trace) - 1; d > 0; d-- {
		vp := func(k int) int { return trace[d][k+d-1] }
		k := x - y
		prevK := k - 1
		if k == -d || (k != d && vp(k-1) < vp(k+1)) {
			prevK = k + 1
		}
		prevX := vp(prevK)
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
			pairs[x] = y
		}
		x, y = prevX, prevY
	}
	for x > 0 && y > 0 {
		x--
		y--
		pairs[x] = y
	}
	return pairs
}
