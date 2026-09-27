package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/pipeline"
	"github.com/liyafly/epub-handbook/internal/report"
)

func TestRunCapabilityUsageErrorsHonorJSON(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{name: "missing capability", argv: []string{"--json"}, want: "缺少 capability-id"},
		{name: "malformed key value", argv: []string{"epub.package.nav.audit", "--json", "malformed"}, want: "KEY=VALUE"},
		{name: "json after key value", argv: []string{"epub.package.nav.audit", "mode=x", "--json"}, want: "before KEY=VALUE"},
		{name: "numeric json boolean", argv: []string{"--json=1"}, want: "缺少 capability-id"},
		{name: "flag parse error", argv: []string{"epub.package.nav.audit", "--json", "--unknown"}, want: "flag provided"},
		{name: "duplicate input flag", argv: []string{"epub.package.nav.audit", "--json", "--input", "a.epub", "--input=b.epub"}, want: "重复全局 flag: --input"},
		{name: "duplicate parameter", argv: []string{"epub.typography.optimize", "--json", "preset=one", "preset=two"}, want: "重复参数"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, _ := captureRunCapability(t, tc.argv)
			if code != 3 {
				t.Fatalf("exit = %d, want 3", code)
			}
			var env struct {
				SchemaVersion string `json:"schemaVersion"`
				Status        string `json:"status"`
				Findings      []struct {
					Detail string `json:"detail"`
				} `json:"findings"`
			}
			if err := json.Unmarshal([]byte(stdout), &env); err != nil {
				t.Fatalf("stdout is not an envelope: %v\n%s", err, stdout)
			}
			if env.SchemaVersion != "2" || env.Status != "failed" || len(env.Findings) != 1 {
				t.Fatalf("unexpected envelope: %#v", env)
			}
			if !strings.Contains(env.Findings[0].Detail, tc.want) {
				t.Fatalf("detail = %q, want substring %q", env.Findings[0].Detail, tc.want)
			}
		})
	}
}

