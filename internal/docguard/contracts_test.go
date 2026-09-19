package docguard

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestContractsValid ports scripts/validate_contracts.py: every
// contracts/capabilities/v1/*.json has the required fields and enum values,
// its filename equals id + ".json", inputSchema/outputSchema point at existing
// files under contracts/schemas/v1/, requires reference known capability ids,
// legacySkillSlugs reference existing skills/ dirs, and the whole document
// validates against capability-manifest.schema.json via a minimal validator.
func TestContractsValid(t *testing.T) {
	root := repoRoot(t)
	manifests := globRel(t, root, "contracts/capabilities/v1/*.json")
	if len(manifests) == 0 {
		t.Fatal("contracts/capabilities/v1: no capability manifests found")
	}
	skills := skillDirs(t, root)

	var schema map[string]any
	if err := json.Unmarshal([]byte(readText(t, root, "contracts/schemas/v1/capability-manifest.schema.json")), &schema); err != nil {
		t.Fatalf("capability-manifest.schema.json: %v", err)
	}

	idRe := regexp.MustCompile(`^epub(?:\.[a-z0-9][a-z0-9-]*){2,}$`)
	semverRe := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	kinds := []string{"detector", "planner", "transformer", "validator"}
	redLines := []string{"text", "metadata", "spine", "anchors", "cover", "drm"}
	networks := []string{"none", "readonly", "full"}
	required := []string{"schemaVersion", "id", "version", "kind", "legacySkillSlugs", "inputSchema", "outputSchema", "redLines", "permissions", "execution", "requires"}
	// 2026-09-07（经仓库所有者决策）：`adapters` 字段已删除。它枚举
	// openai / claude / mcp / cli，把「哪家 harness 能调这条能力」写进了
	// 机器契约 —— 而 CLI 侧从不读它（`pipeline.Contract.Adapters` 声明后
	// 零消费），本工具链要作为 skill 接入任意 harness，厂商名单不该是契约的
	// 一部分。能力面由 `epub capabilities` 自描述，接入方自己决定怎么调。
	// execution 描述 pipeline 怎么调这条能力（输入语义 + 落盘语义）。它此前只
	// 以 internal/pipeline/register.go 里的四张 id 白名单存在，契约完全不提，
	// 于是「contracts/ 是机器契约唯一事实来源」这句话对执行形态不成立 ——
	// 而且有两条契约与执行面直接矛盾（声明需写权限却永不写盘）。
	// 注册方式与本字段的对账断言在 internal/pipeline（见 TestRegistryMatchesContractExecution）。
	executionInputs := []string{"epub", "epub-or-tree", "source-path"}
	executionOutputs := []string{"single", "multi", "none"}

	ids := map[string]string{}
	requiresBy := map[string][]string{}

	for _, rel := range manifests {
		var doc any
		if err := json.Unmarshal([]byte(readText(t, root, rel)), &doc); err != nil {
			t.Errorf("%s: invalid JSON: %v", rel, err)
			continue
		}
		for _, problem := range validateSchema(schema, doc, "$") {
			t.Errorf("%s: schema: %s", rel, problem)
		}
		obj, ok := doc.(map[string]any)
		if !ok {
			t.Errorf("%s: manifest must be a JSON object", rel)
			continue
		}
		missing := false
		for _, key := range required {
			if _, ok := obj[key]; !ok {
				t.Errorf("%s: missing required field %s", rel, key)
				missing = true
			}
		}
		if missing {
			continue
		}
		for key := range obj {
			if !slices.Contains(required, key) {
				t.Errorf("%s: unsupported field %s", rel, key)
			}
		}

		if obj["schemaVersion"] != "1" {
			t.Errorf("%s: schemaVersion must be \"1\"", rel)
		}

		if exec, ok := obj["execution"].(map[string]any); ok {
			input, _ := exec["input"].(string)
			output, _ := exec["output"].(string)
			if !slices.Contains(executionInputs, input) {
				t.Errorf("%s: execution.input %q not in %v", rel, input, executionInputs)
			}
			if !slices.Contains(executionOutputs, output) {
				t.Errorf("%s: execution.output %q not in %v", rel, output, executionOutputs)
			}
			// 自洽：不落盘的能力不该声明需要写权限（此前 popup.normalize 与
			// style.demo.maintain 就是这么自相矛盾的）；反之写单产物必须要写权限。
			perms, _ := obj["permissions"].(map[string]any)
			write, _ := perms["requiresWriteAccess"].(bool)
			if output == "none" && write {
				t.Errorf("%s: execution.output=none 却 permissions.requiresWriteAccess=true", rel)
			}
			if output != "none" && !write {
				t.Errorf("%s: execution.output=%s 却 permissions.requiresWriteAccess=false", rel, output)
			}
		} else {
			t.Errorf("%s: execution must be an object", rel)
		}
		id, _ := obj["id"].(string)
		if !idRe.MatchString(id) {
			t.Errorf("%s: invalid capability id %q", rel, id)
		} else {
			if filepath.Base(rel) != id+".json" {
				t.Errorf("%s: filename must be %s.json", rel, id)
			}
			if prev, dup := ids[id]; dup {
				t.Errorf("%s: duplicate capability id %s (also %s)", rel, id, prev)
			}
			ids[id] = rel
		}
		if v, _ := obj["version"].(string); !semverRe.MatchString(v) {
			t.Errorf("%s: version must be x.y.z, got %q", rel, v)
		}
		if k, _ := obj["kind"].(string); !slices.Contains(kinds, k) {
			t.Errorf("%s: kind %q not in %v", rel, k, kinds)
		}

		slugs, ok := stringList(obj["legacySkillSlugs"])
		if !ok || len(slugs) == 0 {
			t.Errorf("%s: legacySkillSlugs must be a non-empty string array", rel)
		}
		for _, slug := range slugs {
			if !slices.Contains(skills, slug) {
				t.Errorf("%s: unknown legacy skill slug %q (no skills/%s/SKILL.md)", rel, slug, slug)
			}
		}

		for _, field := range []string{"inputSchema", "outputSchema"} {
			ref, _ := obj[field].(string)
			if !strings.HasPrefix(ref, "contracts/schemas/v1/") {
				t.Errorf("%s: %s must reference contracts/schemas/v1/, got %q", rel, field, ref)
				continue
			}
			if len(globRel(t, root, ref)) == 0 {
				t.Errorf("%s: %s target is missing: %s", rel, field, ref)
			}
		}

		if lines, ok := stringList(obj["redLines"]); !ok {
			t.Errorf("%s: redLines must be a string array", rel)
		} else {
			for _, line := range lines {
				if !slices.Contains(redLines, line) {
					t.Errorf("%s: unknown redLine %q", rel, line)
				}
			}
		}

		if perms, ok := obj["permissions"].(map[string]any); !ok {
			t.Errorf("%s: permissions must be an object", rel)
		} else {
			if _, ok := perms["requiresWriteAccess"].(bool); !ok {
				t.Errorf("%s: permissions.requiresWriteAccess must be boolean", rel)
			}
			if n, _ := perms["network"].(string); !slices.Contains(networks, n) {
				t.Errorf("%s: permissions.network %q not in %v", rel, n, networks)
			}
		}

		if reqs, ok := stringList(obj["requires"]); !ok {
			t.Errorf("%s: requires must be a string array", rel)
		} else {
			requiresBy[rel] = reqs
		}
	}

	for rel, reqs := range requiresBy {
		for _, req := range reqs {
			if _, ok := ids[req]; !ok {
				t.Errorf("%s: unknown required capability %q", rel, req)
			}
		}
	}
}

