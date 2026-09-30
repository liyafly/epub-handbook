package pypath

import "strings"

// BaseStem is the extension-free basename, unlike SplitExt's full-path stem.
func BaseStem(p string) string {
	stem, _ := SplitExt(Basename(p))
	return stem
}

// Join preserves the POSIX join semantics used by the migrated tools.
func Join(base string, names ...string) string {
	joined := base
	for _, name := range names {
		if strings.HasPrefix(name, "/") {
			joined = name
			continue
		}
		if joined == "" || strings.HasSuffix(joined, "/") {
			joined += name
			continue
		}
		joined += "/" + name
	}
	return joined
}

// NormPath is POSIX lexical normalization, including the special // prefix.
// It is not an archive safety check; use ValidateArchivePath for that.
func NormPath(p string) string {
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
	var out []string
	for comp := range strings.SplitSeq(p, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(out) == 0) {
			out = append(out, comp)
		} else if len(out) > 0 && out[len(out)-1] != ".." {
			out = out[:len(out)-1]
		} else if initialSlashes == 0 {
			out = append(out, "..")
		}
	}
	joined := strings.Repeat("/", initialSlashes) + strings.Join(out, "/")
	if joined == "" {
		return "."
	}
	return joined
}

// NormJoin strips only a fragment, preserving queries and percent sequences.
// This is intentionally distinct from ResolveRelativePath's URI decoding.
func NormJoin(base, href string) string {
	clean, _, _ := strings.Cut(href, "#")
	return NormPath(Join(base, clean))
}

// RewriteURI updates a local reference after an archive resource moves. Rooted
// hrefs are unsafe inside an EPUB archive, so they are retained and reported.
// Unknown or invalid local targets are also retained with a warning.
func RewriteURI(uri, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool, warn func(string, ...any)) string {
	if uri == "" || strings.HasPrefix(uri, "#") {
		return uri
	}
	parts := URLSplit(uri)
	if parts.Scheme != "" || parts.Netloc != "" || parts.Path == "" {
		return uri
	}
	if strings.HasPrefix(parts.Path, "/") {
		if warn != nil {
			warn("%s: unsafe absolute reference left unchanged: %s", oldDocument, uri)
		}
		return uri
	}
	oldTarget, err := ResolveRelativePath(oldDocument, parts.Path)
	if err != nil {
		if warn != nil {
			warn("%s: unsafe local reference left unchanged: %s", oldDocument, uri)
		}
		return uri
	}
	if !knownFiles[oldTarget] {
		if warn != nil {
			warn("%s: missing local reference left unchanged: %s", oldDocument, uri)
		}
		return uri
	}
	target := oldTarget
	if mapped, ok := pathMap[oldTarget]; ok {
		target = mapped
	}
	if resolved, err := ResolveRelativePath(newDocument, parts.Path); err == nil && resolved == target {
		return uri
	}
	return URLUnsplitPath(RelativeURI(newDocument, target), parts.Query, parts.Fragment)
}

// RewriteURIForChangedTargets updates a reference only when its target exists
// and the path map or document move changes the relative URI. Missing and
// unrelated rooted references stay untouched without generating warnings.
func RewriteURIForChangedTargets(uri, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool, warn func(string, ...any)) string {
	if uri == "" || strings.HasPrefix(uri, "#") {
		return uri
	}
	parts := URLSplit(uri)
	if parts.Scheme != "" || parts.Netloc != "" || parts.Path == "" {
		return uri
	}
	if strings.HasPrefix(parts.Path, "/") {
		if warn != nil && rootedTargetChanged(parts.Path, oldDocument, pathMap, knownFiles) {
			warn("%s: unsafe absolute reference left unchanged: %s", oldDocument, uri)
		}
		return uri
	}
	oldTarget, err := ResolveRelativePath(oldDocument, parts.Path)
	if err != nil || !knownFiles[oldTarget] {
		return uri
	}
	return RewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles, nil)
}

func rootedTargetChanged(uriPath, oldDocument string, pathMap map[string]string, knownFiles map[string]bool) bool {
	rooted := Unquote(strings.TrimLeft(uriPath, "/"))
	targets := []string{rooted}
	if root, _, nested := strings.Cut(oldDocument, "/"); nested && root != "" {
		targets = append(targets, Join(root, rooted))
	}
	for _, target := range targets {
		if mapped, ok := pathMap[target]; ok && mapped != target && knownFiles[target] {
			return true
		}
	}
	return false
}

// RelativePath returns an unescaped path. RelativeURI additionally quotes it.
func RelativePath(from, to string) string {
	base := Dirname(from)
	if base == "" {
		return to
	}
	return RelPath(to, base)
}

// ResolveRootPath decodes and validates a root-relative encryption URI.
func ResolveRootPath(uriPath string) (string, error) {
	return ValidateArchivePath(strings.TrimLeft(Unquote(uriPath), "/"), "encryption URI")
}
