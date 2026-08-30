// pyregex.go 是一个覆盖本 capability 所需语法子集的微型回溯正则引擎。
//
// scripts/epub3_conversion/core.py 的注释/弹注迁移正则大量使用 Go RE2
// 不支持的特性：反向引用（(?P=num)、\1）、前瞻/后顾断言、惰性量词。
// 为字节级复刻 Python re 的左最先匹配与回溯语义，这里按
// 「续延传递（CPS）回溯」实现，特性面：
//
//   - 字面量（re.I 折叠）、`.`（re.S）、字符类 [..]（含取反/区间/\d\w\s）
//   - \s \S \d \D \w \W \b \B（str 模式的 Unicode 语义）
//   - 分组 (...)、(?:...)、(?P<name>...)、反向引用 \1 与 (?P=name)
//   - 前瞻 (?=...) (?!...)、定宽后顾 (?<!...)
//   - 量词 * + ?（含惰性变体；简单子节点迭代展开，避免深递归）
//   - ^ 与 $（$ 匹配串尾或结尾换行前）
//
// 未用到的语法（{m,n}、条件、命名空间内嵌 flag 等）一律报错，
// 避免静默偏离 Python 语义。
package migrateepub3

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// decodeRune 是 utf8.DecodeRuneInString 的简写。
func decodeRune(s string) (rune, int) { return utf8.DecodeRuneInString(s) }

// ---- AST ----

type reNode interface {
	match(m *reMatcher, pos int, cont func(int) bool) bool
}

type reSeq []reNode

func (s reSeq) match(m *reMatcher, pos int, cont func(int) bool) bool {
	if len(s) == 0 {
		return cont(pos)
	}
	return s[0].match(m, pos, func(np int) bool {
		return (reSeq(s[1:])).match(m, np, cont)
	})
}

type reAlt struct {
	options []reNode
}

func (a *reAlt) match(m *reMatcher, pos int, cont func(int) bool) bool {
	for _, opt := range a.options {
		if opt.match(m, pos, cont) {
			return true
		}
	}
	return false
}

type reLit struct {
	r    rune
	fold bool
}

func (l *reLit) match(m *reMatcher, pos int, cont func(int) bool) bool {
	if pos >= len(m.runes) {
		return false
	}
	if reRuneEqual(m.runes[pos], l.r, l.fold) {
		return cont(pos + 1)
	}
	return false
}

type reAny struct{ dotAll bool }

func (a *reAny) match(m *reMatcher, pos int, cont func(int) bool) bool {
	if pos >= len(m.runes) {
		return false
	}
	if !a.dotAll && m.runes[pos] == '\n' {
		return false
	}
	return cont(pos + 1)
}

// reClassItem 是字符类的一个成员。
type reClassItem struct {
	lo, hi rune     // 区间（lo==hi 表示单字符）
	kind   int      // 0=区间 1=\d 2=\s 3=\w
	negIn  bool     // \D \S \W（类内取反）
}

type reClass struct {
	items  []reClassItem
	negate bool
	fold   bool
}

func (c *reClass) contains(r rune) bool {
	in := false
	check := func(x rune) {
		for _, it := range c.items {
			if it.kind == 0 {
				if x >= it.lo && x <= it.hi {
					in = true
				}
			} else {
				matched := false
				switch it.kind {
				case 1:
					matched = unicode.IsDigit(x)
				case 2:
					matched = pyIsSpace(x)
				case 3:
					matched = reIsWord(x)
				}
				if matched != it.negIn {
					in = true
				}
			}
		}
	}
	if c.fold {
		f := r
		for {
			check(f)
			f = unicode.SimpleFold(f)
			if f == r {
				break
			}
		}
	} else {
		check(r)
	}
	return in != c.negate
}

func (c *reClass) match(m *reMatcher, pos int, cont func(int) bool) bool {
	if pos >= len(m.runes) {
		return false
	}
	if c.contains(m.runes[pos]) {
		return cont(pos + 1)
	}
	return false
}

type reBoundary struct {
	negated bool
}

func (b *reBoundary) match(m *reMatcher, pos int, cont func(int) bool) bool {
	before := pos > 0 && reIsWord(m.runes[pos-1])
	after := pos < len(m.runes) && reIsWord(m.runes[pos])
	if (before != after) != b.negated {
		return cont(pos)
	}
	return false
}

type reBOL struct{}

func (reBOL) match(m *reMatcher, pos int, cont func(int) bool) bool {
	if pos == 0 {
		return cont(pos)
	}
	return false
}

