package navaudit

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

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
	return fmt.Sprintf("--- got ---\n%s\n--- want ---\n%s", clip(string(gb), 10000), clip(string(wb), 10000))
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
		slices.Sort(keys)
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
