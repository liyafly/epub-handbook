// xmlmini.go 提供与 Python ElementTree 字节兼容的最小 XML 模型与
// URL/编码工具。全部为包内私有实现：
//
//   - 解析（含命名空间、实体、EOL 归一）→ 树上做与 Python 相同的变更
//     → 按 ET.tostring(root, encoding="utf-8", xml_declaration=True)
//     的确切规则序列化（含 xmlns 前缀注册表、按前缀稳定排序、
//     <tag /> 空元素、属性/文本转义表）。
//   - urlsplit / quote / unquote / relpath 等 urllib 与 posixpath 语义。
//
// 存在的唯一理由是字节级复刻 Python oracle（epub_structure_tool.py 用
// ElementTree 整体重写 OPF 与 encryption.xml）；对外仍只通过
// editset.Edit 交付字节（SPEC §6.1 三段式）。
package structurenormalize

import (
	"bytes"
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/simplifiedchinese"
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

func (e *xmlElem) findChild(local string) *xmlElem {
	for _, c := range e.children {
		if c.name == local {
			return c
		}
	}
	return nil
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

func (e *xmlElem) removeChild(target *xmlElem) {
	for i, c := range e.children {
		if c == target {
			e.children = append(e.children[:i:i], e.children[i+1:]...)
			return
		}
	}
}

// iterAll 按文档序（前序）返回 root 与全部后代，对齐 root.iter()。
// 返回的是快照，删除元素不影响遍历（Python 侧用 list(root.iter())）。
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
	// 消费 "<?pi ... ?>"。
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
	// 消费 <!DOCTYPE ... >，含 [ 内部子集 ]；引号内的 > 不计数。
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

// parseNCName 消费一个名字（含 prefix:local；校验交给命名空间解析）。
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

// resolveRawName 把原始限定名解析为 (uri, local)。scope 是当前元素自身
// 的声明，优先于外层栈。
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
		return "", raw, nil // 无前缀属性不属于默认命名空间
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
	// 解析元素名与属性名（属性前缀可由本元素自身的声明绑定）。
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

	// 内容。
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
			// 注释丢弃，两侧文本合并（expat 行为）。
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
			pending.WriteString(p.src[p.pos : p.pos+end]) // CDATA 不做 EOL 归一
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

// readTextRun 读取到下一个 '<' 为止的文本并做 EOL 归一 + 实体解码。
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

// normalizeTextChunk：字面 \r\n / \r → \n（XML EOL 归一），随后实体解码
// （字符引用产生的字符不再参与 EOL 归一）。
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

// normalizeAttrValue：字面 \r\n / \r → \n → ' '，字面 \t → ' '
// （expat 对无 DTD 文档的属性值归一）；字符引用保持原字符。
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

// decodeEntity 解码 s 开头的一个实体，返回 (替换文本, 消费字节数)。
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

// etreeToBytes 复刻 ET.tostring(root, encoding="utf-8", xml_declaration=True)：
//   - 声明固定为 <?xml version='1.0' encoding='utf-8'?>\n（单引号、小写 utf-8）；
//   - xmlns 声明全部落在根元素上，按前缀稳定排序（空前缀最前，与
//     CPython _serialize_xml 的 sorted(..., key=prefix) 一致）；
//   - 未注册 URI 的前缀按文档序编号 ns0、ns1…（len(namespaces)）；
//   - 空元素输出 <tag />（斜杠前有空格）；
//   - 属性值转义 & < > " \r \n \t，文本/tail 只转义 & < >；
//   - 末尾没有换行。
func etreeToBytes(root *xmlElem) []byte {
	used := map[string]string{} // uri → prefix
	var decls []nsDecl
	qname := func(ns, name string) string {
		if ns == "" {
			return name
		}
		prefix, ok := used[ns]
		if !ok {
			prefix, known := namespacePrefixes[ns]
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

	// 声明按前缀稳定排序（sorted(namespaces.items(), key=prefix)）。
	sortStableByPrefix(decls)

	var b strings.Builder
	b.WriteString("<?xml version='1.0' encoding='utf-8'?>\n")
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

// xmlSourceToUTF8 按 BOM / XML 声明把 XML 字节转换为 UTF-8 文本。
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

// ---- 文本编解码（decode_text / str.encode 的语义） ----

// decodeText 复刻 decode_text：候选编码 = XML 声明 → utf-8-sig BOM →
// utf-16 BOM → utf-8 → gb18030，按序去重尝试，全部失败报错。
func decodeText(data []byte, label string) (string, string, error) {
	var candidates []string
	seen := map[string]bool{}
	add := func(enc string) {
		if !seen[enc] {
			seen[enc] = true
			candidates = append(candidates, enc)
		}
	}
	if m := xmlEncodingRe.FindSubmatch(data[:min(len(data), 256)]); m != nil {
		add(string(m[1]))
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		add("utf-8-sig")
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) || bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
		add("utf-16")
	}
	add("utf-8")
	add("gb18030")
	for _, enc := range candidates {
		if text, ok := tryDecodeText(data, enc); ok {
			return text, enc, nil
		}
	}
	return "", "", toolErrf("%s: cannot decode text resource as UTF or GB18030", label)
}

func tryDecodeText(data []byte, enc string) (string, bool) {
	switch strings.ToLower(enc) {
	case "utf-8", "utf8":
		if !utf8.Valid(data) {
			return "", false
		}
		return string(data), true
	case "ascii", "us-ascii":
		for _, c := range data {
			if c >= 0x80 {
				return "", false
			}
		}
		return string(data), true
	case "utf-8-sig":
		body := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		if !utf8.Valid(body) {
			return "", false
		}
		return string(body), true
	case "utf-16":
		// Python 'utf-16'：BOM 定序，缺省按 little-endian。
		bigEndian := false
		body := data
		if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
			body = data[2:]
		} else if bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
			bigEndian = true
			body = data[2:]
		}
		return decodeUTF16Units(body, bigEndian)
	case "utf-16le":
		return decodeUTF16Units(data, false)
	case "utf-16be":
		return decodeUTF16Units(data, true)
	case "gb18030":
		out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data)
		if err != nil {
			return "", false
		}
		return string(out), true
	default:
		enc2, err := lookupEncoding(enc)
		if err != nil {
			return "", false
		}
		out, err := enc2.NewDecoder().Bytes(data)
		if err != nil {
			return "", false
		}
		return string(out), true
	}
}

// encodeText 复刻 str.encode：utf-16 输出 BOM + little-endian。
func encodeText(text, enc string) ([]byte, error) {
	switch strings.ToLower(enc) {
	case "utf-8", "utf8":
		return []byte(text), nil
	case "ascii", "us-ascii":
		for _, r := range text {
			if r > 0x7F {
				return nil, toolErrf("cannot encode text resource as %s", enc)
			}
		}
		return []byte(text), nil
	case "utf-8-sig":
		return append([]byte{0xEF, 0xBB, 0xBF}, text...), nil
	case "utf-16":
		return encodeUTF16Units(text, false, true), nil
	case "utf-16le":
		return encodeUTF16Units(text, false, false), nil
	case "utf-16be":
		return encodeUTF16Units(text, true, false), nil
	case "gb18030":
		return simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(text))
	default:
		e, err := lookupEncoding(enc)
		if err != nil {
			return nil, toolErrf("unknown encoding: %s", enc)
		}
		return e.NewEncoder().Bytes([]byte(text))
	}
}

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