type reEOL struct{}

func (reEOL) match(m *reMatcher, pos int, cont func(int) bool) bool {
	// Python $：串尾，或结尾换行之前。
	if pos == len(m.runes) || (pos == len(m.runes)-1 && m.runes[pos] == '\n') {
		return cont(pos)
	}
	return false
}

type reGroup struct {
	index int // -1 = 非捕获
	sub   reNode
}

func (g *reGroup) match(m *reMatcher, pos int, cont func(int) bool) bool {
	if g.index < 0 {
		return g.sub.match(m, pos, cont)
	}
	old := m.groups[g.index]
	ok := g.sub.match(m, pos, func(end int) bool {
		prev := m.groups[g.index]
		m.groups[g.index] = [2]int{pos, end}
		if cont(end) {
			return true
		}
		m.groups[g.index] = prev
		return false
	})
	if !ok {
		m.groups[g.index] = old
	}
	return ok
}

type reBackref struct {
	index int
	fold  bool
}

func (b *reBackref) match(m *reMatcher, pos int, cont func(int) bool) bool {
	span := m.groups[b.index]
	if span[0] < 0 {
		return false // 未捕获组的反向引用不匹配
	}
	n := span[1] - span[0]
	if pos+n > len(m.runes) {
		return false
	}
	for i := 0; i < n; i++ {
		if !reRuneEqual(m.runes[pos+i], m.runes[span[0]+i], b.fold) {
			return false
		}
	}
	return cont(pos + n)
}

type reLook struct {
	negated  bool
	behind   bool
	sub      reNode
}

func (l *reLook) match(m *reMatcher, pos int, cont func(int) bool) bool {
	if l.behind {
		// 定宽后顾：从 pos-width 起锚定匹配，要求恰好终止于 pos。
		width := l.fixedWidth()
		if width < 0 || pos-width < 0 {
			if l.negated {
				return cont(pos)
			}
			return false
		}
		saved := append([][2]int(nil), m.groups...)
		start := pos - width
		matched := l.sub.match(m, start, func(end int) bool { return end == pos })
		if l.negated {
			m.groups = saved
			if matched {
				return false
			}
			return cont(pos)
		}
		if !matched {
			m.groups = saved
		}
		return matched && cont(pos)
	}
	if l.negated {
		saved := append([][2]int(nil), m.groups...)
		matched := l.sub.match(m, pos, func(int) bool { return true })
		m.groups = saved
		if matched {
			return false
		}
		return cont(pos)
	}
	// 正向前瞻：断言的每种匹配方式都要给续延一次机会（零宽）。
	return l.sub.match(m, pos, func(int) bool { return cont(pos) })
}

func (l *reLook) fixedWidth() int {
	return nodeFixedWidth(l.sub)
}

func nodeFixedWidth(n reNode) int {
	switch v := n.(type) {
	case reSeq:
		total := 0
		for _, x := range v {
			w := nodeFixedWidth(x)
			if w < 0 {
				return -1
			}
			total += w
		}
		return total
	case *reLit, *reAny, *reClass:
		return 1
	case *reGroup:
		return nodeFixedWidth(v.sub)
	}
	return -1
}

type reRepeat struct {
	sub  reNode
	min  int
	max  int // -1 = 无限
	lazy bool
	// simple 子节点（lit/any/class）每次恰好消费一个 rune，
	// 用迭代展开避免对长文本产生深递归。
	simple reNode
}

func (r *reRepeat) match(m *reMatcher, pos int, cont func(int) bool) bool {
	if r.simple != nil {
		return r.matchSimple(m, pos, cont)
	}
	if r.lazy {
		var rec func(p, count int) bool
		rec = func(p, count int) bool {
			if count >= r.min && cont(p) {
				return true
			}
			if r.max >= 0 && count >= r.max {
				return false
			}
			return r.sub.match(m, p, func(np int) bool {
				if np == p {
					return false // 空迭代不再展开
				}
				return rec(np, count+1)
			})
		}
		return rec(pos, 0)
	}
	var rec func(p, count int) bool
	rec = func(p, count int) bool {
		if r.max < 0 || count < r.max {
			if r.sub.match(m, p, func(np int) bool {
				if np == p {
					return false
				}
				return rec(np, count+1)
			}) {
				return true
			}
		}
		if count >= r.min {
			return cont(p)
		}
		return false
	}
	return rec(pos, 0)
}

