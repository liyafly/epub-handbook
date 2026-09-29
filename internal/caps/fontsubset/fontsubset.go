// Package fontsubset implements the font subsetting capability through the
// separately installed epub-font provider.
package fontsubset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/extern"
	"github.com/liyafly/epub-handbook/internal/report"
	opfscan "github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID is the contract id for font subsetting.
const CapabilityID = "epub.font.subset"

const (
	providerReportSchemaVersion = 1
	maxProviderReportBytes      = 8 << 20
)

type providerArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type providerOutputArtifact struct {
	Path     string   `json:"path"`
	SHA256   string   `json:"sha256"`
	Warnings []string `json:"warnings"`
}

type providerFontMeta struct {
	Source  string `json:"source"`
	SHA256  string `json:"sha256"`
	Bytes   int64  `json:"bytes"`
	Glyphs  int    `json:"glyphs"`
	Outline string `json:"outline"`
}

type providerFontOutput struct {
	SHA256  string `json:"sha256"`
	Bytes   int64  `json:"bytes"`
	Glyphs  int    `json:"glyphs"`
	Outline string `json:"outline"`
}

type providerFontDigest struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type providerFontResult struct {
	Target             string                     `json:"target"`
	ManifestID         string                     `json:"manifestId"`
	MediaType          string                     `json:"mediaType"`
	Action             string                     `json:"action"`
	Reason             string                     `json:"reason"`
	Master             providerFontMeta           `json:"master"`
	Original           providerFontDigest         `json:"original"`
	Output             providerFontOutput         `json:"output"`
	RequiredCodepoints *int                       `json:"requiredCodepoints"`
	NotInMaster        []string                   `json:"notInMaster"`
	NotInMasterCount   *int                       `json:"notInMasterCount"`
	Checks             map[string]json.RawMessage `json:"checks"`
	OK                 bool                       `json:"ok"`
	Warnings           []string                   `json:"warnings"`
}

type providerSubsetReport struct {
	SchemaVersion   int                     `json:"schemaVersion"`
	Tool            string                  `json:"tool"`
	ProviderVersion string                  `json:"providerVersion"`
	FontTools       string                  `json:"fontTools"`
	Input           providerArtifact        `json:"input"`
	Fonts           []providerFontResult    `json:"fonts"`
	OK              bool                    `json:"ok"`
	Output          *providerOutputArtifact `json:"output"`
}

type providerFontSummary struct {
	Target             string                    `json:"target"`
	Action             string                    `json:"action"`
	Reason             string                    `json:"reason"`
	MasterSHA256       string                    `json:"masterSHA256"`
	OriginalSHA256     string                    `json:"originalSHA256"`
	OutputSHA256       string                    `json:"outputSHA256"`
	MasterBytes        int64                     `json:"masterBytes"`
	OriginalBytes      int64                     `json:"originalBytes"`
	OutputBytes        int64                     `json:"outputBytes"`
	MasterGlyphs       int                       `json:"masterGlyphs"`
	OutputGlyphs       int                       `json:"outputGlyphs"`
	RequiredCodepoints *int                      `json:"requiredCodepoints"`
	NotInMaster        []string                  `json:"notInMaster"`
	NotInMasterCount   *int                      `json:"notInMasterCount"`
	Checks             map[string]map[string]any `json:"checks"`
	Warnings           []string                  `json:"warnings"`
}

