package sourceintake

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/report"
)

func writeFile(t *testing.T, dir, rel string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// cmykJPEGHeader 是只含 SOI + SOF0（Nf=4）+ EOI 的最小 JPEG 头。
func cmykJPEGHeader() []byte {
	return []byte{
		0xFF, 0xD8,
		0xFF, 0xC0, 0x00, 0x14, // SOF0, len 20 = 2 + 6 + 4*3
		0x08, 0x00, 0x01, 0x00, 0x01, 0x04,
		0x01, 0x11, 0x00, 0x02, 0x11, 0x00, 0x03, 0x11, 0x00, 0x04, 0x11, 0x00,
		0xFF, 0xD9,
	}
}

func rgbJPEGHeader() []byte {
	return []byte{
		0xFF, 0xD8,
		0xFF, 0xE0, 0x00, 0x04, 0x00, 0x00, // APP0 空段
		0xFF, 0xC0, 0x00, 0x11,
		0x08, 0x00, 0x01, 0x00, 0x01, 0x03,
		0x01, 0x11, 0x00, 0x02, 0x11, 0x00, 0x03, 0x11, 0x00,
		0xFF, 0xDA, 0x00, 0x02, // SOS
		0xFF, 0xD9,
	}
}

func minimalEPUB(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fw, err := w.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("application/epub+zip"))
	fw, _ = w.Create("META-INF/container.xml")
	fw.Write([]byte(`<container/>`))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildFixture 构造覆盖全部角色与风险的临时源目录。
func buildFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "chapter01.txt", append([]byte{0xEF, 0xBB, 0xBF}, []byte("第一章\r\n正文。\r\n")...))
	writeFile(t, dir, "README.md", []byte("# 书名\n\n简介。\n"))
	writeFile(t, dir, "notes/intro.HTML", []byte("<html><body><p>hi</p></body></html>\n"))
	writeFile(t, dir, "notes/legacy.txt", []byte("\xb5\xda\xd2\xbb\xd5\xc2 GBK\n")) // 非 UTF-8
	writeFile(t, dir, "images/cover.png", tinyPNG(t))
	writeFile(t, dir, "images/photo.webp", []byte("RIFF....WEBPVP8 "))
	writeFile(t, dir, "images/print.jpg", cmykJPEGHeader())
	writeFile(t, dir, "images/web.jpeg", rgbJPEGHeader())
	writeFile(t, dir, "scan.pdf", []byte("%PDF-1.4\n%%EOF\n"))
	writeFile(t, dir, "old/book.epub", minimalEPUB(t))
	writeFile(t, dir, "raw.zip", []byte("PK\x05\x06"))
	writeFile(t, dir, ".hidden.txt", []byte("skip"))
	writeFile(t, dir, ".DS_Store", []byte{0})
	writeFile(t, dir, ".git/config", []byte("skip"))
	writeFile(t, dir, "fonts/Body.TTF", []byte{0, 1, 0, 0})
	writeFile(t, dir, "mystery.xyz", []byte("?"))
	return dir
}

func findFile(t *testing.T, files []FileEntry, path string) FileEntry {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("file %q missing from facts.files; have %v", path, paths(files))
	return FileEntry{}
}

func paths(files []FileEntry) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func findingIDs(fs []report.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.ID)
	}
	return out
}