func TestMarshalEnvelopeDoesNotEscapeHTML(t *testing.T) {
	data, err := marshalEnvelope(map[string]string{"value": "<>&"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `\u003c`) || strings.Contains(string(data), `\u003e`) || strings.Contains(string(data), `\u0026`) {
		t.Fatalf("HTML characters were escaped: %s", data)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("JSON envelope missing trailing newline: %q", data)
	}
}

func TestRunRedlineJSONPassAndFail(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	passAfter := filepath.Join(dir, "pass-after.epub")
	failAfter := filepath.Join(dir, "fail-after.epub")
	writeRedlineFixture(t, before, "same text")
	writeRedlineFixture(t, passAfter, "same text")
	writeRedlineFixture(t, failAfter, "changed text")

	for _, tc := range []struct {
		name       string
		after      string
		wantCode   int
		wantStatus string
		wantLevel  string
	}{
		{name: "pass", after: passAfter, wantCode: 0, wantStatus: "complete", wantLevel: "info"},
		{name: "fail", after: failAfter, wantCode: 1, wantStatus: "failed", wantLevel: "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := captureRunFunc(t, func() int {
				return runRedline([]string{"--check", "all", "--json", before, tc.after})
			})
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d; stdout=%s stderr=%s", code, tc.wantCode, stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("JSON mode wrote legacy text to stderr: %s", stderr)
			}
			var env struct {
				SchemaVersion string         `json:"schemaVersion"`
				Capability    string         `json:"capability"`
				Status        string         `json:"status"`
				Facts         map[string]any `json:"facts"`
				Findings      []struct {
					Level string `json:"level"`
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"findings"`
			}
			if err := jsonv2.Unmarshal([]byte(stdout), &env); err != nil {
				t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout)
			}
			if env.SchemaVersion != "2" || env.Capability != "epub.redline" || env.Status != tc.wantStatus {
				t.Fatalf("envelope header = %#v", env)
			}
			if len(env.Facts) == 0 || len(env.Findings) == 0 || env.Findings[0].Level != tc.wantLevel {
				t.Fatalf("envelope facts/findings = %#v / %#v", env.Facts, env.Findings)
			}
		})
	}
}

func TestCapabilitiesUnknownIDIsUsage(t *testing.T) {
	if code := runCapabilities([]string{"--id", "no.such.capability"}); code != 3 {
		t.Fatalf("unknown capability exit = %d, want 3", code)
	}
}

func TestCapabilitiesTextShowsContractDetailsInStableOrder(t *testing.T) {
	code, stdout, stderr := captureRunFunc(t, func() int {
		return runCapabilities([]string{"--id", "epub.typography.optimize"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"应用样式预设", "input:      epub", "output:     single", "preset (string) default=\"literary-cn\"", "scope_paths (json-string-array)"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("capability detail output missing %q:\n%s", want, stdout)
		}
	}
	if strings.Index(stdout, "  allow_font_obfuscation") > strings.Index(stdout, "  preset (string)") || strings.Index(stdout, "  preset (string)") > strings.Index(stdout, "  scope_paths (") {
		t.Fatalf("parameter order is not stable: %s", stdout)
	}
}

func TestStyleCatalogTextShowsMatchesAndNoMatchGuidance(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  string
	}{
		{query: "chapter", want: "scenes:"},
		{query: "no-such-scene-should-match", want: "0 scenes matched; try a shorter query"},
	} {
		code, stdout, stderr := captureRunCapability(t, []string{"epub.style.demo.maintain", "catalog=true", "query=" + tc.query})
		if code != 0 || stderr != "" || !strings.Contains(stdout, tc.want) {
			t.Errorf("query %q: exit=%d stderr=%q stdout=%q", tc.query, code, stderr, stdout)
		}
	}
}

func TestTypographySummaryExplainsCoverageBasisAndScope(t *testing.T) {
	cases := []struct {
		name  string
		facts map[string]any
		want  []string
	}{
		{
			name: "class tokens present",
			facts: map[string]any{
				"epub.typography.optimize.applicationMode": "scoped-additive",
				"epub.typography.optimize.coverageBasis":   "class-token",
				"epub.typography.optimize.scopeFileCount":  1,
				"epub.typography.optimize.coverage": map[string]any{
					"ratio": 0.5, "usedClasses": []string{"known", "unknown"},
					"uncoveredClasses": []string{"unknown"}, "insufficientToDetermineApplicability": false,
				},
			},
			want: []string{"scope:      scoped-additive (1 XHTML file)", "class-token coverage: 50%", "uncovered: unknown", "does not measure CSS cascade"},
		},
		{
			name: "tag-only page",
			facts: map[string]any{
				"epub.typography.optimize.applicationMode": "whole-book",
				"epub.typography.optimize.coverageBasis":   "class-token",
				"epub.typography.optimize.scopeFileCount":  1,
				"epub.typography.optimize.coverage": map[string]any{
					"ratio": 0.0, "usedClasses": []string{}, "uncoveredClasses": []string{},
					"insufficientToDetermineApplicability": true,
					"warning":                              "no class tokens; inspect a representative page",
				},
			},
			want: []string{"scope:      whole-book (1 XHTML file)", "class-token coverage: insufficient class tokens to determine applicability", "warning: no class tokens"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stdout, stderr := captureRunFunc(t, func() int {
				printTypographySummary(tc.facts)
				return 0
			})
			if stderr != "" {
				t.Fatalf("stderr = %q", stderr)
			}
			for _, want := range tc.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("summary missing %q:\n%s", want, stdout)
				}
			}
		})
	}
}

func TestCleanBatchSummarySeparatesPlannedFromWrittenOutputs(t *testing.T) {
	result := pipeline.CleanBatchResult{Books: []pipeline.CleanBookResult{
		{Envelope: report.Envelope{Status: report.StatusPlanned}},
		{Envelope: report.Envelope{Status: report.StatusComplete}, OutputPath: "book.epub"},
		{Envelope: report.Envelope{Status: report.StatusFailed}},
		{Envelope: report.Envelope{Status: report.StatusCancelled}},
	}}
	got := formatCleanBatchSummary(result)
	for _, want := range []string{"planned=1", "complete=1", "failed=1", "cancelled=1", "outputs=1/4", "planned books have no written EPUB"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary missing %q: %s", want, got)
		}
	}
}

func TestRunVersionJSONReportsBuildAndPlatform(t *testing.T) {
	code, stdout, stderr := captureRunFunc(t, func() int {
		return run([]string{"version", "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var got struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		BuiltAt string `json:"builtAt"`
		Go      string `json:"go"`
		GOOS    string `json:"goos"`
		GOARCH  string `json:"goarch"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("version JSON is invalid: %v: %s", err, stdout)
	}
	if got.Version == "" || got.Commit == "" || got.BuiltAt == "" || got.Go == "" || got.GOOS == "" || got.GOARCH == "" {
		t.Fatalf("version JSON is missing build information: %+v", got)
	}
}

func TestRunVersionRejectsArguments(t *testing.T) {
	code, _, stderr := captureRunFunc(t, func() int {
		return run([]string{"version", "unexpected"})
	})
	if code != 3 || !strings.Contains(stderr, "unexpected positional arguments") {
		t.Fatalf("exit=%d stderr=%q, want usage error", code, stderr)
	}
}

func TestRunCleanAcceptsInputBeforeAndAfterFlags(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input.epub")
	writeRedlineFixture(t, input, "same text")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "input first", args: []string{input, "--out", filepath.Join(root, "out-first"), "--steps", "normalize"}},
		{name: "flags first", args: []string{"--out", filepath.Join(root, "out-flags"), "--steps=normalize", input}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := captureRunFunc(t, func() int { return runClean(tc.args) })
			if code != 0 || stderr != "" {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if !strings.Contains(stdout, "planned") || !strings.Contains(stdout, "report ") {
				t.Fatalf("stdout=%q, want planned status and summary path", stdout)
			}
		})
	}
}

