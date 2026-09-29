// URL/path behavior is shared with merge/split through book/pypath.
// Keep narrow adapters for this capability's sentinel error and projection.
package metadata

import "github.com/liyafly/epub-handbook/internal/book/pypath"

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

func strPtr(s string) *string { return &s }
