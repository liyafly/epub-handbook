// Package epubhandbook exposes read-only repository resources embedded in the
// EPUB CLI binary for use when it runs outside a checkout.
package epubhandbook

import (
	"embed"
	"io/fs"
)

//go:embed contracts/capabilities/v1/*.json contracts/parameters/v2/cli.json contracts/schemas/*/*.json templates/style-presets/*/preset.json templates/style-presets/*/Styles/*.css
var resources embed.FS

// EmbeddedFS returns the immutable repository resources bundled with the CLI.
func EmbeddedFS() fs.FS { return resources }
