// xmlmini.go 提供与 Python ElementTree 字节兼容的最小 XML 模型。
// 从 internal/caps/structure_normalize/xmlmini.go 复制并按本 capability
// 的需要裁剪（caps 之间禁止 import，故整份私有拷贝）：
//
//   - 解析（含命名空间、实体、EOL 归一）→ 树上做与 Python 相同的变更
//     → 按 ET.tostring 的确切规则序列化。与 structure_normalize 的差别
//     在命名空间前缀注册表：epub_lib.py import 期最后一次
//     register_namespace("opf", OPF_URI) 会把 OPF 绑定到 "opf" 前缀
//     （空前缀注册被同 URI 的后注册抹掉），因此 OPF 输出是
//     <opf:package ...>；而 XHTML 排版序列化期间 "" → xhtml。
//   - posixpath 的 join/normpath/dirname/basename/splitext/relpath 语义。
package migrateepub3

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
)

// XML 命名空间常量（Python 侧 XML_URI）。
const xmlNamespaceURI = "http://www.w3.org/XML/1998/namespace"

// ---- 树模型 ----

type xmlAttr struct {
	ns    string // "" 表示无命名空间属性
	name  string // local 名
	value string
}

type xmlElem struct {
	ns       string // "" 表示不在任何命名空间
	name     string // local 名
	attrs    []xmlAttr
	text     string
	tail     string
	children []*xmlElem
}

// newElem 复刻 ET.Element(q(uri, name), attrs)：属性按字典序即插入序。
func newElem(ns, name string, attrs ...xmlAttr) *xmlElem {
	return &xmlElem{ns: ns, name: name, attrs: attrs}
}

func (e *xmlElem) childByTag(ns, local string) *xmlElem {
	for _, c := range e.children {
		if c.ns == ns && c.name == local {
			return c
		}
	}
	return nil
}

func (e *xmlElem) childrenByTag(ns, local string) []*xmlElem {
	var out []*xmlElem
	for _, c := range e.children {
		if c.ns == ns && c.name == local {
			out = append(out, c)
		}
	}
	return out
}

func (e *xmlElem) descendantsByTag(ns, local string) []*xmlElem {
	var out []*xmlElem
	for _, d := range iterAll(e) {
		if d != e && d.ns == ns && d.name == local {
			out = append(out, d)
		}
	}
	return out
}

// getAttr 返回无命名空间属性的值（对齐 elem.attrib.get(name)）。
func (e *xmlElem) getAttr(local string) (string, bool) {
	for _, a := range e.attrs {
		if a.ns == "" && a.name == local {
			return a.value, true
		}
	}
	return "", false
}

// getAttrNS 返回带命名空间属性的值。
func (e *xmlElem) getAttrNS(ns, local string) (string, bool) {
	for _, a := range e.attrs {
		if a.ns == ns && a.name == local {
			return a.value, true
		}
	}
	return "", false
}

// insertChildAt 复刻 parent.insert(index, child)。
func (e *xmlElem) insertChildAt(index int, child *xmlElem) {
	if index < 0 || index > len(e.children) {
		e.children = append(e.children, child)
		return
	}
	e.children = append(e.children, nil)
	copy(e.children[index+1:], e.children[index:])
	e.children[index] = child
}

func (e *xmlElem) attrOr(local, fallback string) string {
	if v, ok := e.getAttr(local); ok {
		return v
	}
	return fallback
}

// setAttr 复刻 Element.set：已有键原位更新（保持属性顺序），新键追加。
func (e *xmlElem) setAttr(ns, name, value string) {
	for i := range e.attrs {
		if e.attrs[i].ns == ns && e.attrs[i].name == name {
			e.attrs[i].value = value
			return
		}
	}
	e.attrs = append(e.attrs, xmlAttr{ns: ns, name: name, value: value})
}

func (e *xmlElem) delAttr(ns, name string) {
	for i := range e.attrs {
		if e.attrs[i].ns == ns && e.attrs[i].name == name {
			e.attrs = append(e.attrs[:i:i], e.attrs[i+1:]...)
			return
		}
	}
}

