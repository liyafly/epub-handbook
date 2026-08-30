// parse.go 提供对齐 Python xml.etree.ElementTree 行为子集的只读 XML 投影、
// demo 源树读取器与 posixpath 工具。全部只读：仅 os.Stat / os.ReadFile。
package styledemo

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ---- ET 式元素投影 ----

// element 对齐 ET.Element 的本能力所需投影：
//   - space/local 组成命名空间标签（ET 的 tag == "{"+space+"}"+local）；
//   - attrs 的键与 ET attrib 一致：无前缀属性用裸名，有前缀用 "{uri}local"；
//   - text 只保留首段直接字符数据（ET 的 .text 语义）。
type element struct {
	space string
	local string
	attrs map[string]string
	text  string
	kids  []*element
}

func attrKey(space, local string) string {
	if space == "" {
		return local
	}
	return "{" + space + "}" + local
}

// attr 对齐 element.attrib.get(name)（裸名查找）。
func (e *element) attr(name string) (string, bool) {
	v, ok := e.attrs[name]
	return v, ok
}

// attrOr 对齐 element.attrib.get(name, "")。
func (e *element) attrOr(name string) string {
	return e.attrs[name]
}

// parseXMLDoc 解析一份 XML 字节串并返回根元素。
// 与 ET 一样忽略注释与处理指令；解析失败返回错误（调用方按
// Python 的 "XML parse failed: {path}: {exc}" 形状上报）。
func parseXMLDoc(data []byte) (*element, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = true
	// ET/expat 默认不校验声明编码与实际字节的一致性，这里同样放行。
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) { return input, nil }
	var root *element
	var stack []*element
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			el := &element{space: t.Name.Space, local: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				el.attrs[attrKey(a.Name.Space, a.Name.Local)] = a.Value
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.kids = append(p.kids, el)
			} else if root == nil {
				root = el
			}
			stack = append(stack, el)
		case xml.CharData:
			if len(stack) > 0 {
				el := stack[len(stack)-1]
				if len(el.kids) == 0 {
					el.text += string(t)
				}
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil {
		return nil, errors.New("no root element")
	}
	return root, nil
}

// findKidsNS 对齐 findall 的一段直接子级匹配（命名空间限定）。
func findKidsNS(e *element, ns, local string) []*element {
	var out []*element
	for _, k := range e.kids {
		if k.space == ns && k.local == local {
			out = append(out, k)
		}
	}
	return out
}

// findallPath 对齐 root.findall("opf:manifest/opf:item", NS) 形态的
// 直连路径（逐段直接子级，命名空间限定）。
func findallPath(root *element, steps ...[2]string) []*element {
	cur := []*element{root}
	for _, step := range steps {
		var next []*element
		for _, e := range cur {
			next = append(next, findKidsNS(e, step[0], step[1])...)
		}
		cur = next
	}
	return cur
}

// iterAll 对齐 root.iter()：含根元素自身的前序全元素遍历。
func iterAll(root *element) []*element {
	out := []*element{root}
	for _, k := range root.kids {
		out = append(out, iterAll(k)...)
	}
	return out
}

// findAllDesc 对齐 findall(".//xhtml:a")：全部后代（不含根自身），文档序。
func findAllDesc(root *element, ns, local string) []*element {
	var out []*element
	for _, k := range root.kids {
		if k.space == ns && k.local == local {
			out = append(out, k)
		}
		out = append(out, findAllDesc(k, ns, local)...)
	}
	return out
}

// ---- manifest / href 投影 ----

// manifestMap 对齐 manifest_map：id → item（同 id 后者覆盖），
// values() 迭代序 = id 首次出现序。
type manifestMap struct {
	order []string
	byID  map[string]*element
}

func buildManifestMap(items []*element) *manifestMap {
	m := &manifestMap{byID: map[string]*element{}}
	for _, it := range items {
		id, ok := it.attr("id")
		if !ok {
			continue
		}
		if _, seen := m.byID[id]; !seen {
			m.order = append(m.order, id)
		}
		m.byID[id] = it
	}
	return m
}

func (m *manifestMap) values() []*element {
	out := make([]*element, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.byID[id])
	}
	return out
}

func (m *manifestMap) has(id string) bool {
	_, ok := m.byID[id]
	return ok
}

func (m *manifestMap) get(id string) (*element, bool) {
	it, ok := m.byID[id]
	return it, ok
}

// hrefMap 对齐 href_to_item：href → item（后者覆盖），
// 迭代序 = manifest.values() 中该 href 首次出现的序。
type hrefMap struct {
	order  []string
	byHref map[string]*element
}

func buildHrefMap(items []*element) *hrefMap {
	m := &hrefMap{byHref: map[string]*element{}}
	for _, it := range items {
		href := it.attrOr("href")
		if _, seen := m.byHref[href]; !seen {
			m.order = append(m.order, href)
		}
		m.byHref[href] = it
	}
	return m
}

func (m *hrefMap) get(href string) (*element, bool) {
	it, ok := m.byHref[href]
	return it, ok
}

