package pipeline

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

const cleanCapabilityID = "epub.clean"

type CleanOptions struct {
	RepoRoot  string
	InputPath string
	OutputDir string
	Steps     []string
	Approve   bool
	Jobs      int
}

type CleanBookResult struct {
	InputPath  string
	ReportPath string
	OutputPath string
	Envelope   report.Envelope
	ExitCode   int
	Err        error
}

type CleanBatchResult struct {
	Books    []CleanBookResult
	ExitCode int
}

type cleanInput struct {
	path     string
	relative string
}

type cleanStepDefinition struct {
	name       string
	capability string
	args       Args
}

type cleanStepSummary struct {
	Name         string               `json:"name"`
	Capability   string               `json:"capability"`
	Status       string               `json:"status"`
	InputSHA256  string               `json:"inputSHA256"`
	OutputSHA256 string               `json:"outputSHA256,omitempty"`
	Redline      *cleanRedlineSummary `json:"redline,omitempty"`
	Facts        map[string]any       `json:"facts,omitempty"`
	Findings     []report.Finding     `json:"findings"`
	Events       []report.Event       `json:"events"`
}

type cleanRedlineSummary struct {
	Status   string           `json:"status"`
	Findings []report.Finding `json:"findings"`
	Facts    map[string]any   `json:"facts,omitempty"`
	Events   []report.Event   `json:"events"`
}

// Clean processes one EPUB or every regular .epub file in a directory. Step
// outputs are chained in memory; without approval only per-book JSON reports
// are published, and approved runs write one final candidate per book.
func Clean(ctx context.Context, opts CleanOptions) (CleanBatchResult, error) {
	if strings.TrimSpace(opts.InputPath) == "" {
		return CleanBatchResult{}, &UsageError{Err: errors.New("epub clean requires an input EPUB or directory")}
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return CleanBatchResult{}, &UsageError{Err: errors.New("epub clean requires --out DIR")}
	}
	if opts.Jobs < 0 {
		return CleanBatchResult{}, &UsageError{Err: errors.New("--jobs must be a positive integer")}
	}
	if opts.Jobs == 0 {
		opts.Jobs = 1
	}
	steps, err := normalizeCleanSteps(opts.Steps)
	if err != nil {
		return CleanBatchResult{}, &UsageError{Err: err}
	}
	if err := ctx.Err(); err != nil {
		return CleanBatchResult{}, err
	}

	inputPath, err := filepath.Abs(opts.InputPath)
	if err != nil {
		return CleanBatchResult{}, fmt.Errorf("resolve input path: %w", err)
	}
	inputLexicalPath := filepath.Clean(inputPath)
	inputPath, err = filepath.EvalSymlinks(inputPath)
	if err != nil {
		return CleanBatchResult{}, &UsageError{Err: fmt.Errorf("input not found: %s", opts.InputPath)}
	}
	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		return CleanBatchResult{}, &UsageError{Err: fmt.Errorf("input not found: %s", opts.InputPath)}
	}
	inputIsDir := inputInfo.IsDir()
	if !inputIsDir && (!inputInfo.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(inputPath), ".epub")) {
		return CleanBatchResult{}, &UsageError{Err: fmt.Errorf("input must be an .epub file or directory: %s", opts.InputPath)}
	}
	outputDir, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return CleanBatchResult{}, fmt.Errorf("resolve output directory: %w", err)
	}
	outputLexicalPath := filepath.Clean(outputDir)
	resolvedOutputDir, err := resolveProspectivePath(outputDir)
	if err != nil {
		return CleanBatchResult{}, fmt.Errorf("resolve output directory: %w", err)
	}
	if inputIsDir {
		if pathIsWithin(inputLexicalPath, outputLexicalPath) || pathIsWithin(inputPath, resolvedOutputDir) {
			return CleanBatchResult{}, &UsageError{Err: errors.New("--out must be outside the input directory")}
		}
	}

	inputs, err := discoverCleanInputs(ctx, inputPath, inputIsDir)
	if err != nil {
		return CleanBatchResult{}, err
	}
	if len(inputs) == 0 {
		return CleanBatchResult{}, &UsageError{Err: errors.New("input contains no .epub files")}
	}
	if opts.RepoRoot == "" {
		opts.RepoRoot, err = FindRepoRoot()
		if err != nil {
			return CleanBatchResult{}, err
		}
	}
	if err := preflightCleanOutputs(inputs, inputPath, inputIsDir, outputDir, opts.Approve); err != nil {
		return CleanBatchResult{}, err
	}

	jobs := opts.Jobs
	if jobs > len(inputs) {
		jobs = len(inputs)
	}
	work := make(chan cleanInput, len(inputs))
	results := make(chan CleanBookResult, len(inputs))
	for _, input := range inputs {
		work <- input
	}
	close(work)
	var workers sync.WaitGroup
	for range jobs {
		workers.Go(func() {
			for input := range work {
				results <- cleanOneBook(ctx, opts, input, inputIsDir, outputDir, steps)
			}
		})
	}
	workers.Wait()
	close(results)

	bookResults := make([]CleanBookResult, 0, len(inputs))
	for result := range results {
		bookResults = append(bookResults, result)
	}
	slices.SortFunc(bookResults, func(a, b CleanBookResult) int { return cmp.Compare(a.InputPath, b.InputPath) })
	batch := CleanBatchResult{Books: bookResults, ExitCode: ExitOK}
	for index := range batch.Books {
		bookResult := &batch.Books[index]
		if bookResult.Envelope.Status == "" {
			continue
		}
		data, marshalErr := MarshalEnvelope(bookResult.Envelope)
		if marshalErr == nil {
			marshalErr = zipfs.WriteNewFileContext(context.WithoutCancel(ctx), bookResult.ReportPath, data, 0o644)
		}
		if marshalErr != nil {
			bookResult.Err = fmt.Errorf("write summary report %s: %w", bookResult.ReportPath, marshalErr)
			bookResult.ExitCode = ExitFailed
			bookResult.Envelope.Status = report.StatusFailed
			bookResult.Envelope.Findings = append(bookResult.Envelope.Findings, report.Finding{
				Level: "error", ID: "clean.report-write-failed", Title: "Failed to write clean summary report",
				Detail: bookResult.Err.Error(), Location: bookResult.ReportPath,
			})
		}
		if bookResult.ExitCode != ExitOK {
			batch.ExitCode = ExitFailed
		}
	}
	return batch, nil
}

