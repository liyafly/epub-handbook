package pipeline

// epub book new 的起始书个性化：每本新书独立的 urn:uuid 标识符、建书时刻的
// dcterms:modified，以及可选的 --title / --author。起始书是受控模板，这里按
// 占位文本做替换；每个占位在其文件中必须恰好命中一次，否则建书失败。

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

const (
	starterIdentifier = "urn:uuid:00000000-0000-4000-8000-000000000001"
	starterModified   = "2026-06-12T00:00:00Z"
	starterTitle      = "书名（请修改）"
	starterAuthor     = "作者（请修改）"
)

// starterReplacement 是起始书某个文件里一处必须恰好命中一次的占位。
type starterReplacement struct {
	file string // 相对 epub/ 的路径
	old  string
	new  string
}

// newBookUUID 生成 RFC 4122 v4 UUID 的 urn 形式。
func newBookUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func xmlText(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// starterReplacements 列出新书需要替换的全部占位。title/author 为空时保留占位文字。
func starterReplacements(id string, now time.Time, title, author string) []starterReplacement {
	modified := now.UTC().Format("2006-01-02T15:04:05Z")
	reps := []starterReplacement{
		{"OEBPS/package.opf", `<dc:identifier id="bookid">` + starterIdentifier + `</dc:identifier>`, `<dc:identifier id="bookid">` + id + `</dc:identifier>`},
		{"OEBPS/toc.ncx", `<meta name="dtb:uid" content="` + starterIdentifier + `"/>`, `<meta name="dtb:uid" content="` + id + `"/>`},
		{"OEBPS/package.opf", `<meta property="dcterms:modified">` + starterModified + `</meta>`, `<meta property="dcterms:modified">` + modified + `</meta>`},
	}
	if title != "" {
		t := xmlText(title)
		reps = append(reps,
			starterReplacement{"OEBPS/package.opf", "<dc:title>" + starterTitle + "</dc:title>", "<dc:title>" + t + "</dc:title>"},
			starterReplacement{"OEBPS/toc.ncx", "<docTitle><text>" + starterTitle + "</text></docTitle>", "<docTitle><text>" + t + "</text></docTitle>"},
			starterReplacement{"OEBPS/Text/00-title.xhtml", "<h1>" + starterTitle + "</h1>", "<h1>" + t + "</h1>"},
			starterReplacement{"OEBPS/Text/00-copyright.xhtml", "<dd>" + starterTitle + "</dd>", "<dd>" + t + "</dd>"},
		)
	}
	if author != "" {
		a := xmlText(author)
		reps = append(reps,
			starterReplacement{"OEBPS/package.opf", "<dc:creator>" + starterAuthor + "</dc:creator>", "<dc:creator>" + a + "</dc:creator>"},
			starterReplacement{"OEBPS/Text/00-title.xhtml", "<p>" + starterAuthor + "</p>", "<p>" + a + "</p>"},
			starterReplacement{"OEBPS/Text/00-copyright.xhtml", "<dd>" + starterAuthor + "</dd>", "<dd>" + a + "</dd>"},
		)
	}
	return reps
}

// personalizeStarter 在已复制的起始书上应用占位替换。全部替换先在内存里算好，
// 任一占位不是恰好命中一次就报错且不写任何文件。
func personalizeStarter(epubDir string, reps []starterReplacement) error {
	contents := map[string]string{}
	var order []string
	for _, r := range reps {
		data, ok := contents[r.file]
		if !ok {
			raw, err := os.ReadFile(filepath.Join(epubDir, filepath.FromSlash(r.file)))
			if err != nil {
				return err
			}
			data = string(raw)
			order = append(order, r.file)
		}
		if n := strings.Count(data, r.old); n != 1 {
			return fmt.Errorf("starter placeholder %q in %s matched %d times, want exactly 1", r.old, r.file, n)
		}
		contents[r.file] = strings.Replace(data, r.old, r.new, 1)
	}
	for _, file := range order {
		if err := zipfs.WriteFileAtomic(filepath.Join(epubDir, filepath.FromSlash(file)), []byte(contents[file]), 0o644); err != nil {
			return err
		}
	}
	return nil
}
