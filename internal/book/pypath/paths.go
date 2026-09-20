package pypath

import "strings"

// BaseStem is the extension-free basename, unlike SplitExt's full-path stem.
func BaseStem(p string) string {
	stem, _ := SplitExt(Basename(p))
	return stem
}

// Join preserves the POSIX two-part join used by the migrated tools.
func Join(base, name string) string {
	if strings.HasPrefix(name, "/") {
		return name
	}
	if base == "" || strings.HasSuffix(base, "/") {
		return base + name
	}
	return base + "/" + name
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

// RelativePath returns an unescaped path. RelativeURI additionally quotes it.
func RelativePath(from, to string) string {
	base := Dirname(from)
	if base == "" {
		return to
	}
	return RelPath(to, base)
}