// clearAttrs 复刻 elem.attrib.clear()。
func (e *xmlElem) clearAttrs() { e.attrs = nil }

func (e *xmlElem) appendChild(c *xmlElem) {
	e.children = append(e.children, c)
}

func (e *xmlElem) removeChild(target *xmlElem) {
	for i, c := range e.children {
		if c == target {
			e.children = append(e.children[:i:i], e.children[i+1:]...)
			return
		}
	}
}

func (e *xmlElem) indexOfChild(target *xmlElem) int {
	for i, c := range e.children {
		if c == target {
			return i
		}
	}
	return -1
}

// iterAll 按文档序（前序）返回 root 与全部后代，对齐 root.iter()。
func iterAll(root *xmlElem) []*xmlElem {
	var out []*xmlElem
	var walk func(e *xmlElem)
	walk = func(e *xmlElem) {
		out = append(out, e)
		for _, c := range e.children {
			walk(c)
		}
	}
	walk(root)
	return out
}

// itertext 复刻 elem.itertext()：text 与后代 text/tail 的文档序拼接。
func (e *xmlElem) itertext() string {
	var b strings.Builder
	b.WriteString(e.text)
	for _, c := range e.children {
		b.WriteString(c.itertext())
		b.WriteString(c.tail)
	}
	return b.String()
}

// ---- 解析 ----

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

func errToken() error { return &parseError{"not well-formed (invalid token)"} }

type nsScope struct {
	bindings   map[string]string // prefix → uri
	defaultURI string
	hasDefault bool
}

type xmlParser struct {
	src    string
	pos    int
	scopes []nsScope
}

func (p *xmlParser) hasPrefix(s string) bool { return strings.HasPrefix(p.src[p.pos:], s) }

func (p *xmlParser) lookupPrefix(prefix string) (string, bool) {
	for i := len(p.scopes) - 1; i >= 0; i-- {
		if uri, ok := p.scopes[i].bindings[prefix]; ok {
			return uri, true
		}
	}
	return "", false
}

func (p *xmlParser) lookupDefault() (string, bool) {
	for i := len(p.scopes) - 1; i >= 0; i-- {
		if p.scopes[i].hasDefault {
			return p.scopes[i].defaultURI, true
		}
	}
	return "", false
}

// parseXMLTree 把字节解析为 ET 形状的树（注释 / PI / DOCTYPE 一律丢弃，
// 与 ET.fromstring 一致）。输入按声明编码先行转换为 UTF-8。
func parseXMLTree(data []byte) (*xmlElem, error) {
	src, err := xmlSourceToUTF8(data)
	if err != nil {
		return nil, err
	}
	p := &xmlParser{src: src}

	// 序言：空白 / XML 声明 / 注释 / PI / DOCTYPE。
	for {
		p.skipSpace()
		switch {
		case p.pos < len(p.src) && p.src[p.pos] == '<':
			switch {
			case p.hasPrefix("<?"):
				if err := p.skipPI(); err != nil {
					return nil, err
				}
			case p.hasPrefix("<!--"):
				if err := p.skipComment(); err != nil {
					return nil, err
				}
			case p.hasPrefix("<!DOCTYPE"):
				if err := p.skipDoctype(); err != nil {
					return nil, err
				}
			default:
				goto root
			}
		case p.pos >= len(p.src):
			return nil, &parseError{"no element found"}
		default:
			return nil, errToken()
		}
	}
root:
	root, err := p.parseElement()
	if err != nil {
		return nil, err
	}
	// 根元素之后允许空白 / 注释 / PI（XML Misc）。
	for {
		p.skipSpace()
		switch {
		case p.pos >= len(p.src):
			return root, nil
		case p.hasPrefix("<!--"):
			if err := p.skipComment(); err != nil {
				return nil, err
			}
		case p.hasPrefix("<?"):
			if err := p.skipPI(); err != nil {
				return nil, err
			}
		default:
			return nil, &parseError{"junk after document element"}
		}
	}
}