func stringList(value any) ([]string, bool) {
	raw, ok := value.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// validateSchema is a deliberately minimal JSON Schema subset (type / const /
// enum / required / properties / additionalProperties / items / minItems /
// pattern), which is all capability-manifest.schema.json uses. No $ref. If the
// schema grows past this subset, extend here rather than silently passing.
func validateSchema(schema map[string]any, doc any, path string) []string {
	var out []string

	if want, ok := schema["const"]; ok && fmt.Sprint(want) != fmt.Sprint(doc) {
		out = append(out, fmt.Sprintf("%s: expected const %v, got %v", path, want, doc))
	}
	if raw, ok := schema["enum"].([]any); ok {
		hit := slices.ContainsFunc(raw, func(v any) bool { return fmt.Sprint(v) == fmt.Sprint(doc) })
		if !hit {
			out = append(out, fmt.Sprintf("%s: value %v not in enum %v", path, doc, raw))
		}
	}

	switch typ, _ := schema["type"].(string); typ {
	case "object":
		obj, ok := doc.(map[string]any)
		if !ok {
			return append(out, fmt.Sprintf("%s: expected object, got %T", path, doc))
		}
		props, _ := schema["properties"].(map[string]any)
		if raw, ok := schema["required"].([]any); ok {
			for _, r := range raw {
				if _, present := obj[fmt.Sprint(r)]; !present {
					out = append(out, fmt.Sprintf("%s: missing required %q", path, fmt.Sprint(r)))
				}
			}
		}
		additional, hasAdditional := schema["additionalProperties"].(bool)
		for key, value := range obj {
			sub, known := props[key].(map[string]any)
			if !known {
				if hasAdditional && !additional {
					out = append(out, fmt.Sprintf("%s: unexpected property %q", path, key))
				}
				continue
			}
			out = append(out, validateSchema(sub, value, path+"."+key)...)
		}
	case "array":
		arr, ok := doc.([]any)
		if !ok {
			return append(out, fmt.Sprintf("%s: expected array, got %T", path, doc))
		}
		if minItems, ok := schema["minItems"].(float64); ok && float64(len(arr)) < minItems {
			out = append(out, fmt.Sprintf("%s: expected at least %v items, got %d", path, minItems, len(arr)))
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, item := range arr {
				out = append(out, validateSchema(items, item, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
	case "string":
		s, ok := doc.(string)
		if !ok {
			return append(out, fmt.Sprintf("%s: expected string, got %T", path, doc))
		}
		if pattern, ok := schema["pattern"].(string); ok {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return append(out, fmt.Sprintf("%s: schema pattern %q is not RE2: %v", path, pattern, err))
			}
			if !re.MatchString(s) {
				out = append(out, fmt.Sprintf("%s: %q does not match %s", path, s, pattern))
			}
		}
	case "boolean":
		if _, ok := doc.(bool); !ok {
			out = append(out, fmt.Sprintf("%s: expected boolean, got %T", path, doc))
		}
	case "":
		// no type constraint (e.g. schemaVersion only carries const)
	default:
		out = append(out, fmt.Sprintf("%s: unsupported schema type %q in minimal validator", path, typ))
	}
	return out
}
