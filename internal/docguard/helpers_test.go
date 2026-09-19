// Package docguard is a test-only companion guard (like internal/legacy_surface)
// that mechanically checks the documentation and contract layer: SKILL.md
// frontmatter and section shape, agents/openai.yaml flatness, capability
// manifests against their schema, and the AGENTS.md canonical-entrypoint tokens.
//
// It replaces the deleted Python meta-validators (scripts/validate_skills_basic.py,
// scripts/validate_contracts.py, scripts/validate_ai_entrypoints.py). It contains
// no production code and sits outside the SPEC §1 layer graph. Like
// internal/archguard, a red guard means fix the documents, not the guard; edits
// to this package are expected to go through human review.
package docguard

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// repoRoot walks upward from the working directory to the go.mod directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above working directory")
		}
		dir = parent
	}
}

// readText reads a UTF-8 text file relative to root and fails the test on error.
func readText(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

// skillDirs returns the sorted names of every skills/<dir> that holds a
// SKILL.md. The rule (not an allowlist) is "a skill is a directory with a
// SKILL.md", so skills/README.md is excluded by construction.
func skillDirs(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatalf("read skills/: %v", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "skills", entry.Name(), "SKILL.md")); err == nil {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		t.Fatal("skills/: no skill directories with SKILL.md found")
	}
	slices.Sort(names)
	return names
}

// globRel returns sorted repo-relative slash paths matching pattern under root.
func globRel(t *testing.T, root, pattern string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	var out []string
	for _, match := range matches {
		rel, err := filepath.Rel(root, match)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	slices.Sort(out)
	return out
}

// unquote strips one layer of surrounding double quotes, mirroring the
// Python validators' `.strip('"')` on scalar values.
func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value[1 : len(value)-1]
	}
	return value
}