func (p *xmlParser) skipSpace() {
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *xmlParser) skipPI() error {
	end := strings.Index(p.src[p.pos:], "?>")
	if end < 0 {
		return errToken()
	}
	p.pos += end + 2
	return nil
}

func (p *xmlParser) skipComment() error {
	end := strings.Index(p.src[p.pos:], "-->")
	if end < 0 {
		return errToken()
	}
	p.pos += end + 3
	return nil
}

func (p *xmlParser) skipDoctype() error {
	depth := 0
	var quote byte
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			p.pos++
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '[':
			depth++
		case ']':
			depth--
		case '>':
			if depth == 0 {
				p.pos++
				return nil
			}
		}
		p.pos++
	}
	return errToken()
}

func (p *xmlParser) parseNCName() (string, error) {
	start := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '=' ||
			c == '>' || c == '<' || c == '/' || c == '"' || c == '\'' {
			break
		}
		p.pos++
	}
	if p.pos == start {
		return "", errToken()
	}
	return p.src[start:p.pos], nil
}

func (p *xmlParser) resolveRawName(raw string, scope *nsScope, isAttr bool) (string, string, error) {
	if i := strings.IndexByte(raw, ':'); i >= 0 {
		prefix, local := raw[:i], raw[i+1:]
		switch prefix {
		case "xml":
			return xmlNamespaceURI, local, nil
		case "xmlns":
			return "", "", &parseError{"reserved prefix xmlns"}
		}
		if scope != nil {
			if uri, ok := scope.bindings[prefix]; ok {
				return uri, local, nil
			}
		}
		if uri, ok := p.lookupPrefix(prefix); ok {
			return uri, local, nil
		}
		return "", "", &parseError{"unbound prefix"}
	}
	if isAttr {
		return "", raw, nil
	}
	if scope != nil && scope.hasDefault {
		return scope.defaultURI, raw, nil
	}
	if uri, ok := p.lookupDefault(); ok {
		return uri, raw, nil
	}
	return "", raw, nil
}

