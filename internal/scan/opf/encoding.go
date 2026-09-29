package opf

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// RequireUTF8 rejects text that cannot be edited as UTF-8 while allowing a
// UTF-8 BOM and the equivalent XML declaration labels.
func RequireUTF8(path string, data []byte) error {
	body := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(body) {
		return fmt.Errorf("%s: text bytes are not valid UTF-8", path)
	}
	if encoding := declaredXMLEncoding(body); encoding != "" {
		switch strings.ToLower(strings.TrimSpace(encoding)) {
		case "utf-8", "utf8", "ascii", "us-ascii":
		default:
			return fmt.Errorf("%s: XML declaration encoding %q is not UTF-8", path, encoding)
		}
	}
	return nil
}

func declaredXMLEncoding(data []byte) string {
	if !bytes.HasPrefix(data, []byte("<?xml")) {
		return ""
	}
	if len(data) > len("<?xml") && !isXMLSpace(data[len("<?xml")]) && data[len("<?xml")] != '?' {
		return ""
	}
	end := bytes.Index(data, []byte("?>"))
	if end < 0 {
		return ""
	}
	match := xmlDeclarationEncoding.FindSubmatch(data[:end+2])
	if len(match) != 2 {
		return ""
	}
	return string(match[1])
}
