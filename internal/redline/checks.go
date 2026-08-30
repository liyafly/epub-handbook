package redline

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// ErrInput 对齐 Python 的 ErrInput：输入不可处理，legacy 语义退出码 2。
var ErrInput = errors.New("redline: input error")

func inputErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInput, fmt.Sprintf(format, args...))
}

// xhtmlNames 返回状态里全部 XHTML 成员名（按 Python 的 lower().endswith 判定）。
func xhtmlNames(s State) []string {
	var out []string
	for _, name := range s.Names() {
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".html") {
			out = append(out, name)
		}
	}
	return out
}

func hasDRM(s State) bool {
	for _, name := range s.Names() {
		if name == "META-INF/encryption.xml" {
			return true
		}
	}
	return false
}

// opfFor 解析状态的 OPF。缺 container / rootfile 不解析时返回 Python 同款错误。
func opfFor(s State) (*opf.Package, error) {
	data, err := s.Read(opf.ContainerPath)
	if err != nil {
		return nil, inputErr("%s: cannot resolve OPF from META-INF/container.xml", s.Path())
	}
	opfPath, err := opf.FindOPFPath([]byte(sanitizeXML(data)))
	if err != nil {
		return nil, inputErr("%s", err.Error())
	}
	raw, err := s.Read(opfPath)
	if err != nil {
		return nil, inputErr("%s", err.Error())
	}
	pkg, err := opf.Parse(opfPath, []byte(sanitizeXML(raw)))
	if err != nil {
		return nil, inputErr("%s", err.Error())
	}
	return pkg, nil
}

// ---- text ----

type textCheck struct{}

func (textCheck) Check(before, after State, o Options) ([]Finding, error) {
	var out []Finding
	beforePaths := sortedNames(xhtmlNames(before))
	afterPaths := map[string]bool{}
	for _, name := range xhtmlNames(after) {
		afterPaths[name] = true
	}
	for _, name := range beforePaths {
		afterName := MappedPath(o.PathMap, name)
		if skipped(name, o.AllowList) || skipped(afterName, o.AllowList) {
			continue
		}
		if !afterPaths[afterName] {
			out = append(out, Finding{CheckText, fmt.Sprintf("text: deleted XHTML file: %s", name), false})
			continue
		}
		beforeRaw, err := before.Read(name)
		if err != nil {
			return nil, inputErr("%s", err.Error())
		}
		afterRaw, err := after.Read(afterName)
		if err != nil {
			return nil, inputErr("%s", err.Error())
		}
		beforeBlocks, err := ExtractTextBlocks(beforeRaw, fmt.Sprintf("%s:%s", before.Path(), name))
		if err != nil {
			return nil, inputErr("%s", err.Error())
		}
		afterBlocks, err := ExtractTextBlocks(afterRaw, fmt.Sprintf("%s:%s", after.Path(), afterName))
		if err != nil {
			return nil, inputErr("%s", err.Error())
		}
		beforeHashes := BlockHashes(beforeBlocks)
		afterHashes := BlockHashes(afterBlocks)
		if equalStrings(beforeHashes, afterHashes) {
			out = append(out, Finding{CheckText,
				fmt.Sprintf("verbose: text unchanged: %s -> %s (%d blocks)", name, afterName, len(beforeHashes)), true})
			continue
		}
		out = append(out, Finding{CheckText,
			fmt.Sprintf("text: modified %s -> %s: %d blocks before, %d after", name, afterName, len(beforeHashes), len(afterHashes)), false})
		for i := 0; i < len(beforeHashes) && i < len(afterHashes); i++ {
			if beforeHashes[i] != afterHashes[i] {
				out = append(out, Finding{CheckText,
					fmt.Sprintf("  block %d: %s != %s", i, beforeHashes[i][:12], afterHashes[i][:12]), false})
				out = append(out, Finding{CheckText,
					fmt.Sprintf("    before: %s", truncateRunes(beforeBlocks[i], 160)), false})
				out = append(out, Finding{CheckText,
					fmt.Sprintf("    after:  %s", truncateRunes(afterBlocks[i], 160)), false})
				break
			}
		}
		if len(beforeHashes) != len(afterHashes) {
			out = append(out, Finding{CheckText, "  block count differs", false})
		}
	}
	expectedAfter := map[string]bool{}
	for _, name := range beforePaths {
		expectedAfter[MappedPath(o.PathMap, name)] = true
	}
	var added []string
	for _, name := range xhtmlNames(after) {
		if !expectedAfter[name] && !skipped(name, o.AllowList) {
			added = append(added, name)
		}
	}
	sort.Strings(added)
	for _, name := range added {
		out = append(out, Finding{CheckText, fmt.Sprintf("text: added XHTML file: %s", name), false})
	}
	return out, nil
}

// ---- anchors ----

type anchorsCheck struct{}

