package xhtml

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// DecodeAttrWithMap decodes the predefined XML entities and numeric character
// references in raw. rawOff maps every decoded byte to the byte offset in raw
// where its source character or entity begins; its final entry is len(raw).
func DecodeAttrWithMap(raw string) (decoded string, rawOff []int, err error) {
	var out strings.Builder
	out.Grow(len(raw))
	rawOff = make([]int, 0, len(raw)+1)
	for i := 0; i < len(raw); {
		start := i
		if raw[i] == '&' {
			semi := strings.IndexByte(raw[i+1:], ';')
			if semi < 0 {
				return "", nil, fmt.Errorf("unterminated XML character reference at byte %d", i)
			}
			semi += i + 1
			entity := raw[i+1 : semi]
			r, decodeErr := decodeXMLCharacterReference(entity)
			if decodeErr != nil {
				return "", nil, fmt.Errorf("XML character reference &%s;: %w", entity, decodeErr)
			}
			value := string(r)
			out.WriteString(value)
			for range []byte(value) {
				rawOff = append(rawOff, start)
			}
			i = semi + 1
			continue
		}
		r, size := utf8.DecodeRuneInString(raw[i:])
		if r == utf8.RuneError && size == 1 {
			return "", nil, fmt.Errorf("invalid UTF-8 at byte %d", i)
		}
		if !isXMLCharacter(r) {
			return "", nil, fmt.Errorf("invalid XML character U+%04X at byte %d", r, i)
		}
		out.WriteString(raw[i : i+size])
		for range []byte(raw[i : i+size]) {
			rawOff = append(rawOff, start)
		}
		i += size
	}
	rawOff = append(rawOff, len(raw))
	return out.String(), rawOff, nil
}

func decodeXMLCharacterReference(entity string) (rune, error) {
	switch entity {
	case "amp":
		return '&', nil
	case "lt":
		return '<', nil
	case "gt":
		return '>', nil
	case "quot":
		return '"', nil
	case "apos":
		return '\'', nil
	}
	if !strings.HasPrefix(entity, "#") {
		return 0, fmt.Errorf("unsupported named entity")
	}
	numeric := entity[1:]
	base := 10
	if len(numeric) > 1 && (numeric[0] == 'x' || numeric[0] == 'X') {
		base = 16
		numeric = numeric[1:]
	}
	if numeric == "" {
		return 0, fmt.Errorf("empty numeric reference")
	}
	for _, c := range numeric {
		valid := c >= '0' && c <= '9'
		if base == 16 {
			valid = valid || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
		}
		if !valid {
			return 0, fmt.Errorf("invalid numeric reference")
		}
	}
	value, err := strconv.ParseUint(numeric, base, 32)
	if err != nil || value > utf8.MaxRune || !isXMLCharacter(rune(value)) {
		return 0, fmt.Errorf("invalid XML code point")
	}
	return rune(value), nil
}

func isXMLCharacter(r rune) bool {
	return r == 0x9 || r == 0xA || r == 0xD ||
		r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}
