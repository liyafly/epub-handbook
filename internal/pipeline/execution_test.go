package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestRegistryMatchesContractExecution 是契约与注册方式的对账断言。
//
// 为什么需要它：`epub run` 怎么调一条能力（输入是 EPUB 还是目录、写一个产物
// 还是多个还是不写）此前**只**以 register.go 里的四张 id 白名单存在，契约里
// 没有任何字段描述它。后果有两个：
//
//  1. `contracts/` 被声明为「机器契约的唯一事实来源」，但对执行形态并不成立；
//  2. 两者可以静默分叉 —— 而且当时已经分叉了：`epub.notes.popup.normalize`
//     契约写 `requiresWriteAccess: true` 却注册为只读，
//     `epub.style.demo.maintain` 契约写 planner + 需写权限却永不写盘。
//
// 现在运行时读契约（run.go 的 noBookCap / sourceInputCap / multiOutputCap /
// chainNeedsWrite），注册点的四张表退化为「作者声明的形态」。这条测试要求
// 二者逐条一致：谁改了一边忘了另一边，立刻红。
func TestRegistryMatchesContractExecution(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "contracts", "capabilities", "v1")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	seen := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var c Contract
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}
		seen++

		// 契约声明的形态 vs 注册点声明的形态。
		wantInput := ExecInputEpub
		switch {
		case IsSourceInput(c.ID):
			wantInput = ExecInputSourcePath
		case IsNoBook(c.ID):
			wantInput = ExecInputEpubOrTree
		}
		if c.Execution.Input != wantInput {
			t.Errorf("%s: 契约 execution.input=%q，但注册方式对应 %q",
				c.ID, c.Execution.Input, wantInput)
		}

		// 注册点只显式声明 multi 与 none 两种形态（registerMultiOutput /
		// registerReadOnly / registerNoBook / registerSourceInput）；用普通
		// register 登记的能力没在注册点表态，它的落盘语义由
		// permissions.requiresWriteAccess 区分 —— 那一条由下面的自洽断言覆盖。
		// 未实现的 B 类能力没有注册点，同理只走自洽断言。
		if Implemented(c.ID) {
			switch {
			case IsMultiOutput(c.ID):
				if c.Execution.Output != ExecOutputMulti {
					t.Errorf("%s: registerMultiOutput 但契约 execution.output=%q",
						c.ID, c.Execution.Output)
				}
			case IsReadOnly(c.ID), IsNoBook(c.ID), IsSourceInput(c.ID):
				if c.Execution.Output != ExecOutputNone {
					t.Errorf("%s: 注册为只读/无书/源材料输入，但契约 execution.output=%q",
						c.ID, c.Execution.Output)
				}
			default:
				if c.Execution.Output == ExecOutputMulti {
					t.Errorf("%s: 契约 execution.output=multi，但没有走 registerMultiOutput",
						c.ID)
				}
			}
		}

		// 自洽：不落盘的能力不该声明需要写权限（这正是此前那两条契约在说谎的
		// 地方）；反之要落盘就必须声明写权限。docguard 也查同一条，这里再查
		// 一遍是因为 pipeline 的 chainNeedsWrite 直接依赖它。
		if (c.Execution.Output == ExecOutputNone) == c.Permissions.RequiresWriteAccess {
			t.Errorf("%s: execution.output=%q 与 permissions.requiresWriteAccess=%v 矛盾",
				c.ID, c.Execution.Output, c.Permissions.RequiresWriteAccess)
		}
	}
	if seen == 0 {
		t.Fatal("没有读到任何契约 —— 用例失效")
	}
}

// TestChainNeedsWriteFollowsContract 断言「要不要 --output」这个判断来自契约
// 而不是 Go 侧白名单：只读链（含 planner / 只读能力）不得索要 --output，
// 写出型链必须索要。此前这个判断读的是 permissions.requiresWriteAccess 再减去
// 三张白名单，等于把契约里那两处矛盾用代码绕开。
func TestChainNeedsWriteFollowsContract(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id   string
		want bool
	}{
		{"epub.structure.normalize", true}, // 写单产物
		{"epub.package.migrate.epub3", true},
		{"epub.package.split", true},      // 多产物仍是写出型，只是走 output_dir
		{"epub.package.nav.audit", false}, // 只读
		{"epub.notes.popup.normalize", false},
		{"epub.style.demo.maintain", false},
		{"epub.source.intake", false},
	}
	for _, tc := range cases {
		chain, err := ResolveChain(root, tc.id)
		if err != nil {
			t.Errorf("%s: ResolveChain: %v", tc.id, err)
			continue
		}
		if got := chainNeedsWrite(chain); got != tc.want {
			t.Errorf("%s: chainNeedsWrite = %v, want %v", tc.id, got, tc.want)
		}
	}
}
