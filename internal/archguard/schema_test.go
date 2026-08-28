package archguard

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestReportSchema 断言 INV-6：所有 golden 报告都合 contracts/schemas/v1 的 schema。
//
// 报告格式是 agent、未来的 GUI、以及 parity gate P2 三方的契约，不能悄悄漂移。
//
// 这里用的是**最小可用**的 JSON Schema 子集校验器（required / type / const /
// enum / properties / additionalProperties / items / $ref-本地文件），
// 足够覆盖 v1 这批 schema 的形状。若日后 schema 用上 allOf/oneOf/pattern 等，
// 换成 SPEC §9.3 已预授权的测试专用 schema 库，不要在这里堆特例。
func TestReportSchema(t *testing.T) {
	root := repoRoot(t)
	schemaDir := filepath.Join(root, "contracts", "schemas", "v1")

	// 注意：Go 的 filepath.Glob 不支持 ** 递归，必须自己走目录树。
	var goldens []string
	if dirExists(root, "testdata") {
		err := filepath.WalkDir(filepath.Join(root, "testdata"),
			func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(p, ".report.json") {
					goldens = append(goldens, p)
				}
				return nil
			})
		if err != nil {
			t.Fatalf("遍历 testdata 失败: %v", err)
		}
	}

	if len(goldens) == 0 {
		if !dirExists(root, "testdata") {
			t.Skip("bootstrap：testdata/ 尚未创建，暂缓。" +
				"  每迁移一个 capability 都必须落 golden 报告（SPEC §6.1 第 5 项）。")
		}
		t.Fatal("testdata/ 存在但没有任何 *.report.json。\n" +
			"  §4 禁止清单第 8 条：不写 parity 测试就迁移 capability 是架构跑偏。")
	}

	loader := &schemaLoader{dir: schemaDir, cache: map[string]map[string]any{}}
	sort.Strings(goldens)

	for _, g := range goldens {
		raw, err := os.ReadFile(g)
		if err != nil {
			t.Errorf("读取 %s 失败: %v", g, err)
			continue
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Errorf("%s 不是合法 JSON: %v", g, err)
			continue
		}
		schema, err := loader.load("run-report.schema.json")
		if err != nil {
			t.Fatalf("加载 run-report schema 失败: %v", err)
		}
		rel, _ := filepath.Rel(root, g)
		for _, problem := range validate(loader, schema, doc, "$") {
			t.Errorf("%s: %s\n"+
				"  INV-6：报告必须合 contracts/schemas/v1 的 schema。\n"+
				"  不要改 schema 去迁就报告 —— schema 是对外契约。", rel, problem)
		}
	}
}

// ---- 最小 JSON Schema 子集 ----

type schemaLoader struct {
	dir   string
	cache map[string]map[string]any
}

func (l *schemaLoader) load(name string) (map[string]any, error) {
	if s, ok := l.cache[name]; ok {
		return s, nil
	}
	raw, err := os.ReadFile(filepath.Join(l.dir, name))
	if err != nil {
		return nil, err
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	l.cache[name] = s
	return s, nil
}

func validate(l *schemaLoader, schema map[string]any, doc any, path string) []string {
	var out []string

	if ref, ok := schema["$ref"].(string); ok {
		target, err := l.load(filepath.Base(ref))
		if err != nil {
			return []string{fmt.Sprintf("%s: 无法解析 $ref %q: %v", path, ref, err)}
		}
		return validate(l, target, doc, path)
	}

	if want, ok := schema["const"]; ok && fmt.Sprint(want) != fmt.Sprint(doc) {
		out = append(out, fmt.Sprintf("%s: 应为常量 %v，实际 %v", path, want, doc))
	}

	if raw, ok := schema["enum"].([]any); ok {
		hit := false
		for _, v := range raw {
			if fmt.Sprint(v) == fmt.Sprint(doc) {
				hit = true
				break
			}
		}
		if !hit {
			out = append(out, fmt.Sprintf("%s: 值 %v 不在枚举 %v 中", path, doc, raw))
		}
	}

	switch typ, _ := schema["type"].(string); typ {
	case "object":
		obj, ok := doc.(map[string]any)
		if !ok {
			return append(out, fmt.Sprintf("%s: 应为 object，实际 %T", path, doc))
		}
		props, _ := schema["properties"].(map[string]any)
		if raw, ok := schema["required"].([]any); ok {
			for _, r := range raw {
				key := fmt.Sprint(r)
				if _, ok := obj[key]; !ok {
					out = append(out, fmt.Sprintf("%s: 缺少必填字段 %q", path, key))
				}
			}
		}
		if extra, ok := schema["additionalProperties"].(bool); ok && !extra {
			var unknown []string
			for k := range obj {
				if _, ok := props[k]; !ok {
					unknown = append(unknown, k)
				}
			}
			sort.Strings(unknown)
			for _, k := range unknown {
				out = append(out, fmt.Sprintf("%s: 出现 schema 未定义的字段 %q", path, k))
			}
		}
		for k, v := range obj {
			sub, ok := props[k].(map[string]any)
			if !ok {
				continue
			}
			out = append(out, validate(l, sub, v, path+"."+k)...)
		}

	case "array":
		arr, ok := doc.([]any)
		if !ok {
			return append(out, fmt.Sprintf("%s: 应为 array，实际 %T", path, doc))
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, v := range arr {
				out = append(out, validate(l, items, v, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}

	case "string":
		if _, ok := doc.(string); !ok {
			out = append(out, fmt.Sprintf("%s: 应为 string，实际 %T", path, doc))
		}
	}

	return out
}
