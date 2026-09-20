// URL/path behavior is shared with merge/split through book/pypath.
// Keep narrow adapters for this capability's sentinel error and projection.
package metadata

import "github.com/liyafly/epub-handbook/internal/book/pypath"

type urlParts struct{ scheme, netloc, path, query, fragment string }

func pyURLSplit(raw string) urlParts {
	p := pypath.URLSplit(raw)
	return urlParts{p.Scheme, p.Netloc, p.Path, p.Query, p.Fragment}
}

func pyIsExternalURI(uri string) bool { return pypath.IsExternalURI(uri) }

func validateArchivePath(name, label string) (string, error) {
	value, err := pypath.ValidateArchivePath(name, label)
	if err != nil {
		return "", toolErrf("%v", err)
	}
	return value, nil
}

func resolveRelativePath(baseFile, uriPath string) (string, error) {
	value, err := pypath.ResolveRelativePath(baseFile, uriPath)
	if err != nil {
		return "", toolErrf("%v", err)
	}
	return value, nil
}

func pyDirname(p string) string { return pypath.Dirname(p) }

func strPtr(s string) *string { return &s }
