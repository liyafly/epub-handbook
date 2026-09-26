package pipeline

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cleanBenchmarkTextBytes = 2 << 20

func BenchmarkCleanPipelineTextBook(b *testing.B) {
	input := benchmarkCleanTextEPUB(b, cleanBenchmarkTextBytes)
	root := b.TempDir()
	b.SetBytes(cleanBenchmarkTextBytes)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		outputDir := filepath.Join(root, fmt.Sprintf("out-%d", i))
		_, err := Clean(b.Context(), CleanOptions{
			InputPath: input, OutputDir: outputDir, Steps: []string{"normalize"}, Jobs: 1,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkCleanTextEPUB(b *testing.B, payloadBytes int) string {
	b.Helper()
	base := epubFixtureBytes(b)
	reader, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		b.Fatal(err)
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	text := benchmarkText(payloadBytes)
	for _, entry := range reader.File {
		source, err := entry.Open()
		if err != nil {
			b.Fatal(err)
		}
		content, readErr := io.ReadAll(source)
		closeErr := source.Close()
		if readErr != nil {
			b.Fatal(readErr)
		}
		if closeErr != nil {
			b.Fatal(closeErr)
		}
		if entry.Name == "OEBPS/c1.xhtml" {
			content = bytes.Replace(content, []byte("</body>"), []byte("<p>"+text+"</p></body>"), 1)
		}
		header := entry.FileHeader
		out, err := writer.CreateHeader(&header)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := out.Write(content); err != nil {
			b.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(b.TempDir(), "clean-benchmark.epub")
	if err := os.WriteFile(path, archive.Bytes(), 0o644); err != nil {
		b.Fatal(err)
	}
	return path
}

func benchmarkText(size int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .,;:!?"
	var text strings.Builder
	text.Grow(size)
	state := uint32(0x9e3779b9)
	for text.Len() < size {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		text.WriteByte(alphabet[state%uint32(len(alphabet))])
	}
	return text.String()
}
