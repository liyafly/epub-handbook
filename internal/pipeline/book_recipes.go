package pipeline

// epub book recipes：列出组合效果配方、打印配方 CSS，或把它追加进一本书的 design.css。
//
// 配方来自 templates/recipes/<名称>/（recipe.md + recipe.css），在仓库外运行时用二进制
// 内嵌的副本。追加只在 design.css 末尾加一段带标记的 CSS，不解析、不改写已有规则；
// 同名配方已在文件里时拒绝重复追加。

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// BookRecipesOptions 是 epub book recipes 的输入。
type BookRecipesOptions struct {
	Name string // 空表示列出全部
	Add  string // 非空时把配方追加进该书的 design.css
}

type recipeInfo struct {
	Name     string
	Title    string
	Fallback string
}

func recipeMarker(name string) string { return "/* recipe: " + name + " " }

// BookRecipes 执行 epub book recipes，返回退出码。
func BookRecipes(opts BookRecipesOptions, bio BookIO) int {
	return bookExit(bio, bookRecipes(opts, bio))
}

func bookRecipes(opts BookRecipesOptions, bio BookIO) error {
	root, _ := FindRepoRoot()
	recipes, err := repositoryFS(root, "templates/recipes")
	if err != nil {
		return err
	}
	if opts.Name == "" {
		if opts.Add != "" {
			return usageErrorf("--add needs a recipe name: epub book recipes <name> --add <book-directory>")
		}
		return listRecipes(recipes, bio)
	}
	if !validRecipeName(opts.Name) {
		return usageErrorf("invalid recipe name: %s", opts.Name)
	}
	css, err := fs.ReadFile(recipes, path.Join(opts.Name, "recipe.css"))
	if err != nil {
		return usageErrorf("unknown recipe %q (run `epub book recipes` for the list)", opts.Name)
	}
	if opts.Add == "" {
		_, _ = bio.out().Write(css)
		return nil
	}
	return addRecipe(opts, css, bio)
}

func validRecipeName(name string) bool {
	return name != "" && !strings.ContainsFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
	})
}

func listRecipes(recipes fs.FS, bio BookIO) error {
	entries, err := fs.ReadDir(recipes, ".")
	if err != nil {
		return err
	}
	var infos []recipeInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		md, err := fs.ReadFile(recipes, path.Join(entry.Name(), "recipe.md"))
		if err != nil {
			continue // dist/、reader-evidence/ 等不是配方
		}
		if _, err := fs.Stat(recipes, path.Join(entry.Name(), "recipe.css")); err != nil {
			continue
		}
		info := parseRecipeMD(md)
		info.Name = entry.Name()
		infos = append(infos, info)
	}
	slices.SortFunc(infos, func(a, b recipeInfo) int { return strings.Compare(a.Name, b.Name) })
	width := 0
	for _, info := range infos {
		width = max(width, len(info.Name))
	}
	out := bio.out()
	for _, info := range infos {
		fmt.Fprintf(out, "%-*s  %s", width, info.Name, info.Title)
		if info.Fallback != "" {
			fmt.Fprintf(out, "（%s）", info.Fallback)
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, "\nShow CSS: epub book recipes <name>    Add to a book: epub book recipes <name> --add <book-directory>")
	return nil
}

// parseRecipeMD 取一级标题，以及第一张表（“阅读器一览”）首个数据行的最后一列（降级路线）。
func parseRecipeMD(md []byte) recipeInfo {
	var info recipeInfo
	scanner := bufio.NewScanner(bytes.NewReader(md))
	tableRow := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if info.Title == "" && strings.HasPrefix(line, "# ") {
			info.Title = strings.TrimSpace(line[2:])
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		tableRow++
		if tableRow == 3 { // 表头、分隔行之后的第一行
			cells := strings.Split(strings.Trim(line, "|"), "|")
			info.Fallback = strings.TrimSpace(cells[len(cells)-1])
			break
		}
	}
	return info
}

func addRecipe(opts BookRecipesOptions, css []byte, bio BookIO) error {
	layout, err := resolveBookLayout(opts.Add, readBookEnv())
	if err != nil {
		return err
	}
	design := filepath.Join(layout.EpubDir, "OEBPS", "Styles", "design.css")
	existing, err := os.ReadFile(design)
	if err != nil {
		return usageErrorf("no OEBPS/Styles/design.css in %s; copy the CSS by hand: epub book recipes %s", layout.EpubDir, opts.Name)
	}
	if bytes.Contains(existing, []byte(recipeMarker(opts.Name))) {
		return usageErrorf("recipe %s is already in %s", opts.Name, layout.displayPath(design))
	}
	var b bytes.Buffer
	b.Write(existing)
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		b.WriteByte('\n')
	}
	if len(existing) > 0 {
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "%s(templates/recipes/%s) */\n", recipeMarker(opts.Name), opts.Name)
	b.Write(css)
	if !bytes.HasSuffix(css, []byte("\n")) {
		b.WriteByte('\n')
	}
	if err := zipfs.WriteFileAtomic(design, b.Bytes(), 0o644); err != nil {
		return err
	}
	out := bio.out()
	fmt.Fprintf(out, "Added recipe %s to %s\n", opts.Name, layout.displayPath(design))
	fmt.Fprintf(out, "Next: use its classes in your XHTML (see templates/recipes/%s/sample.xhtml), then epub book preview.\n", opts.Name)
	return nil
}
