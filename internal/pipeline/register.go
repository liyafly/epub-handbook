// register.go 是 capability 执行注册表（INV-7 白名单：注册表文件，
// 仅 init() 期写入）。pipeline 是唯一允许 import 全部 caps 包的地方。
package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	alite "github.com/liyafly/epub-handbook/internal/caps/alite"
	contentanalyze "github.com/liyafly/epub-handbook/internal/caps/content_analyze"
	covercap "github.com/liyafly/epub-handbook/internal/caps/cover"
	csscleanup "github.com/liyafly/epub-handbook/internal/caps/css_cleanup"
	fontcoverage "github.com/liyafly/epub-handbook/internal/caps/fontcoverage"
	imagelayout "github.com/liyafly/epub-handbook/internal/caps/image_layout"
	mergecap "github.com/liyafly/epub-handbook/internal/caps/merge"
	metadatacap "github.com/liyafly/epub-handbook/internal/caps/metadata"
	migrateepub3 "github.com/liyafly/epub-handbook/internal/caps/migrate_epub3"
	navaudit "github.com/liyafly/epub-handbook/internal/caps/navaudit"
	popupnotes "github.com/liyafly/epub-handbook/internal/caps/popupnotes"
	splitcap "github.com/liyafly/epub-handbook/internal/caps/split"
	structurenormalize "github.com/liyafly/epub-handbook/internal/caps/structure_normalize"
	styledemo "github.com/liyafly/epub-handbook/internal/caps/styledemo"
	typographycap "github.com/liyafly/epub-handbook/internal/caps/typography"
	"github.com/liyafly/epub-handbook/internal/report"
)

// Args 是 CLI 透传给 capability 的参数（--input/--output 之外的自定义键值）。
type Args map[string]string

// Get 返回参数值，缺省为空串。
func (a Args) Get(k string) string { return a[k] }

// Bool 按 Python 风格的 truthy 约定解析布尔参数。
func (a Args) Bool(k string) bool {
	switch strings.ToLower(a[k]) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Upstream 是按 capability id 索引的上游运行结果。
type Upstream map[string]report.Result

// Runner 是 capability 的统一执行入口。
// 具体包遵循 SPEC §6.1 模板导出 Run(ctx, b, Params)；这里持有的是
// 适配后的闭包，负责把 Args / Upstream 翻译成各自的 Params。
type Runner func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error)

// registry 是 capability id → 执行入口 的注册表。
var registry = map[string]Runner{}

// multiOutput 记录自行落盘多个产物的能力（如 split）：pipeline 跳过统一写盘。
var multiOutput = map[string]bool{}

// readOnly 记录契约虽是 transformer、但 Go 执行面为只读校验的能力
// （如 epub.notes.popup.normalize —— 其转换动作在 migrate.epub3 内）。
var readOnly = map[string]bool{}

// noBook 记录不要求 --input 是 EPUB 的能力：--input 为空或指向目录时
// 以 b=nil 进入源树模式，指向文件时才经 book.Open 进入产物模式
// （如 epub.style.demo.maintain 的 demo 源树 / 构建产物双模式）。
var noBook = map[string]bool{}

// register 登记一个 capability。仅供本文件 init() 调用。
func register(id string, r Runner) {
	registry[id] = r
}

// registerMultiOutput 登记多产物能力（split 等）：产物由包内自行写盘，
// pipeline 不再统一 WriteTo。
func registerMultiOutput(id string, r Runner) {
	registry[id] = r
	multiOutput[id] = true
}

// IsMultiOutput 报告能力是否自行落盘多产物。
func IsMultiOutput(id string) bool { return multiOutput[id] }

// registerReadOnly 登记只读执行面能力。
func registerReadOnly(id string, r Runner) {
	registry[id] = r
	readOnly[id] = true
}

// IsReadOnly 报告能力是否只读执行面（忽略契约的 requiresWriteAccess）。
func IsReadOnly(id string) bool { return readOnly[id] }

// registerNoBook 登记无 EPUB 输入也能运行的能力（只读；--input 为空或
// 指向目录时 b=nil，指向文件时照常 book.Open）。
func registerNoBook(id string, r Runner) {
	registry[id] = r
	noBook[id] = true
}

// IsNoBook 报告能力是否支持无 EPUB 输入（源树/目录模式）。
func IsNoBook(id string) bool { return noBook[id] }

// Implemented 报告 capability 是否已有 Go 实现。
func Implemented(id string) bool {
	_, ok := registry[id]
	return ok
}