func (r *reRepeat) matchSimple(m *reMatcher, pos int, cont func(int) bool) bool {
	limit := len(m.runes) - pos
	if r.max >= 0 && r.max < limit {
		limit = r.max
	}
	count := 0
	for count < limit {
		if !r.simple.match(m, pos+count, func(int) bool { return true }) {
			break
		}
		count++
	}
	if count < r.min {
		return false
	}
	if r.lazy {
		for c := r.min; c <= count; c++ {
			if cont(pos + c) {
				return true
			}
		}
		return false
	}
	for c := count; c >= r.min; c-- {
		if cont(pos + c) {
			return true
		}
	}
	return false
}

func reIsWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

func reRuneEqual(a, b rune, fold bool) bool {
	if a == b {
		return true
	}
	if !fold {
		return false
	}
	f := a
	for {
		f = unicode.SimpleFold(f)
		if f == b {
			return true
		}
		if f == a {
			return false
		}
	}
}

// ---- 编译 ----

type reParser struct {
	pat     string
	pos     int
	fold    bool
	dotAll  bool
	nGroups int
	named   map[string]int
}

func mustCompilePy(pattern string, fold, dotAll bool) *pyRegexp {
	re, err := compilePy(pattern, fold, dotAll)
	if err != nil {
		panic("pyregex: " + err.Error() + " in pattern: " + pattern)
	}
	return re
}

type pyRegexp struct {
	root   reNode
	named  map[string]int
	nGroup int
}

func compilePy(pattern string, fold, dotAll bool) (*pyRegexp, error) {
	p := &reParser{pat: pattern, fold: fold, dotAll: dotAll, named: map[string]int{}}
	node, err := p.parseAlt()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.pat) {
		return nil, fmt.Errorf("unexpected %q at %d", p.pat[p.pos], p.pos)
	}
	// 用合成捕获组 0 包住整棵树：reGroup.match 成功时写 groups[0] =
	// 整体匹配 span —— Python re 的 m.span(0) 语义。没有这层，
	// spans[0] 恒为 (-1,-1)，byteStart(0)/subTemplate 全部失效。
	root := &reGroup{index: 0, sub: node}
	return &pyRegexp{root: root, named: p.named, nGroup: p.nGroups}, nil
}

func (p *reParser) peek() (byte, bool) {
	if p.pos < len(p.pat) {
		return p.pat[p.pos], true
	}
	return 0, false
}

func (p *reParser) parseAlt() (reNode, error) {
	var options []reNode
	for {
		seq, err := p.parseSeq()
		if err != nil {
			return nil, err
		}
		options = append(options, seq)
		if c, ok := p.peek(); ok && c == '|' {
			p.pos++
			continue
		}
		break
	}
	if len(options) == 1 {
		return options[0], nil
	}
	return &reAlt{options: options}, nil
}

func (p *reParser) parseSeq() (reNode, error) {
	var nodes []reNode
	for {
		c, ok := p.peek()
		if !ok || c == '|' || c == ')' {
			break
		}
		atom, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		atom, err = p.parseQuant(atom)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, atom)
	}
	return reSeq(nodes), nil
}

func (p *reParser) parseQuant(atom reNode) (reNode, error) {
	c, ok := p.peek()
	if !ok {
		return atom, nil
	}
	var min, max int
	switch c {
	case '*':
		min, max = 0, -1
		p.pos++
	case '+':
		min, max = 1, -1
		p.pos++
	case '?':
		min, max = 0, 1
		p.pos++
	case '{':
		return nil, fmt.Errorf("{m,n} quantifiers unsupported")
	default:
		return atom, nil
	}
	lazy := false
	if c2, ok := p.peek(); ok && c2 == '?' {
		lazy = true
		p.pos++
	} else if c2, ok := p.peek(); ok && c2 == '+' {
		return nil, fmt.Errorf("possessive quantifiers unsupported")
	}
	return &reRepeat{sub: atom, min: min, max: max, lazy: lazy, simple: simpleNode(atom)}, nil
}

// simpleNode 报告该原子是否每次匹配恰好消费一个 rune。
func simpleNode(n reNode) reNode {
	switch n.(type) {
	case *reLit, *reAny, *reClass:
		return n
	}
	return nil
}

