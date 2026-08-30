// navtoc.go 复刻 core.py 的 TOC 解析（parse_toc_nav / parse_toc_ncx /
// spine_toc_entries / href_with_fragment）与 navigation.py 的 build_nav /
// build_ncx 字符串模板（逐字节照抄）。
package merge

import (
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

type tocEntry struct {
	title string
	href  string
	level int
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
	resolved, err := resolveRelativePath(baseFile, pyURLSplit(pathPart).path)
	if err != nil {
		return "", err
	}
	if sep {
		return resolved + "#" + fragment, nil
	}
	return resolved, nil
}

// parseTocNav 复刻 core.parse_toc_nav。
func parseTocNav(navPath string, data []byte) ([]tocEntry, error) {
	if data == nil {
		return nil, nil
	}
	root, err := opf.ScanSpanTree(data)
	if err != nil {
		return nil, toolErrf("%s: XML parse failed: %v", navPath, err)
	}
	var findToc func(e *opf.SpanNode) *opf.SpanNode
	findToc = func(e *opf.SpanNode) *opf.SpanNode {
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
	var entries []tocEntry
	toc := findToc(root)
	if toc == nil {
		return entries, nil
	}
	var walkList func(e *opf.SpanNode, level int)
	walkList = func(e *opf.SpanNode, level int) {
		for _, child := range e.Kids {
			if child.Name.Local != "li" {
				continue
			}
			for _, gc := range child.Kids {
				switch gc.Name.Local {
				case "a":
					title := collapseSpace(gc.IterText())
					href, _ := gc.AttrByLocal("", "href")
					resolved := ""
					if href != "" {
						resolved, err = hrefWithFragment(navPath, href)
						if err != nil {
							return
						}
					}
					entries = append(entries, tocEntry{title: title, href: resolved, level: level})
				case "span":
					title := collapseSpace(gc.IterText())
					entries = append(entries, tocEntry{title: title, level: level})
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
func isTOCNav(e *opf.SpanNode) bool {
	if e.Name.Local != "nav" {
		return false
	}
	epubType, _ := e.AttrByLocal(opf.OPSURI, "type")
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

// parseTocNcx 复刻 core.parse_toc_ncx。
func parseTocNcx(ncxPath string, data []byte) ([]tocEntry, error) {
	if data == nil {
		return nil, nil
	}
	root, err := opf.ScanSpanTree(data)
	if err != nil {
		return nil, toolErrf("%s: XML parse failed: %v", ncxPath, err)
	}
	var entries []tocEntry
	var walk func(e *opf.SpanNode, level int) error
	walk = func(e *opf.SpanNode, level int) error {
		for _, child := range e.Kids {
			if child.Name.Local != "navPoint" {
				continue
			}
			title := ""
			href := ""
			for _, gc := range child.Kids {
				switch gc.Name.Local {
				case "navLabel":
					title = collapseSpace(gc.IterText())
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
			entries = append(entries, tocEntry{title: title, href: href, level: level})
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

// spineTocEntries 复刻 core.spine_toc_entries。
func spineTocEntries(pkg *pkgInfo) []tocEntry {
	var entries []tocEntry
	for _, sp := range pkg.spine {
		item, ok := pkg.byID(sp.idref)
		if !ok || hasNavProp(item.properties) {
			continue
		}
		lower := strings.ToLower(item.archivePath)
		if item.mediaType == "application/xhtml+xml" ||
			strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".html") {
			entries = append(entries, tocEntry{title: pyBasename(item.href), href: item.archivePath, level: 1})
		}
	}
	return entries
}

// parseToc 复刻 core.parse_toc：nav → ncx → spine 回退。
func parseToc(names map[string]bool, read func(string) ([]byte, error), pkg *pkgInfo) ([]tocEntry, error) {
	for _, item := range pkg.manifest {
		if !hasNavProp(item.properties) {
			continue
		}
		if !names[item.archivePath] {
			continue // parse_toc_nav 对缺失文件返回 []
		}
		data, err := read(item.archivePath)
		if err != nil {
			data = nil
		}
		entries, perr := parseTocNav(item.archivePath, data)
		if perr != nil {
			return nil, perr
		}
		if len(entries) > 0 {
			return entries, nil
		}
	}
	if pkg.tocID != "" {
		if item, ok := pkg.byID(pkg.tocID); ok {
			if names[item.archivePath] {
				data, err := read(item.archivePath)
				if err == nil {
					entries, perr := parseTocNcx(item.archivePath, data)
					if perr != nil {
						return nil, perr
					}
					if len(entries) > 0 {
						return entries, nil
					}
				}
			}
		}
	}
	return spineTocEntries(pkg), nil
}

// buildNav 复刻 navigation.build_nav 的字符串模板（逐字节）。
func buildNav(title string, groups []tocGroup, navPath string, pathMap map[string]string) []byte {
	lines := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">`,
		"<head>",
		"  <title>" + pyEscapeText(title) + "</title>",
		"</head>",
		"<body>",
		`<nav epub:type="toc" id="toc">`,
		"  <h1>" + pyEscapeText(title) + "</h1>",
		"  <ol>",
	}
	appendEntry := func(entry tocEntry, indent string) {
		if entry.href != "" {
			href := entry.href
			fragment := ""
			sep := false
			if i := strings.IndexByte(href, '#'); i >= 0 {
				href, fragment, sep = href[:i], href[i+1:], true
			}
			target := href
			if mapped, ok := pathMap[href]; ok {
				target = mapped
			}
			rendered := relativeURI(navPath, target)
			if sep {
				rendered += "#" + fragment
			}
			fallback := pyBasename(target)
			if entry.title != "" {
				fallback = entry.title
			}
			lines = append(lines, indent+"<li><a href="+pyQuoteAttr(rendered)+">"+pyEscapeText(fallback)+"</a></li>")
		} else {
			lines = append(lines, indent+"<li><span>"+pyEscapeText(entry.title)+"</span></li>")
		}
	}
	for _, group := range groups {
		if len(groups) > 1 {
			lines = append(lines, "    <li>")
			lines = append(lines, "      <span>"+pyEscapeText(group.title)+"</span>")
			lines = append(lines, "      <ol>")
			for _, entry := range group.entries {
				appendEntry(entry, "        ")
			}
			lines = append(lines, "      </ol>")
			lines = append(lines, "    </li>")
		} else {
			for _, entry := range group.entries {
				appendEntry(entry, "    ")
			}
		}
	}
	lines = append(lines, "  </ol>", "</nav>", "</body>", "</html>")
	return []byte(strings.Join(lines, "\n") + "\n")
}

// buildNcx 复刻 navigation.build_ncx 的字符串模板（逐字节）。
func buildNcx(title string, groups []tocGroup, ncxPath string, pathMap map[string]string) []byte {
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
		"  <docTitle><text>" + pyEscapeText(title) + "</text></docTitle>",
		"  <navMap>",
	}
	playOrder := 1
	for _, group := range groups {
		for _, entry := range group.entries {
			if entry.href == "" {
				continue
			}
			href := entry.href
			fragment := ""
			sep := false
			if i := strings.IndexByte(href, '#'); i >= 0 {
				href, fragment, sep = href[:i], href[i+1:], true
			}
			target := href
			if mapped, ok := pathMap[href]; ok {
				target = mapped
			}
			rendered := relativeURI(ncxPath, target)
			if sep {
				rendered += "#" + fragment
			}
			fallback := pyBasename(target)
			if entry.title != "" {
				fallback = entry.title
			}
			lines = append(lines,
				"    "+`<navPoint id="navPoint-`+itoa(playOrder)+`" playOrder="`+itoa(playOrder)+`">`,
				"      <navLabel><text>"+pyEscapeText(fallback)+"</text></navLabel>",
				"      <content src="+pyQuoteAttr(rendered)+"/>",
				"    </navPoint>")
			playOrder++
		}
	}
	lines = append(lines, "  </navMap>", "</ncx>")
	return []byte(strings.Join(lines, "\n") + "\n")
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
