// register.go 是本包的正则与常量表（INV-7 白名单：仅 init 期写入）。
// 全部逐字对齐 scripts/validate_epub_style_demo.py 的顶层定义。
package styledemo

import "regexp"

// 命名空间（Python OPF_NS / XHTML_NS / NCX_NS / MATHML_URI / SVG_URI）。
const (
	opfNS   = "http://www.idpf.org/2007/opf"
	xhtmlNS = "http://www.w3.org/1999/xhtml"
	ncxNS   = "http://www.daisy.org/z3986/2005/ncx/"

	mathmlURI = "http://www.w3.org/1998/Math/MathML"
	svgURI    = "http://www.w3.org/2000/svg"
)

// demo 源树内相对 OEBPS 的固定文件（Python 顶部 OEBPS 常量）。
const (
	relPackage     = "OEBPS/package.opf"
	relNav         = "OEBPS/nav.xhtml"
	relNCX         = "OEBPS/toc.ncx"
	relBaseCSS     = "OEBPS/Styles/base.css"
	relFontsCSS    = "OEBPS/Styles/fonts.css"
	relPosterCSS   = "OEBPS/Styles/poster.css"
	relNotesCSS    = "OEBPS/Styles/notes.css"
	relMediaCSS    = "OEBPS/Styles/media.css"
	relLiterary    = "OEBPS/Styles/literary.css"
	relEffects     = "OEBPS/Styles/effects.css"
	relPosterPage  = "OEBPS/Text/03c-poster-contain.xhtml"
	relRubyPage    = "OEBPS/Text/02-ruby-note.xhtml"
	relFrontPage   = "OEBPS/Text/15-frontmatter.xhtml"
	relImagePage   = "OEBPS/Text/17-image-layout.xhtml"
	relEnglish     = "OEBPS/Text/18-english-fiction.xhtml"
	relNoteBoxes   = "OEBPS/Text/19-border-shadow-notes.xhtml"
	relChapterHead = "OEBPS/Text/20-chapter-head-image.xhtml"
	relClassical   = "OEBPS/Text/21-classical-modern.xhtml"
	relMathPage    = "OEBPS/Text/16-math.xhtml"
)

// artifact 模式里的固定 zip 路径。
const (
	zipBaseCSS  = "OEBPS/Styles/base.css"
	zipFontsCSS = "OEBPS/Styles/fonts.css"
	zipPackage  = "OEBPS/package.opf"
)

// methodStore 对齐 zipfile.ZIP_STORED。
const methodStore uint16 = 0

// 正则表（对齐 validate_epub_style_demo.py 内联 re.search / re.compile）。
var (
	// selector_blocks / has_direct_body_font_family 的 finditer（re.S）。
	reCSSRule = regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)

	// percentage_width 的 width 声明。
	rePercentageWidth = regexp.MustCompile(`width\s*:\s*([0-9]+(?:\.[0-9]+)?)%\s*;`)

	// strip_css_comments。
	reCSSComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

	// has_body_font_locked_markup：Python 用了反向引用 \1，RE2 不支持；
	// 展开成单引号/双引号两个等价模式（[^'"]* 同时排除两种引号，与
	// Python 的 [^'\"] 字符类语义一致，闭引号必须等于开引号）。
	reBodyFontLockedDQ = regexp.MustCompile(`(?i)<body[^>]*\bclass\s*=\s*"[^"']*\bbody-font-locked\b[^"']*"`)
	reBodyFontLockedSQ = regexp.MustCompile(`(?i)<body[^>]*\bclass\s*=\s*'[^'"]*\bbody-font-locked\b[^'"]*'`)

	// font-family 声明探测。
	reFontFamilyColon = regexp.MustCompile(`(?i)\bfont-family\s*:`)

	// fonts.css 的 legacy .body-font-locked 规则（re.S | re.I）。
	reBodyFontClassRule = regexp.MustCompile(`(?is)\.body-font-locked\b[^{}]*\{[^}]*\bfont-family\s*:`)

	// poster.css 禁用 vh/vw。
	reVhVw = regexp.MustCompile(`\b[0-9.]+v[hw]\b`)

	// effects.css 禁用 transform: rotate()（SPEC §5.10）。
	reCSSTransformRotate = regexp.MustCompile(`(?:-webkit-)?transform\s*:\s*[^;{}]*\brotate`)

	// media.css 禁用通用 img width:100%。
	reGenericImgWidth = regexp.MustCompile(`(?ms)^img\s*\{[^}]*\bwidth\s*:\s*100%`)

	// .math-data-table math 的 em 字号候选。
	reFontSizeEm = regexp.MustCompile(`\bfont-size\s*:\s*[0-9]+(?:\.[0-9]+)?em\s*;`)

	// 16-math 的 data-table fixture 与 TeX 注释。
	reMathDataTable = regexp.MustCompile(`(?s)<table class="math-data-table">(.*?)</table>`)
	reTeXAnnotation = regexp.MustCompile(`(?s)<annotation encoding="application/x-tex">\s*(.*?)\s*</annotation>`)
)