func pathIsWithin(base, candidate string) bool {
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// resolveProspectivePath resolves symlinks in existing parents while retaining
// any not-yet-created suffix. This makes input/output overlap checks reliable
// when temporary directories use aliases such as /var versus /private/var.
func resolveProspectivePath(path string) (string, error) {
	path = filepath.Clean(path)
	missing := []string{}
	for {
		if _, err := os.Lstat(path); err == nil {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return "", err
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("no existing parent for %s", path)
		}
		missing = append(missing, filepath.Base(path))
		path = parent
	}
}

func normalizeCleanSteps(requested []string) ([]cleanStepDefinition, error) {
	all := cleanStepDefinitions()
	if requested == nil {
		return all, nil
	}
	if len(requested) == 0 {
		return nil, errors.New("--steps must name at least one step")
	}
	byName := make(map[string]int, len(all))
	for index, step := range all {
		byName[step.name] = index
	}
	selected := make([]cleanStepDefinition, 0, len(requested))
	previous := -1
	for _, name := range requested {
		index, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown clean step %q (choose normalize,migrate,css,typography)", name)
		}
		if index <= previous {
			return nil, errors.New("--steps must be unique and in this order: normalize,migrate,css,typography")
		}
		selected = append(selected, all[index])
		previous = index
	}
	return selected, nil
}

func cleanStepDefinitions() []cleanStepDefinition {
	return []cleanStepDefinition{
		{name: "normalize", capability: "epub.structure.normalize", args: Args{"mode": "normalize"}},
		{name: "migrate", capability: "epub.package.migrate.epub3", args: Args{}},
		{name: "css", capability: "epub.css.layering.optimize", args: Args{}},
		{name: "typography", capability: "epub.typography.optimize", args: Args{"preset": "literary-cn"}},
	}
}

func discoverCleanInputs(ctx context.Context, inputPath string, inputIsDir bool) ([]cleanInput, error) {
	if !inputIsDir {
		return []cleanInput{{path: inputPath, relative: filepath.Base(inputPath)}}, nil
	}
	inputs := []cleanInput{}
	err := filepath.WalkDir(inputPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".epub") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(inputPath, path)
		if err != nil {
			return err
		}
		inputs = append(inputs, cleanInput{path: path, relative: relative})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan input directory: %w", err)
	}
	slices.SortFunc(inputs, func(a, b cleanInput) int { return cmp.Compare(a.path, b.path) })
	return inputs, nil
}

