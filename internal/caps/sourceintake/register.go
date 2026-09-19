// register.go 是本包的静态查找表（INV-7 白名单：仅 init 期写入，运行期只读）。
package sourceintake

// 文件角色（facts.files[].role / facts.roleCounts 的键）。
const (
	RoleText     = "text"
	RoleHTML     = "html"
	RoleImage    = "image"
	RolePDF      = "pdf"
	RoleEPUB     = "epub"
	RoleFont     = "font"
	RoleCSS      = "css"
	RoleAudio    = "audio"
	RoleVideo    = "video"
	RoleArchive  = "archive"
	RoleDocument = "document"
	RoleUnknown  = "unknown"
)

// 风险标签（facts.files[].risks[]）。
const (
	RiskEncodingNotUTF8 = "encoding-not-utf8"
	RiskUTF8BOM         = "utf8-bom"
	RiskCRLF            = "crlf"
	RiskImageFormat     = "image-format-risk"
	RiskJPEGCMYK        = "jpeg-cmyk"
	RiskPDFOutOfScope   = "pdf-out-of-scope"
	RiskAlreadyEPUB     = "already-epub"
	RiskNestedArchive   = "nested-archive"
	RiskSymlink         = "symlink"
	// RiskUnreadable 标记打不开/读不出的条目：size=0 与空 sha256 是"未读到"，
	// 不是"文件为空"；blocker 推断会跳过这类条目。
	RiskUnreadable = "unreadable"
)

// roleByExt 按小写扩展名（含点）映射角色；未命中即 RoleUnknown。
var roleByExt = map[string]string{
	".txt": RoleText, ".md": RoleText, ".markdown": RoleText,
	".html": RoleHTML, ".htm": RoleHTML, ".xhtml": RoleHTML,
	".jpg": RoleImage, ".jpeg": RoleImage, ".png": RoleImage, ".gif": RoleImage,
	".svg": RoleImage, ".webp": RoleImage, ".tif": RoleImage, ".tiff": RoleImage,
	".bmp": RoleImage, ".avif": RoleImage, ".heic": RoleImage,
	".pdf":  RolePDF,
	".epub": RoleEPUB,
	".ttf":  RoleFont, ".otf": RoleFont, ".woff": RoleFont, ".woff2": RoleFont,
	".css": RoleCSS,
	".mp3": RoleAudio, ".m4a": RoleAudio, ".aac": RoleAudio, ".ogg": RoleAudio,
	".oga": RoleAudio, ".wav": RoleAudio, ".flac": RoleAudio,
	".mp4": RoleVideo, ".m4v": RoleVideo, ".mov": RoleVideo, ".webm": RoleVideo,
	".mkv": RoleVideo,
	".zip": RoleArchive, ".rar": RoleArchive, ".7z": RoleArchive,
	".docx": RoleDocument, ".doc": RoleDocument, ".odt": RoleDocument, ".rtf": RoleDocument,
}

// riskyImageExt 是 EPUB 核心媒体类型（JPEG/PNG/GIF/SVG）之外的图片扩展名。
var riskyImageExt = map[string]bool{
	".webp": true, ".tif": true, ".tiff": true, ".bmp": true, ".avif": true, ".heic": true,
}

// jpegExt 是需要做 CMYK 头部探测的扩展名。
var jpegExt = map[string]bool{".jpg": true, ".jpeg": true}

// workspacePlan 是角色 → 书级工作区目录的静态建议
// （docs/pipeline/book-workspace.md 的三目录约定）。
var workspacePlan = map[string]string{
	"01 源文件/":   "全部入选源文件原样保留，记录 SHA-256，不原地修改",
	"02 校对材料/":  "异版 EPUB、参考文本、外部抽取/OCR 结果与抽样校对记录",
	"03 制作工作区/": "唯一可编辑的 EPUB 解包树（epub/）与 .pipeline/ 中间产物",
	"制作说明.md":   "来源记录：文件角色、抽取工具名与版本、需人工确认的页码/脚注/表格/公式/图片",
}
