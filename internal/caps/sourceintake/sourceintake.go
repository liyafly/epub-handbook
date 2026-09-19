// Package sourceintake 实现 planner 能力 epub.source.intake：对非 EPUB 源材料
// （目录或单个文件）做只读盘点，按扩展名分角色、按头部/字节特征打风险标签，
// 并给出后续 EPUB 制作链的 execution-plan 建议。
//
// 边界（仓库所有者决策，2026-09-07）：本仓不解析 PDF、不做 OCR、不做图片转码；
// 这些只被标记为风险与 blocker，由外部工具完成并在来源记录中登记工具名与版本。
//
// 三段式：扫描（只读遍历 + 流式哈希）→ 无应用阶段（planner 不改任何文件）→ 报告。
// b 恒为 nil：pipeline 以 registerSourceInput 注册本能力，从不 book.Open 输入。
package sourceintake

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// CapabilityID 是本能力的契约 id。
const CapabilityID = "epub.source.intake"

// DefaultMaxFiles 是默认的文件数上限；超出即 error intake.too-many-files 并停止遍历。
const DefaultMaxFiles = 5000

// maxNextCommands 限制 nextCommands 中 nav.audit 建议的条数。
const maxNextCommands = 5

// Finding id（保持稳定；SKILL.md「依据返回怎么判断」按此对账）。
const (
	FindingEmpty          = "intake.empty"
	FindingTooManyFiles   = "intake.too-many-files"
	FindingPDFOutOfScope  = "intake.pdf-out-of-scope"
	FindingImageFormat    = "intake.image-format-risk"
	FindingAlreadyEPUB    = "intake.already-epub"
	FindingEncoding       = "intake.encoding"
	FindingNestedArchive  = "intake.nested-archive"
	FindingSymlink        = "intake.symlink"
	FindingUnreadableFile = "intake.unreadable-file"
	FindingUnreadableDir  = "intake.unreadable-dir"
	FindingSummary        = "intake.summary"
)

// ErrNotRegularFile 表示单文件输入不是普通文件（FIFO、字符设备、socket）。
// 目录内的同类条目本来就被跳过；单文件入口必须显式拒绝，否则流式哈希的
// io.Copy 会永久阻塞。pipeline 把它映射为用法错误（SPEC §8.5 退出码 3）。
var ErrNotRegularFile = errors.New("sourceintake: input is not a regular file")

// Params 是本能力的参数。
type Params struct {
	// SourcePath 是源目录或单个源文件的路径（pipeline 传入绝对路径）。必填。
	SourcePath string
	// MaxFiles 是文件数上限；<=0 时取 DefaultMaxFiles。
	MaxFiles int
}

// FileEntry 是 facts.files[] 的元素。
type FileEntry struct {
	Path   string   `json:"path"`
	Role   string   `json:"role"`
	Size   int64    `json:"size"`
	SHA256 string   `json:"sha256"`
	Risks  []string `json:"risks"`

	// abs 是扫描期算好的宿主绝对路径，只用于拼 nextCommands / finding detail。
	// 单文件输入时 root 本身就是该文件，不能用 Join(root, Path) 反推（会得到
	// <file>/<basename>）；不导出，不进 JSON。
	abs string
}

// PlanStep 对齐 contracts/schemas/v1/execution-plan.schema.json 的 steps 元素。
type PlanStep struct {
	ID               string   `json:"id"`
	Capability       string   `json:"capability"`
	Kind             string   `json:"kind"`
	DependsOn        []string `json:"dependsOn"`
	RequiresApproval bool     `json:"requiresApproval"`
}

// PlanArtifact 对齐 contracts/schemas/v1/artifact-reference.schema.json。
type PlanArtifact struct {
	URI           string `json:"uri"`
	Kind          string `json:"kind"`
	ContentDigest string `json:"contentDigest,omitempty"`
}

// Plan 对齐 contracts/schemas/v1/execution-plan.schema.json（契约 outputSchema）。
type Plan struct {
	SchemaVersion string       `json:"schemaVersion"`
	Artifact      PlanArtifact `json:"artifact"`
	Steps         []PlanStep   `json:"steps"`
	Blockers      []string     `json:"blockers"`
}

// nonNilFiles 保证 files fact 恒为数组：空盘点输出 []，不是 null。
func nonNilFiles(in []FileEntry) []FileEntry {
	out := make([]FileEntry, 0, len(in))
	return append(out, in...)
}