func (m *hrefMap) has(href string) bool {
	_, ok := m.byHref[href]
	return ok
}

// ---- demo 源树读取器（只读） ----

// diskSource 以 demo 源树根目录为根的只读读取器，Has/Read 语义与
// book.Book 对齐（这里表现为 exists/read）。
type diskSource struct {
	root string
}

func newDiskSource(root string) diskSource { return diskSource{root: root} }

// abs 对齐 pathlib 的路径拼接：相对路径直接串接（不做清理，OS 端解析
// ".." 与 Python Path.exists() 行为一致）；绝对路径整体替换。
func (s diskSource) abs(rel string) string {
	if strings.HasPrefix(rel, "/") {
		return rel
	}
	return s.root + "/" + rel
}

// read 读取 demo 根下的相对路径文件。
func (s diskSource) read(rel string) ([]byte, error) {
	return os.ReadFile(s.abs(rel))
}

// hrefPath 对齐 href_path：OEBPS 目录 + href（已去 #fragment）。
func (s diskSource) hrefPath(href string) string {
	return s.abs(pyJoin("OEBPS", stripFragment(href)))
}

// hrefExists 对齐 href_path(href).exists()（目录也算存在）。
func (s diskSource) hrefExists(href string) bool {
	_, err := os.Stat(s.hrefPath(href))
	return err == nil
}

// readUTF8 读取并校验 UTF-8；无效字节返回错误以对齐 Python
// read_text(encoding="utf-8") 的未捕获 UnicodeDecodeError（脚本崩溃）。
func (s diskSource) readUTF8(rel string) (string, error) {
	data, err := s.read(rel)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("styledemo: %s is not valid UTF-8 (Python oracle would crash on decode)", s.abs(rel))
	}
	return string(data), nil
}

// readHrefUTF8 读取 href_path(href) 指向的文件。
func (s diskSource) readHrefUTF8(href string) (string, error) {
	data, err := os.ReadFile(s.hrefPath(href))
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("styledemo: %s is not valid UTF-8 (Python oracle would crash on decode)", s.hrefPath(href))
	}
	return string(data), nil
}

// ---- posix 路径工具 ----

// stripFragment 对齐 href.split("#", 1)[0]。
func stripFragment(href string) string {
	if i := strings.IndexByte(href, '#'); i >= 0 {
		return href[:i]
	}
	return href
}

// pyJoin 对齐 posixpath.join 的相关投影（绝对分量整体替换）。
func pyJoin(a, b string) string {
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" {
		return b
	}
	return a + "/" + b
}

// pyNormPath 对齐 posixpath.normpath（不含 "…" 特例）。
func pyNormPath(p string) string {
	parts := strings.Split(p, "/")
	var out []string
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
				continue
			}
			out = append(out, part)
		default:
			out = append(out, part)
		}
	}
	joined := strings.Join(out, "/")
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(joined, "/") {
		return "/" + joined
	}
	if joined == "" {
		return "."
	}
	return joined
}

// ---- Python 标量格式化 ----

// pyFormatG 对齐 f"{value:g}"（6 位有效数字、去尾零）。
func pyFormatG(v float64) string {
	return strconv.FormatFloat(v, 'g', 6, 64)
}

// pyBool 对齐 str(True)/str(False)。
func pyBool(v bool) string {
	if v {
		return "True"
	}
	return "False"
}

// pyListOrNone 对齐 f"{locked_hrefs or 'none'}"：空列表 → "none"，
// 否则 Python list repr（单引号元素、", " 分隔）。
func pyListOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "'" + s + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// sortedStrings 对齐 sorted(...)。
func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// pyDecodeUTF8 对齐 bytes.decode("utf-8")：成功返回文本，失败返回
// 近似 CPython 的 UnicodeDecodeError 消息（parity 路径不覆盖坏字节）。
func pyDecodeUTF8(data []byte) (string, error) {
	if utf8.Valid(data) {
		return string(data), nil
	}
	for i := 0; i < len(data); {
		c := data[i]
		var size int
		switch {
		case c < 0x80:
			i++
			continue
		case c >= 0xC2 && c <= 0xDF:
			size = 2
		case c >= 0xE0 && c <= 0xEF:
			size = 3
		case c >= 0xF0 && c <= 0xF4:
			size = 4
		default:
			return "", fmt.Errorf("'utf-8' codec can't decode byte 0x%02x in position %d: invalid start byte", c, i)
		}
		for j := 1; j < size; j++ {
			if i+j >= len(data) {
				return "", fmt.Errorf("'utf-8' codec can't decode byte 0x%02x in position %d: unexpected end of data", data[len(data)-1], len(data)-1)
			}
			if data[i+j] < 0x80 || data[i+j] > 0xBF {
				return "", fmt.Errorf("'utf-8' codec can't decode byte 0x%02x in position %d: invalid continuation byte", data[i+j], i+j)
			}
		}
		i += size
	}
	// 不可达：utf8.Valid 已排除全部坏序列。
	return string(data), nil
}
