// register.go contains immutable scan tables for kindle_check.
package kindlecheck

import "regexp"

// directImgSelectorRE matches a selector branch whose last compound selector
// is img, optionally followed by a class, id, attribute, or pseudo selector.
var directImgSelectorRE = regexp.MustCompile(`(^|[\s>+~])img([.#\[:][^\s>+~]*)?\s*$`)