// scanResult 是扫描阶段的只读产物。
type scanResult struct {
	root           string
	isDir          bool
	files          []FileEntry
	truncated      bool
	symlinks       []string
	unreadableFile []string
	unreadableDir  []string
}

// Run 执行源材料盘点（只读）。b 被忽略（planner 无 EPUB 输入）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	_ = b
	if p.SourcePath == "" {
		return report.Result{}, errors.New("sourceintake: source_path is required")
	}
	maxFiles := p.MaxFiles
	if maxFiles <= 0 {
		maxFiles = DefaultMaxFiles
	}
	// 1. 扫描：只读遍历，流式哈希与风险探测。
	sc, err := scanPhase(ctx, p.SourcePath, maxFiles)
	if err != nil {
		return report.Result{}, err
	}
	// 2. 应用：planner 无写入阶段。
	// 3. 报告。
	return buildReport(sc, maxFiles), nil
}

// ---- 扫描阶段 ----

func scanPhase(ctx context.Context, src string, maxFiles int) (*scanResult, error) {
	root, err := filepath.Abs(src)
	if err != nil {
		return nil, err
	}
	// 只解析 root 自身的符号链接：filepath.WalkDir 对 root 做 Lstat，符号链接
	// 目录会被当成叶子而不下降（iCloud / Dropbox 镜像与软链的「01 源文件/」
	// 会因此被误判为空目录）。树**内部**仍然只记录、不跟随。
	walkRoot := root
	if resolved, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
		walkRoot = resolved
	}
	st, err := os.Stat(walkRoot)
	if err != nil {
		return nil, fmt.Errorf("sourceintake: %w", err)
	}
	sc := &scanResult{root: root, isDir: st.IsDir()}
	if !sc.isDir {
		// 目录内的非普通文件在下面被跳过；单文件入口必须同样拒绝，否则
		// FIFO / 字符设备（/dev/zero）会让 sha256 的 io.Copy 永不返回。
		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: %s", ErrNotRegularFile, root)
		}
		entry, err := inspectFile(ctx, walkRoot, filepath.Base(root))
		if err != nil {
			return nil, err
		}
		entry.abs = root // root 本身即该文件，不能 Join(root, Path)
		sc.files = append(sc.files, entry)
		return sc, nil
	}

	walkErr := filepath.WalkDir(walkRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// ReadDir 失败（如 chmod 000 的子目录）只丢这一棵子树：只读盘点
			// 不该因为一个不可读目录，把已经哈希过的全部条目一起丢掉。
			if d != nil && d.IsDir() {
				sc.unreadableDir = append(sc.unreadableDir, displayRel(sc.root, walkRoot, p))
				return filepath.SkipDir
			}
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if p == walkRoot {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") { // 隐藏文件/目录（含 .DS_Store、.git）
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(walkRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if len(sc.files) >= maxFiles {
			sc.truncated = true
			return filepath.SkipAll
		}
		// 对外一律用用户给出的 root 拼路径，不暴露 EvalSymlinks 的结果。
		abs := filepath.Join(sc.root, filepath.FromSlash(rel))
		if d.Type()&fs.ModeSymlink != 0 {
			// 不跟随符号链接：只记录存在，不读取目标。
			sc.symlinks = append(sc.symlinks, rel)
			sc.files = append(sc.files, FileEntry{Path: rel, Role: RoleUnknown, Risks: []string{RiskSymlink}, abs: abs})
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		entry, err := inspectFile(ctx, p, rel)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			sc.unreadableFile = append(sc.unreadableFile, rel)
			// 标记 unreadable：size 0 与空 sha256 不是"这个文件就是空的"，
			// 消费者必须能分辨，blocker 推断也不能把它当正常文本。
			sc.files = append(sc.files, FileEntry{Path: rel, Role: classify(rel), Risks: []string{RiskUnreadable}, abs: abs})
			return nil
		}
		entry.abs = abs
		sc.files = append(sc.files, entry)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("sourceintake: walk: %w", walkErr)
	}
	// WalkDir 已按字典序遍历；再按 rel 路径稳定排序，确保跨平台一致。
	sort.Slice(sc.files, func(i, j int) bool { return sc.files[i].Path < sc.files[j].Path })
	return sc, nil
}

// displayRel 把遍历路径转成对用户可见的位置：walkRoot 内的相对正斜杠路径，
// root 自身则返回用户给出的绝对路径。
func displayRel(root, walkRoot, p string) string {
	rel, err := filepath.Rel(walkRoot, p)
	if err != nil || rel == "." {
		return root
	}
	return filepath.ToSlash(rel)
}

// classify 按扩展名（不区分大小写）给出角色。
func classify(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if role, ok := roleByExt[ext]; ok {
		return role
	}
	return RoleUnknown
}

// ctxReader 让单个超大文件的流式哈希也能被取消：每次底层读取前检查一次
// ctx。它包在 os.File 外、bufio 之内，因此检查频率是缓冲区大小（256KiB）。
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// inspectFile 流式计算 sha256，并按角色做廉价的风险探测。
func inspectFile(ctx context.Context, abs, rel string) (FileEntry, error) {
	ext := strings.ToLower(filepath.Ext(rel))
	entry := FileEntry{Path: rel, Role: classify(rel), Risks: []string{}}

	f, err := os.Open(abs)
	if err != nil {
		return entry, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return entry, err
	}
	entry.Size = st.Size()

	h := sha256.New()
	var w io.Writer = h
	var text *textProbe
	if entry.Role == RoleText || entry.Role == RoleHTML {
		text = &textProbe{valid: true}
		w = io.MultiWriter(h, text)
	}
	if _, err := io.Copy(w, bufio.NewReaderSize(&ctxReader{ctx: ctx, r: f}, 256<<10)); err != nil {
		return entry, err
	}
	entry.SHA256 = hex.EncodeToString(h.Sum(nil))

	switch entry.Role {
	case RoleText, RoleHTML:
		text.finish()
		if !text.valid {
			entry.Risks = append(entry.Risks, RiskEncodingNotUTF8)
		}
		if text.bom {
			entry.Risks = append(entry.Risks, RiskUTF8BOM)
		}
		if text.crlf {
			entry.Risks = append(entry.Risks, RiskCRLF)
		}
	case RoleImage:
		if riskyImageExt[ext] {
			entry.Risks = append(entry.Risks, RiskImageFormat)
		}
		if jpegExt[ext] {
			if _, err := f.Seek(0, io.SeekStart); err == nil && jpegIsCMYK(f) {
				entry.Risks = append(entry.Risks, RiskJPEGCMYK)
			}
		}
	case RolePDF:
		entry.Risks = append(entry.Risks, RiskPDFOutOfScope)
	case RoleEPUB:
		entry.Risks = append(entry.Risks, RiskAlreadyEPUB)
	case RoleArchive:
		entry.Risks = append(entry.Risks, RiskNestedArchive)
	}
	return entry, nil
}

// textProbe 以 io.Writer 形式流式探测 UTF-8 合法性、BOM 与 CRLF，
// 正确处理跨块边界的多字节序列与 \r\n。
type textProbe struct {
	started bool
	valid   bool
	bom     bool
	crlf    bool
	lastCR  bool
	carry   []byte // 上一块末尾未完整的多字节序列
}

func (t *textProbe) Write(p []byte) (int, error) {
	n := len(p)
	if n == 0 {
		return 0, nil
	}
	if !t.started {
		t.started = true
		if bytes.HasPrefix(p, []byte{0xEF, 0xBB, 0xBF}) {
			t.bom = true
		}
	}
	if !t.crlf {
		if t.lastCR && p[0] == '\n' {
			t.crlf = true
		} else if bytes.Contains(p, []byte("\r\n")) {
			t.crlf = true
		}
	}
	t.lastCR = p[n-1] == '\r'

	if !t.valid {
		return n, nil
	}
	buf := p
	if len(t.carry) > 0 {
		buf = append(t.carry, p...)
		t.carry = nil
	}
	if utf8.Valid(buf) {
		return n, nil
	}
	// 慢路径：定位首个非法位置；若只是块末尾的不完整序列则留待下一块。
	for len(buf) > 0 {
		r, size := utf8.DecodeRune(buf)
		if r == utf8.RuneError && size == 1 {
			if !utf8.FullRune(buf) && len(buf) < utf8.UTFMax {
				t.carry = bytes.Clone(buf)
				return n, nil
			}
			t.valid = false
			return n, nil
		}
		buf = buf[size:]
	}
	return n, nil
}

// finish 在流结束时结算尾部残留。
func (t *textProbe) finish() {
	if len(t.carry) > 0 {
		t.valid = false
		t.carry = nil
	}
}

// jpegIsCMYK 只读 JPEG 头部 marker 段，SOF 段声明 4 个分量即视为 CMYK/YCCK。
// 遇到 SOS、EOF、格式异常或超过 1MiB 头部即停止，返回 false。
func jpegIsCMYK(r io.Reader) bool {
	br := bufio.NewReader(io.LimitReader(r, 1<<20))
	var soi [2]byte
	if _, err := io.ReadFull(br, soi[:]); err != nil || soi != [2]byte{0xFF, 0xD8} {
		return false
	}
	for {
		b, err := br.ReadByte()
		if err != nil {
			return false
		}
		if b != 0xFF {
			return false
		}
		marker, err := br.ReadByte()
		if err != nil {
			return false
		}
		for marker == 0xFF { // 填充字节
			if marker, err = br.ReadByte(); err != nil {
				return false
			}
		}
		switch {
		case marker == 0xD8, marker >= 0xD0 && marker <= 0xD7, marker == 0x01:
			continue // 无长度段
		case marker == 0xD9, marker == 0xDA:
			return false // EOI / SOS：头部结束
		}
		var lenBuf [2]byte
		if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
			return false
		}
		segLen := int(binary.BigEndian.Uint16(lenBuf[:]))
		if segLen < 2 {
			return false
		}
		body := make([]byte, segLen-2)
		if _, err := io.ReadFull(br, body); err != nil {
			return false
		}
		isSOF := marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC
		if isSOF {
			// P(1) Y(2) X(2) Nf(1)
			return len(body) >= 6 && body[5] == 4
		}
	}
}