func preflightCleanOutputs(inputs []cleanInput, inputPath string, inputIsDir bool, outputDir string, approve bool) error {
	if info, err := os.Stat(outputDir); err == nil && !info.IsDir() {
		return &UsageError{Err: fmt.Errorf("--out is not a directory: %s", outputDir)}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output directory: %w", err)
	}
	planned := map[string]string{}
	for _, input := range inputs {
		outputPath, reportPath := cleanOutputPaths(input, inputIsDir, outputDir)
		for path, label := range map[string]string{reportPath: "summary report"} {
			if previous, exists := planned[path]; exists {
				return &UsageError{Err: fmt.Errorf("output collision: %s and %s both map to %s", previous, label, path)}
			}
			planned[path] = label
		}
		if approve {
			for _, path := range []string{outputPath} {
				if previous, exists := planned[path]; exists {
					return &UsageError{Err: fmt.Errorf("output collision: %s and %s both map to %s", previous, "approved EPUB output", path)}
				}
				planned[path] = "approved EPUB output"
			}
		}
	}
	for path := range planned {
		if _, err := os.Lstat(path); err == nil {
			return &UsageError{Err: fmt.Errorf("output already exists: %s", path)}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect output path %s: %w", path, err)
		}
	}
	return nil
}

func cleanOutputPaths(input cleanInput, inputIsDir bool, outputDir string) (outputPath, reportPath string) {
	relative := input.relative
	if !inputIsDir {
		relative = filepath.Base(input.path)
	}
	parent := filepath.Dir(relative)
	if parent == "." {
		parent = ""
	}
	stem := strings.TrimSuffix(filepath.Base(relative), filepath.Ext(relative))
	outputPath = filepath.Join(outputDir, relative)
	reportPath = filepath.Join(outputDir, parent, stem+".clean.json")
	return outputPath, reportPath
}