// ImplementedIDs 返回已注册的 capability id（排序）。
func ImplementedIDs() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func init() {
	register("epub.package.nav.audit", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return navaudit.Run(ctx, b, navaudit.Params{LegacyReport: args.Bool("legacy_report")})
	})
	register("epub.layout.audit", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return navaudit.Run(ctx, b, navaudit.Params{
			LegacyReport: args.Bool("legacy_report"),
			Report:       "layout-audit",
		})
	})
	register("epub.text.content.analyze", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return contentanalyze.Run(ctx, b, contentanalyze.Params{
			IncludeSnippets: args.Bool("include_snippets"),
			LegacyReport:    args.Bool("legacy_report"),
			SourceName:      args.Get("source_name"),
			SourceContent:   args.Get("source_content"),
		})
	})
	register("epub.image.layout.optimize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return imagelayout.Run(ctx, b, imagelayout.Params{LegacyReport: args.Bool("legacy_report")})
	})
	register("epub.font.coverage.analyze", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		profile := args.Get("profile")
		if profile == "" {
			profile = "kindle-pessimistic"
		}
		return fontcoverage.Run(ctx, b, fontcoverage.Params{
			Profile:      profile,
			LegacyReport: args.Bool("legacy_report"),
		})
	})
	registerReadOnly("epub.notes.popup.normalize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return popupnotes.Run(ctx, b, popupnotes.Params{LegacyReport: args.Bool("legacy_report")})
	})
	registerNoBook("epub.style.demo.maintain", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return styledemo.Run(ctx, b, styledemo.Params{
			DemoDir:      args.Get("demo_dir"),
			LegacyReport: args.Bool("legacy_report"),
		})
	})
	register("epub.package.migrate.epub3", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return migrateepub3.Run(ctx, b, migrateepub3.Params{
			PopupNotes:   !args.Bool("no_popup_notes"),
			Typography:   !args.Bool("no_typography"),
			DryRun:       args.Bool("dry_run"),
			LegacyReport: args.Bool("legacy_report"),
			Output:       args.Get("output"),
		})
	})
	register("epub.css.layering.optimize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return csscleanup.Run(ctx, b, csscleanup.Params{
			Output:              args.Get("output"),
			MergeScopedLocalCSS: args.Bool("merge_scoped_local_css"),
			LegacyReport:        args.Bool("legacy_report"),
		})
	})
	register("epub.typography.optimize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		preset := args.Get("preset")
		if preset == "" {
			preset = "literary-cn"
		}
		return typographycap.Run(ctx, b, typographycap.Params{
			Preset:       preset,
			PresetDir:    args.Get("preset_dir"),
			Output:       args.Get("output"),
			DryRun:       args.Bool("dry_run"),
			LegacyReport: args.Bool("legacy_report"),
		})
	})
	register("epub.package.merge", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		inputs := []string{b.InputPath()}
		if extra := args.Get("extra_inputs"); extra != "" {
			for _, p := range strings.Split(extra, ",") {
				if p = strings.TrimSpace(p); p != "" {
					inputs = append(inputs, p)
				}
			}
		}
		var title *string
		if t := args.Get("title"); t != "" {
			title = &t
		}
		return mergecap.Run(ctx, b, mergecap.Params{
			Inputs: inputs, Title: title,
			Output: args.Get("output"), LegacyReport: args.Bool("legacy_report"),
		})
	})
	registerMultiOutput("epub.package.split", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		var points []int
		for _, f := range strings.Split(args.Get("split_points"), ",") {
			if f = strings.TrimSpace(f); f != "" {
				n, err := strconv.Atoi(f)
				if err != nil {
					return report.Result{}, fmt.Errorf("split_points: %w", err)
				}
				points = append(points, n)
			}
		}
		return splitcap.Run(ctx, b, splitcap.Params{
			SplitPoints: points, OutputDir: args.Get("output_dir"), DryRun: args.Bool("dry_run"), LegacyReport: args.Bool("legacy_report"),
		})
	})
	register("epub.metadata.edit", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return metadatacap.Run(ctx, b, metadatacap.Params{
			MetadataJSON: args.Get("metadata_json"), Output: args.Get("output"), LegacyReport: args.Bool("legacy_report"),
		})
	})
	register("epub.cover.replace", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return covercap.Run(ctx, b, covercap.Params{
			Cover: args.Get("cover"), Output: args.Get("output"), LegacyReport: args.Bool("legacy_report"),
		})
	})
	register("epub.structure.normalize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		mode := args.Get("mode")
		if mode == "" {
			mode = "normalize"
		}
		return structurenormalize.Run(ctx, b, structurenormalize.Params{
			Mode:         structurenormalize.Mode(mode),
			DryRun:       args.Bool("dry_run"),
			LegacyReport: args.Bool("legacy_report"),
			Output:       args.Get("output"),
		})
	})
	register("epub.alite.convert", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		var expect *int
		if v := args.Get("expect_volumes"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return report.Result{}, fmt.Errorf("expect_volumes: %w", err)
			}
			expect = &n
		}
		return alite.Run(ctx, b, alite.Params{
			ExpectVolumes: expect,
			LegacyReport:  args.Bool("legacy_report"),
			Output:        args.Get("output"),
		})
	})
}
