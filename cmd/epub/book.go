package main

// epub book ...：书级工作流子命令组。只做参数解析，业务全在 internal/pipeline。

import (
	"flag"
	"fmt"
	"os"

	"github.com/liyafly/epub-handbook/internal/pipeline"
)

const bookUsage = `用法:
  epub book new <书目录> [--from-preset <名称>] [--title <书名>] [--author <作者>] [--git]
  epub book import <原书.epub> <书目录> [--git]
  epub book build [<书目录>] [--variant <名称>]
  epub book preview [<书目录>] [--variant <名称>] [--open[=<应用>]]
  epub book fonts [<书目录>] [--update-lock]
  epub book clean [<书目录>]
  epub book accept-dist [<书目录>] [--variant <名称>]
  epub book recipes [<配方> [--add <书目录>]]
  epub book reader-layer [<书目录>]

新布局（含 book.json）与旧布局（03 制作工作区/epub/、fonts.json）自动识别；
省略书目录表示当前目录。--git 在书根建立可选的独立 Git。
`

func runBook(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprint(os.Stderr, bookUsage)
		return pipeline.ExitUsage
	}
	sub, rest := argv[0], argv[1:]
	bio := pipeline.BookIO{Out: os.Stdout, Err: os.Stderr}
	ctx, stop := runCtx()
	defer stop()

	switch sub {
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, bookUsage)
		return pipeline.ExitOK
	case "new":
		fs := flag.NewFlagSet("epub book new", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		preset := fs.String("from-preset", "", "以 templates/style-presets/<名称> 为起点")
		git := fs.Bool("git", false, "在书根初始化独立 Git")
		title := fs.String("title", "", "书名（写进 OPF、NCX、书名页与 book.json）")
		author := fs.String("author", "", "作者（写进 OPF 与书名页）")
		args, code := parseInterspersed(fs, rest)
		if code >= 0 {
			return code
		}
		if len(args) != 1 {
			return bookUsageError("new 需要一个书目录")
		}
		return pipeline.BookNew(ctx, pipeline.BookNewOptions{Dir: args[0], Preset: *preset, Git: *git, Title: *title, Author: *author}, bio)
	case "recipes":
		fs := flag.NewFlagSet("epub book recipes", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		add := fs.String("add", "", "把配方 CSS 追加进该书的 OEBPS/Styles/design.css")
		args, code := parseInterspersed(fs, rest)
		if code >= 0 {
			return code
		}
		if len(args) > 1 {
			return bookUsageError("recipes 最多接受一个配方名")
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		return pipeline.BookRecipes(pipeline.BookRecipesOptions{Name: name, Add: *add}, bio)
	case "reader-layer":
		fs := flag.NewFlagSet("epub book reader-layer", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		args, code := parseInterspersed(fs, rest)
		if code >= 0 {
			return code
		}
		if len(args) > 1 {
			return bookUsageError("reader-layer 最多接受一个书目录")
		}
		dir := ""
		if len(args) == 1 {
			dir = args[0]
		}
		return pipeline.BookReaderLayer(ctx, pipeline.BookReaderLayerOptions{Dir: dir}, bio)
	case "import":
		fs := flag.NewFlagSet("epub book import", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		git := fs.Bool("git", false, "在书根初始化独立 Git 并提交基线")
		_ = fs.Bool("legacy-layout", false, "已默认：非 OEBPS 布局自动原样保留（保留此参数只为兼容）")
		args, code := parseInterspersed(fs, rest)
		if code >= 0 {
			return code
		}
		if len(args) != 2 {
			return bookUsageError("import 需要原书 EPUB 与目标书目录")
		}
		return pipeline.BookImport(ctx, pipeline.BookImportOptions{Original: args[0], Dir: args[1], Git: *git}, bio)
	case pipeline.BookCommandBuild, pipeline.BookCommandPreview, pipeline.BookCommandFonts, pipeline.BookCommandClean, pipeline.BookCommandAcceptDist:
		fs := flag.NewFlagSet("epub book "+sub, flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		updateLock := false
		variant := ""
		open := &optionalValue{}
		if sub == pipeline.BookCommandBuild || sub == pipeline.BookCommandAcceptDist || sub == pipeline.BookCommandPreview {
			fs.StringVar(&variant, "variant", "", "只处理该变体（不碰通用版成品与收据）")
		}
		if sub == pipeline.BookCommandPreview {
			fs.Var(open, "open", "构建后打开预览；--open=<应用> 指定应用（macOS 如 Reeden、Books）")
		}
		if sub == pipeline.BookCommandFonts {
			fs.BoolVar(&updateLock, "update-lock", false, "显式更新字体锁（先审阅母版变化）")
		}
		args, code := parseInterspersed(fs, rest)
		if code >= 0 {
			return code
		}
		if len(args) > 1 {
			return bookUsageError(sub + " 最多接受一个书目录")
		}
		dir := ""
		if len(args) == 1 {
			dir = args[0]
		}
		return pipeline.BookRun(ctx, pipeline.BookRunOptions{Dir: dir, Command: sub, UpdateLock: updateLock, Variant: variant, Open: open.set, OpenApp: open.value}, bio)
	default:
		fmt.Fprintf(os.Stderr, "epub book: unknown subcommand %q\n\n%s", sub, bookUsage)
		return pipeline.ExitUsage
	}
}

func bookUsageError(message string) int {
	fmt.Fprintf(os.Stderr, "epub book: %s\n\n%s", message, bookUsage)
	return pipeline.ExitUsage
}

// parseInterspersed 允许 flag 与位置参数交错（`epub book new 我的书 --git`）。
// 返回位置参数；code >= 0 表示应立即以该退出码返回。
func parseInterspersed(fs *flag.FlagSet, argv []string) (positional []string, code int) {
	for {
		if err := fs.Parse(argv); err != nil {
			return nil, pipeline.ExitUsage
		}
		if fs.NArg() == 0 {
			return positional, -1
		}
		positional = append(positional, fs.Arg(0))
		argv = fs.Args()[1:]
	}
}

// optionalValue 是可带可不带值的 flag：`--open` 或 `--open=<应用>`。
type optionalValue struct {
	set   bool
	value string
}

func (o *optionalValue) IsBoolFlag() bool { return true }

func (o *optionalValue) String() string {
	if o == nil {
		return ""
	}
	return o.value
}

func (o *optionalValue) Set(text string) error {
	switch text {
	case "true":
		o.set = true
	case "false":
		o.set, o.value = false, ""
	default:
		o.set, o.value = true, text
	}
	return nil
}