func (anchorsCheck) Check(before, after State, o Options) ([]Finding, error) {
	var out []Finding
	for _, name := range sortedNames(xhtmlNames(before)) {
		afterName := MappedPath(o.PathMap, name)
		if skipped(name, o.AllowList) || skipped(afterName, o.AllowList) {
			continue
		}
		if !stateHas(after, afterName) {
			out = append(out, Finding{CheckAnchors, fmt.Sprintf("anchors: XHTML file deleted: %s", name), false})
			continue
		}
		beforeRaw, err := before.Read(name)
		if err != nil {
			return nil, inputErr("%s", err.Error())
		}
		afterRaw, err := after.Read(afterName)
		if err != nil {
			return nil, inputErr("%s", err.Error())
		}
		beforeIDs, err := ExtractAnchorIDs(beforeRaw, fmt.Sprintf("%s:%s", before.Path(), name))
		if err != nil {
			return nil, inputErr("%s", err.Error())
		}
		afterIDs, err := ExtractAnchorIDs(afterRaw, fmt.Sprintf("%s:%s", after.Path(), afterName))
		if err != nil {
			return nil, inputErr("%s", err.Error())
		}
		var missing []string
		for id := range beforeIDs {
			if !afterIDs[id] {
				missing = append(missing, id)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			shown := missing
			suffix := ""
			if len(missing) > 12 {
				shown = missing[:12]
				suffix = fmt.Sprintf(", ... (%d total)", len(missing))
			}
			out = append(out, Finding{CheckAnchors,
				fmt.Sprintf("anchors: deleted id in %s: %s%s", name, strings.Join(shown, ", "), suffix), false})
		} else if o.Verbose {
			out = append(out, Finding{CheckAnchors,
				fmt.Sprintf("verbose: anchors unchanged: %s (%d ids)", name, len(beforeIDs)), true})
		}
	}
	return out, nil
}

// ---- metadata ----

type metadataCheck struct{}

func (metadataCheck) Check(before, after State, _ Options) ([]Finding, error) {
	bPkg, err := opfFor(before)
	if err != nil {
		return nil, err
	}
	aPkg, err := opfFor(after)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, field := range coreMetadataFields {
		b := normalizeMetaList(bPkg.Metadata[field])
		a := normalizeMetaList(aPkg.Metadata[field])
		if !equalStrings(b, a) {
			out = append(out, Finding{CheckMetadata,
				fmt.Sprintf("metadata: dc:%s changed: %s -> %s", field, pythonRepr(b), pythonRepr(a)), false})
		}
	}
	return out, nil
}

// normalizeMetaList 复刻 metadata_values：每个 dc 字段全文 itertext 后归一化。
func normalizeMetaList(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = normalizeText(v)
	}
	return out
}

// ---- spine ----

type spineCheck struct{}

func (spineCheck) Check(before, after State, _ Options) ([]Finding, error) {
	bPkg, err := opfFor(before)
	if err != nil {
		return nil, err
	}
	aPkg, err := opfFor(after)
	if err != nil {
		return nil, err
	}
	bSpine := spineIDRefs(bPkg)
	aSpine := spineIDRefs(aPkg)
	if equalStrings(bSpine, aSpine) {
		return nil, nil
	}
	return []Finding{{CheckSpine,
		fmt.Sprintf("spine: itemref sequence changed: %s -> %s", pythonRepr(bSpine), pythonRepr(aSpine)), false}}, nil
}

func spineIDRefs(p *opf.Package) []string {
	out := make([]string, len(p.Spine))
	for i, it := range p.Spine {
		out[i] = it.IDRef
	}
	return out
}

// ---- cover ----

type coverCheck struct{}

func (coverCheck) Check(before, after State, o Options) ([]Finding, error) {
	bCover, err := coverPath(before)
	if err != nil {
		return nil, err
	}
	aCover, err := coverPath(after)
	if err != nil {
		return nil, err
	}
	if MappedPath(o.PathMap, orEmpty(bCover)) != orEmpty(aCover) {
		return []Finding{{CheckCover,
			fmt.Sprintf("cover: cover-image path changed: %s -> %s", pythonReprValue(bCover), pythonReprValue(aCover)), false}}, nil
	}
	if bCover == "" {
		return nil, nil
	}
	if !stateHas(before, bCover) || aCover == "" || !stateHas(after, aCover) {
		return []Finding{{CheckCover,
			fmt.Sprintf("cover: cover-image missing from zip: %s -> %s", pythonReprValue(bCover), pythonReprValue(aCover)), false}}, nil
	}
	bRaw, err := before.Read(bCover)
	if err != nil {
		return nil, inputErr("%s", err.Error())
	}
	aRaw, err := after.Read(aCover)
	if err != nil {
		return nil, inputErr("%s", err.Error())
	}
	bHash, aHash := sha256Hex(bRaw), sha256Hex(aRaw)
	if bHash != aHash {
		return []Finding{{CheckCover,
			fmt.Sprintf("cover: cover-image bytes changed: %s != %s (%s)", bHash[:12], aHash[:12], bCover), false}}, nil
	}
	return nil, nil
}

