// Capability-specific ID allocation does not mutate the caller's used set.
package typography

import (
	"strconv"
	"strings"
)

// uniqueID 复刻 epub_lib.unique_id：候选名净化 → 数字开头加 x- →
// 与 idSeen 冲突时追加 -2/-3…（调用方需把新 id 写回 idSeen）。
func uniqueID(idSeen map[string]bool, base string) string {
	candidate := idSanitizeRe.ReplaceAllString(base, "-")
	candidate = strings.Trim(candidate, "-")
	if candidate == "" {
		candidate = "item"
	}
	if candidate[0] >= '0' && candidate[0] <= '9' {
		candidate = "x-" + candidate
	}
	index := 2
	result := candidate
	for idSeen[result] {
		result = candidate + "-" + strconv.Itoa(index)
		index++
	}
	return result
}