func cleanOneBook(ctx context.Context, opts CleanOptions, input cleanInput, inputIsDir bool, outputDir string, steps []cleanStepDefinition) CleanBookResult {
	outputPath, reportPath := cleanOutputPaths(input, inputIsDir, outputDir)
	result := CleanBookResult{InputPath: input.path, ReportPath: reportPath, ExitCode: ExitFailed}
	env := report.Envelope{SchemaVersion: "2", Capability: cleanCapabilityID, Status: report.StatusFailed}
	inputSHA, err := book.FileSHA256Context(ctx, input.path)
	if err != nil {
		return cleanBookFailure(result, env, "clean.input-read-failed", "Unable to read input EPUB", err)
	}
	env.Input = &report.Artifact{Path: input.path, SHA256: inputSHA}

	stepSummaries := []cleanStepSummary{}
	allEvents := []report.Event{}
	allFindings := []report.Finding{}
	var normalizeEnvelope []byte
	var currentBytes []byte
	currentSHA := inputSHA
	var failure error
	cancelled := false
	redlineAttempted := false

	audit, auditErr := Run(ctx, Options{RepoRoot: opts.RepoRoot, CapabilityID: "epub.package.nav.audit", InputPath: input.path, DryRun: true})
	if auditErr != nil {
		failure = auditErr
		stepSummaries = append(stepSummaries, cleanStepSummaryFrom("audit", "epub.package.nav.audit", currentSHA, "", "failed", report.Envelope{}, auditErr))
	} else {
		cancelled = audit.Envelope.Status == report.StatusCancelled
		stepSummaries = append(stepSummaries, cleanStepSummaryFrom("audit", "epub.package.nav.audit", currentSHA, "", audit.Envelope.Status, audit.Envelope, nil))
		allEvents = append(allEvents, audit.Envelope.Events...)
		allFindings = appendCleanFindings(allFindings, "audit", audit.Envelope.Findings)
		if audit.ExitCode != ExitOK {
			failure = fmt.Errorf("nav audit exited with code %d", audit.ExitCode)
		}
	}

	for _, step := range steps {
		if failure != nil {
			break
		}
		if err := ctx.Err(); err != nil {
			failure = err
			break
		}
		runOptions := Options{
			RepoRoot: opts.RepoRoot, CapabilityID: step.capability,
			InputPath: input.path, Args: step.args, CaptureOutput: true,
		}
		if currentBytes != nil {
			runOptions.InputBytes = currentBytes
		}
		outcome, runErr := Run(ctx, runOptions)
		candidateBytes := outcome.OutputBytes
		candidateSHA := cleanBytesSHA256(candidateBytes)
		summary := cleanStepSummaryFrom(step.name, step.capability, currentSHA, candidateSHA, outcome.Envelope.Status, outcome.Envelope, runErr)
		cancelled = cancelled || outcome.Envelope.Status == report.StatusCancelled
		stepSummaries = append(stepSummaries, summary)
		allEvents = append(allEvents, outcome.Envelope.Events...)
		allFindings = appendCleanFindings(allFindings, step.name, outcome.Envelope.Findings)
		if step.name == "normalize" && candidateBytes != nil {
			normalizeEnvelope, err = MarshalEnvelope(outcome.Envelope)
			if err != nil {
				failure = fmt.Errorf("serialize normalize path map: %w", err)
			}
		}
		if candidateBytes != nil {
			currentBytes = candidateBytes
			currentSHA = candidateSHA
		}
		if runErr != nil {
			failure = errors.Join(failure, runErr)
		} else if outcome.ExitCode != ExitOK {
			failure = errors.Join(failure, fmt.Errorf("%s exited with code %d", step.name, outcome.ExitCode))
		}
		if failure != nil {
			break
		}
	}

	var redlineSummary cleanRedlineSummary
	if currentBytes != nil {
		redlineAttempted = true
		var redlineErr error
		redlineSummary, redlineErr = compareCleanRedline(ctx, input.path, currentBytes, inputSHA, currentSHA, normalizeEnvelope)
		allEvents = append(allEvents, redlineSummary.Events...)
		allFindings = appendCleanFindings(allFindings, "redline", redlineSummary.Findings)
		if redlineErr != nil {
			failure = errors.Join(failure, redlineErr)
		} else if redlineSummary.Status != report.StatusComplete {
			failure = errors.Join(failure, errors.New("final all-item redline failed"))
		}
	} else {
		redlineSummary = cleanRedlineSummary{Status: "skipped", Findings: []report.Finding{}, Events: []report.Event{{Step: "redline", Status: "skipped", Message: "no in-memory candidate was produced"}}}
		allEvents = append(allEvents, redlineSummary.Events...)
	}

	if opts.Approve && redlineAttempted && currentBytes != nil && ctx.Err() == nil {
		if writeErr := zipfs.WriteNewFileContext(ctx, outputPath, currentBytes, 0o644); writeErr != nil {
			failure = errors.Join(failure, fmt.Errorf("write approved candidate: %w", writeErr))
		} else {
			result.OutputPath = outputPath
			env.Output = &report.Artifact{Path: outputPath, SHA256: currentSHA}
		}
	}

	status := report.StatusPlanned
	if opts.Approve {
		status = report.StatusComplete
	}
	if failure != nil {
		status = report.StatusFailed
		if cancelled || errors.Is(failure, context.Canceled) || errors.Is(failure, context.DeadlineExceeded) || ctx.Err() != nil {
			status = report.StatusCancelled
			allEvents = append(allEvents, report.Event{Step: "clean", Status: "cancelled", Message: failure.Error()})
		}
		findingID := "clean.book-failed"
		title := "EPUB clean did not finish successfully"
		if status == report.StatusCancelled {
			findingID = "clean.book-cancelled"
			title = "EPUB clean was cancelled"
		}
		env.Findings = append(env.Findings, report.Finding{
			Level: "error", ID: findingID, Title: title,
			Detail: failure.Error(), Location: input.path,
		})
		result.Err = failure
		result.ExitCode = ExitFailed
	} else {
		result.ExitCode = ExitOK
	}
	env.Status = status
	facts := map[string]any{
		"epub.clean.approved": opts.Approve,
		"epub.clean.steps":    stepSummaries,
		"epub.clean.redline":  redlineSummary,
	}
	for _, step := range stepSummaries {
		if step.Name == "normalize" {
			if mappings, ok := step.Facts["epub.structure.normalize.mappings"]; ok {
				facts["epub.clean.normalize.mappings"] = mappings
			}
			break
		}
	}
	if !opts.Approve {
		facts["epub.clean.previewSHA256"] = currentSHA
	}
	env.Facts = facts
	env.Events = allEvents
	env.Findings = append(env.Findings, allFindings...)
	result.Envelope = env
	return result
}