type providerReportSummary struct {
	SchemaVersion    int                   `json:"schemaVersion"`
	ProviderVersion  string                `json:"providerVersion"`
	FontToolsVersion string                `json:"fontToolsVersion"`
	InputSHA256      string                `json:"inputSHA256"`
	OutputSHA256     string                `json:"outputSHA256"`
	OK               bool                  `json:"ok"`
	Fonts            []providerFontSummary `json:"fonts"`
	Warnings         []string              `json:"warnings"`
}

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
	if err := ctx.Err(); err != nil {
		return res, err
	}
	if len(b.ModifiedNames()) > 0 {
		return failure(&res, "font-subset.stale-input", "font subsetting requires the unmodified input EPUB; run it before applying in-memory edits")
	}
	input := b.InputPath()
	if input == "" {
		return failure(&res, "font-subset.input-missing", "font subset requires an EPUB file input")
	}
	input, err := filepath.Abs(input)
	if err != nil {
		return failure(&res, "font-subset.input-path-invalid", err.Error())
	}
	manifestFontItems, err := manifestFonts(ctx, b)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return res, err
		}
		return failure(&res, "font-subset.manifest-invalid", err.Error())
	}
	if len(manifestFontItems) == 0 {
		return failure(&res, "font-subset.no-fonts", "the EPUB has no manifest fonts to subset")
	}
	fontPaths := make([]string, 0, len(manifestFontItems))
	for _, item := range manifestFontItems {
		fontPaths = append(fontPaths, item.ArchivePath)
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
	providerReportPath := filepath.Join(workspace, "subset.font-report.json")
	providerReportBytes, err := readProviderReport(ctx, providerReportPath)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return res, err
		}
		return failure(&res, "font-subset.report-invalid", err.Error())
	}
	var reportedTargets struct {
		Fonts []struct {
			Target string `json:"target"`
		} `json:"fonts"`
	}
	if err := json.Unmarshal(providerReportBytes, &reportedTargets); err != nil {
		return failure(&res, "font-subset.report-invalid", fmt.Sprintf("decode provider report: %v", err))
	}
	manifestPaths := make(map[string]struct{}, len(manifestFontItems))
	for _, item := range manifestFontItems {
		manifestPaths[item.ArchivePath] = struct{}{}
	}
	reportedFonts := make(map[string]struct{}, len(reportedTargets.Fonts))
	for _, font := range reportedTargets.Fonts {
		if _, ok := manifestPaths[font.Target]; !ok {
			return failure(&res, "font-subset.report-invalid", fmt.Sprintf("provider report target %q is not a current manifest font", font.Target))
		}
		if _, duplicate := reportedFonts[font.Target]; duplicate {
			return failure(&res, "font-subset.report-invalid", fmt.Sprintf("provider report repeats font target %q", font.Target))
		}
		reportedFonts[font.Target] = struct{}{}
	}
	unhandledFonts := make([]string, 0)
	for _, item := range manifestFontItems {
		if _, ok := reportedFonts[item.ArchivePath]; !ok {
			unhandledFonts = append(unhandledFonts, item.ArchivePath)
		}
	}
	if len(unhandledFonts) > 0 {
		return failure(&res, "font-subset.unhandled-font", strings.Join(unhandledFonts, ", "))
	}

	candidate, err := book.OpenContext(ctx, candidatePath)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return res, err
		}
		return failure(&res, "font-subset.output-invalid", fmt.Sprintf("provider did not produce a valid EPUB: %v", err))
	}
	defer candidate.Close()
	if !slices.Equal(b.OriginalNames(), candidate.OriginalNames()) {
		return failure(&res, "font-subset.entry-set-changed", "provider changed EPUB entry names or order")
	}
	providerReport, findings, err := validateProviderReport(ctx, providerReportBytes, input, candidatePath, b, candidate, manifestFontItems)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return res, err
		}
		return failure(&res, "font-subset.report-invalid", err.Error())
	}

	edits := make([]editset.Edit, 0, len(fontPaths))
	changed := make([]string, 0, len(fontPaths))
	for _, fontPath := range fontPaths {
		original, err := b.CurrentContext(ctx, fontPath)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return res, err
			}
			return failure(&res, "font-subset.input-font-missing", fmt.Sprintf("%s: %v", fontPath, err))
		}
		subset, err := candidate.OriginalContext(ctx, fontPath)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return res, err
			}
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
		"provider":       "epub-font",
		"fontEntries":    fontPaths,
		"changedFonts":   changed,
		"providerLog":    strings.ReplaceAll(providerDetail(provider, nil), workspace, "<provider-workspace>"),
		"providerReport": providerReport,
	}
	res.Findings = append(res.Findings, findings...)
	return res, nil
}