// ---- 报告阶段 ----

func buildReport(sc *scanResult, maxFiles int) report.Result {
	res := report.Result{Capability: CapabilityID, Status: report.StatusComplete}
	roleCounts := map[string]int{}
	var totalBytes int64
	var epubs []string
	for _, f := range sc.files {
		roleCounts[f.Role]++
		totalBytes += f.Size
		if f.Role == RoleEPUB {
			epubs = append(epubs, f.abs)
		}
	}

	var findings []report.Finding
	if len(sc.files) == 0 {
		findings = append(findings, report.Finding{
			Level: "error", ID: FindingEmpty, Title: "No source files found",
			Detail:   "输入目录为空或只含隐藏文件；源材料接入需要至少一个非隐藏文件",
			Location: sc.root,
		})
	}
	if sc.truncated {
		findings = append(findings, report.Finding{
			Level: "error", ID: FindingTooManyFiles, Title: "Too many source files",
			Detail:   fmt.Sprintf("超过 max_files=%d 上限，遍历已停止；请缩小输入目录或提高 max_files", maxFiles),
			Location: sc.root,
		})
	}
	for _, f := range sc.files {
		for _, risk := range f.Risks {
			switch risk {
			case RiskPDFOutOfScope:
				findings = append(findings, report.Finding{
					Level: "warn", ID: FindingPDFOutOfScope, Title: "PDF input is out of scope for this repository",
					Detail: "本仓不解析 PDF、不做 OCR。请用外部工具（如 pdftotext / mutool / OCR 引擎）抽取，" +
						"在来源记录中登记工具名与版本，把抽取文本视为不可信输入并抽样校对；扫描件必须标记 OCR 风险",
					Location: f.Path,
				})
			case RiskImageFormat:
				findings = append(findings, report.Finding{
					Level: "warn", ID: FindingImageFormat, Title: "Image format is not an EPUB core media type",
					Detail:   "EPUB 核心图片类型仅 JPEG / PNG / GIF / SVG；WebP / TIFF / BMP / AVIF / HEIC 需外部转码后回到本项目验证",
					Location: f.Path,
				})
			case RiskJPEGCMYK:
				findings = append(findings, report.Finding{
					Level: "warn", ID: FindingImageFormat, Title: "JPEG declares 4 color components (CMYK/YCCK)",
					Detail:   "多数阅读器不正确渲染 CMYK JPEG；需外部转为 sRGB 后回到本项目验证",
					Location: f.Path,
				})
			case RiskAlreadyEPUB:
				findings = append(findings, report.Finding{
					Level: "info", ID: FindingAlreadyEPUB, Title: "Input already contains an EPUB",
					// detail 与 nextCommands 必须逐字相同：SKILL.md 把它当成
					// 可直接执行的命令。
					Detail:   "已有 EPUB 不走源材料接入；直接运行 " + navAuditCommand(f.abs),
					Location: f.Path,
				})
			case RiskEncodingNotUTF8:
				findings = append(findings, report.Finding{
					Level: "warn", ID: FindingEncoding, Title: "Text file is not valid UTF-8",
					Detail:   "转码为 UTF-8（无 BOM）后再进入结构化；转码前后各记录一次 SHA-256",
					Location: f.Path,
				})
			case RiskNestedArchive:
				findings = append(findings, report.Finding{
					Level: "warn", ID: FindingNestedArchive, Title: "Nested archive must be unpacked first",
					Detail:   "本能力不展开压缩包；解包到 01 源文件/ 后重新运行 epub run epub.source.intake",
					Location: f.Path,
				})
			case RiskSymlink:
				findings = append(findings, report.Finding{
					Level: "warn", ID: FindingSymlink, Title: "Symbolic link was not followed",
					Detail:   "为避免越出输入目录，符号链接只记录不读取；如需纳入请复制真实文件",
					Location: f.Path,
				})
			}
		}
	}
	for _, rel := range sc.unreadableFile {
		findings = append(findings, report.Finding{
			Level: "warn", ID: FindingUnreadableFile, Title: "File could not be read",
			Detail: "无法打开或读取该文件，未计算 SHA-256；条目带 risks=[\"unreadable\"]，" +
				"其 size=0 与空 sha256 不代表文件真的为空",
			Location: rel,
		})
	}
	for _, rel := range sc.unreadableDir {
		findings = append(findings, report.Finding{
			Level: "warn", ID: FindingUnreadableDir, Title: "Directory could not be listed",
			Detail: "无法列出该目录，整棵子树未纳入盘点；其余条目照常盘点。" +
				"修复权限后重跑，否则不要据本次 facts.files 写来源记录",
			Location: rel,
		})
	}

	if (sc.isDir && len(sc.files) == 0) || sc.truncated {
		res.Status = report.StatusFailed
	}
	findings = append(findings, report.Finding{
		Level: "info", ID: FindingSummary,
		Title:  fmt.Sprintf("%d files, %d bytes, roles: %s", len(sc.files), totalBytes, formatRoleCounts(roleCounts)),
		Detail: "facts.files 列出每个文件的角色、SHA-256 与风险；facts.plan 是后续 EPUB 链的 execution-plan 建议",
	})
	res.Findings = findings

	plan := buildPlan(sc)

	res.Facts = map[string]any{
		"sourcePath": sc.root,
		"inputKind":  inputKind(sc),
		"fileCount":  len(sc.files),
		"totalBytes": totalBytes,
		"roleCounts": roleCounts,
		// 空目录（intake.empty，本能力的一等错误路径）时 sc.files 是 nil，
		// 直接放进 facts 就序列化成 null；SKILL.md 声明的是 files[] 数组。
		"files": nonNilFiles(sc.files),
		// 交出副本：workspacePlan 是包级静态表（INV-7 白名单，仅 init 期写入），
		// 直接放进 Facts 会把它交给调用方随意改写，也会与并发 JSON 编码竞争。
		"workspacePlan": maps.Clone(workspacePlan),
		"plan":          plan,
		"blockers":      plan.Blockers,
	}
	res.Events = []report.Event{{
		Step: "scan", Status: "completed",
		Message: fmt.Sprintf("%d files inspected (read-only)", len(sc.files)),
	}}
	for i, abs := range epubs {
		if i >= maxNextCommands {
			break
		}
		res.NextCommands = append(res.NextCommands, navAuditCommand(abs))
	}
	return res
}

