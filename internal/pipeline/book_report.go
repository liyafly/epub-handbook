package pipeline

// 书级构建的输出整理：终端每项检查一行（必要时附几条缩进说明），
// 详细内容汇总进 <状态目录>/report.html。报告只是给人看的摘要，不是 gate；
// 各步骤原始报告（JSON / 文本）照旧保留在同一目录。

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// reportHTMLName 是构建摘要的文件名（位于状态目录）。
const reportHTMLName = "report.html"

// kindleSizeHint 超过它时提示 Send to Kindle 可能拒收（邮件与 App 通常限 50 MB，
// 网页上传更大；具体数值以亚马逊当前说明为准）。
const kindleSizeHint = 50 << 20

// maxFindingLines 是终端里每项检查最多列出的 finding 条数。
const maxFindingLines = 3

// bookStep 是一项检查的结果，供终端与 report.html 共用。
type bookStep struct {
	Status string // PASS | FAIL | WARN | INFO
	Label  string
	Report string   // 相对书根的原始报告路径，可为空
	Lines  []string // 补充说明（警告标题、缺字摘要等）
}

func (r *bookRunner) addStep(step bookStep) {
	r.steps = append(r.steps, step)
}

// findingLines 把 findings 压缩成去重后的短行：error 全列，warn 最多 maxFindingLines 条。
func findingLines(findings []report.Finding, level string, limit int) []string {
	seen := map[string]bool{}
	var lines []string
	extra := 0
	for _, f := range findings {
		if f.Level != level {
			continue
		}
		text := f.Title
		if text == "" {
			text = f.ID
		}
		if f.Location != "" {
			text += " — " + f.Location
		}
		if level == "error" && f.Detail != "" {
			text += " (" + f.Detail + ")"
		}
		if seen[text] {
			continue
		}
		seen[text] = true
		if limit > 0 && len(lines) >= limit {
			extra++
			continue
		}
		lines = append(lines, text)
	}
	if extra > 0 {
		lines = append(lines, fmt.Sprintf("… %d more (see report)", extra))
	}
	return lines
}

func printIndented(w interface{ Write([]byte) (int, error) }, lines []string) {
	for _, line := range lines {
		fmt.Fprintf(w, "  %s\n", line)
	}
}

// fontWarningSummary 把 provider 报告里各字体的 warning 汇总：缺字合成一行，
// 正文缺字与 CSS/配置/生成文本里保守加入的字分开计数；其他 warning 去重保留。
func fontWarningSummary(data []byte) []string {
	var doc struct {
		Facts map[string]json.RawMessage `json:"facts"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return nil
	}
	var provider struct {
		Fonts []struct {
			Target    string   `json:"target"`
			Missing   int      `json:"notInMasterCount"`
			BodyCount *int     `json:"notInMasterBodyCount"`
			Warnings  []string `json:"warnings"`
		} `json:"fonts"`
	}
	if json.Unmarshal(doc.Facts["epub.font.subset.providerReport"], &provider) != nil {
		return nil
	}
	var lines []string
	// 各字体缺字会重叠，不能相加；只报缺字最多的那一款。
	fontsMissing, worstBody, worstOther, worstTotal := 0, -1, 0, 0
	worstTarget := ""
	bodyKnown := true
	seen := map[string]bool{}
	for _, font := range provider.Fonts {
		if font.Missing > 0 {
			fontsMissing++
			body := font.Missing
			if font.BodyCount != nil {
				body = *font.BodyCount
			} else {
				bodyKnown = false
			}
			if body > worstBody || (body == worstBody && font.Missing > worstTotal) {
				worstBody, worstOther, worstTotal, worstTarget = body, font.Missing-body, font.Missing, font.Target
			}
		}
		for _, warning := range font.Warnings {
			if font.Missing > 0 && (strings.Contains(warning, "required characters are not in the master font") ||
				strings.Contains(warning, "source coverage warning")) {
				continue
			}
			if !seen[warning] {
				seen[warning] = true
				lines = append(lines, warning)
			}
		}
	}
	if fontsMissing > 0 {
		var summary string
		switch {
		case !bodyKnown:
			summary = fmt.Sprintf("fonts: %d of %d fonts lack characters (most: %s, %d)", fontsMissing, len(provider.Fonts), worstTarget, worstTotal)
		case worstBody == 0:
			summary = fmt.Sprintf("fonts: body text fully covered; %d of %d fonts lack only CSS/config/generated characters (most: %s, %d)",
				fontsMissing, len(provider.Fonts), worstTarget, worstOther)
		default:
			summary = fmt.Sprintf("fonts: %d of %d fonts lack body characters (most: %s, %d in body + %d from CSS/config/generated text)",
				fontsMissing, len(provider.Fonts), worstTarget, worstBody, worstOther)
		}
		if worstBody > 0 {
			summary += "; this counts the whole book, and the fixed-role fonts check lists what is really used"
		}
		lines = append([]string{summary}, lines...)
	}
	return lines
}

// maxFixedRoleLines 限制终端列出的缺字条数；完整清单在报告里。
const maxFixedRoleLines = 10

// fixedRoleWarningLines 把 epub-font 的固定角色检查结果转成警告说明；没有缺字时返回空。
// 缺少结果字段说明 provider 版本太旧，不能当作没有缺字。
func fixedRoleWarningLines(data []byte) ([]string, error) {
	var doc struct {
		Missing *[]struct {
			Char      string `json:"char"`
			Codepoint string `json:"codepoint"`
			Family    string `json:"family"`
			Font      string `json:"font"`
			Count     int    `json:"count"`
			Location  string `json:"first_location"`
		} `json:"fixed_role_missing"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("epub-font coverage did not return JSON: %w", err)
	}
	if doc.Missing == nil {
		return nil, errors.New("epub-font is too old for this check; reinstall it with `uv tool install --editable --reinstall tools-font/epub-font`")
	}
	if len(*doc.Missing) == 0 {
		return nil, nil
	}
	lines := []string{fmt.Sprintf("characters not in the fonts chosen for their text: %d; readers show them in a fallback font", len(*doc.Missing))}
	for i, m := range *doc.Missing {
		if i == maxFixedRoleLines {
			lines = append(lines, fmt.Sprintf("... and %d more, see the report", len(*doc.Missing)-maxFixedRoleLines))
			break
		}
		lines = append(lines, fmt.Sprintf("%s %s: not in %q (%s), used %d, first in %s", m.Char, m.Codepoint, m.Family, m.Font, m.Count, m.Location))
	}
	lines = append(lines, "check them in your real readers: the fallback may look fine. If not, use a font with these characters or wrap them in a .rare font")
	return lines, nil
}