func encodeUTF16Units(text string, bigEndian, withBOM bool) []byte {
	var out []byte
	if withBOM {
		if bigEndian {
			out = append(out, 0xFE, 0xFF)
		} else {
			out = append(out, 0xFF, 0xFE)
		}
	}
	put := func(u rune) {
		if bigEndian {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	for _, r := range text {
		switch {
		case r <= 0xFFFF:
			put(r)
		default:
			r -= 0x10000
			put(0xD800 + (r >> 10))
			put(0xDC00 + (r & 0x3FF))
		}
	}
	return out
}

// ---- URL / 路径工具（urllib.parse 与 posixpath 的语义） ----

type urlParts struct {
	scheme, netloc, path, query, fragment string
}

// pyURLSplit 复刻 urllib.parse.urlsplit 对本工具相关输入的行为：
// 先在首个 '#' 处切 fragment，再识别 scheme（':' 前缀全为合法 scheme
// 字符且首字符为 ASCII 字母），再识别 '//' netloc，再在首个 '?' 处切 query。
func pyURLSplit(raw string) urlParts {
	var p urlParts
	// Python 3 urlsplit 剥离首尾的 C0 控制符与空格，并移除内嵌的 \t\r\n
	// （WHATWG 规则）—— 书里实际存在带前导空格的引用，必须复刻。
	raw = strings.TrimFunc(raw, func(r rune) bool { return r <= ' ' })
	if strings.ContainsAny(raw, "\t\r\n") {
		raw = strings.Map(func(r rune) rune {
			if r == '\t' || r == '\r' || r == '\n' {
				return -1
			}
			return r
		}, raw)
	}
	rest := raw
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		p.fragment = rest[i+1:]
		rest = rest[:i]
	}
	if i := strings.IndexByte(rest, ':'); i > 0 && isASCIILetter(rest[0]) {
		valid := true
		for k := 0; k < i; k++ {
			c := rest[k]
			if !isASCIILetter(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
				valid = false
				break
			}
		}
		if valid {
			p.scheme = strings.ToLower(rest[:i])
			rest = rest[i+1:]
		}
	}
	if strings.HasPrefix(rest, "//") {
		j := 2
		for j < len(rest) {
			c := rest[j]
			if c == '/' || c == '?' || c == '#' {
				break
			}
			j++
		}
		p.netloc = rest[2:j]
		rest = rest[j:]
	}
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		p.query = rest[i+1:]
		rest = rest[:i]
	}
	p.path = rest
	return p
}