func TestRunDirectoryClassifiesRolesAndRisks(t *testing.T) {
	dir := buildFixture(t)
	res, err := Run(t.Context(), nil, Params{SourcePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Errorf("status = %q, want complete; findings %v", res.Status, findingIDs(res.Findings))
	}
	files := res.Facts["files"].([]FileEntry)
	want := []string{
		"README.md", "chapter01.txt", "fonts/Body.TTF", "images/cover.png", "images/photo.webp",
		"images/print.jpg", "images/web.jpeg", "mystery.xyz", "notes/intro.HTML", "notes/legacy.txt",
		"old/book.epub", "raw.zip", "scan.pdf",
	}
	if got := paths(files); !slices.Equal(got, want) {
		t.Errorf("files order/contents = %v, want %v", got, want)
	}
	if res.Facts["fileCount"] != len(want) {
		t.Errorf("fileCount = %v", res.Facts["fileCount"])
	}

	cases := map[string]struct {
		role  string
		risks []string
	}{
		"chapter01.txt":     {RoleText, []string{RiskUTF8BOM, RiskCRLF}},
		"README.md":         {RoleText, []string{}},
		"notes/intro.HTML":  {RoleHTML, []string{}},
		"notes/legacy.txt":  {RoleText, []string{RiskEncodingNotUTF8}},
		"images/cover.png":  {RoleImage, []string{}},
		"images/photo.webp": {RoleImage, []string{RiskImageFormat}},
		"images/print.jpg":  {RoleImage, []string{RiskJPEGCMYK}},
		"images/web.jpeg":   {RoleImage, []string{}},
		"scan.pdf":          {RolePDF, []string{RiskPDFOutOfScope}},
		"old/book.epub":     {RoleEPUB, []string{RiskAlreadyEPUB}},
		"raw.zip":           {RoleArchive, []string{RiskNestedArchive}},
		"fonts/Body.TTF":    {RoleFont, []string{}},
		"mystery.xyz":       {RoleUnknown, []string{}},
	}
	for path, c := range cases {
		f := findFile(t, files, path)
		if f.Role != c.role {
			t.Errorf("%s role = %q, want %q", path, f.Role, c.role)
		}
		if !slices.Equal(f.Risks, c.risks) {
			t.Errorf("%s risks = %v, want %v", path, f.Risks, c.risks)
		}
		if f.SHA256 == "" || f.Size <= 0 {
			t.Errorf("%s: sha256/size missing (%q, %d)", path, f.SHA256, f.Size)
		}
	}
	sum := sha256.Sum256([]byte("# 书名\n\n简介。\n"))
	if got := findFile(t, files, "README.md").SHA256; got != hex.EncodeToString(sum[:]) {
		t.Errorf("README.md sha256 = %s", got)
	}

	rc := res.Facts["roleCounts"].(map[string]int)
	if rc[RoleImage] != 4 || rc[RoleText] != 3 || rc[RoleHTML] != 1 || rc[RolePDF] != 1 {
		t.Errorf("roleCounts = %v", rc)
	}

	ids := findingIDs(res.Findings)
	for _, want := range []string{FindingPDFOutOfScope, FindingImageFormat, FindingAlreadyEPUB, FindingEncoding, FindingNestedArchive, FindingSummary} {
		if !slices.Contains(ids, want) {
			t.Errorf("missing finding %s in %v", want, ids)
		}
	}
	for _, f := range res.Findings {
		if f.Level == "error" {
			t.Errorf("unexpected error finding %s", f.ID)
		}
	}
	if n := countID(res.Findings, FindingImageFormat); n != 2 {
		t.Errorf("image-format-risk findings = %d, want 2 (webp + cmyk jpeg)", n)
	}

	if len(res.NextCommands) != 1 || !strings.Contains(res.NextCommands[0], "epub run epub.package.nav.audit --input ") ||
		!strings.HasSuffix(res.NextCommands[0], filepath.Join("old", "book.epub")+" --json") {
		t.Errorf("nextCommands = %v", res.NextCommands)
	}
	if _, ok := res.Facts["workspacePlan"].(map[string]string); !ok {
		t.Errorf("workspacePlan missing")
	}
	if res.Facts["inputKind"] != "directory" || res.Facts["sourcePath"] != dir {
		t.Errorf("inputKind/sourcePath = %v / %v", res.Facts["inputKind"], res.Facts["sourcePath"])
	}
}

func countID(fs []report.Finding, id string) int {
	n := 0
	for _, f := range fs {
		if f.ID == id {
			n++
		}
	}
	return n
}

func TestRunPlanMatchesExecutionPlanSchema(t *testing.T) {
	dir := buildFixture(t)
	res, err := Run(t.Context(), nil, Params{SourcePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	plan := res.Facts["plan"].(Plan)
	if plan.SchemaVersion != "1" || plan.Artifact.URI != dir || plan.Artifact.Kind != "source-directory" {
		t.Errorf("plan header = %+v", plan)
	}
	wantSteps := []string{"epub.package.nav.audit", "epub.structure.normalize", "epub.package.migrate.epub3", "epub.layout.audit"}
	var gotSteps []string
	for _, s := range plan.Steps {
		gotSteps = append(gotSteps, s.Capability)
	}
	if !slices.Equal(gotSteps, wantSteps) {
		t.Errorf("steps = %v", gotSteps)
	}
	if !plan.Steps[1].RequiresApproval || !plan.Steps[2].RequiresApproval || plan.Steps[0].RequiresApproval {
		t.Errorf("requiresApproval flags wrong: %+v", plan.Steps)
	}
	if !slices.Equal(plan.Steps[2].DependsOn, []string{"structure-normalize"}) {
		t.Errorf("dependsOn = %v", plan.Steps[2].DependsOn)
	}
	if !slices.ContainsFunc(plan.Blockers, func(s string) bool { return strings.Contains(s, "PDF") }) {
		t.Errorf("blockers should mention PDF: %v", plan.Blockers)
	}
	blockers, _ := res.Facts["blockers"].([]string)
	if !slices.Equal(blockers, plan.Blockers) {
		t.Errorf("facts.blockers != plan.blockers")
	}

	// 用契约 schema 的键集做最小形状校验（required + additionalProperties=false）。
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	schemaRaw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", "v1", "execution-plan.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Items struct {
				Required   []string       `json:"required"`
				Properties map[string]any `json:"properties"`
			} `json:"items"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		t.Fatal(err)
	}
	for _, k := range schema.Required {
		if _, ok := doc[k]; !ok {
			t.Errorf("plan missing required key %q", k)
		}
	}
	for k := range doc {
		if _, ok := schema.Properties[k]; !ok {
			t.Errorf("plan has key %q not in schema", k)
		}
	}
	stepSchema := schema.Properties["steps"].Items
	for _, s := range doc["steps"].([]any) {
		obj := s.(map[string]any)
		for _, k := range stepSchema.Required {
			if _, ok := obj[k]; !ok {
				t.Errorf("step missing %q", k)
			}
		}
		for k := range obj {
			if _, ok := stepSchema.Properties[k]; !ok {
				t.Errorf("step has extra key %q", k)
			}
		}
	}
}

func TestRunSingleFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "novel.md", []byte("# 标题\n"))
	res, err := Run(t.Context(), nil, Params{SourcePath: p})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Errorf("status = %q", res.Status)
	}
	files := res.Facts["files"].([]FileEntry)
	if len(files) != 1 || files[0].Path != "novel.md" || files[0].Role != RoleText {
		t.Errorf("files = %+v", files)
	}
	plan := res.Facts["plan"].(Plan)
	if plan.Artifact.Kind != "markdown" || !strings.HasPrefix(plan.Artifact.ContentDigest, "sha256:") {
		t.Errorf("artifact = %+v", plan.Artifact)
	}
	if res.Facts["inputKind"] != "file" {
		t.Errorf("inputKind = %v", res.Facts["inputKind"])
	}
}

func TestRunEmptyDirectoryFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".DS_Store", []byte{0})
	res, err := Run(t.Context(), nil, Params{SourcePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed {
		t.Errorf("status = %q, want failed", res.Status)
	}
	if !slices.Contains(findingIDs(res.Findings), FindingEmpty) {
		t.Errorf("findings = %v", findingIDs(res.Findings))
	}
}

func TestRunTooManyFiles(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, dir, n, []byte("x"))
	}
	res, err := Run(t.Context(), nil, Params{SourcePath: dir, MaxFiles: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || !slices.Contains(findingIDs(res.Findings), FindingTooManyFiles) {
		t.Errorf("status=%q findings=%v", res.Status, findingIDs(res.Findings))
	}
	if res.Facts["fileCount"] != 2 {
		t.Errorf("fileCount = %v, want 2 (stopped at cap)", res.Facts["fileCount"])
	}
}

func TestRunSymlinkNotFollowed(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "real.txt", []byte("real"))
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_ = target
	res, err := Run(t.Context(), nil, Params{SourcePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	files := res.Facts["files"].([]FileEntry)
	link := findFile(t, files, "link.txt")
	if link.Role != RoleUnknown || !slices.Equal(link.Risks, []string{RiskSymlink}) || link.SHA256 != "" {
		t.Errorf("symlink entry = %+v", link)
	}
	if !slices.Contains(findingIDs(res.Findings), FindingSymlink) {
		t.Errorf("missing intake.symlink finding")
	}
}

func TestRunMissingParams(t *testing.T) {
	if _, err := Run(t.Context(), nil, Params{}); err == nil {
		t.Error("expected error for empty SourcePath")
	}
	if _, err := Run(t.Context(), nil, Params{SourcePath: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Error("expected error for missing path")
	}
}

func TestTextProbeChunkBoundaries(t *testing.T) {
	// 多字节序列与 \r\n 跨块切分时仍判定正确。
	body := []byte("行一\r\n行二 中文")
	for cut := 1; cut < len(body); cut++ {
		p := &textProbe{valid: true}
		p.Write(body[:cut])
		p.Write(body[cut:])
		p.finish()
		if !p.valid || !p.crlf || p.bom {
			t.Errorf("cut=%d valid=%v crlf=%v bom=%v", cut, p.valid, p.crlf, p.bom)
		}
	}
	p := &textProbe{valid: true}
	p.Write([]byte("ok\xe4\xb8"))
	p.finish()
	if p.valid {
		t.Error("truncated multibyte tail must be invalid")
	}
}

// TestRunSingleEPUBFileCommandBuildsRealPath 回归 HIGH-1 / LOW-2：单文件输入时
// root 本身就是那个文件，nextCommands 与 intake.already-epub 的 detail 都必须
// 是可直接执行的同一条命令，不能把文件名拼两遍（<file>/<basename>）。
func TestRunSingleEPUBFileCommandBuildsRealPath(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "book.epub", minimalEPUB(t))
	res, err := Run(t.Context(), nil, Params{SourcePath: p})
	if err != nil {
		t.Fatal(err)
	}
	want := "epub run epub.package.nav.audit --input " + p + " --json"
	if len(res.NextCommands) != 1 || res.NextCommands[0] != want {
		t.Errorf("nextCommands = %v, want [%q]", res.NextCommands, want)
	}
	var detail string
	for _, f := range res.Findings {
		if f.ID == FindingAlreadyEPUB {
			detail = f.Detail
		}
	}
	if !strings.HasSuffix(detail, want) {
		t.Errorf("already-epub detail = %q, want suffix %q", detail, want)
	}
	// 命令里的路径必须真实存在（doubled path 会让 CLI 报 input not found）。
	cut := strings.Index(want, "--input ") + len("--input ")
	got := strings.TrimSuffix(want[cut:], " --json")
	if _, err := os.Stat(got); err != nil {
		t.Errorf("emitted --input path does not exist: %v", err)
	}
	files := res.Facts["files"].([]FileEntry)
	if len(files) != 1 || files[0].Path != "book.epub" {
		t.Errorf("files = %+v", files)
	}
}

// TestNavAuditCommandQuotesPaths 回归 LOW-1：含空格/括号的书名（这类文件极常见）
// 必须被 shell 引用，否则 agent 逐字粘贴会被拆成多个参数。
func TestNavAuditCommandQuotesPaths(t *testing.T) {
	cases := map[string]string{
		"/a/b/book.epub":             "/a/b/book.epub",
		"/a/sp ace/my book.epub":     "'/a/sp ace/my book.epub'",
		"/a/b/指南 (赤霓).epub":          "'/a/b/指南 (赤霓).epub'",
		"/a/b/中文书名.epub":             "/a/b/中文书名.epub",
		"/a/it's/book.epub":          `'/a/it'\''s/book.epub'`,
		"/a/b/$(rm -rf ~)/book.epub": "'/a/b/$(rm -rf ~)/book.epub'",
	}
	for in, wantQuoted := range cases {
		if got := shellQuote(in); got != wantQuoted {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, wantQuoted)
		}
		if got, want := navAuditCommand(in), "epub run epub.package.nav.audit --input "+wantQuoted+" --json"; got != want {
			t.Errorf("navAuditCommand(%q) = %q, want %q", in, got, want)
		}
	}
	if got := shellQuote(""); got != "''" {
		t.Errorf("shellQuote(\"\") = %q", got)
	}
}

// TestRunSymlinkedRootIsScanned 回归 HIGH-2：root 本身是符号链接目录时
// （iCloud / Dropbox 镜像、软链的「01 源文件/」）必须照常下降，不能被
// WalkDir 的 Lstat 当成叶子而报 intake.empty。树内部仍不跟随符号链接。
func TestRunSymlinkedRootIsScanned(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "realdir")
	if err := os.MkdirAll(filepath.Join(real, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "notes", "b.md"), []byte("# b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "linkdir")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	res, err := Run(t.Context(), nil, Params{SourcePath: link})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %q, findings %v", res.Status, findingIDs(res.Findings))
	}
	files := res.Facts["files"].([]FileEntry)
	if got := paths(files); !slices.Equal(got, []string{"a.txt", "notes/b.md"}) {
		t.Errorf("files = %v, want [a.txt notes/b.md]", got)
	}
	// 对外报告保留用户给出的路径，不替换成 EvalSymlinks 的结果。
	if res.Facts["sourcePath"] != link {
		t.Errorf("sourcePath = %v, want %q", res.Facts["sourcePath"], link)
	}
	if slices.Contains(findingIDs(res.Findings), FindingEmpty) {
		t.Errorf("symlinked root must not report intake.empty")
	}
}

