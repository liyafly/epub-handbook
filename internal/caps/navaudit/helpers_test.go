package navaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func testContext() context.Context { return context.Background() }

// runPyPreflight 调 Python preflight oracle，返回 JSON 解析后的 map 与退出码。
func runPyPreflight(t *testing.T, repoRoot, scripts, epub string) (map[string]any, int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	script := filepath.Join(scripts, "epub_preflight_harness.py")
	cmd := exec.Command("python3", script, epub, "--format", "json")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("运行 python oracle 失败: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("oracle 输出不是 JSON: %v", err)
	}
	return report, code
}

// diffJSON 比较任意 JSON 值，返回不一致时的双方摘要（空串表示一致）。
func diffJSON(t *testing.T, got, want any) string {
	t.Helper()
	gb, err := json.Marshal(normalizeJSON(roundTrip(t, got)))
	if err != nil {
		return fmt.Sprintf("marshal got: %v", err)
	}
	wb, err := json.Marshal(normalizeJSON(roundTrip(t, want)))
	if err != nil {
		return fmt.Sprintf("marshal want: %v", err)
	}
	if string(gb) == string(wb) {
		return ""
	}
	return fmt.Sprintf("--- go ---\n%s\n--- python ---\n%s", clip(string(gb), 2000), clip(string(wb), 2000))
}

// roundTrip 把任意值经 JSON 往返成 map/slice，消除结构体字段序差异。
func roundTrip(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("roundTrip marshal: %v", err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("roundTrip unmarshal: %v", err)
	}
	return out
}

// normalizeJSON 把 map 键序归一（Go 侧 legacyReport 内含 map[string]bool 等），
// 便于比较语义内容而非键排列。
func normalizeJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			out[k] = normalizeJSON(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalizeJSON(val)
		}
		return out
	case map[string]bool:
		m := map[string]any{}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			m[k] = x[k]
		}
		return m
	default:
		return v
	}
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
