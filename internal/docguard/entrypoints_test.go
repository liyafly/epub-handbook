package docguard

import (
	"strings"
	"testing"
)

// TestAIEntrypointsCanonical ports scripts/validate_ai_entrypoints.py: AGENTS.md
// is the single maintained source of AI working rules, and every other
// model-facing entry document only routes to it. Tokens that referenced the
// deleted scripts/ were dropped; the Go-era SPEC references were added.
func TestAIEntrypointsCanonical(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		file   string
		tokens []string
	}{
		{"AGENTS.md", []string{
			"唯一维护源",
			"docs/final/SPEC-go-architecture.md",
			"docs/final/SPEC-实现约束.md",
			"docs/pipeline/cleanup-flow.md",
			"THIRD_PARTY.md",
		}},
		{"CLAUDE.md", []string{"兼容入口", "[AGENTS.md](AGENTS.md)"}},
		{"README.md", []string{"[AGENTS.md](AGENTS.md)"}},
		{"CONTRIBUTING.md", []string{"[AGENTS.md](AGENTS.md)"}},
		{"docs/README.md", []string{"`AGENTS.md`"}},
		{"skills/README.md", []string{"`AGENTS.md`"}},
		{"docs/learn/04-skills.md", []string{"[AGENTS.md](../../AGENTS.md)"}},
	}
	for _, tc := range cases {
		text := readText(t, root, tc.file)
		for _, token := range tc.tokens {
			if !strings.Contains(text, token) {
				t.Errorf("%s: missing required reference: %s", tc.file, token)
			}
		}
	}
}
