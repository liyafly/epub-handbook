// navtoc.go 复刻 core.py 的 TOC 解析（parse_toc_nav / parse_toc_ncx /
// href_with_fragment）与 navigation.py 的 build_nav / build_ncx 字符串模板
// （逐字节照抄）。
//
// spine_toc_entries 与 parse_toc 里「按 pkgInfo 遍历 manifest/spine 取
// TOC」的部分没有搬到这里：pkgInfo 是 internal/caps/merge、
// internal/caps/split 各自私有的包投影类型（层 2），本包是层 4，只能
// import 层号更大的包，不能反向依赖 caps。因此本文件只保留不依赖
// pkgInfo 的通用部分——纯 XML 解析（ParseTocNav/ParseTocNcx）与纯字符串
// 生成（BuildNav/BuildNCX）；pkgInfo 相关的调用方逻辑仍留在
// internal/caps/merge/merge.go 与 internal/caps/split/split.go 里。
//
// BuildNav / BuildNCX 返回 string 而非 []byte：它们从结构化 TOC 条目生成
// 一份新文档，输入侧没有原文档，不属于 INV-2 要防的「解析→重序列化
// 往返」（对应 archguard.TestNoWholeDocSerializer 顶部注释里记录的这条
// 判断）。
package opf

import (
	"fmt"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
)

// TocEntry 是一条解析出的 TOC 条目（nav / ncx / spine 回退统一形状）。
type TocEntry struct {
	Title string
	Href  string
	Level int
}

// TocGroup 是 BuildNav / BuildNCX 的 (标题, 条目列表) 分组，对应
// navigation.build_nav / build_ncx 里的 (group_title, entries)。
type TocGroup struct {
	Title   string
	Entries []TocEntry
}

// hrefWithFragment 复刻 core.href_with_fragment。
func hrefWithFragment(baseFile, href string) (string, error) {
	pathPart := href
	fragment := ""
	sep := false
	if i := strings.IndexByte(href, '#'); i >= 0 {
		pathPart, fragment, sep = href[:i], href[i+1:], true
	}
	if pathPart == "" {
		if sep {
			return "#" + fragment, nil
		}
		return "", nil
	}
	resolved, err := pypath.ResolveRelativePath(baseFile, pypath.URLSplit(pathPart).Path)
	if err != nil {
		return "", err
	}
	if sep {
		return resolved + "#" + fragment, nil
	}
	return resolved, nil
}

// ParseTocNav 复刻 core.parse_toc_nav。
func ParseTocNav(navPath string, data []byte) ([]TocEntry, error) {
	if data == nil {
		return nil, nil
	}
	root, err := ScanSpanTree(data)
	if err != nil {
		return nil, fmt.Errorf("%s: XML parse failed: %v", navPath, err)
	}
	var findToc func(e *SpanNode) *SpanNode
	findToc = func(e *SpanNode) *SpanNode {
		if isTOCNav(e) {
			return e
		}
		for _, c := range e.Kids {
			if found := findToc(c); found != nil {
				return found
			}
		}
		return nil
	}
	var entries []TocEntry
	toc := findToc(root)
	if toc == nil {
		return entries, nil
	}
	var walkList func(e *SpanNode, level int)
	walkList = func(e *SpanNode, level int) {
		for _, child := range e.Kids {
			if child.Name.Local != "li" {
				continue
			}
			for _, gc := range child.Kids {
				switch gc.Name.Local {
				case "a":
					title := pypath.CollapseSpace(gc.IterText())
					href, _ := gc.AttrByLocal("", "href")
					resolved := ""
					if href != "" {
						resolved, err = hrefWithFragment(navPath, href)
						if err != nil {
							return
						}
					}
					entries = append(entries, TocEntry{Title: title, Href: resolved, Level: level})
				case "span":
					title := pypath.CollapseSpace(gc.IterText())
					entries = append(entries, TocEntry{Title: title, Level: level})
				case "ol":
					walkList(gc, level+1)
				}
			}
		}
	}
	for _, child := range toc.Kids {
		if child.Name.Local == "ol" {
			walkList(child, 1)
			if err != nil {
				return nil, err
			}
		}
	}
	return entries, nil
}

// isTOCNav 复刻 is_toc_nav：local 名为 nav 且 epub:type 含 "toc"。
func isTOCNav(e *SpanNode) bool {
	if e.Name.Local != "nav" {
		return false
	}
	epubType, _ := e.AttrByLocal(OPSURI, "type")
	if epubType == "" {
		epubType, _ = e.AttrByLocal("", "epub:type")
	}
	for _, tok := range strings.Fields(epubType) {
		if tok == "toc" {
			return true
		}
	}
	return false
}