func (p *reParser) parseAtom() (reNode, error) {
	c, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("unexpected end of pattern")
	}
	switch c {
	case '(':
		return p.parseGroup()
	case '[':
		return p.parseClass()
	case '.':
		p.pos++
		return &reAny{dotAll: p.dotAll}, nil
	case '^':
		p.pos++
		return reBOL{}, nil
	case '$':
		p.pos++
		return reEOL{}, nil
	case '\\':
		return p.parseEscape()
	case '*', '+', '?':
		return nil, fmt.Errorf("quantifier without atom at %d", p.pos)
	case ')':
		return nil, fmt.Errorf("unbalanced paren at %d", p.pos)
	default:
		r := p.nextRune()
		return &reLit{r: r, fold: p.fold}, nil
	}
}

func (p *reParser) nextRune() rune {
	r, size := decodeRune(p.pat[p.pos:])
	p.pos += size
	return r
}

func (p *reParser) parseGroup() (reNode, error) {
	p.pos++ // '('
	if strings.HasPrefix(p.pat[p.pos:], "?") {
		p.pos++
		c, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("truncated group")
		}
		switch c {
		case ':':
			p.pos++
			sub, err := p.parseAlt()
			if err != nil {
				return nil, err
			}
			if c2, ok := p.peek(); !ok || c2 != ')' {
				return nil, fmt.Errorf("missing ) for (?:")
			}
			p.pos++
			return &reGroup{index: -1, sub: sub}, nil
		case '=':
			p.pos++
			sub, err := p.parseAlt()
			if err != nil {
				return nil, err
			}
			if c2, ok := p.peek(); !ok || c2 != ')' {
				return nil, fmt.Errorf("missing ) for (?=")
			}
			p.pos++
			return &reLook{sub: sub}, nil
		case '!':
			p.pos++
			sub, err := p.parseAlt()
			if err != nil {
				return nil, err
			}
			if c2, ok := p.peek(); !ok || c2 != ')' {
				return nil, fmt.Errorf("missing ) for (?!")
			}
			p.pos++
			return &reLook{negated: true, sub: sub}, nil
		case '<':
			p.pos++
			c2, ok := p.peek()
			if !ok || c2 != '!' {
				return nil, fmt.Errorf("only (?<!...) lookbehind supported")
			}
			p.pos++
			sub, err := p.parseAlt()
			if err != nil {
				return nil, err
			}
			if c2, ok := p.peek(); !ok || c2 != ')' {
				return nil, fmt.Errorf("missing ) for (?<!")
			}
			p.pos++
			return &reLook{negated: true, behind: true, sub: sub}, nil
		case 'P':
			p.pos++
			c2, ok := p.peek()
			if !ok {
				return nil, fmt.Errorf("truncated (?P")
			}
			if c2 == '=' {
				p.pos++
				name := p.readIdent()
				idx, ok := p.named[name]
				if !ok {
					return nil, fmt.Errorf("unknown group name %q", name)
				}
				if c3, ok := p.peek(); !ok || c3 != ')' {
					return nil, fmt.Errorf("missing ) for (?P=")
				}
				p.pos++
				return &reBackref{index: idx, fold: p.fold}, nil
			}
			if c2 != '<' {
				return nil, fmt.Errorf("unsupported (?P%c", c2)
			}
			p.pos++
			name := p.readIdent()
			if c3, ok := p.peek(); !ok || c3 != '>' {
				return nil, fmt.Errorf("missing > for (?P<")
			}
			p.pos++
			p.nGroups++
			p.named[name] = p.nGroups
			sub, err := p.parseAlt()
			if err != nil {
				return nil, err
			}
			if c3, ok := p.peek(); !ok || c3 != ')' {
				return nil, fmt.Errorf("missing ) for group")
			}
			p.pos++
			return &reGroup{index: p.nGroups, sub: sub}, nil
		default:
			return nil, fmt.Errorf("unsupported group (?%c", c)
		}
	}
	p.nGroups++
	idx := p.nGroups
	sub, err := p.parseAlt()
	if err != nil {
		return nil, err
	}
	if c2, ok := p.peek(); !ok || c2 != ')' {
		return nil, fmt.Errorf("missing ) for group")
	}
	p.pos++
	return &reGroup{index: idx, sub: sub}, nil
}