// sizeHint 在成品超过 Send to Kindle 常见上限时返回提示。
func sizeHint(path string) string {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= kindleSizeHint {
		return ""
	}
	return fmt.Sprintf("%s is %.1f MB: Send to Kindle by email or app may reject files over 50 MB (the web uploader takes more; check Amazon's current limits)",
		filepath.Base(path), float64(info.Size())/(1<<20))
}

// writeReportHTML 把本次构建的各项检查写成一页 HTML 摘要。失败也写，方便定位。
func (r *bookRunner) writeReportHTML(layout bookLayout, ok bool) string {
	if len(r.steps) == 0 {
		return ""
	}
	var b strings.Builder
	result := "Build passed"
	if !ok {
		result = "Build failed"
	}
	title := filepath.Base(layout.BookDir)
	fmt.Fprintf(&b, `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s – %s</title>
<style>
:root{color-scheme:light dark;--fg:#1d1d1f;--bg:#fff;--muted:#6e6e73;--line:#d2d2d7;--pass:#1a7f37;--fail:#cf222e;--warn:#9a6700;--info:#0969da}
@media (prefers-color-scheme:dark){:root{--fg:#f5f5f7;--bg:#1c1c1e;--muted:#a1a1a6;--line:#3a3a3c;--pass:#3fb950;--fail:#f85149;--warn:#d29922;--info:#58a6ff}}
body{margin:0 auto;max-width:860px;padding:24px 16px;font:15px/1.55 -apple-system,"Segoe UI",system-ui,sans-serif;color:var(--fg);background:var(--bg)}
h1{font-size:20px;margin:0 0 4px}.meta{color:var(--muted);margin:0 0 20px}
.step{border-top:1px solid var(--line);padding:10px 0}.tag{display:inline-block;min-width:3.2em;font-weight:600}
.PASS{color:var(--pass)}.FAIL{color:var(--fail)}.WARN{color:var(--warn)}.INFO{color:var(--info)}
ul{margin:6px 0 0;padding-left:20px}li{overflow-wrap:anywhere}code{font-size:13px;color:var(--muted)}
</style></head><body>
<h1>%s</h1><p class="meta">%s · %s</p>
`, html.EscapeString(title), result, html.EscapeString(title), result, time.Now().Format("2006-01-02 15:04"))
	for _, step := range r.steps {
		fmt.Fprintf(&b, `<div class="step"><span class="tag %s">%s</span> %s`, step.Status, step.Status, html.EscapeString(step.Label))
		if step.Report != "" {
			fmt.Fprintf(&b, ` <code>%s</code>`, html.EscapeString(step.Report))
		}
		if len(step.Lines) > 0 {
			b.WriteString("<ul>")
			for _, line := range step.Lines {
				fmt.Fprintf(&b, "<li>%s</li>", html.EscapeString(line))
			}
			b.WriteString("</ul>")
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</body></html>\n")
	path := filepath.Join(layout.StateDir, reportHTMLName)
	if err := zipfs.WriteFileAtomic(path, []byte(b.String()), 0o644); err != nil {
		return ""
	}
	return path
}

// warnStepLines 返回 PASS 步骤要在终端附带的警告说明。
func warnStepLines(env report.Envelope) []string {
	return findingLines(slices.Clone(env.Findings), "warn", maxFindingLines)
}
