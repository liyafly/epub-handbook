package docguard

import (
	"encoding/json"
	"testing"
)

// TestGuardParsersReject pins the negative behaviour of the tiny parsers so
// the document guards above cannot pass vacuously.
func TestGuardParsersReject(t *testing.T) {
	badFrontmatter := map[string]string{
		"no opening marker": "name: x\n---\nbody",
		"no closing marker": "---\nname: x\nbody",
		"non key-value":     "---\nname: x\njust text\n---\nbody",
		"duplicate key":     "---\nname: x\nname: y\n---\nbody",
		"empty body":        "---\nname: x\n---\n\n",
	}
	for label, text := range badFrontmatter {
		if _, err := parseFrontmatter(text); err == nil {
			t.Errorf("parseFrontmatter accepted %s", label)
		}
	}
	fm, err := parseFrontmatter("---\nname: x\ndescription: \"y\"\n---\n# T\n")
	if err != nil || fm.values["description"] != "y" {
		t.Errorf("parseFrontmatter rejected valid input: %v %v", err, fm.values)
	}

	badYAML := map[string]string{
		"no interface map": `  display_name: "x"`,
		"list child":       "interface:\n  tags:\n    - a",
		"nested map":       "interface:\n  more:\n    key: \"v\"",
		"block scalar":     "interface:\n  default_prompt: |\n    text",
		"unquoted scalar":  "interface:\n  display_name: x",
		"second top key":   "interface:\n  display_name: \"x\"\nother: \"y\"",
	}
	for label, text := range badYAML {
		if _, err := parseFlatYAML(text); err == nil {
			t.Errorf("parseFlatYAML accepted %s", label)
		}
	}
	values, err := parseFlatYAML("# c\ninterface:\n  display_name: \"x\"\n")
	if err != nil || values["display_name"] != "x" {
		t.Errorf("parseFlatYAML rejected valid input: %v %v", err, values)
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(`{"type":"object","required":["a"],"additionalProperties":false,
		"properties":{"a":{"type":"array","minItems":1,"items":{"type":"string","enum":["x"]}},
		"b":{"type":"string","pattern":"^[a-z]+$"},"c":{"const":"1"}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	for label, raw := range map[string]string{
		"missing required":    `{"b":"ok"}`,
		"extra property":      `{"a":["x"],"z":1}`,
		"minItems":            `{"a":[]}`,
		"enum":                `{"a":["y"]}`,
		"pattern":             `{"a":["x"],"b":"NO"}`,
		"const":               `{"a":["x"],"c":"2"}`,
		"wrong type":          `{"a":"x"}`,
		"non-object document": `[]`,
	} {
		var doc any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatal(err)
		}
		if len(validateSchema(schema, doc, "$")) == 0 {
			t.Errorf("validateSchema accepted %s: %s", label, raw)
		}
	}
	var good any
	_ = json.Unmarshal([]byte(`{"a":["x"],"b":"ok","c":"1"}`), &good)
	if problems := validateSchema(schema, good, "$"); len(problems) != 0 {
		t.Errorf("validateSchema rejected valid document: %v", problems)
	}

	// 中.footnote-w is not a selector: a preceding letter (Python's Unicode \w)
	// suppresses the match, so only three tokens are expected.
	if got := footnoteClassTokens("<a class=\"noteref-x other\">`footnote-y` .duokan-footnote-z 中.footnote-w"); len(got) != 3 {
		t.Errorf("footnoteClassTokens: want 3 tokens, got %v", got)
	}
}
