package docguard

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// skillNamePattern is the allowed skill slug shape (also used for table cells).
const skillNamePattern = `^[a-z0-9-]{1,63}$`

// requiredSkillSections are the four fixed SKILL.md sections of SPEC §8.4, in order.
func requiredSkillSections() []string {
	return []string{"## 何时用", "## 调什么", "## 返回怎么读", "## 依据返回怎么判断"}
}

// frontmatter is the parsed YAML-ish header of a SKILL.md: flat `key: value`
// lines only, exactly as the deleted validate_skills_basic.py accepted.
type frontmatter struct {
	values map[string]string
	order  []string
	body   string
}

func parseFrontmatter(text string) (frontmatter, error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return frontmatter{}, fmt.Errorf("missing opening frontmatter marker")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return frontmatter{}, fmt.Errorf("missing closing frontmatter marker")
	}
	fm := frontmatter{values: map[string]string{}}
	for _, raw := range lines[1:end] {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		key, value, ok := strings.Cut(raw, ":")
		if !ok {
			return frontmatter{}, fmt.Errorf("invalid frontmatter line: %q", raw)
		}
		key = strings.TrimSpace(key)
		if _, dup := fm.values[key]; dup {
			return frontmatter{}, fmt.Errorf("duplicate frontmatter key: %s", key)
		}
		fm.values[key] = unquote(value)
		fm.order = append(fm.order, key)
	}
	fm.body = strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	if fm.body == "" {
		return frontmatter{}, fmt.Errorf("empty skill body")
	}
	return fm, nil
}

// TestSkillFrontmatter: every skills/<dir>/SKILL.md opens with a frontmatter
// holding exactly `name` and `description` (AGENTS.md: "frontmatter 只保留
// name 和 description"), name == directory name and matches the slug pattern,
// description is a non-empty single line, agents/openai.yaml exists, and the
// body carries exactly the four SPEC §8.4 sections in order.
func TestSkillFrontmatter(t *testing.T) {
	root := repoRoot(t)
	nameRe := regexp.MustCompile(skillNamePattern)
	headingRe := regexp.MustCompile(`(?m)^## .*$`)

	for _, dir := range skillDirs(t, root) {
		rel := "skills/" + dir + "/SKILL.md"
		text := readText(t, root, rel)
		fm, err := parseFrontmatter(text)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}

		keys := slices.Clone(fm.order)
		slices.Sort(keys)
		if !slices.Equal(keys, []string{"description", "name"}) {
			t.Errorf("%s: frontmatter keys must be exactly name and description, got %v", rel, fm.order)
		}
		name := fm.values["name"]
		if name != dir {
			t.Errorf("%s: name %q must equal directory name %q", rel, name, dir)
		}
		if !nameRe.MatchString(name) {
			t.Errorf("%s: invalid skill name %q (want %s)", rel, name, skillNamePattern)
		}
		desc := fm.values["description"]
		switch {
		case desc == "":
			t.Errorf("%s: description is empty", rel)
		case strings.Contains(desc, "TODO"):
			t.Errorf("%s: description still contains TODO", rel)
		case len([]rune(desc)) < 20:
			t.Errorf("%s: description too short (%d runes)", rel, len([]rune(desc)))
		}

		if len(globRel(t, root, "skills/"+dir+"/agents/openai.yaml")) == 0 {
			t.Errorf("skills/%s: missing agents/openai.yaml", dir)
		}

		headings := headingRe.FindAllString(fm.body, -1)
		if !slices.Equal(headings, requiredSkillSections()) {
			t.Errorf("%s: SPEC §8.4 requires exactly the sections %v in order; got %v", rel, requiredSkillSections(), headings)
		}
	}
}

// parseFlatYAML mirrors parse_simple_yaml_strings from the deleted Python:
// a single top-level `interface:` map whose children are `key: "string"`
// scalars. Anything else (lists, deeper nesting, block scalars, unquoted or
// multi-line values) is rejected so the file stays trivially machine-readable.
func parseFlatYAML(text string) (map[string]string, error) {
	values := map[string]string{}
	sawInterface := false
	childRe := regexp.MustCompile(`^  ([a-z_][a-z0-9_]*): "([^"]*)"$`)
	for lineNo, raw := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if raw == "interface:" {
			if sawInterface {
				return nil, fmt.Errorf("line %d: duplicate interface: map", lineNo+1)
			}
			sawInterface = true
			continue
		}
		if !sawInterface {
			return nil, fmt.Errorf("line %d: only a top-level `interface:` map is allowed, got %q", lineNo+1, raw)
		}
		m := childRe.FindStringSubmatch(raw)
		if m == nil {
			return nil, fmt.Errorf("line %d: expected `  key: \"string\"` under interface:, got %q", lineNo+1, raw)
		}
		if _, dup := values[m[1]]; dup {
			return nil, fmt.Errorf("line %d: duplicate key %s", lineNo+1, m[1])
		}
		values[m[1]] = m[2]
	}
	if !sawInterface {
		return nil, fmt.Errorf("missing top-level interface: map")
	}
	return values, nil
}

