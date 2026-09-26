// Package fontsubset implements the font subsetting capability through the
// separately installed epub-font provider.
package fontsubset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/extern"
	"github.com/liyafly/epub-handbook/internal/report"
	opfscan "github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID is the contract id for font subsetting.
const CapabilityID = "epub.font.subset"

// Params contains the user-selected font master configuration.
type Params struct {
	FontConfig string
	ToolPath   string // Test-only provider override.
}

// Run subsets manifest fonts against the complete font masters available to
// the provider. It applies only changed font entries to the in-memory book;
// the pipeline writes the final EPUB once after the complete redline gate.
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	res := report.Result{Capability: CapabilityID, Status: report.StatusComplete}
	input := b.InputPath()
	if input == "" {
		return failure(&res, "font-subset.input-missing", "font subset requires an EPUB file input")
	}
	input, err := filepath.Abs(input)
	if err != nil {
		return failure(&res, "font-subset.input-path-invalid", err.Error())
	}
	fontPaths, err := manifestFonts(ctx, b)
	if err != nil {
		return failure(&res, "font-subset.manifest-invalid", err.Error())
	}
	if len(fontPaths) == 0 {
		return failure(&res, "font-subset.no-fonts", "the EPUB has no manifest fonts to subset")
	}
	tool := p.ToolPath
	if tool == "" {
		tool = "epub-font"
	}
	if err := extern.Require(tool); err != nil {
		return failure(&res, "font-subset.provider-missing", "install the epub-font provider and retry")
	}
	workspace, cleanup, err := extern.TempDir("", "epub-font-subset-")
	if err != nil {
		return failure(&res, "font-subset.workspace-failed", err.Error())
	}
	defer func() { _ = cleanup() }()

	candidatePath := filepath.Join(workspace, "subset.epub")
	argv := []string{tool, "subset", input, "--out", candidatePath}
	if p.FontConfig != "" {
		configPath, err := filepath.Abs(p.FontConfig)
		if err != nil {
			return failure(&res, "font-subset.config-path-invalid", err.Error())
		}
		argv = append(argv, "--config", configPath)
	}
	provider, err := extern.Run(ctx, workspace, argv)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return res, err
		}
		return failure(&res, "font-subset.provider-failed", providerDetail(provider, err))
	}
	if provider.ExitCode != 0 {
		return failure(&res, "font-subset.check-failed", providerDetail(provider, fmt.Errorf("provider exit code %d", provider.ExitCode)))
	}

	candidate, err := book.OpenContext(ctx, candidatePath)
	if err != nil {
		return failure(&res, "font-subset.output-invalid", fmt.Sprintf("provider did not produce a valid EPUB: %v", err))
	}
	defer candidate.Close()
	if !slices.Equal(b.OriginalNames(), candidate.OriginalNames()) {
		return failure(&res, "font-subset.entry-set-changed", "provider changed EPUB entry names or order")
	}

	edits := make([]editset.Edit, 0, len(fontPaths))
	changed := make([]string, 0, len(fontPaths))
	for _, fontPath := range fontPaths {
		original, err := b.CurrentContext(ctx, fontPath)
		if err != nil {
			return failure(&res, "font-subset.input-font-missing", fmt.Sprintf("%s: %v", fontPath, err))
		}
		subset, err := candidate.OriginalContext(ctx, fontPath)
		if err != nil {
			return failure(&res, "font-subset.output-font-missing", fmt.Sprintf("%s: %v", fontPath, err))
		}
		if bytes.Equal(original, subset) {
			continue
		}
		edits = append(edits, editset.Edit{Path: fontPath, Offset: 0, Length: int64(len(original)), Replacement: subset})
		changed = append(changed, fontPath)
	}
	if err := b.Apply(edits); err != nil {
		return failure(&res, "font-subset.apply-failed", err.Error())
	}
	res.Facts = map[string]any{
		"provider":     "epub-font",
		"fontEntries":  fontPaths,
		"changedFonts": changed,
		"providerLog":  providerDetail(provider, nil),
	}
	return res, nil
}

func manifestFonts(ctx context.Context, b *book.Book) ([]string, error) {
	container, err := b.CurrentContext(ctx, opfscan.ContainerPath)
	if err != nil {
		return nil, fmt.Errorf("read container.xml: %w", err)
	}
	opfPath, err := opfscan.FindOPFPath(container)
	if err != nil {
		return nil, err
	}
	opfBytes, err := b.CurrentContext(ctx, opfPath)
	if err != nil {
		return nil, fmt.Errorf("read package document: %w", err)
	}
	pkg, err := opfscan.Parse(opfPath, opfBytes)
	if err != nil {
		return nil, err
	}
	fonts := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range pkg.Manifest {
		if item.ArchivePath == "" || !isFont(item.ArchivePath, item.MediaType) {
			continue
		}
		if _, ok := seen[item.ArchivePath]; ok {
			continue
		}
		seen[item.ArchivePath] = struct{}{}
		fonts = append(fonts, item.ArchivePath)
	}
	return fonts, nil
}

func isFont(archivePath, mediaType string) bool {
	if strings.Contains(strings.ToLower(mediaType), "font") {
		return true
	}
	switch strings.ToLower(filepath.Ext(archivePath)) {
	case ".ttf", ".otf", ".woff", ".woff2", ".ttc", ".otc":
		return true
	default:
		return false
	}
}

func providerDetail(result extern.CmdResult, err error) string {
	parts := make([]string, 0, 3)
	if err != nil {
		parts = append(parts, err.Error())
	}
	if detail := excerpt(result.Stderr); detail != "" {
		parts = append(parts, detail)
	}
	if detail := excerpt(result.Stdout); detail != "" {
		parts = append(parts, detail)
	}
	if len(parts) == 0 {
		return "provider did not return diagnostic output"
	}
	return strings.Join(parts, ": ")
}

func excerpt(data []byte) string {
	const limit = 4096
	value := strings.ToValidUTF8(strings.TrimSpace(string(data)), "�")
	if len(value) > limit {
		value = value[:limit] + "…"
	}
	return value
}

func failure(res *report.Result, id, detail string) (report.Result, error) {
	res.Status = report.StatusFailed
	res.Findings = append(res.Findings, report.Finding{
		Level: "error", ID: id, Title: "Font subsetting failed", Detail: detail,
	})
	return *res, errors.New(detail)
}
