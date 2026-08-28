package archguard

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// skillCmdRef 匹配 SKILL.md 里对本项目 CLI 的调用，形如 `epub run epub.structure.normalize`。
var skillCmdRef = regexp.MustCompile(`\bepub\s+([a-z][a-z0-9-]*)\s+([a-z0-9.\-]+)?`)

// legacyRef 匹配必须被消灭的旧执行面。
var legacyRef = regexp.MustCompile(`(python3?\s+scripts/[a-zA-Z0-9_]+\.py|\bscripts/[a-z0-9-]+\.sh)`)

// TestSkillsHaveNoScripts 断言 skills/ 是纯文档层。
//
// 架构决定：所有可执行逻辑收口到 Go CLI，skills/ 只留 SKILL.md，
// 内容是「何时用 + 调哪个命令 + 返回结构怎么读」。
func TestSkillsHaveNoScripts(t *testing.T) {
	root := repoRoot(t)
	if !dirExists(root, "skills") {
		t.Skip("bootstrap：skills/ 不存在")
	}
	err := filepath.WalkDir(filepath.Join(root, "skills"),
		func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			ext := filepath.Ext(p)
			if ext == ".py" || ext == ".sh" {
				rel, _ := filepath.Rel(root, p)
				t.Errorf("%s: skills/ 下不得有可执行脚本。\n"+
					"  所有逻辑收口到 Go CLI；SKILL.md 只描述如何调用与如何读返回结构。", rel)
			}
			return nil
		})
	if err != nil {
		t.Fatalf("遍历 skills 失败: %v", err)
	}
}

// TestSkillCommandsExist 断言 SKILL.md 引用的每个 CLI 命令都真实存在。
//
// 这是最容易腐烂的一处：文档写着某个命令，CLI 早改名了，AI 照着跑就炸。
// CLI 必须提供 `epub capabilities --json` 输出可用命令清单；
// 本测试拿它当事实来源，与 SKILL.md 里的引用对账。
func TestSkillCommandsExist(t *testing.T) {
	root := repoRoot(t)
	if !dirExists(root, "skills") {
		t.Skip("bootstrap：skills/ 不存在")
	}

	// 事实来源：契约目录里的 capability id 全集。
	known := map[string]bool{}
	paths, _ := filepath.Glob(filepath.Join(root, "contracts", "capabilities", "v1", "*.json"))
	for _, p := range paths {
		id := strings.TrimSuffix(filepath.Base(p), ".json")
		known[id] = true
	}
	if len(known) == 0 {
		t.Fatal("未找到任何 capability 契约，无法校验 SKILL.md 的命令引用")
	}

	var bad []string
	err := filepath.WalkDir(filepath.Join(root, "skills"),
		func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Base(p) != "SKILL.md" {
				return err
			}
			raw, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			rel, _ := filepath.Rel(root, p)
			for _, m := range skillCmdRef.FindAllStringSubmatch(string(raw), -1) {
				verb, arg := m[1], m[2]
				if verb != "run" || arg == "" {
					continue // 只校验 `epub run <capability-id>` 这一形态
				}
				if !known[arg] {
					bad = append(bad, rel+": 引用了不存在的 capability "+arg)
				}
			}
			return nil
		})
	if err != nil {
		t.Fatalf("遍历 skills 失败: %v", err)
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("%s\n"+
			"  SKILL.md 只能引用 contracts/capabilities/v1/ 里真实存在的 capability id。\n"+
			"  改名 capability 时必须同步全部 SKILL.md —— 这正是本测试存在的理由。", b)
	}
}

// TestNoLegacyExecutionSurface 是迁移期的**棘轮**：
// 记录在案的旧执行面引用只许减少，不许新增。
//
// 基线文件 tools/parity/legacy-refs.txt 每行一个 "<相对路径>\t<引用>"。
// 迁移一个 capability 就从基线里删掉对应行；**永远不许往里加行**。
func TestNoLegacyExecutionSurface(t *testing.T) {
	root := repoRoot(t)
	baselinePath := filepath.Join(root, "tools", "parity", "legacy-refs.txt")

	baseline := map[string]bool{}
	raw, err := os.ReadFile(baselinePath)
	if os.IsNotExist(err) {
		t.Skip("bootstrap：tools/parity/legacy-refs.txt 尚未生成。\n" +
			"  W5 开始前必须生成基线，否则棘轮无从起步。")
	}
	if err != nil {
		t.Fatalf("读取基线失败: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			baseline[line] = true
		}
	}

	// 唯一豁免：迁移 SPEC 本身必须点名旧执行面，否则没法描述要迁什么。
	// 精确路径豁免，不用通配 —— 否则"把旧引用塞进文档目录"就能绕过棘轮。
	const specExempt = "docs/final/SPEC-go-architecture.md"

	// 根目录的高流量指令文档也纳入棘轮。
	// CHANGELOG.md 故意不纳入 —— 它是历史记录，不该被改写。
	var targets []string
	for _, f := range []string{"AGENTS.md", "README.md", "CONTRIBUTING.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			targets = append(targets, f)
		}
	}

	var added []string
	for _, dir := range append(targets, "skills", "docs", "templates", "hooks") {
		if !dirExists(root, dir) && !strings.HasSuffix(dir, ".md") {
			continue
		}
		_ = filepath.WalkDir(filepath.Join(root, dir),
			func(p string, d fs.DirEntry, err error) error {
				// hooks/ 是第三执行面（git hook 里直接调 scripts/），
				// 不是 markdown，但同样必须随迁移收敛。
				inHooks := strings.Contains(filepath.ToSlash(p), "/hooks/")
				if err != nil || d.IsDir() || (!strings.HasSuffix(p, ".md") && !inHooks) {
					return nil
				}
				body, rerr := os.ReadFile(p)
				if rerr != nil {
					return nil
				}
				rel, _ := filepath.Rel(root, p)
				if filepath.ToSlash(rel) == specExempt {
					return nil
				}
				for _, m := range legacyRef.FindAllString(string(body), -1) {
					entry := rel + "\t" + m
					if !baseline[entry] {
						added = append(added, entry)
					}
				}
				return nil
			})
	}
	sort.Strings(added)
	for _, a := range added {
		t.Errorf("新增了旧执行面引用：%s\n"+
			"  棘轮只许减少。要调用能力请用 `epub run <capability-id>`。\n"+
			"  不要把这条加进 tools/parity/legacy-refs.txt —— 基线只删不增。",
			strings.ReplaceAll(a, "\t", " → "))
	}
}
