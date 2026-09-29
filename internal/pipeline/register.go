// register.go 是 capability 执行注册表（INV-7 白名单：注册表文件，
// 仅 init() 期写入）。pipeline 是唯一允许 import 全部 caps 包的地方。
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	epubhandbook "github.com/liyafly/epub-handbook"
	"github.com/liyafly/epub-handbook/internal/book"
	alite "github.com/liyafly/epub-handbook/internal/caps/alite"
	contentanalyze "github.com/liyafly/epub-handbook/internal/caps/content_analyze"
	covercap "github.com/liyafly/epub-handbook/internal/caps/cover"
	csscleanup "github.com/liyafly/epub-handbook/internal/caps/css_cleanup"
	englishtypography "github.com/liyafly/epub-handbook/internal/caps/english_typography"
	fontcoverage "github.com/liyafly/epub-handbook/internal/caps/fontcoverage"
	fontsubset "github.com/liyafly/epub-handbook/internal/caps/fontsubset"
	imagelayout "github.com/liyafly/epub-handbook/internal/caps/image_layout"
	kindlecheck "github.com/liyafly/epub-handbook/internal/caps/kindle_check"
	literarystructure "github.com/liyafly/epub-handbook/internal/caps/literary_structure"
	mergecap "github.com/liyafly/epub-handbook/internal/caps/merge"
	metadatacap "github.com/liyafly/epub-handbook/internal/caps/metadata"
	migrateepub3 "github.com/liyafly/epub-handbook/internal/caps/migrate_epub3"
	navaudit "github.com/liyafly/epub-handbook/internal/caps/navaudit"
	notesfallback "github.com/liyafly/epub-handbook/internal/caps/notes_fallback"
	popupnotes "github.com/liyafly/epub-handbook/internal/caps/popupnotes"
	sourceintake "github.com/liyafly/epub-handbook/internal/caps/sourceintake"
	splitcap "github.com/liyafly/epub-handbook/internal/caps/split"
	structurenormalize "github.com/liyafly/epub-handbook/internal/caps/structure_normalize"
	styledemo "github.com/liyafly/epub-handbook/internal/caps/styledemo"
	typographycap "github.com/liyafly/epub-handbook/internal/caps/typography"
	verticalruby "github.com/liyafly/epub-handbook/internal/caps/vertical_ruby"
	"github.com/liyafly/epub-handbook/internal/report"
)

// Args 是 CLI 透传给 capability 的参数（--input/--output 之外的自定义键值）。
type Args map[string]string

const embeddedPresetsArg = "__pipeline_embedded_presets"
const embeddedDemoArg = "__pipeline_embedded_demo"
const presetCatalogDirArg = "__pipeline_preset_catalog_dir"

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

// 执行形态取值见 capability manifest 的 execution 字段；运行时直接读取契约。
const (
	ExecInputEpub       = "epub"
	ExecInputEpubOrTree = "epub-or-tree"
	ExecInputSourcePath = "source-path"
	ExecOutputSingle    = "single"
	ExecOutputMulti     = "multi"
	ExecOutputNone      = "none"
)

// register 登记一个 capability。仅供本文件 init() 调用。
func register(id string, r Runner) {
	registry[id] = r
}

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

func parseScopePaths(args Args) ([]string, error) {
	raw, ok := args["scope_paths"]
	if !ok {
		return nil, nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil || len(paths) == 0 {
		return nil, usageErrorf("scope_paths 必须是非空 JSON 字符串数组")
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			return nil, usageErrorf("scope_paths 不能包含空路径")
		}
	}
	return paths, nil
}