// pyURLUnsplitPath 复刻 urlunsplit(("", "", path, query, fragment))。
func pyURLUnsplitPath(pathPart, query, fragment string) string {
	out := pathPart
	if query != "" {
		out += "?" + query
	}
	if fragment != "" {
		out += "#" + fragment
	}
	return out
}

// pyIsExternalURI 复刻 is_external_uri：有 scheme 或以 / // 开头。
func pyIsExternalURI(uri string) bool {
	if p := pyURLSplit(uri); p.scheme != "" {
		return true
	}
	return strings.HasPrefix(uri, "/") || strings.HasPrefix(uri, "//")
}

// pyQuote 复刻 quote(value, safe="/:@-._~")：unreserved（字母数字与
// -._~）加上 safe 集合原样保留，其余按 UTF-8 字节转 %XX（大写）。
func pyQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~', c == '/', c == ':', c == '@':
			b.WriteByte(c)
		default:
			const hex = "0123456789ABCDEF"
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xF])
		}
	}
	return b.String()
}

// pyUnquote 复刻 unquote：字节级 %XX 解码后按 utf-8 / errors=replace 解码。
func pyUnquote(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	raw := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			h1, ok1 := hexVal(s[i+1])
			h2, ok2 := hexVal(s[i+2])
			if ok1 && ok2 {
				raw = append(raw, h1<<4|h2)
				i += 3
				continue
			}
		}
		raw = append(raw, s[i])
		i++
	}
	// 与 CPython 的 utf-8 errors=replace 近似：每个非法子序列一个 U+FFFD。
	return strings.ToValidUTF8(string(raw), "\uFFFD")
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

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

// pyJoin 复刻 posixpath.join 对本工具输入的等价行为（空段跳过）。
func pyJoin(parts ...string) string {
	var out []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}

// pySplitExt 复刻 posixpath.splitext（含「basename 前导点不算扩展名」：
// splitext(".bashrc") == (".bashrc", "")）。
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

// pathExt / pathStem 是 pySplitExt 的单值便捷形式。
func pathExt(p string) string {
	_, ext := pySplitExt(p)
	return ext
}

func pathStem(p string) string {
	stem, _ := pySplitExt(p)
	return stem
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

// validateArchivePath 复刻 epub_lib.validate_archive_path。
func validateArchivePath(name, label string) (string, error) {
	if name == "" || strings.HasPrefix(name, "/") {
		return "", toolErrf("%s: invalid absolute or empty ZIP path: %q", label, name)
	}
	normalized := path.Clean(name)
	if normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", toolErrf("%s: ZIP path escapes archive root: %q", label, name)
	}
	return normalized, nil
}

// resolveRelativePath 复刻 epub_lib.resolve_relative_path。
func resolveRelativePath(baseFile, uriPath string) (string, error) {
	decoded := pyUnquote(uriPath)
	return validateArchivePath(path.Join(pyDirname(baseFile), decoded), "resource href")
}

// resolveRootPath 复刻 resolve_root_path（encryption.xml 的 URI 目标）。
func resolveRootPath(uriPath string) (string, error) {
	return validateArchivePath(strings.TrimLeft(pyUnquote(uriPath), "/"), "encryption URI")
}

// relativeURI 复刻 relative_uri：relpath 后做 percent-quote。
func relativeURI(fromArchivePath, toArchivePath string) string {
	base := pyDirname(fromArchivePath)
	rel := toArchivePath
	if base != "" {
		rel = pyRelPath(toArchivePath, base)
	}
	return pyQuote(rel)
}

// pyRepr 近似 Python 的 str repr（错误消息里的 {uri!r}）。
func pyRepr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('\'')
	return b.String()
}