func (p *xmlParser) parseElement() (*xmlElem, error) {
	if p.pos >= len(p.src) || p.src[p.pos] != '<' {
		return nil, errToken()
	}
	p.pos++
	rawName, err := p.parseNCName()
	if err != nil {
		return nil, err
	}
	elem := &xmlElem{}
	scope := nsScope{}
	selfClose := false
	for {
		p.skipSpace()
		if p.hasPrefix("/>") {
			p.pos += 2
			selfClose = true
			break
		}
		if p.hasPrefix(">") {
			p.pos++
			break
		}
		attrRaw, err := p.parseNCName()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(attrRaw, "xmlns") &&
			(attrRaw == "xmlns" || strings.HasPrefix(attrRaw, "xmlns:")) {
			// 命名空间声明稍后统一处理；先跳过重复检查。
		} else {
			for _, a := range elem.attrs {
				if a.name == attrRaw {
					return nil, &parseError{"duplicate attribute"}
				}
			}
		}
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != '=' {
			return nil, errToken()
		}
		p.pos++
		p.skipSpace()
		if p.pos >= len(p.src) || (p.src[p.pos] != '"' && p.src[p.pos] != '\'') {
			return nil, errToken()
		}
		quote := p.src[p.pos]
		p.pos++
		end := strings.IndexByte(p.src[p.pos:], quote)
		if end < 0 {
			return nil, errToken()
		}
		rawValue := p.src[p.pos : p.pos+end]
		p.pos += end + 1
		switch {
		case attrRaw == "xmlns":
			scope.defaultURI = rawValue
			scope.hasDefault = true
		case strings.HasPrefix(attrRaw, "xmlns:"):
			if scope.bindings == nil {
				scope.bindings = map[string]string{}
			}
			scope.bindings[attrRaw[len("xmlns:"):]] = rawValue
		default:
			value, verr := normalizeAttrValue(rawValue)
			if verr != nil {
				return nil, verr
			}
			elem.attrs = append(elem.attrs, xmlAttr{name: attrRaw, value: value})
		}
	}
	if elem.ns, elem.name, err = p.resolveRawName(rawName, &scope, false); err != nil {
		return nil, err
	}
	for i := range elem.attrs {
		a := &elem.attrs[i]
		if a.ns, a.name, err = p.resolveRawName(a.name, &scope, true); err != nil {
			return nil, err
		}
	}
	if selfClose {
		return elem, nil
	}

	p.scopes = append(p.scopes, scope)
	defer func() { p.scopes = p.scopes[:len(p.scopes)-1] }()

	var pending strings.Builder
	var lastChild *xmlElem
	flush := func() {
		if pending.Len() == 0 {
			return
		}
		if lastChild == nil {
			elem.text += pending.String()
		} else {
			lastChild.tail += pending.String()
		}
		pending.Reset()
	}
	for {
		chunk, err := p.readTextRun()
		if err != nil {
			return nil, err
		}
		pending.WriteString(chunk)
		if p.pos >= len(p.src) {
			return nil, &parseError{"no element found"}
		}
		switch {
		case p.hasPrefix("</"):
			p.pos += 2
			endName, err := p.parseNCName()
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if p.pos >= len(p.src) || p.src[p.pos] != '>' {
				return nil, errToken()
			}
			p.pos++
			if endName != rawName {
				return nil, &parseError{"mismatched tag"}
			}
			flush()
			return elem, nil
		case p.hasPrefix("<!--"):
			if err := p.skipComment(); err != nil {
				return nil, err
			}
		case p.hasPrefix("<?"):
			if err := p.skipPI(); err != nil {
				return nil, err
			}
		case p.hasPrefix("<![CDATA["):
			p.pos += len("<![CDATA[")
			end := strings.Index(p.src[p.pos:], "]]>")
			if end < 0 {
				return nil, errToken()
			}
			pending.WriteString(p.src[p.pos : p.pos+end])
			p.pos += end + 3
		case p.hasPrefix("<!"):
			return nil, errToken()
		default:
			flush()
			child, err := p.parseElement()
			if err != nil {
				return nil, err
			}
			elem.children = append(elem.children, child)
			lastChild = child
		}
	}
}

func (p *xmlParser) readTextRun() (string, error) {
	end := strings.IndexByte(p.src[p.pos:], '<')
	var raw string
	if end < 0 {
		raw = p.src[p.pos:]
		p.pos = len(p.src)
	} else {
		raw = p.src[p.pos : p.pos+end]
		p.pos += end
	}
	return normalizeTextChunk(raw)
}