// navAuditCommand 是 intake.already-epub 的 detail 与 nextCommands 共用的
// 单一来源：两处必须逐字相同，且能被直接粘进 shell（书名常含空格与括号）。
func navAuditCommand(abs string) string {
	return "epub run epub.package.nav.audit --input " + shellQuote(abs) + " --json"
}

// shellQuote 让路径可以被安全地粘进 POSIX shell：只含安全字符时原样返回
// （保持既有命令文本不变），否则用单引号包裹并转义内部单引号。
// 非 ASCII 字符对 shell 没有特殊含义，按安全处理，避免中文书名被整体引号包住。
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		if r > 0x7F {
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			continue
		}
		switch r {
		case '@', '%', '+', '=', ':', ',', '.', '/', '-', '_':
			continue
		}
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

func inputKind(sc *scanResult) string {
	if sc.isDir {
		return "directory"
	}
	return "file"
}

func formatRoleCounts(rc map[string]int) string {
	if len(rc) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(rc))
	for k := range rc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, rc[k]))
	}
	return strings.Join(parts, " ")
}

// buildPlan 生成契约 outputSchema（execution-plan）形状的下游链建议。
//
// blocker 推断只看**读得到**的条目：unreadable 条目没有 sha256、没有做过
// 编码/图片探测，把它们当成正常文本或正常图片会给出假的"可以进入下游链"。
func buildPlan(sc *scanResult) Plan {
	art := PlanArtifact{URI: sc.root, Kind: "source-directory"}
	if !sc.isDir && len(sc.files) == 1 {
		f := sc.files[0]
		art.Kind = artifactKind(f)
		if f.SHA256 != "" {
			art.ContentDigest = "sha256:" + f.SHA256
		}
	}
	steps := []PlanStep{
		{ID: "nav-audit", Capability: "epub.package.nav.audit", Kind: "inspect", DependsOn: []string{}, RequiresApproval: false},
		{ID: "structure-normalize", Capability: "epub.structure.normalize", Kind: "transform", DependsOn: []string{"nav-audit"}, RequiresApproval: true},
		{ID: "migrate-epub3", Capability: "epub.package.migrate.epub3", Kind: "transform", DependsOn: []string{"structure-normalize"}, RequiresApproval: true},
		{ID: "layout-audit", Capability: "epub.layout.audit", Kind: "inspect", DependsOn: []string{"migrate-epub3"}, RequiresApproval: false},
	}
	roleCounts := map[string]int{}
	for _, f := range sc.files {
		if slices.Contains(f.Risks, RiskUnreadable) {
			continue
		}
		roleCounts[f.Role]++
	}
	blockers := []string{}
	if len(sc.files) == 0 {
		blockers = append(blockers, "无输入文件：先把入选源材料放入 01 源文件/")
	}
	if sc.truncated {
		blockers = append(blockers, "文件数超过上限，盘点不完整；缩小输入目录后重跑")
	}
	if n := len(sc.unreadableFile) + len(sc.unreadableDir); n > 0 {
		blockers = append(blockers,
			fmt.Sprintf("%d 个文件/目录无法读取，盘点不完整；修复权限后重跑再写来源记录", n))
	}
	if roleCounts[RolePDF] > 0 {
		blockers = append(blockers, "PDF 输入需外部抽取 + 抽样校对（本仓不解析 PDF / 不做 OCR）；扫描件须标记 OCR 风险并记录工具名与版本")
	}
	risky := 0
	nonUTF8 := 0
	for _, f := range sc.files {
		for _, r := range f.Risks {
			switch r {
			case RiskImageFormat, RiskJPEGCMYK:
				risky++
			case RiskEncodingNotUTF8:
				nonUTF8++
			}
		}
	}
	if risky > 0 {
		blockers = append(blockers, fmt.Sprintf("%d 个图片不是 EPUB 核心媒体类型或为 CMYK JPEG，需外部转码", risky))
	}
	if nonUTF8 > 0 {
		blockers = append(blockers, fmt.Sprintf("%d 个文本/HTML 文件不是合法 UTF-8，需先转码", nonUTF8))
	}
	if roleCounts[RoleArchive] > 0 {
		blockers = append(blockers, "嵌套压缩包需先解包再重新接入")
	}
	if len(sc.files) > 0 && roleCounts[RoleEPUB] == 0 && roleCounts[RoleText]+roleCounts[RoleHTML]+roleCounts[RoleDocument] == 0 {
		blockers = append(blockers, "尚无 EPUB 或可结构化文本（txt/md/html/docx），需人工建立 source tree 后才能进入下游链")
	}
	return Plan{SchemaVersion: "1", Artifact: art, Steps: steps, Blockers: blockers}
}

// artifactKind 把单文件映射到 artifact-reference.schema.json 的 kind 枚举。
func artifactKind(f FileEntry) string {
	switch f.Role {
	case RoleEPUB:
		return "epub"
	case RoleText:
		if ext := strings.ToLower(filepath.Ext(f.Path)); ext == ".md" || ext == ".markdown" {
			return "markdown"
		}
		return "unknown"
	case RoleHTML:
		return "html"
	case RolePDF:
		return "pdf"
	case RoleImage:
		return "image-set"
	default:
		return "unknown"
	}
}