// TestRunUnreadableDirKeepsInventory 回归 MEDIUM-1：一个不可读子目录只丢那棵
// 子树（warn + blocker），不得让整次盘点变成 capability.run-failed 并丢掉
// 已经哈希过的全部条目。
func TestRunUnreadableDirKeepsInventory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 可以读取 chmod 000 目录")
	}
	dir := t.TempDir()
	writeFile(t, dir, "keep.txt", []byte("kept\n"))
	writeFile(t, dir, "images/cover.png", tinyPNG(t))
	locked := filepath.Join(dir, "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "locked/inner.txt", []byte("x\n"))
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Skipf("chmod unsupported: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	res, err := Run(t.Context(), nil, Params{SourcePath: dir})
	if err != nil {
		t.Fatalf("unreadable subdirectory must not abort the scan: %v", err)
	}
	if res.Status != report.StatusComplete {
		t.Errorf("status = %q, want complete", res.Status)
	}
	files := res.Facts["files"].([]FileEntry)
	if got := paths(files); !slices.Equal(got, []string{"images/cover.png", "keep.txt"}) {
		t.Fatalf("files = %v, want the readable entries to survive", got)
	}
	if findFile(t, files, "keep.txt").SHA256 == "" {
		t.Error("readable entry lost its sha256")
	}
	var loc string
	for _, f := range res.Findings {
		if f.ID == FindingUnreadableDir {
			loc = f.Location
			if f.Level != "warn" {
				t.Errorf("unreadable-dir level = %q, want warn", f.Level)
			}
		}
	}
	if loc != "locked" {
		t.Errorf("unreadable-dir location = %q, want %q", loc, "locked")
	}
	blockers := res.Facts["blockers"].([]string)
	if !slices.ContainsFunc(blockers, func(s string) bool { return strings.Contains(s, "无法读取") }) {
		t.Errorf("incomplete inventory must show up in blockers: %v", blockers)
	}
}

// TestRunUnreadableFileIsTaggedAndNotABlockerSource 回归 MEDIUM-1 的后半：
// 读不出的条目必须带 risks=["unreadable"]（size=0 / 空 sha256 不等于"文件为空"），
// 并且不得被 blocker 推断当作可结构化正文。
func TestRunUnreadableFileIsTaggedAndNotABlockerSource(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 可以读取 chmod 000 文件")
	}
	dir := t.TempDir()
	locked := writeFile(t, dir, "chapter.txt", []byte("正文\n"))
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Skipf("chmod unsupported: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })

	res, err := Run(t.Context(), nil, Params{SourcePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	files := res.Facts["files"].([]FileEntry)
	entry := findFile(t, files, "chapter.txt")
	if !slices.Equal(entry.Risks, []string{RiskUnreadable}) {
		t.Errorf("risks = %v, want [%s]", entry.Risks, RiskUnreadable)
	}
	if entry.SHA256 != "" || entry.Size != 0 {
		t.Errorf("unreadable entry should carry no digest: %+v", entry)
	}
	if !slices.Contains(findingIDs(res.Findings), FindingUnreadableFile) {
		t.Errorf("missing %s: %v", FindingUnreadableFile, findingIDs(res.Findings))
	}
	// 唯一的 .txt 读不出来 ⇒ 依然「尚无可结构化正文」，不能因为扩展名就放行。
	blockers := res.Facts["blockers"].([]string)
	if !slices.ContainsFunc(blockers, func(s string) bool { return strings.Contains(s, "尚无 EPUB 或可结构化文本") }) {
		t.Errorf("unreadable text must not satisfy the source-tree blocker: %v", blockers)
	}
}

// TestWorkspacePlanFactIsACopy 回归 MEDIUM-5：facts 里交出的必须是副本，
// 消费者改它不得污染包级静态表（INV-7 的意图是只读注册表）。
func TestWorkspacePlanFactIsACopy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", []byte("a\n"))
	res, err := Run(t.Context(), nil, Params{SourcePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Facts["workspacePlan"].(map[string]string)
	const key = "01 源文件/"
	before := workspacePlan[key]
	got[key] = "TAMPERED"
	delete(got, "制作说明.md")
	if workspacePlan[key] != before {
		t.Errorf("package-level workspacePlan was mutated through Facts: %q", workspacePlan[key])
	}
	if _, ok := workspacePlan["制作说明.md"]; !ok {
		t.Error("package-level workspacePlan lost a key through Facts")
	}
	res2, err := Run(t.Context(), nil, Params{SourcePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	if fresh := res2.Facts["workspacePlan"].(map[string]string); fresh[key] != before {
		t.Errorf("second run inherited the mutation: %q", fresh[key])
	}
}

// TestRunHonoursCancelledContext 回归 LOW-4：ctx 已取消时扫描立即返回，
// 不是只在条目之间才检查。
func TestRunHonoursCancelledContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", bytes.Repeat([]byte("x"), 1<<20))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Run(ctx, nil, Params{SourcePath: dir}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	single := writeFile(t, dir, "b.txt", bytes.Repeat([]byte("y"), 1<<20))
	if _, err := Run(ctx, nil, Params{SourcePath: single}); !errors.Is(err, context.Canceled) {
		t.Errorf("single-file err = %v, want context.Canceled", err)
	}
}