func (p *reParser) readIdent() string {
	start := p.pos
	for p.pos < len(p.pat) && (isASCIILetter(p.pat[p.pos]) || (p.pat[p.pos] >= '0' && p.pat[p.pos] <= '9') || p.pat[p.pos] == '_') {
		p.pos++
	}
	return p.pat[start:p.pos]
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func (p *reParser) parseEscape() (reNode, error) {
	p.pos++ // '\'
	c, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("trailing backslash")
	}
	p.pos++
	switch c {
	case 'd', 'D':
		return &reClass{items: []reClassItem{{kind: 1, negIn: c == 'D'}}, fold: p.fold}, nil
	case 's', 'S':
		return &reClass{items: []reClassItem{{kind: 2, negIn: c == 'S'}}, fold: p.fold}, nil
	case 'w', 'W':
		return &reClass{items: []reClassItem{{kind: 3, negIn: c == 'W'}}, fold: p.fold}, nil
	case 'b':
		return &reBoundary{}, nil
	case 'B':
		return &reBoundary{negated: true}, nil
	case 'n':
		return &reLit{r: '\n'}, nil
	case 't':
		return &reLit{r: '\t'}, nil
	case 'r':
		return &reLit{r: '\r'}, nil
	case 'f':
		return &reLit{r: '\f'}, nil
	case 'v':
		return &reLit{r: '\v'}, nil
	case 'a':
		return &reLit{r: '\a'}, nil
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return &reBackref{index: int(c - '0'), fold: p.fold}, nil
	default:
		return &reLit{r: rune(c), fold: p.fold}, nil
	}
}

func (p *reParser) parseClass() (reNode, error) {
	p.pos++ // '['
	cls := &reClass{fold: p.fold}
	if c, ok := p.peek(); ok && c == '^' {
		cls.negate = true
		p.pos++
	}
	first := true
	for {
		c, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("unterminated class")
		}
		if c == ']' && !first {
			p.pos++
			break
		}
		first = false
		var lo rune
		if c == '\\' {
			p.pos++
			e, ok := p.peek()
			if !ok {
				return nil, fmt.Errorf("trailing backslash in class")
			}
			p.pos++
			switch e {
			case 'd', 'D', 's', 'S', 'w', 'W':
				kind := 0
				negIn := false
				switch e {
				case 'd', 'D':
					kind = 1
				case 's', 'S':
					kind = 2
				case 'w', 'W':
					kind = 3
				}
				negIn = e == 'D' || e == 'S' || e == 'W'
				cls.items = append(cls.items, reClassItem{kind: kind, negIn: negIn})
				continue
			case 'n':
				lo = '\n'
			case 't':
				lo = '\t'
			case 'r':
				lo = '\r'
			case 'f':
				lo = '\f'
			case 'v':
				lo = '\v'
			default:
				lo = rune(e)
			}
		} else {
			lo = p.nextRune()
		}
		// 区间？
		if cNext, ok := p.peek(); ok && cNext == '-' && p.pos+1 < len(p.pat) && p.pat[p.pos+1] != ']' {
			p.pos++ // '-'
			hiChar, ok := p.peek()
			if !ok {
				return nil, fmt.Errorf("unterminated range")
			}
			var hi rune
			if hiChar == '\\' {
				p.pos++
				e, ok := p.peek()
				if !ok {
					return nil, fmt.Errorf("trailing backslash in class")
				}
				p.pos++
				hi = rune(escapeCharByte(e))
			} else {
				hi = p.nextRune()
			}
			cls.items = append(cls.items, reClassItem{lo: lo, hi: hi})
			continue
		}
		cls.items = append(cls.items, reClassItem{lo: lo, hi: lo})
	}
	return cls, nil
}

func escapeCharByte(e byte) byte {
	switch e {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case 'f':
		return '\f'
	case 'v':
		return '\v'
	}
	return e
}

// ---- 匹配入口 ----

type reMatcher struct {
	runes  []rune
	groups [][2]int
}

type pyMatch struct {
	re    *pyRegexp
	runes []rune
	off   []int // rune index → byte offset（len+1 项）
	spans [][2]int
}

func (m *pyMatch) span(i int) [2]int {
	if i < len(m.spans) {
		return m.spans[i]
	}
	return [2]int{-1, -1}
}

// groupI 返回第 i 组文本（0 = 整体）。
func (m *pyMatch) groupI(i int) string {
	sp := m.span(i)
	if sp[0] < 0 {
		return ""
	}
	return string(m.runes[sp[0]:sp[1]])
}

func (m *pyMatch) hasGroupI(i int) bool {
	return m.span(i)[0] >= 0
}

func (m *pyMatch) groupName(name string) string {
	i, ok := m.re.named[name]
	if !ok {
		return ""
	}
	return m.groupI(i)
}

func (m *pyMatch) hasGroup(name string) bool {
	i, ok := m.re.named[name]
	if !ok {
		return false
	}
	return m.hasGroupI(i)
}