// coverPath 复刻 cover_path：第一个 properties 含 cover-image 的 manifest 项。
func coverPath(s State) (string, error) {
	pkg, err := opfFor(s)
	if err != nil {
		return "", err
	}
	item, ok := pkg.CoverItem()
	if !ok || item.Href == "" {
		return "", nil
	}
	return path.Join(pkg.OPFDir(), unquoteURLPath(item.Href)), nil
}

// ---- drm ----

type drmCheck struct{}

func (drmCheck) Check(before, after State, o Options) ([]Finding, error) {
	if !hasDRM(before) && !hasDRM(after) {
		return nil, nil
	}
	staleAllowed := isStaleOnly(before) && isStaleOnly(after)
	fontAllowed := o.AllowFontObfuscation && isFontObfOnly(before) && isFontObfOnly(after)
	if !staleAllowed && !fontAllowed {
		return []Finding{{CheckDRM, "DRM detected, refusing to process.", false}}, nil
	}
	return nil, nil
}

// encryptionRecords 读取并解析 encryption.xml（存在时）。
func encryptionRecords(s State) ([]opf.EncryptionRecord, bool, error) {
	if !hasDRM(s) {
		return nil, false, nil
	}
	raw, err := s.Read("META-INF/encryption.xml")
	if err != nil {
		return nil, true, inputErr("%s", err.Error())
	}
	records, err := opf.ParseEncryption([]byte(sanitizeXML(raw)))
	if err != nil {
		return nil, true, inputErr("%s", err.Error())
	}
	return records, true, nil
}

// isStaleOnly 复刻 has_only_stale_encryption_references。
func isStaleOnly(s State) (ok bool) {
	records, has, err := encryptionRecords(s)
	if err != nil {
		return false
	}
	if !has {
		return true
	}
	recordsCount := 0
	names := map[string]bool{}
	for _, n := range s.Names() {
		names[n] = true
	}
	for _, rec := range records {
		for _, raw := range rec.RawTargets {
			if raw == "" {
				return false
			}
			archivePath := opf.EncryptionTargetPath(raw)
			if names[archivePath] {
				return false
			}
			recordsCount++
		}
	}
	return recordsCount > 0
}

// isFontObfOnly 复刻 has_only_standard_font_obfuscation。
func isFontObfOnly(s State) (ok bool) {
	records, has, err := encryptionRecords(s)
	if err != nil {
		return false
	}
	if !has {
		return true
	}
	fonts, err := manifestFontPaths(s)
	if err != nil {
		return false
	}
	recordsCount := 0
	for _, rec := range records {
		if !fontObfuscationAlgorithms[rec.Algorithm] {
			return false
		}
		references := 0
		for _, raw := range rec.RawTargets {
			if raw == "" {
				return false
			}
			archivePath := opf.EncryptionTargetPath(raw)
			if !fonts[archivePath] {
				return false
			}
			references++
			recordsCount++
		}
		if references == 0 {
			return false
		}
	}
	return recordsCount > 0
}

// manifestFontPaths 复刻 manifest_font_paths。
func manifestFontPaths(s State) (map[string]bool, error) {
	pkg, err := opfFor(s)
	if err != nil {
		return nil, err
	}
	base := path.Dir(pkg.Path)
	paths := map[string]bool{}
	for _, item := range pkg.Manifest {
		hrefPath := unquoteURLPath(item.Href)
		if hrefPath == "" {
			continue
		}
		mediaType := strings.ToLower(item.MediaType)
		if strings.Contains(mediaType, "font") || hasFontExt(hrefPath) {
			paths[path.Clean(path.Join(base, hrefPath))] = true
		}
	}
	return paths, nil
}

func hasFontExt(p string) bool {
	lower := strings.ToLower(p)
	for _, ext := range []string{".otf", ".ttf", ".woff", ".woff2"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// ---- 小工具 ----

func stateHas(s State, name string) bool {
	for _, n := range s.Names() {
		if n == name {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// truncateRunes 按 Python 的字符切片语义截断（非字节）。
func truncateRunes(s string, n int) string {
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

// unquoteURLPath 复刻 unquote(urlsplit(uri).path)。
func unquoteURLPath(uri string) string {
	clean := uri
	if i := strings.IndexByte(clean, '#'); i >= 0 {
		clean = clean[:i]
	}
	if i := strings.IndexByte(clean, '?'); i >= 0 {
		clean = clean[:i]
	}
	return unquotePercent(clean)
}

func unquotePercent(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			h := unhex(s[i+1])
			l := unhex(s[i+2])
			if h >= 0 && l >= 0 {
				b.WriteByte(byte(h<<4 | l))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func orEmpty(s string) string { return s }

// pythonReprValue 渲染 Python 的 str/None repr。
func pythonReprValue(v any) string {
	switch s := v.(type) {
	case nil:
		return "None"
	case string:
		return pythonStrRepr(s)
	default:
		return fmt.Sprintf("%v", v)
	}
}