func init() {
	register("epub.package.nav.audit", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return navaudit.Run(ctx, b, navaudit.Params{})
	})
	register("epub.text.content.analyze", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return contentanalyze.Run(ctx, b, contentanalyze.Params{
			IncludeSnippets: args.Bool("include_snippets"),
		})
	})
	register("epub.image.layout.optimize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return imagelayout.Run(ctx, b, imagelayout.Params{})
	})
	register("epub.font.coverage.analyze", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		profile := args.Get("profile")
		if profile == "" {
			profile = "kindle-pessimistic"
		}
		return fontcoverage.Run(ctx, b, fontcoverage.Params{Profile: profile})
	})
	register("epub.font.subset", func(ctx context.Context, b *book.Book, args Args, _ Upstream) (report.Result, error) {
		return fontsubset.Run(ctx, b, fontsubset.Params{FontConfig: args.Get("font_config")})
	})
	register("epub.kindle.compatibility.check", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return kindlecheck.Run(ctx, b, kindlecheck.Params{})
	})
	register("epub.notes.popup.normalize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return popupnotes.Run(ctx, b, popupnotes.Params{})
	})
	register("epub.notes.legacy-fallback", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		violations := -1
		noterefs := -1
		if result, ok := up[notesfallback.UpstreamID]; ok {
			if count, ok := result.Facts["standardViolations"].(int); ok {
				violations = count
			}
			if count, ok := result.Facts["noterefs"].(int); ok {
				noterefs = count
			}
		}
		return notesfallback.Run(ctx, b, notesfallback.Params{UpstreamViolations: violations, UpstreamNoterefs: noterefs})
	})
	register("epub.typography.english.optimize", func(ctx context.Context, b *book.Book, args Args, _ Upstream) (report.Result, error) {
		lang, ok := args["lang"]
		if !ok {
			lang = "en"
		} else if !englishtypography.ValidLang(lang) {
			return report.Result{}, usageErrorf("lang 必须符合 BCP 47 子集，如 en 或 en-GB")
		}
		scope, err := parseScopePaths(args)
		if err != nil {
			return report.Result{}, err
		}
		return englishtypography.Run(ctx, b, englishtypography.Params{Lang: lang, ScopePaths: scope})
	})
	register("epub.vertical.ruby.optimize", func(ctx context.Context, b *book.Book, args Args, _ Upstream) (report.Result, error) {
		op, ok := args["op"]
		if !ok {
			return report.Result{}, usageErrorf("op 必填，取值为 ruby-rp 或 writing-mode-prefix")
		}
		if op != verticalruby.OpRubyRP && op != verticalruby.OpWritingModePrefix {
			return report.Result{}, usageErrorf("op 必须是 ruby-rp 或 writing-mode-prefix")
		}
		scope, err := parseScopePaths(args)
		if err != nil {
			return report.Result{}, err
		}
		return verticalruby.Run(ctx, b, verticalruby.Params{Op: op, ScopePaths: scope})
	})
	register("epub.literary.structure.format", func(ctx context.Context, b *book.Book, args Args, _ Upstream) (report.Result, error) {
		raw, ok := args["assignments"]
		if !ok {
			return report.Result{}, usageErrorf("assignments 必填，值为非空 JSON 数组")
		}
		var assignments []literarystructure.Assignment
		if err := json.Unmarshal([]byte(raw), &assignments); err != nil || len(assignments) == 0 {
			return report.Result{}, usageErrorf("assignments 必须是非空 JSON 数组文本")
		}
		for _, assignment := range assignments {
			hasID := assignment.ID != ""
			hasTag := assignment.Tag != ""
			if assignment.Index != nil && *assignment.Index < 0 {
				return report.Result{}, usageErrorf("assignment index must be non-negative")
			}
			if hasID == hasTag {
				return report.Result{}, usageErrorf("每个 assignment 必须且只能提供 id 或 tag")
			}
			if hasID && assignment.Index != nil {
				return report.Result{}, usageErrorf("使用 id 定位时不能提供 index")
			}
			if hasTag && assignment.Index == nil {
				return report.Result{}, usageErrorf("使用 tag 定位时必须提供 index")
			}
		}
		return literarystructure.Run(ctx, b, literarystructure.Params{
			Assignments: assignments,
			Stylesheet:  args.Get("stylesheet"),
		})
	})
	register("epub.style.demo.maintain", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		if !args.Bool("catalog") {
			if args.Get("query") != "" {
				return report.Result{}, usageErrorf("query requires catalog=true")
			}
			if _, exists := args["collection"]; exists {
				return report.Result{}, usageErrorf("collection requires catalog=true")
			}
		}
		var demoFS, presetFS fs.FS
		if args.Bool(embeddedDemoArg) {
			var err error
			demoFS, err = fs.Sub(epubhandbook.EmbeddedFS(), "templates/epub-style-demo")
			if err != nil {
				return report.Result{}, fmt.Errorf("open embedded demo catalog: %w", err)
			}
		}
		if args.Bool(embeddedPresetsArg) {
			var err error
			presetFS, err = fs.Sub(epubhandbook.EmbeddedFS(), "templates/style-presets")
			if err != nil {
				return report.Result{}, fmt.Errorf("open embedded preset catalog: %w", err)
			}
		}
		return styledemo.Run(ctx, b, styledemo.Params{
			DemoDir: args.Get("demo_dir"), DemoFS: demoFS,
			PresetDir: args.Get(presetCatalogDirArg), PresetFS: presetFS,
			Catalog: args.Bool("catalog"), Collection: args.Get("collection"), Query: args.Get("query"),
		})
	})
	register("epub.source.intake", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		maxFiles := 0
		if v := args.Get("max_files"); v != "" {
			// 参数非法是用法错误（SPEC §8.5 退出码 3），不是能力失败；
			// <=0 必须显式拒绝：静默回落到默认 5000 会被读成"不限"。
			n, err := strconv.Atoi(v)
			if err != nil {
				return report.Result{}, usageErrorf("max_files 必须是正整数，得到 %q", v)
			}
			if n <= 0 {
				return report.Result{}, usageErrorf(
					"max_files 必须大于 0，得到 %q（省略该参数即用默认上限 %d）", v, sourceintake.DefaultMaxFiles)
			}
			maxFiles = n
		}
		res, err := sourceintake.Run(ctx, b, sourceintake.Params{
			SourcePath: args.Get("source_path"),
			MaxFiles:   maxFiles,
		})
		if errors.Is(err, sourceintake.ErrNotRegularFile) {
			return report.Result{}, &UsageError{Err: err}
		}
		return res, err
	})
	register("epub.package.migrate.epub3", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return migrateepub3.Run(ctx, b, migrateepub3.Params{
			DryRun: args.Bool("dry_run"),
		})
	})
	register("epub.css.layering.optimize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return csscleanup.Run(ctx, b, csscleanup.Params{
			Output: args.Get("output"),
		})
	})
	register("epub.typography.optimize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		scope, err := parseScopePaths(args)
		if err != nil {
			return report.Result{}, err
		}
		preset := args.Get("preset")
		if preset == "" {
			preset = "literary-cn"
		}
		var presetFS fs.FS
		if args.Bool(embeddedPresetsArg) {
			var err error
			presetFS, err = fs.Sub(epubhandbook.EmbeddedFS(), typographycap.DefaultPresetsDir)
			if err != nil {
				return report.Result{}, fmt.Errorf("open embedded typography presets: %w", err)
			}
		}
		return typographycap.Run(ctx, b, typographycap.Params{
			Preset:     preset,
			PresetDir:  args.Get("preset_dir"),
			PresetFS:   presetFS,
			Output:     args.Get("output"),
			DryRun:     args.Bool("dry_run"),
			ScopePaths: scope,
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
			Output: args.Get("output"),
		})
	})
	register("epub.package.split", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		var points []int
		for _, f := range strings.Split(args.Get("split_points"), ",") {
			if f = strings.TrimSpace(f); f != "" {
				n, err := strconv.Atoi(f)
				if err != nil {
					// 参数非法是用法错误（SPEC §8.5 退出码 3），不是能力失败。
					return report.Result{}, usageErrorf("split_points 必须是逗号分隔的整数，得到 %q", f)
				}
				points = append(points, n)
			}
		}
		return splitcap.Run(ctx, b, splitcap.Params{
			SplitPoints: points, OutputDir: args.Get("output_dir"), DryRun: args.Bool("dry_run"),
		})
	})
	register("epub.metadata.edit", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return metadatacap.Run(ctx, b, metadatacap.Params{
			MetadataJSON: args.Get("metadata_json"), Output: args.Get("output"),
		})
	})
	register("epub.cover.replace", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		return covercap.Run(ctx, b, covercap.Params{
			Cover: args.Get("cover"), Output: args.Get("output"),
		})
	})
	register("epub.structure.normalize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		mode := args.Get("mode")
		if mode == "" {
			mode = "normalize"
		}
		return structurenormalize.Run(ctx, b, structurenormalize.Params{
			Mode:   structurenormalize.Mode(mode),
			DryRun: args.Bool("dry_run"),
		})
	})
	register("epub.alite.convert", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
		var expect *int
		if v := args.Get("expect_volumes"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				// 同 split_points：用法错误走退出码 3。
				return report.Result{}, usageErrorf("expect_volumes 必须是整数，得到 %q", v)
			}
			expect = &n
		}
		return alite.Run(ctx, b, alite.Params{
			ExpectVolumes: expect,
			Output:        args.Get("output"),
		})
	})
}