// byteStart / byteEnd 返回组在原文中的字节区间（用于前缀切片）。
func (m *pyMatch) byteStart(i int) int {
	sp := m.span(i)
	if sp[0] < 0 {
		return -1
	}
	return m.off[sp[0]]
}

func (m *pyMatch) byteEnd(i int) int {
	sp := m.span(i)
	if sp[0] < 0 {
		return -1
	}
	return m.off[sp[1]]
}

func byteOffsets(s string) []int {
	off := make([]int, utf8len(s)+1)
	i, r := 0, 0
	for i < len(s) {
		off[r] = i
		_, size := decodeRune(s[i:])
		i += size
		r++
	}
	off[r] = len(s)
	return off
}

func utf8len(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// searchFrom 在 runes[pos:] 里找最左匹配（start >= pos）。
func (re *pyRegexp) searchFrom(runes []rune, pos int) (*pyMatch, bool) {
	for start := pos; start <= len(runes); start++ {
		groups := make([][2]int, re.nGroup+1)
		for i := range groups {
			groups[i] = [2]int{-1, -1}
		}
		m := &reMatcher{runes: runes, groups: groups}
		if re.root.match(m, start, func(int) bool { return true }) {
			return &pyMatch{re: re, runes: runes, spans: m.groups}, true
		}
	}
	return nil, false
}

// search 复刻 re.search。
func (re *pyRegexp) search(text string) (*pyMatch, bool) {
	return re.searchRunes([]rune(text), 0)
}

// hasMatch 报告是否存在匹配。
func (re *pyRegexp) hasMatch(text string) bool {
	_, ok := re.search(text)
	return ok
}

func (re *pyRegexp) searchRunes(runes []rune, pos int) (*pyMatch, bool) {
	m, ok := re.searchFrom(runes, pos)
	if !ok {
		return nil, false
	}
	m.off = byteOffsets(string(runes))
	return m, true
}

// findAll 复刻 re.finditer（非重叠；空匹配前进一符）。
func (re *pyRegexp) findAll(text string) []*pyMatch {
	runes := []rune(text)
	off := byteOffsets(text)
	var out []*pyMatch
	pos := 0
	for pos <= len(runes) {
		m, ok := re.searchFrom(runes, pos)
		if !ok {
			break
		}
		m.off = off
		out = append(out, m)
		if m.spans[0][1] == m.spans[0][0] {
			pos = m.spans[0][1] + 1
		} else {
			pos = m.spans[0][1]
		}
	}
	return out
}

// subFunc 复刻 re.sub(pattern, repl_function, text, count)。
func (re *pyRegexp) subFunc(text string, count int, repl func(*pyMatch) string) (string, int) {
	runes := []rune(text)
	off := byteOffsets(text)
	var b strings.Builder
	pos, last, n := 0, 0, 0
	for count <= 0 || n < count {
		if pos > len(runes) {
			break
		}
		m, ok := re.searchFrom(runes, pos)
		if !ok {
			break
		}
		if len(m.spans) == 0 || m.spans[0][0] < 0 || m.spans[0][1] < m.spans[0][0] {
			break // 无整体匹配 span：防御性退出，避免 [-1] 索引 panic
		}
		m.off = off
		b.WriteString(text[off[last]:off[m.spans[0][0]]])
		b.WriteString(repl(m))
		n++
		last = m.spans[0][1]
		if m.spans[0][1] == m.spans[0][0] {
			pos = last + 1
		} else {
			pos = last
		}
	}
	if last <= len(runes) {
		b.WriteString(text[off[last]:])
	}
	return b.String(), n
}

// subTemplate 复刻 re.sub(pattern, template, text, count)，模板支持 \N 与 \\。
func (re *pyRegexp) subTemplate(text, tmpl string, count int) (string, int) {
	return re.subFunc(text, count, func(m *pyMatch) string {
		return expandTemplate(tmpl, m)
	})
}

func expandTemplate(tmpl string, m *pyMatch) string {
	var b strings.Builder
	i := 0
	for i < len(tmpl) {
		c := tmpl[i]
		if c == '\\' && i+1 < len(tmpl) {
			n := tmpl[i+1]
			if n >= '0' && n <= '9' {
				b.WriteString(m.groupI(int(n - '0')))
				i += 2
				continue
			}
			if n == '\\' {
				b.WriteByte('\\')
				i += 2
				continue
			}
			b.WriteByte('\\')
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}