func TestRunCleanRejectsMissingAndDuplicateFlags(t *testing.T) {
	for _, args := range [][]string{
		{}, {"some.epub"}, {"--out", "out"}, {"input.epub", "--out", "out", "--out", "again"},
		{"input.epub", "--out", "out", "--jobs", "0"},
	} {
		if code := runClean(args); code != 3 {
			t.Errorf("runClean(%q) exit=%d, want 3", args, code)
		}
	}
	if err := rejectDuplicateCleanFlags([]string{"--scope", "a.xhtml", "--scope", "b.xhtml"}); err != nil {
		t.Fatalf("repeatable --scope rejected: %v", err)
	}
}

func TestRunCleanJSONWritesOnlyBatchEnvelopeToStdout(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input.epub")
	writeRedlineFixture(t, input, "same text")
	code, stdout, stderr := captureRunFunc(t, func() int {
		return runClean([]string{input, "--out", filepath.Join(root, "out"), "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	var envelope struct {
		SchemaVersion string         `json:"schemaVersion"`
		Capability    string         `json:"capability"`
		Status        string         `json:"status"`
		Facts         map[string]any `json:"facts"`
	}
	if err := jsonv2.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not a batch envelope: %v\n%s", err, stdout)
	}
	if envelope.SchemaVersion != "2" || envelope.Capability != "epub.clean" || envelope.Status != "planned" {
		t.Fatalf("unexpected batch envelope: %#v", envelope)
	}
	if _, ok := envelope.Facts["epub.clean.books"]; !ok {
		t.Fatalf("batch book summaries missing: %#v", envelope.Facts)
	}
}

func TestRunCleanJSONUsageErrorReturnsEnvelope(t *testing.T) {
	code, stdout, stderr := captureRunFunc(t, func() int {
		return runClean([]string{"input.epub", "--out", "out", "--steps", "typography", "--preset", "literary-cn", "--json"})
	})
	if code != 3 || stderr == "" {
		t.Fatalf("exit=%d stderr=%q, want usage error", code, stderr)
	}
	var envelope struct {
		Capability string `json:"capability"`
		Status     string `json:"status"`
	}
	if err := jsonv2.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not a usage envelope: %v\n%s", err, stdout)
	}
	if envelope.Capability != "epub.clean" || envelope.Status != "failed" {
		t.Fatalf("usage envelope=%#v", envelope)
	}
}

func captureRunCapability(t *testing.T, argv []string) (code int, stdout, stderr string) {
	t.Helper()
	return captureRunFunc(t, func() int { return runCapability(argv) })
}

func captureRunFunc(t *testing.T, runFunc func() int) (code int, stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalOut, originalErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() {
		os.Stdout, os.Stderr = originalOut, originalErr
	}()

	code = runFunc()
	if err := outW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errW.Close(); err != nil {
		t.Fatal(err)
	}
	outBytes, err := io.ReadAll(outR)
	if err != nil {
		t.Fatal(err)
	}
	errBytes, err := io.ReadAll(errR)
	if err != nil {
		t.Fatal(err)
	}
	if err := outR.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errR.Close(); err != nil {
		t.Fatal(err)
	}
	return code, string(outBytes), string(errBytes)
}

func writeRedlineFixture(t *testing.T, path, paragraph string) {
	t.Helper()
	entries := []struct {
		name   string
		data   []byte
		method uint16
	}{
		{name: "mimetype", data: []byte("application/epub+zip"), method: zip.Store},
		{name: "META-INF/container.xml", data: []byte(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", data: []byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="book-id"><metadata><dc:identifier id="book-id">urn:uuid:test-redline</dc:identifier><dc:title>CLI fixture</dc:title><dc:language>en</dc:language></metadata><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="chapter"/></spine></package>`)},
		{name: "OEBPS/nav.xhtml", data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><nav><a href="Text/chapter.xhtml">Chapter</a></nav></body></html>`)},
		{name: "OEBPS/Text/chapter.xhtml", data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml" lang="en"><body><p id="p1">` + paragraph + `</p></body></html>`)},
	}
	var buffer bytes.Buffer
	w := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: entry.method}
		writer, err := w.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