// ParseTocNcx 复刻 core.parse_toc_ncx。
func ParseTocNcx(ncxPath string, data []byte) ([]TocEntry, error) {
	if data == nil {
		return nil, nil
	}
	root, err := ScanSpanTree(data)
	if err != nil {
		return nil, fmt.Errorf("%s: XML parse failed: %v", ncxPath, err)
	}
	var entries []TocEntry
	var walk func(e *SpanNode, level int) error
	walk = func(e *SpanNode, level int) error {
		for _, child := range e.Kids {
			if child.Name.Local != "navPoint" {
				continue
			}
			title := ""
			href := ""
			for _, gc := range child.Kids {
				switch gc.Name.Local {
				case "navLabel":
					title = pypath.CollapseSpace(gc.IterText())
				case "content":
					src, _ := gc.AttrByLocal("", "src")
					if src != "" {
						resolved, err := hrefWithFragment(ncxPath, src)
						if err != nil {
							return err
						}
						href = resolved
					}
				}
			}
			entries = append(entries, TocEntry{Title: title, Href: href, Level: level})
			if err := walk(child, level+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range root.Kids {
		if child.Name.Local == "navMap" {
			if err := walk(child, 1); err != nil {
				return nil, err
			}
		}
	}
	return entries, nil
}

// BuildNav 复刻 navigation.build_nav 的字符串模板（逐字节）。
func BuildNav(title string, groups []TocGroup, navPath string, pathMap map[string]string) string {
	lines := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">`,
		"<head>",
		"  <title>" + pypath.EscapeText(title) + "</title>",
		"</head>",
		"<body>",
		`<nav epub:type="toc" id="toc">`,
		"  <h1>" + pypath.EscapeText(title) + "</h1>",
		"  <ol>",
	}
	appendEntry := func(entry TocEntry, indent string) {
		if entry.Href != "" {
			href := entry.Href
			fragment := ""
			sep := false
			if i := strings.IndexByte(href, '#'); i >= 0 {
				href, fragment, sep = href[:i], href[i+1:], true
			}
			target := href
			if mapped, ok := pathMap[href]; ok {
				target = mapped
			}
			rendered := pypath.RelativeURI(navPath, target)
			if sep {
				rendered += "#" + fragment
			}
			fallback := pypath.Basename(target)
			if entry.Title != "" {
				fallback = entry.Title
			}
			lines = append(lines, indent+"<li><a href="+pypath.QuoteAttr(rendered)+">"+pypath.EscapeText(fallback)+"</a></li>")
		} else {
			lines = append(lines, indent+"<li><span>"+pypath.EscapeText(entry.Title)+"</span></li>")
		}
	}
	for _, group := range groups {
		if len(groups) > 1 {
			lines = append(lines, "    <li>")
			lines = append(lines, "      <span>"+pypath.EscapeText(group.Title)+"</span>")
			lines = append(lines, "      <ol>")
			for _, entry := range group.Entries {
				appendEntry(entry, "        ")
			}
			lines = append(lines, "      </ol>")
			lines = append(lines, "    </li>")
		} else {
			for _, entry := range group.Entries {
				appendEntry(entry, "    ")
			}
		}
	}
	lines = append(lines, "  </ol>", "</nav>", "</body>", "</html>")
	return strings.Join(lines, "\n") + "\n"
}

// BuildNCX 复刻 navigation.build_ncx 的字符串模板（逐字节）。
func BuildNCX(title string, groups []TocGroup, ncxPath string, pathMap map[string]string) string {
	lines := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">`,
		`<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">`,
		"  <head>",
		`    <meta name="dtb:uid" content="epub-package-tool"/>`,
		`    <meta name="dtb:depth" content="1"/>`,
		`    <meta name="dtb:totalPageCount" content="0"/>`,
		`    <meta name="dtb:maxPageNumber" content="0"/>`,
		"  </head>",
		"  <docTitle><text>" + pypath.EscapeText(title) + "</text></docTitle>",
		"  <navMap>",
	}
	playOrder := 1
	for _, group := range groups {
		for _, entry := range group.Entries {
			if entry.Href == "" {
				continue
			}
			href := entry.Href
			fragment := ""
			sep := false
			if i := strings.IndexByte(href, '#'); i >= 0 {
				href, fragment, sep = href[:i], href[i+1:], true
			}
			target := href
			if mapped, ok := pathMap[href]; ok {
				target = mapped
			}
			rendered := pypath.RelativeURI(ncxPath, target)
			if sep {
				rendered += "#" + fragment
			}
			fallback := pypath.Basename(target)
			if entry.Title != "" {
				fallback = entry.Title
			}
			lines = append(lines,
				"    "+`<navPoint id="navPoint-`+itoa(playOrder)+`" playOrder="`+itoa(playOrder)+`">`,
				"      <navLabel><text>"+pypath.EscapeText(fallback)+"</text></navLabel>",
				"      <content src="+pypath.QuoteAttr(rendered)+"/>",
				"    </navPoint>")
			playOrder++
		}
	}
	lines = append(lines, "  </navMap>", "</ncx>")
	return strings.Join(lines, "\n") + "\n"
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