func readProviderReport(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("provider report is missing or unreadable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("provider report is not a regular file")
	}
	if info.Size() <= 0 || info.Size() > maxProviderReportBytes {
		return nil, fmt.Errorf("provider report size %d is outside the 1..%d byte limit", info.Size(), maxProviderReportBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open provider report: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat provider report: %w", err)
	}
	if !openedInfo.Mode().IsRegular() || openedInfo.Size() <= 0 || openedInfo.Size() > maxProviderReportBytes {
		return nil, fmt.Errorf("provider report changed or exceeds the %d byte limit", maxProviderReportBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxProviderReportBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read provider report: %w", err)
	}
	if len(data) == 0 || len(data) > maxProviderReportBytes {
		return nil, fmt.Errorf("provider report exceeds the %d byte limit or is empty", maxProviderReportBytes)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("provider report is not valid UTF-8")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func validateProviderReport(ctx context.Context, data []byte, inputPath, outputPath string, source, candidate *book.Book,
	manifestFontItems []opfscan.ManifestItem) (providerReportSummary, []report.Finding, error) {
	var providerReport providerSubsetReport
	if err := json.Unmarshal(data, &providerReport); err != nil {
		return providerReportSummary{}, nil, fmt.Errorf("decode provider report: %w", err)
	}
	if providerReport.SchemaVersion != providerReportSchemaVersion {
		return providerReportSummary{}, nil, fmt.Errorf("unsupported provider report schemaVersion %d", providerReport.SchemaVersion)
	}
	if providerReport.Tool != "epub-font subset" || providerReport.ProviderVersion == "" || providerReport.FontTools == "" {
		return providerReportSummary{}, nil, fmt.Errorf("provider report is missing tool or version identity")
	}
	if len(providerReport.ProviderVersion) > 100 || len(providerReport.FontTools) > 100 {
		return providerReportSummary{}, nil, fmt.Errorf("provider report version field is too long")
	}
	if !providerReport.OK || providerReport.Output == nil {
		return providerReportSummary{}, nil, fmt.Errorf("provider report does not describe a successful output")
	}
	inputPathMatches := filepath.Clean(providerReport.Input.Path) == filepath.Clean(inputPath)
	outputPathMatches := filepath.Clean(providerReport.Output.Path) == filepath.Clean(outputPath)
	if !inputPathMatches || !outputPathMatches {
		return providerReportSummary{}, nil, fmt.Errorf("provider report paths do not match the input and candidate")
	}
	if !validSHA256(providerReport.Input.SHA256) || !validSHA256(providerReport.Output.SHA256) {
		return providerReportSummary{}, nil, fmt.Errorf("provider report contains an invalid EPUB SHA-256")
	}
	inputSHA, err := source.InputSHA256Context(ctx)
	if err != nil {
		return providerReportSummary{}, nil, fmt.Errorf("hash provider input: %w", err)
	}
	if providerReport.Input.SHA256 != inputSHA {
		return providerReportSummary{}, nil, fmt.Errorf("provider input SHA-256 does not match the open source EPUB")
	}
	outputSHA, err := candidate.InputSHA256Context(ctx)
	if err != nil {
		return providerReportSummary{}, nil, fmt.Errorf("hash provider candidate: %w", err)
	}
	if providerReport.Output.SHA256 != outputSHA {
		return providerReportSummary{}, nil, fmt.Errorf("provider output SHA-256 does not match the candidate EPUB")
	}
	if providerReport.Output.Warnings == nil {
		return providerReportSummary{}, nil, fmt.Errorf("provider report output.warnings must be an array")
	}
	if err := validateWarningList("output.warnings", providerReport.Output.Warnings); err != nil {
		return providerReportSummary{}, nil, err
	}
	if len(providerReport.Fonts) != len(manifestFontItems) {
		return providerReportSummary{}, nil, fmt.Errorf("provider report fonts count %d does not match manifest scope", len(providerReport.Fonts))
	}

	manifestFonts := make(map[string]opfscan.ManifestItem, len(manifestFontItems))
	fontPaths := make([]string, 0, len(manifestFontItems))
	for _, item := range manifestFontItems {
		manifestFonts[item.ArchivePath] = item
		fontPaths = append(fontPaths, item.ArchivePath)
	}
	seen := make(map[string]struct{}, len(providerReport.Fonts))
	summary := providerReportSummary{
		SchemaVersion:    providerReport.SchemaVersion,
		ProviderVersion:  providerReport.ProviderVersion,
		FontToolsVersion: providerReport.FontTools,
		InputSHA256:      providerReport.Input.SHA256,
		OutputSHA256:     providerReport.Output.SHA256,
		OK:               providerReport.OK,
		Fonts:            make([]providerFontSummary, 0, len(providerReport.Fonts)),
		Warnings:         cloneStrings(providerReport.Output.Warnings),
	}
	findings := make([]report.Finding, 0)
	for _, font := range providerReport.Fonts {
		if font.Target == "" || len(font.Target) > 2048 || font.ManifestID == "" || font.MediaType == "" {
			return providerReportSummary{}, nil, fmt.Errorf("provider report has an incomplete font target")
		}
		manifestItem, ok := manifestFonts[font.Target]
		if !ok {
			return providerReportSummary{}, nil, fmt.Errorf("provider report target %q is not a current manifest font", font.Target)
		}
		if font.ManifestID != manifestItem.ID || font.MediaType != manifestItem.MediaType {
			return providerReportSummary{}, nil, fmt.Errorf("provider report manifest identity does not match font target %q", font.Target)
		}
		if _, duplicate := seen[font.Target]; duplicate {
			return providerReportSummary{}, nil, fmt.Errorf("provider report repeats font target %q", font.Target)
		}
		seen[font.Target] = struct{}{}
		if !font.OK {
			return providerReportSummary{}, nil, fmt.Errorf("provider report marks font %q as failed", font.Target)
		}
		if font.Master.Source == "" || len(font.Master.Source) > 4096 || font.Master.Bytes < 0 || font.Master.Glyphs < 0 || font.Master.Outline == "" ||
			!validSHA256(font.Master.SHA256) {
			return providerReportSummary{}, nil, fmt.Errorf("provider report has invalid master facts for %q", font.Target)
		}
		if font.Output.Bytes < 0 || font.Output.Glyphs < 0 || font.Output.Outline == "" || !validSHA256(font.Output.SHA256) {
			return providerReportSummary{}, nil, fmt.Errorf("provider report has invalid output facts for %q", font.Target)
		}
		if err := validateWarningList("font.warnings", font.Warnings); err != nil {
			return providerReportSummary{}, nil, fmt.Errorf("%s: %w", font.Target, err)
		}
		checks, err := summarizeProviderChecks(font.Checks)
		if err != nil {
			return providerReportSummary{}, nil, fmt.Errorf("%s: %w", font.Target, err)
		}
		original, err := source.OriginalContext(ctx, font.Target)
		if err != nil {
			return providerReportSummary{}, nil, fmt.Errorf("read source font %s: %w", font.Target, err)
		}
		if !validSHA256(font.Original.SHA256) || font.Original.SHA256 != sha256Hex(original) || font.Original.Bytes != int64(len(original)) {
			return providerReportSummary{}, nil, fmt.Errorf("provider original SHA-256 or size does not match %q", font.Target)
		}
		outputFont, err := candidate.OriginalContext(ctx, font.Target)
		if err != nil {
			return providerReportSummary{}, nil, fmt.Errorf("read candidate font %s: %w", font.Target, err)
		}
		if font.Output.SHA256 != sha256Hex(outputFont) || font.Output.Bytes != int64(len(outputFont)) {
			return providerReportSummary{}, nil, fmt.Errorf("provider output SHA-256 or size does not match %q", font.Target)
		}
		if strings.HasPrefix(font.Master.Source, "epub:") && font.Master.SHA256 != font.Original.SHA256 {
			return providerReportSummary{}, nil, fmt.Errorf("embedded master SHA-256 does not match original font %q", font.Target)
		}
		switch font.Action {
		case "subset":
			if font.RequiredCodepoints == nil || *font.RequiredCodepoints < 0 || font.NotInMasterCount == nil || *font.NotInMasterCount < 0 || font.NotInMaster == nil {
				return providerReportSummary{}, nil, fmt.Errorf("provider report has incomplete subset counts for %q", font.Target)
			}
			if len(font.NotInMaster) > 200 || len(font.NotInMaster) > *font.NotInMasterCount {
				return providerReportSummary{}, nil, fmt.Errorf("provider report has inconsistent notInMaster details for %q", font.Target)
			}
			for _, missing := range font.NotInMaster {
				if missing == "" || len(missing) > 128 {
					return providerReportSummary{}, nil, fmt.Errorf("provider report has invalid notInMaster item for %q", font.Target)
				}
			}
		case "preserve":
			if font.Reason != "math-table" || font.RequiredCodepoints != nil || font.NotInMaster != nil || font.NotInMasterCount != nil {
				return providerReportSummary{}, nil, fmt.Errorf("provider report has invalid preserve facts for %q", font.Target)
			}
			if font.Original.SHA256 != font.Output.SHA256 || !bytes.Equal(original, outputFont) {
				return providerReportSummary{}, nil, fmt.Errorf("provider did not preserve MATH font bytes for %q", font.Target)
			}
		default:
			return providerReportSummary{}, nil, fmt.Errorf("provider report has unsupported action %q for %q", font.Action, font.Target)
		}
		fontSummary := providerFontSummary{
			Target: font.Target, Action: font.Action, Reason: font.Reason,
			MasterSHA256: font.Master.SHA256, OriginalSHA256: font.Original.SHA256, OutputSHA256: font.Output.SHA256,
			MasterBytes: font.Master.Bytes, OriginalBytes: font.Original.Bytes, OutputBytes: font.Output.Bytes,
			MasterGlyphs: font.Master.Glyphs, OutputGlyphs: font.Output.Glyphs,
			RequiredCodepoints: font.RequiredCodepoints, NotInMaster: cloneOptionalStrings(font.NotInMaster),
			NotInMasterCount: font.NotInMasterCount, Checks: checks, Warnings: cloneStrings(font.Warnings),
		}
		summary.Fonts = append(summary.Fonts, fontSummary)
		if font.NotInMasterCount != nil && *font.NotInMasterCount > 0 {
			detail := fmt.Sprintf("%d required characters are absent from the master font; fallback fonts must cover them", *font.NotInMasterCount)
			if len(font.NotInMaster) > 0 {
				examples := font.NotInMaster
				if len(examples) > 10 {
					examples = examples[:10]
				}
				detail += "; examples: " + strings.Join(examples, ", ")
			}
			findings = append(findings, report.Finding{
				Level: "warn", ID: "font-subset.not-in-master", Title: "Font master lacks required characters",
				Detail: detail, Location: font.Target,
			})
		}
		for _, warning := range font.Warnings {
			if font.NotInMasterCount != nil && *font.NotInMasterCount > 0 && strings.Contains(warning, "required characters are not in the master font") {
				continue
			}
			findings = append(findings, report.Finding{
				Level: "warn", ID: "font-subset.provider-warning", Title: "Font subset provider reported a warning",
				Detail: warning, Location: font.Target,
			})
		}
	}

	for _, fontPath := range fontPaths {
		original, err := source.OriginalContext(ctx, fontPath)
		if err != nil {
			return providerReportSummary{}, nil, fmt.Errorf("read source font %s: %w", fontPath, err)
		}
		outputFont, err := candidate.OriginalContext(ctx, fontPath)
		if err != nil {
			return providerReportSummary{}, nil, fmt.Errorf("read candidate font %s: %w", fontPath, err)
		}
		if !bytes.Equal(original, outputFont) {
			if _, reported := seen[fontPath]; !reported {
				return providerReportSummary{}, nil, fmt.Errorf("provider changed unreported font target %q", fontPath)
			}
		}
	}
	for _, warning := range providerReport.Output.Warnings {
		findings = append(findings, report.Finding{
			Level: "warn", ID: "font-subset.provider-output-warning", Title: "Font subset provider reported an EPUB warning",
			Detail: warning, Location: "EPUB output",
		})
	}
	return summary, findings, nil
}

func summarizeProviderChecks(raw map[string]json.RawMessage) (map[string]map[string]any, error) {
	if len(raw) == 0 || len(raw) > 32 {
		return nil, fmt.Errorf("provider report checks must contain 1..32 entries")
	}
	checks := make(map[string]map[string]any, len(raw))
	for name, encoded := range raw {
		if name == "" || len(name) > 100 {
			return nil, fmt.Errorf("provider report has an invalid check name")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil || fields == nil {
			return nil, fmt.Errorf("provider report check %q is not an object", name)
		}
		passedRaw, ok := fields["ok"]
		if !ok {
			return nil, fmt.Errorf("provider report check %q is missing ok", name)
		}
		var passed bool
		if err := json.Unmarshal(passedRaw, &passed); err != nil || !passed {
			return nil, fmt.Errorf("provider report check %q did not pass", name)
		}
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.UseNumber()
		var details map[string]any
		if err := decoder.Decode(&details); err != nil || details == nil {
			return nil, fmt.Errorf("provider report check %q cannot be retained", name)
		}
		checks[name] = details
	}
	return checks, nil
}

func validateWarningList(name string, warnings []string) error {
	if warnings == nil || len(warnings) > 64 {
		return fmt.Errorf("provider report %s must be an array of at most 64 warnings", name)
	}
	for _, warning := range warnings {
		if strings.TrimSpace(warning) == "" || len(warning) > 4096 {
			return fmt.Errorf("provider report %s contains an empty or oversized warning", name)
		}
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func cloneStrings(values []string) []string {
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneOptionalStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return cloneStrings(values)
}

func manifestFonts(ctx context.Context, b *book.Book) ([]opfscan.ManifestItem, error) {
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
	fonts := make([]opfscan.ManifestItem, 0)
	seen := make(map[string]struct{})
	for _, item := range pkg.Manifest {
		if item.ArchivePath == "" || !isFont(item.ArchivePath, item.MediaType) {
			continue
		}
		if _, ok := seen[item.ArchivePath]; ok {
			continue
		}
		seen[item.ArchivePath] = struct{}{}
		fonts = append(fonts, item)
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
	return *res, nil
}