func cleanBytesSHA256(data []byte) string {
	if data == nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func compareCleanRedline(ctx context.Context, inputPath string, candidate []byte, inputSHA, candidateSHA string, normalizeEnvelope []byte) (cleanRedlineSummary, error) {
	before, err := book.OpenContext(ctx, inputPath)
	if err != nil {
		return cleanRedlineSummary{}, err
	}
	defer before.Close()
	after, err := book.OpenBytesContext(ctx, inputPath, candidate)
	if err != nil {
		return cleanRedlineSummary{}, err
	}
	defer after.Close()

	pathMap := map[string]string{}
	if len(normalizeEnvelope) > 0 {
		pathMap, err = redline.LoadPathMap(normalizeEnvelope)
		if err != nil {
			return cleanRedlineSummary{}, err
		}
	}
	findings, err := redline.Check(redline.OriginalState(before), redline.CurrentState(after), nil, redline.Options{PathMap: pathMap})
	result := cleanRedlineSummary{
		Status: report.StatusComplete,
		Facts: map[string]any{
			"epub.redline.check":          "all",
			"epub.redline.before":         inputPath,
			"epub.redline.after":          "in-memory final candidate",
			"epub.redline.beforeSHA256":   inputSHA,
			"epub.redline.afterSHA256":    candidateSHA,
			"epub.redline.findingCount":   len(findings),
			"epub.redline.pathMapEntries": len(pathMap),
		},
		Findings: []report.Finding{},
		Events:   []report.Event{},
	}
	if err != nil {
		result.Status = report.StatusFailed
		result.Findings = append(result.Findings, report.Finding{Level: "error", ID: "redline.check-failed", Title: "Redline validation failed", Detail: err.Error(), Location: "redline"})
		result.Events = append(result.Events, report.Event{Step: "redline", Status: "failed", Message: err.Error()})
		return result, err
	}
	for index, finding := range findings {
		result.Findings = append(result.Findings, report.Finding{
			Level: "error", ID: fmt.Sprintf("redline.%s.%d", finding.Check, index),
			Title: finding.Message, Detail: finding.Check,
		})
	}
	if len(findings) > 0 {
		result.Status = report.StatusFailed
		result.Events = append(result.Events, report.Event{Step: "redline", Status: "failed", Message: fmt.Sprintf("%d findings", len(findings))})
		return result, nil
	}
	result.Findings = append(result.Findings, report.Finding{Level: "info", ID: "redline.pass", Title: "All requested red-line checks passed."})
	result.Events = append(result.Events, report.Event{Step: "redline", Status: "completed", Message: "0 findings"})
	return result, nil
}

func cleanStepSummaryFrom(name, capability, inputSHA, outputSHA, status string, env report.Envelope, err error) cleanStepSummary {
	findings := nonNilCleanFindings(env.Findings)
	if err != nil {
		findings = append(findings, report.Finding{Level: "error", ID: "clean.step-run-failed", Title: "Clean stage failed", Detail: err.Error(), Location: name})
	}
	summary := cleanStepSummary{
		Name: name, Capability: capability, Status: status, InputSHA256: inputSHA, OutputSHA256: outputSHA,
		Facts: env.Facts, Findings: findings, Events: nonNilCleanEvents(env.Events),
	}
	if capability != "epub.package.nav.audit" {
		summary.Redline = cleanStepRedline(env)
	}
	return summary
}

func cleanStepRedline(env report.Envelope) *cleanRedlineSummary {
	redline := &cleanRedlineSummary{Status: "skipped", Findings: []report.Finding{}, Events: []report.Event{}}
	for _, event := range env.Events {
		if event.Step != "redline" {
			continue
		}
		redline.Events = append(redline.Events, event)
		switch event.Status {
		case "completed":
			redline.Status = report.StatusComplete
		case "failed":
			redline.Status = report.StatusFailed
		default:
			redline.Status = event.Status
		}
	}
	for _, finding := range env.Findings {
		if strings.HasPrefix(finding.ID, "redline.") {
			redline.Findings = append(redline.Findings, finding)
		}
	}
	return redline
}

func cleanBookFailure(result CleanBookResult, env report.Envelope, id, title string, err error) CleanBookResult {
	env.Status = report.StatusFailed
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		env.Status = report.StatusCancelled
		env.Events = append(env.Events, report.Event{Step: "clean", Status: "cancelled", Message: err.Error()})
	}
	env.Findings = []report.Finding{{Level: "error", ID: id, Title: title, Detail: err.Error(), Location: result.InputPath}}
	result.Envelope = env
	result.Err = err
	result.ExitCode = ExitFailed
	return result
}

func appendCleanFindings(dst []report.Finding, step string, findings []report.Finding) []report.Finding {
	for _, finding := range findings {
		finding.ID = "clean." + step + "." + finding.ID
		if finding.Location != "" {
			finding.Location = step + ":" + finding.Location
		}
		dst = append(dst, finding)
	}
	return dst
}

func nonNilCleanFindings(findings []report.Finding) []report.Finding {
	if findings == nil {
		return []report.Finding{}
	}
	return findings
}

func nonNilCleanEvents(events []report.Event) []report.Event {
	if events == nil {
		return []report.Event{}
	}
	return events
}