// token 表（逐条对齐 validate_source 里的字面量列表）。
var (
	posterCSSTokens = [...]string{
		"body.poster-bg-contain",
		"background-size: contain",
		".poster-fallback",
		"max-height: 100%",
		"@supports (background-size: contain)",
		"visibility: hidden",
	}

	posterPageTokens = [...]string{
		`body class="fullpage poster-bg-contain"`,
		`section class="fullframe"`,
		`class="poster-fallback"`,
	}

	noteCSSTokens = [...]string{
		"sup.note-marker",
		"line-height: 0",
		"position: relative",
		"top: -0.14em",
		"height: 0.72em",
		"sup.note-marker > .noteref-icon > img",
	}

	imageLayoutSizeTokens = [...]string{
		`class="img-center image-instance-wide"`,
		`class="figure-pair image-pair"`,
		`class="image-pair-narrow"`,
		`class="image-pair-wide"`,
		`class="figure-stage"`,
	}

	mediaCSSTokens = [...]string{
		".image-instance-wide",
		".figure-pair",
		".image-pair-narrow",
		".image-pair-wide",
		".figure-stage",
		"width: 72%",
		"flex: 0 1 34%",
		"flex: 0 1 60%",
		"@media (min-width: 40em)",
		"display: flex",
		"align-items: center",
	}

	mathmlTokens = [...]string{
		"<mfrac", "<msqrt", "<mroot", "<msub", "<msup", "<msubsup",
		"<mover", "<munder", "<munderover", "<menclose", "<mfenced",
		"<mtable", "<mtr", "<mtd",
		"<semantics", "<annotation", "<mmultiscripts", "<ms>",
	}

	eqLayoutPageTokens = [...]string{
		`class="eq-table"`,
		`role="presentation"`,
		`class="eq-formula"`,
		`class="eq-grid"`,
		`class="eq-num"`,
		`class="sys-row"`,
		`class="sys-label"`,
	}

	dataTablePageTokens = [...]string{
		`class="math-data-table"`,
		"<thead>",
		"<tbody>",
		`scope="col"`,
		`scope="rowgroup" rowspan="2"`,
		`scope="row"`,
	}

	dataTableCSSTokens = [...]string{
		".math-data-table",
		"table-layout: fixed",
		".math-data-table math",
	}

	eqLayoutCSSTokens = [...]string{
		".eq-table",
		"border-collapse: collapse",
		".eq-formula",
		".eq-grid",
		"grid-template-columns",
		".eq-num",
		".sys-row",
		"flex-wrap: wrap",
	}

	englishPageTokens = [...]string{
		`xml:lang="en"`,
		`body class="english-fiction"`,
		`class="english-chapter-title"`,
		`class="en-noindent"`,
		`class="en-noindent en-first-letter"`,
		`class="en-noindent en-dropcap-host"`,
		`class="en-dropcap"`,
		`class="en-illustration"`,
		`class="en-large-probe"`,
	}

	literaryEnglishTokens = [...]string{
		".en-first-letter::first-letter",
		".en-dropcap-host",
		".en-dropcap",
		"Snell Roundhand",
		"float: left",
	}

	frontPageTokens = [...]string{
		`class="frontmatter copyright-page"`,
		`class="copyright-heading"`,
		`class="copyright-transcript"`,
		`class="cp cp-kai"`,
		`class="cp-line-rule"`,
		`<dl class="copyright-meta"`,
		`class="copyright-meta-item"`,
		"<dt>",
		"<dd>",
	}

	frontCSSTokens = [...]string{
		".copyright-page",
		".copyright-heading",
		".copyright-page .cp",
		".cp-line-rule",
		".copyright-meta",
		".copyright-meta-item",
		"grid-template-columns",
		".copyright-meta dt",
		".copyright-meta dd",
	}

	effectsCSSTokens = [...]string{
		".note-square", ".note-dashed", ".note-double", ".note-left-rule",
		".note-shadow", ".note-inset", ".note-slant", ".note-corner-ornament",
		".note-ornate-rule", ".note-ornate-svg", ".note-corner-frame",
		".note-long-shadow", ".note-irregular", ".note-handcut",
	}

	noteBoxPageTokens = [...]string{
		`class="note-box note-square"`,
		`class="note-box note-shadow"`,
		`class="note-box note-slant"`,
		`class="note-box note-corner-ornament"`,
		`class="note-ornate-svg"`,
		`<path class="note-ornate-main"`,
		`class="note-box note-long-shadow"`,
		`class="note-box note-irregular"`,
		`class="note-box note-handcut"`,
	}

	chapterHeadCSSTokens = [...]string{
		".chapter-header",
		".chapter-head-art",
		".chapter-head-art-roomy",
		".chapter-head-banner",
		".decorated-chapter-title",
		".chapter-head-note",
	}

	chapterHeadPageTokens = [...]string{
		`class="chapter-with-head-image"`,
		`class="chapter-header"`,
		`class="chapter-head-art"`,
		`class="chapter-head-banner"`,
		`class="decorated-chapter-title"`,
		`class="chapter-head-art chapter-head-art-roomy"`,
		"Images/chapter-banner.png",
	}

	classicalPageTokens = [...]string{
		`class="classical-modern"`,
		`id="classical-modern-toc"`,
		`class="parallel-entry"`,
		`class="parallel-entry-title"`,
		`class="parallel-source"`,
		"parallel-float-pair",
		"parallel-ratio-balanced",
		"parallel-ratio-source-wide",
		"parallel-stack-pair",
		`class="parallel-clear"`,
		`class="classical-text font-st"`,
		`class="modern-text font-kt"`,
		`class="parallel-return"`,
		`class="parallel-entry parallel-large-probe"`,
	}

	classicalCSSTokens = [...]string{
		".classical-modern",
		".classical-modern-local-toc",
		".parallel-entry",
		".parallel-entry-title",
		".parallel-source",
		".parallel-pair",
		".parallel-float-pair",
		".parallel-float-pair.parallel-ratio-balanced",
		".parallel-float-pair.parallel-ratio-source-wide",
		".parallel-stack-pair",
		".parallel-clear",
		".classical-text",
		".modern-text",
		".parallel-return",
		".parallel-large-probe",
	}
)