// TestOpenAIYAMLShape: skills/*/agents/openai.yaml is a flat string-only
// `interface:` map (AGENTS.md: "只使用扁平字符串 metadata") with display_name /
// short_description / default_prompt, and default_prompt mentions $<skill>.
func TestOpenAIYAMLShape(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range skillDirs(t, root) {
		rel := "skills/" + dir + "/agents/openai.yaml"
		if len(globRel(t, root, rel)) == 0 {
			t.Errorf("%s: missing (reported by TestSkillFrontmatter too)", rel)
			continue
		}
		values, err := parseFlatYAML(readText(t, root, rel))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		for _, key := range []string{"display_name", "short_description", "default_prompt"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s: missing or empty %s", rel, key)
			}
		}
		if !strings.Contains(values["default_prompt"], "$"+dir) {
			t.Errorf("%s: default_prompt must mention $%s", rel, dir)
		}
	}
}

// parseSkillTable extracts skill slugs from rows shaped `| `epub...` | ... |`,
// as skills/README.md and docs/learn/04-skills.md index skills.
func parseSkillTable(text string) []string {
	nameRe := regexp.MustCompile(skillNamePattern)
	var names []string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "| `epub") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 2 {
			continue
		}
		cell := strings.Trim(strings.TrimSpace(cells[1]), "`")
		if strings.HasPrefix(cell, "epub") && nameRe.MatchString(cell) {
			names = append(names, cell)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// TestSkillIndexTables: the skill tables in skills/README.md and the learn
// skills page list exactly the skills/ directories (no stale or missing rows).
func TestSkillIndexTables(t *testing.T) {
	root := repoRoot(t)
	expected := skillDirs(t, root)
	for _, rel := range []string{"skills/README.md", "docs/learn/04-skills.md"} {
		got := parseSkillTable(readText(t, root, rel))
		if !slices.Equal(got, expected) {
			var missing, extra []string
			for _, name := range expected {
				if !slices.Contains(got, name) {
					missing = append(missing, name)
				}
			}
			for _, name := range got {
				if !slices.Contains(expected, name) {
					extra = append(extra, name)
				}
			}
			t.Errorf("%s: skill table disagrees with skills/ directories: missing %v, extra %v", rel, missing, extra)
		}
	}
}

// footnoteClassTokens collects noteref-/footnote-/duokan-footnote- class
// tokens from class="..." attributes, CSS selectors and inline code spans.
func footnoteClassTokens(text string) []string {
	classRe := regexp.MustCompile(`^(?:noteref-|footnote-|duokan-footnote-)[a-z0-9_-]+$`)
	attrRe := regexp.MustCompile(`\bclass\s*=\s*["']([^"']+)["']`)
	// RE2 has no lookbehind; group 1 stands in for Python's (?<![\w-]).
	selectorRe := regexp.MustCompile(`(^|[^\pL\pN_-])\.((?:noteref-|footnote-|duokan-footnote-)[a-z0-9_-]+)`)
	codeRe := regexp.MustCompile("`([^`\n]+)`")

	var tokens []string
	for _, m := range attrRe.FindAllStringSubmatch(text, -1) {
		for _, tok := range strings.Fields(m[1]) {
			if classRe.MatchString(tok) {
				tokens = append(tokens, tok)
			}
		}
	}
	for _, m := range selectorRe.FindAllStringSubmatch(text, -1) {
		tokens = append(tokens, m[2])
	}
	for _, m := range codeRe.FindAllStringSubmatch(text, -1) {
		if tok := strings.TrimSpace(m[1]); classRe.MatchString(tok) {
			tokens = append(tokens, tok)
		}
	}
	slices.Sort(tokens)
	return slices.Compact(tokens)
}

// TestFootnoteClassVocabulary: skills/*/SKILL.md and docs/how-to/*.md may only
// use footnote class tokens that SPEC-实现约束.md §1 (弹注) declares, plus the
// three stable presentation hooks outside §1's structural vocabulary.
func TestFootnoteClassVocabulary(t *testing.T) {
	root := repoRoot(t)
	spec := readText(t, root, "docs/final/SPEC-实现约束.md")
	sectionRe := regexp.MustCompile(`(?ms)^## 1\) 弹注.*?(?:^## |\z)`)
	section := sectionRe.FindString(spec)
	if section == "" {
		t.Fatal("docs/final/SPEC-实现约束.md: missing §1 弹注 section")
	}
	canonical := footnoteClassTokens(section)
	if len(canonical) == 0 {
		t.Fatal("docs/final/SPEC-实现约束.md §1: declares no footnote class tokens; guard would be vacuous")
	}
	canonical = append(canonical, "noteref-icon", "footnote-back", "footnote-line")
	slices.Sort(canonical)

	targets := slices.Concat(globRel(t, root, "skills/*/SKILL.md"), globRel(t, root, "docs/how-to/*.md"))
	for _, rel := range targets {
		for _, tok := range footnoteClassTokens(readText(t, root, rel)) {
			if !slices.Contains(canonical, tok) {
				t.Errorf("%s: footnote class token not declared by SPEC §1: %s", rel, tok)
			}
		}
	}
}