func normalizeTextChunk(raw string) (string, error) {
	if !strings.ContainsAny(raw, "&\r") {
		return raw, nil
	}
	var b strings.Builder
	i := 0
	for i < len(raw) {
		c := raw[i]
		switch {
		case c == '\r':
			b.WriteByte('\n')
			i++
			if i < len(raw) && raw[i] == '\n' {
				i++
			}
		case c == '&':
			rep, n, err := decodeEntity(raw[i:])
			if err != nil {
				return "", err
			}
			b.WriteString(rep)
			i += n
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), nil
}

func normalizeAttrValue(raw string) (string, error) {
	if !strings.ContainsAny(raw, "&\r\n\t") {
		return raw, nil
	}
	var b strings.Builder
	i := 0
	for i < len(raw) {
		c := raw[i]
		switch {
		case c == '\r':
			b.WriteByte(' ')
			i++
			if i < len(raw) && raw[i] == '\n' {
				i++
			}
		case c == '\n' || c == '\t':
			b.WriteByte(' ')
			i++
		case c == '&':
			rep, n, err := decodeEntity(raw[i:])
			if err != nil {
				return "", err
			}
			b.WriteString(rep)
			i += n
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), nil
}

func decodeEntity(s string) (string, int, error) {
	if !strings.HasPrefix(s, "&") {
		return "", 0, errToken()
	}
	for _, ent := range [...]struct {
		name string
		repl string
	}{
		{"&amp;", "&"}, {"&lt;", "<"}, {"&gt;", ">"},
		{"&apos;", "'"}, {"&quot;", `"`},
	} {
		if strings.HasPrefix(s, ent.name) {
			return ent.repl, len(ent.name), nil
		}
	}
	if strings.HasPrefix(s, "&#x") || strings.HasPrefix(s, "&#X") {
		end := strings.IndexByte(s, ';')
		if end < 3 {
			return "", 0, errToken()
		}
		v, err := strconv.ParseUint(s[3:end], 16, 32)
		if err != nil || !validXMLRune(rune(v)) {
			return "", 0, errToken()
		}
		return string(rune(v)), end + 1, nil
	}
	if strings.HasPrefix(s, "&#") {
		end := strings.IndexByte(s, ';')
		if end < 3 {
			return "", 0, errToken()
		}
		v, err := strconv.ParseUint(s[2:end], 10, 32)
		if err != nil || !validXMLRune(rune(v)) {
			return "", 0, errToken()
		}
		return string(rune(v)), end + 1, nil
	}
	return "", 0, &parseError{"undefined entity"}
}

func validXMLRune(r rune) bool {
	return r != 0 && r <= utf8.MaxRune && !(r >= 0xD800 && r <= 0xDFFF)
}

// ---- 序列化（复刻 ET.tostring 的确切输出） ----

type nsDecl struct {
	prefix string
	uri    string
}

// serializeTree 复刻 ET.tostring：qname 前缀查 prefixes（uri → prefix），
// 不在表内的 URI 按 "ns%d"（当前已登记数量）生成；声明全部落在根元素、
// 按前缀稳定排序；空前缀输出 xmlns="..."；空元素 <tag />；
// 属性转义 & < > " \r \n \t，文本/tail 只转义 & < >。
// withDecl 为 true 时输出 <?xml version='1.0' encoding='utf-8'?>\n 前缀。
func serializeTree(root *xmlElem, prefixes map[string]string, withDecl bool) []byte {
	used := map[string]string{} // uri → prefix
	var decls []nsDecl
	qname := func(ns, name string) string {
		if ns == "" {
			return name
		}
		prefix, ok := used[ns]
		if !ok {
			var known bool
			prefix, known = prefixes[ns] // 不能用 := —— 会遮蔽外层 prefix
			if !known {
				prefix = fmt.Sprintf("ns%d", len(used))
			}
			if prefix != "xml" { // xml 前缀不声明（与 CPython 一致）
				used[ns] = prefix
				decls = append(decls, nsDecl{prefix: prefix, uri: ns})
			}
		}
		if prefix == "" {
			return name
		}
		return prefix + ":" + name
	}
	// 收集命名空间：文档序（前序），元素 tag 先于其属性。
	var walk func(e *xmlElem)
	walk = func(e *xmlElem) {
		qname(e.ns, e.name)
		for _, a := range e.attrs {
			qname(a.ns, a.name)
		}
		for _, c := range e.children {
			walk(c)
		}
	}
	walk(root)

	sortStableByPrefix(decls)

	var b strings.Builder
	if withDecl {
		b.WriteString("<?xml version='1.0' encoding='utf-8'?>\n")
	}
	writeXMLElement(&b, root, qname, decls, true)
	return []byte(b.String())
}

func sortStableByPrefix(decls []nsDecl) {
	for i := 1; i < len(decls); i++ {
		for j := i; j > 0 && decls[j].prefix < decls[j-1].prefix; j-- {
			decls[j], decls[j-1] = decls[j-1], decls[j]
		}
	}
}

func writeXMLElement(b *strings.Builder, e *xmlElem, qname func(string, string) string, decls []nsDecl, isRoot bool) {
	b.WriteByte('<')
	tag := qname(e.ns, e.name)
	b.WriteString(tag)
	if isRoot {
		for _, d := range decls {
			if d.prefix != "" {
				b.WriteString(` xmlns:` + d.prefix + `="` + attribEscaper.Replace(d.uri) + `"`)
			} else {
				b.WriteString(` xmlns="` + attribEscaper.Replace(d.uri) + `"`)
			}
		}
	}
	for _, a := range e.attrs {
		b.WriteByte(' ')
		b.WriteString(qname(a.ns, a.name))
		b.WriteString(`="`)
		b.WriteString(attribEscaper.Replace(a.value))
		b.WriteByte('"')
	}
	if e.text != "" || len(e.children) > 0 {
		b.WriteByte('>')
		if e.text != "" {
			b.WriteString(cdataEscaper.Replace(e.text))
		}
		for _, c := range e.children {
			writeXMLElement(b, c, qname, nil, false)
		}
		b.WriteString("</")
		b.WriteString(tag)
		b.WriteByte('>')
	} else {
		b.WriteString(" />")
	}
	if e.tail != "" {
		b.WriteString(cdataEscaper.Replace(e.tail))
	}
}

// ---- 输入编码转换 ----

func xmlSourceToUTF8(data []byte) (string, error) {
	body := data
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		body = data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		s, ok := decodeUTF16Units(data[2:], false)
		if !ok {
			return "", errToken()
		}
		return s, nil
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		s, ok := decodeUTF16Units(data[2:], true)
		if !ok {
			return "", errToken()
		}
		return s, nil
	}
	if m := xmlEncodingRe.FindSubmatch(body[:min(len(body), 256)]); m != nil {
		declared := string(m[1])
		switch strings.ToLower(declared) {
		case "utf-8", "utf8", "ascii", "us-ascii":
			// 直接按 UTF-8 解析。
		default:
			enc, err := lookupEncoding(declared)
			if err != nil {
				return "", &parseError{"unknown encoding: " + declared}
			}
			out, derr := enc.NewDecoder().Bytes(body)
			if derr != nil {
				return "", &parseError{"cannot decode document: " + derr.Error()}
			}
			body = out
		}
	}
	if !utf8.Valid(body) {
		return "", errToken()
	}
	return string(body), nil
}

// lookupEncoding 按编码名解析 x/text 编码（IANA / MIME 名）。
func lookupEncoding(name string) (encoding.Encoding, error) {
	if e, err := ianaindex.MIME.Encoding(name); err == nil && e != nil {
		return e, nil
	}
	if e, err := ianaindex.IANA.Encoding(name); err == nil && e != nil {
		return e, nil
	}
	return nil, fmt.Errorf("unknown encoding %q", name)
}

func decodeUTF16Units(b []byte, bigEndian bool) (string, bool) {
	if len(b)%2 != 0 {
		return "", false
	}
	var sb strings.Builder
	for i := 0; i < len(b); i += 2 {
		var u rune
		if bigEndian {
			u = rune(b[i])<<8 | rune(b[i+1])
		} else {
			u = rune(b[i]) | rune(b[i+1])<<8
		}
		switch {
		case u >= 0xD800 && u < 0xDC00:
			if i+4 > len(b) {
				return "", false
			}
			var u2 rune
			if bigEndian {
				u2 = rune(b[i+2])<<8 | rune(b[i+3])
			} else {
				u2 = rune(b[i+2]) | rune(b[i+3])<<8
			}
			if u2 < 0xDC00 || u2 >= 0xE000 {
				return "", false
			}
			sb.WriteRune(0x10000 + (u-0xD800)<<10 + (u2 - 0xDC00))
			i += 2
		case u >= 0xDC00 && u < 0xE000:
			return "", false // 未配对的低位代理
		default:
			sb.WriteRune(u)
		}
	}
	return sb.String(), true
}

// ---- posixpath 工具 ----

func pyDirname(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

func pyBasename(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// pyJoin 复刻 posixpath.join（两参数形式，含绝对路径与空段语义）。
func pyJoin(a, b string) string {
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" {
		return b
	}
	if b == "" {
		return a + "/"
	}
	return a + "/" + b
}

// pyNormPath 逐行复刻 posixpath.normpath（含 POSIX 双斜杠特例）。
func pyNormPath(p string) string {
	if p == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(p, "/") {
		initialSlashes = 1
		if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
			initialSlashes = 2
		}
	}
	comps := strings.Split(p, "/")
	var newComps []string
	for _, comp := range comps {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 && newComps[len(newComps)-1] != ".." {
			newComps = newComps[:len(newComps)-1]
		} else if initialSlashes > 0 {
			continue
		} else {
			newComps = append(newComps, comp)
		}
	}
	out := strings.Join(newComps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// normJoin 复刻 epub_lib.norm_join：剥 fragment 后 join + normpath。
func normJoin(base, href string) string {
	clean := href
	if i := strings.IndexByte(href, '#'); i >= 0 {
		clean = href[:i]
	}
	return pyNormPath(pyJoin(base, clean))
}

// pySplitExt 复刻 posixpath.splitext（含「basename 前导点不算扩展名」）。
func pySplitExt(p string) (stem, ext string) {
	sep := strings.LastIndexByte(p, '/')
	dot := strings.LastIndexByte(p, '.')
	if dot > sep {
		for k := sep + 1; k < dot; k++ {
			if p[k] != '.' {
				return p[:dot], p[dot:]
			}
		}
	}
	return p, ""
}

// pyRelPath 复刻 posixpath.relpath 对已归一相对路径的段级计算。
func pyRelPath(target, base string) string {
	startList := splitSegments(base)
	pathList := splitSegments(target)
	i := 0
	for i < len(startList) && i < len(pathList) && startList[i] == pathList[i] {
		i++
	}
	rel := make([]string, 0, len(startList)-i+len(pathList)-i)
	for k := 0; k < len(startList)-i; k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pathList[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

func splitSegments(p string) []string {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// relHref 复刻 epub_lib.rel_href（relpath，不做百分号转义）。
func relHref(fromZipPath, toZipPath string) string {
	base := pyDirname(fromZipPath)
	if base == "" {
		return toZipPath
	}
	return pyRelPath(toZipPath, base)
}

// ---- saxutils.escape 语义 ----

// saxEscape 复刻 xml.sax.saxutils.escape：& > <（CPython 替换序无关紧要，
// 单趟替换等价）。
func saxEscape(s string) string {
	if !strings.ContainsAny(s, "&<>") {
		return s
	}
	return strings.NewReplacer("&", "&amp;", ">", "&gt;", "<", "&lt;").Replace(s)
}

// saxEscapeAttr 复刻 escape(value, {'"': "&quot;"})。
func saxEscapeAttr(s string) string {
	if !strings.ContainsAny(s, "&<>\"") {
		return s
	}
	return strings.NewReplacer("&", "&amp;", ">", "&gt;", "<", "&lt;", `"`, "&quot;").Replace(s)
}

// ---- Python 文本工具 ----

// pyIsSpace 对齐 Python str.isspace（含 \x1c-\x1f）。
func pyIsSpace(r rune) bool {
	if r >= 0x1C && r <= 0x1F {
		return true
	}
	return unicode.IsSpace(r)
}

func pyStrip(s string) string {
	return strings.TrimFunc(s, pyIsSpace)
}

// pyStripLeft 复刻 str.lstrip()。
func pyStripLeft(s string) string {
	return strings.TrimLeftFunc(s, pyIsSpace)
}

// pySplitWS 复刻 str.split()（按 Unicode 空白切分、去空段）。
func pySplitWS(s string) []string {
	var out []string
	start := -1
	for i, r := range s {
		if pyIsSpace(r) {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

// pyTextContent 复刻 core.text_content：" ".join("".join(itertext()).split())。
func pyTextContent(e *xmlElem) string {
	if e == nil {
		return ""
	}
	return strings.Join(pySplitWS(e.itertext()), " ")
}

// pathSuffix 复刻 pathlib.Path(href).suffix（含 query/fragment 计入 name）。
func pathSuffix(href string) string {
	name := pyBasename(href)
	i := strings.LastIndexByte(name, '.')
	if 0 < i && i < len(name)-1 {
		return name[i:]
	}
	return ""
}
