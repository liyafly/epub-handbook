// Package pipeline 读契约 → 依 requires 排序 → 跑 stage → 汇总报告（SPEC §1 第 1 层）。
//
// 它是唯一知道「全部 capability」的地方：capability 之间禁止互相 import，
// 上游结果经 Upstream 注入（SPEC §4 第 3 条）。
package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	epubhandbook "github.com/liyafly/epub-handbook"
)

// Contract 是 capability 契约的运行时投影（contracts/capabilities/v1/*.json）。
type Contract struct {
	SchemaVersion string   `json:"schemaVersion"`
	ID            string   `json:"id"`
	Version       string   `json:"version"`
	Kind          string   `json:"kind"`
	RedLines      []string `json:"redLines"`
	Requires      []string `json:"requires"`
	Permissions   struct {
		RequiresWriteAccess bool   `json:"requiresWriteAccess"`
		Network             string `json:"network"`
	} `json:"permissions"`
	// Execution 是执行形态：pipeline 据此决定怎么调这条能力。它此前只以
	// register.go 里的四张 id 白名单存在，契约完全不提 —— 于是「contracts/
	// 是机器契约唯一事实来源」对执行形态并不成立，而且有两条契约与执行面
	// 直接矛盾（声明需写权限却永不写盘）。现在运行时读契约，注册方式与
	// 契约的一致性由 TestRegistryMatchesContractExecution 对账。
	Execution struct {
		// Input: epub（必须是 EPUB 文件，会被 book.Open）
		//      | epub-or-tree（可缺省或指向目录 = 源树模式）
		//      | source-path（目录或任意普通文件，永不 book.Open）
		Input string `json:"input"`
		// Output: single（pipeline 写一次 --output）
		//       | multi（能力自行写 output_dir 下的多个产物）
		//       | none（只读，不写产物）
		Output string `json:"output"`
	} `json:"execution"`
}

// ErrUnknownCapability 表示契约目录里没有这个 id。
var ErrUnknownCapability = errors.New("pipeline: unknown capability")

// FindRepoRoot 优先使用显式仓库根或从 cwd 向上寻找；没有检出时返回空根，
// 表示调用方应使用二进制内嵌资源。
func FindRepoRoot() (string, error) {
	if root := strings.TrimSpace(os.Getenv("EPUB_HANDBOOK_ROOT")); root != "" {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", fmt.Errorf("pipeline: EPUB_HANDBOOK_ROOT 无效: %w", err)
		}
		if hasRepositoryContracts(abs) {
			return abs, nil
		}
		return "", fmt.Errorf("pipeline: EPUB_HANDBOOK_ROOT %q 缺少 contracts/capabilities/v1", abs)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if hasRepositoryContracts(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func hasRepositoryContracts(root string) bool {
	info, err := os.Stat(filepath.Join(root, "contracts", "capabilities", "v1"))
	return err == nil && info.IsDir()
}

func repositoryFS(root, subdir string) (fs.FS, error) {
	base := epubhandbook.EmbeddedFS()
	if root != "" {
		base = os.DirFS(root)
	}
	return fs.Sub(base, filepath.ToSlash(subdir))
}

func readRepositoryFile(root, name string) ([]byte, error) {
	if root != "" {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	}
	return fs.ReadFile(epubhandbook.EmbeddedFS(), filepath.ToSlash(name))
}

// ContractsDir 返回仓库内的契约目录。
func ContractsDir(root string) string {
	return filepath.Join(root, "contracts", "capabilities", "v1")
}

// LoadContract 读取单个契约。
func LoadContract(root, id string) (*Contract, error) {
	if !validID(id) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownCapability, id)
	}
	raw, err := readRepositoryFile(root, filepath.ToSlash(filepath.Join("contracts", "capabilities", "v1", id+".json")))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownCapability, id)
	}
	var c Contract
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("pipeline: 契约 %s 解析失败: %w", id, err)
	}
	if c.ID != id {
		return nil, fmt.Errorf("pipeline: 契约文件 %s 的 id 字段不符: %q", id, c.ID)
	}
	return &c, nil
}

// AllContracts 读取全部契约，按 id 排序。
func AllContracts(root string) ([]Contract, error) {
	contractFS, err := repositoryFS(root, filepath.Join("contracts", "capabilities", "v1"))
	if err != nil {
		return nil, err
	}
	paths, err := fs.Glob(contractFS, "*.json")
	if err != nil {
		return nil, err
	}
	out := make([]Contract, 0, len(paths))
	for _, p := range paths {
		raw, err := fs.ReadFile(contractFS, p)
		if err != nil {
			return nil, err
		}
		var c Contract
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("pipeline: 契约 %s 解析失败: %w", p, err)
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ResolveChain 计算从 id 出发（含自身）的执行链：拓扑序，依赖在前。
// 根 capability 或任一 requires 缺失都报 ErrUnknownCapability，环形依赖报错。
func ResolveChain(root, id string) ([]Contract, error) {
	seen := map[string]bool{}
	var out []Contract
	visiting := map[string]bool{}
	var visit func(cid string) error
	visit = func(cid string) error {
		if seen[cid] {
			return nil
		}
		if visiting[cid] {
			return fmt.Errorf("pipeline: requires 环: %s", cid)
		}
		visiting[cid] = true
		c, err := LoadContract(root, cid)
		if err != nil {
			visiting[cid] = false
			return err
		}
		for _, dep := range c.Requires {
			if err := visit(dep); err != nil {
				visiting[cid] = false
				if errors.Is(err, ErrUnknownCapability) {
					return fmt.Errorf("pipeline: %s requires unknown capability %s: %w", cid, dep, err)
				}
				return err
			}
		}
		visiting[cid] = false
		seen[cid] = true
		out = append(out, *c)
		return nil
	}
	if err := visit(id); err != nil {
		return nil, err
	}
	return out, nil
}

func validID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-':
		default:
			return false
		}
	}
	return true
}
