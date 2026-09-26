package pipeline

import (
	"cmp"
	"context"
	jsonv2 "encoding/json/v2"
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
	RepoRoot              string
	InputPath             string
	OutputDir             string
	Steps                 []string
	Preset                string
	Scope                 []string
	Approve               bool
	RetainReviewCandidate bool
	Jobs                  int
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
	Envelope report.Envelope
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
	Name           string               `json:"name"`
	Capability     string               `json:"capability"`
	Status         string               `json:"status"`
	InputState     string               `json:"inputState"`
	OutputState    string               `json:"outputState"`
	ChangedEntries []string             `json:"changedEntries"`
	Redline        *cleanRedlineSummary `json:"redline,omitempty"`
	Facts          map[string]any       `json:"facts,omitempty"`
	Findings       []report.Finding     `json:"findings"`
	Events         []report.Event       `json:"events"`
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
	steps, err := normalizeCleanSteps(opts.Steps, opts.Preset, opts.Scope)
	if err != nil {
		return CleanBatchResult{}, &UsageError{Err: err}
	}
	if opts.Approve && len(steps) == 0 {
		return CleanBatchResult{}, &UsageError{Err: errors.New("--approve requires at least one transform step")}
	}
	if opts.RetainReviewCandidate && !opts.Approve {
		return CleanBatchResult{}, &UsageError{Err: errors.New("--retain-review-candidate requires --approve")}
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
	if err := preflightCleanOutputs(inputs, inputPath, inputIsDir, outputDir, opts.Approve, opts.RetainReviewCandidate); err != nil {
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
	bookSummaries := make([]report.CleanBookSummary, 0, len(batch.Books))
	for _, bookResult := range batch.Books {
		disposition, _ := bookResult.Envelope.Facts["pipeline.artifactDisposition"].(string)
		bookSummaries = append(bookSummaries, report.CleanBookSummary{
			InputPath: bookResult.InputPath, ReportPath: bookResult.ReportPath,
			OutputPath: bookResult.OutputPath, ArtifactDisposition: disposition,
			Status: bookResult.Envelope.Status, ExitCode: bookResult.ExitCode,
			Error: errorString(bookResult.Err), Findings: nonNilCleanFindings(bookResult.Envelope.Findings),
		})
	}
	batch.Envelope = report.CleanBatchEnvelope(bookSummaries)
	return batch, nil
}

// CleanFailureEnvelope constructs a batch-level envelope for failures that
// prevent clean from discovering or scheduling any input book.
func CleanFailureEnvelope(err error) report.Envelope {
	status := report.StatusFailed
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		status = report.StatusCancelled
	}
	return report.Envelope{
		SchemaVersion: "2",
		Capability:    cleanCapabilityID,
		Status:        status,
		Facts: map[string]any{
			"pipeline.artifactDisposition": "none",
			"pipeline.blockers":            []string{"clean.batch-failed"},
		},
		Findings: []report.Finding{{
			Level: "error", ID: "clean.batch-failed", Title: "EPUB clean batch could not start",
			Detail: err.Error(),
		}},
		Events: []report.Event{},
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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

func normalizeCleanSteps(requested []string, preset string, scope []string) ([]cleanStepDefinition, error) {
	all := cleanStepDefinitions()
	if requested == nil {
		if preset != "" || len(scope) > 0 {
			return nil, errors.New("--preset and --scope require the typography step")
		}
		return []cleanStepDefinition{}, nil
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
	typographySelected := slices.ContainsFunc(selected, func(step cleanStepDefinition) bool { return step.name == "typography" })
	if !typographySelected {
		if preset != "" || len(scope) > 0 {
			return nil, errors.New("--preset and --scope require --steps typography")
		}
		return selected, nil
	}
	if strings.TrimSpace(preset) == "" {
		return nil, errors.New("the typography step requires an explicit --preset")
	}
	if len(scope) == 0 {
		return nil, errors.New("the typography step requires --scope all or one or more exact spine XHTML paths")
	}
	allScope := slices.Contains(scope, "all")
	if allScope && len(scope) != 1 {
		return nil, errors.New("--scope all cannot be combined with individual paths")
	}
	seenScope := make(map[string]struct{}, len(scope))
	for _, path := range scope {
		if path == "all" {
			continue
		}
		if strings.TrimSpace(path) == "" || path == "." || filepath.IsAbs(path) || strings.ContainsRune(path, '\\') ||
			filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))) != path || strings.HasPrefix(path, "../") || path == ".." {
			return nil, fmt.Errorf("--scope requires exact relative EPUB paths using forward slashes: %q", path)
		}
		if _, exists := seenScope[path]; exists {
			return nil, fmt.Errorf("duplicate --scope path %q", path)
		}
		seenScope[path] = struct{}{}
	}
	for index := range selected {
		if selected[index].name != "typography" {
			continue
		}
		selected[index].args["preset"] = strings.TrimSpace(preset)
		if !allScope {
			encodedScope, err := jsonv2.Marshal(scope)
			if err != nil {
				return nil, fmt.Errorf("encode --scope: %w", err)
			}
			selected[index].args["scope_paths"] = string(encodedScope)
		}
	}
	return selected, nil
}

func cleanStepDefinitions() []cleanStepDefinition {
	return []cleanStepDefinition{
		{name: "normalize", capability: "epub.structure.normalize", args: Args{"mode": "normalize"}},
		{name: "migrate", capability: "epub.package.migrate.epub3", args: Args{}},
		{name: "css", capability: "epub.css.layering.optimize", args: Args{}},
		{name: "typography", capability: "epub.typography.optimize", args: Args{}},
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

func preflightCleanOutputs(inputs []cleanInput, inputPath string, inputIsDir bool, outputDir string, approve, retainReviewCandidate bool) error {
	if info, err := os.Stat(outputDir); err == nil && !info.IsDir() {
		return &UsageError{Err: fmt.Errorf("--out is not a directory: %s", outputDir)}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output directory: %w", err)
	}
	planned := map[string]string{}
	for _, input := range inputs {
		outputPath, reportPath := cleanOutputPaths(input, inputIsDir, outputDir)
		for _, candidate := range []struct{ path, label string }{{reportPath, "summary report"}} {
			path, label := candidate.path, candidate.label
			if previous, exists := planned[path]; exists {
				return &UsageError{Err: fmt.Errorf("output collision: %s and %s both map to %s", previous, label, path)}
			}
			planned[path] = label
		}
		if approve {
			paths := []struct{ path, label string }{{outputPath, "approved EPUB output"}}
			if retainReviewCandidate {
				paths = append(paths, struct{ path, label string }{cleanReviewOutputPath(outputPath), "review-only EPUB output"})
			}
			for _, candidate := range paths {
				path := candidate.path
				if previous, exists := planned[path]; exists {
					return &UsageError{Err: fmt.Errorf("output collision: %s and %s both map to %s", previous, candidate.label, path)}
				}
				planned[path] = candidate.label
			}
		}
	}
	plannedPaths := make([]string, 0, len(planned))
	for path := range planned {
		plannedPaths = append(plannedPaths, path)
	}
	slices.Sort(plannedPaths)
	for _, path := range plannedPaths {
		if _, err := os.Lstat(path); err == nil {
			return &UsageError{Err: fmt.Errorf("output already exists: %s", path)}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect output path %s: %w", path, err)
		}
	}
	return nil
}

func cleanReviewOutputPath(outputPath string) string {
	extension := filepath.Ext(outputPath)
	return strings.TrimSuffix(outputPath, extension) + ".review-only" + extension
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
	original, err := book.OpenContext(ctx, input.path)
	if err != nil {
		if inputSHA, hashErr := book.FileSHA256Context(ctx, input.path); hashErr == nil {
			env.Input = &report.Artifact{Path: input.path, SHA256: inputSHA}
		}
		return cleanBookFailure(result, env, "clean.input-read-failed", "Unable to read input EPUB", err)
	}
	defer original.Close()
	inputSHA, err := original.InputSHA256Context(ctx)
	if err != nil {
		return cleanBookFailure(result, env, "clean.input-read-failed", "Unable to hash input EPUB", err)
	}
	env.Input = &report.Artifact{Path: input.path, SHA256: inputSHA}
	session := newCleanSession(original)

	stepSummaries := []cleanStepSummary{}
	allEvents := []report.Event{}
	allFindings := []report.Finding{}
	var failure error
	cancelled := false
	redlineAttempted := false

	audit, auditErr := runWithBook(ctx, Options{RepoRoot: opts.RepoRoot, CapabilityID: "epub.package.nav.audit", InputPath: input.path, DryRun: true}, session.current, nil)
	if auditErr != nil {
		failure = auditErr
		stepSummaries = append(stepSummaries, cleanStepSummaryFrom("audit", "epub.package.nav.audit", "input", "input", []string{}, "failed", report.Envelope{}, auditErr))
	} else {
		cancelled = audit.Envelope.Status == report.StatusCancelled
		stepSummaries = append(stepSummaries, cleanStepSummaryFrom("audit", "epub.package.nav.audit", "input", "input", []string{}, audit.Envelope.Status, audit.Envelope, nil))
		allEvents = append(allEvents, audit.Envelope.Events...)
		blockers := cleanAuditBlockers(audit.Envelope.Findings, steps)
		for _, finding := range audit.Envelope.Findings {
			if finding.Level == "error" && !containsCleanFinding(blockers, finding) {
				allFindings = appendCleanFindings(allFindings, "audit", []report.Finding{{
					Level: "info", ID: finding.ID, Title: finding.Title,
					Detail: "This issue will be checked again after the selected repair steps.", Location: finding.Location,
				}})
				continue
			}
			if finding.Level != "error" || len(blockers) > 0 {
				allFindings = appendCleanFindings(allFindings, "audit", []report.Finding{finding})
			}
		}
		if len(blockers) > 0 {
			failure = fmt.Errorf("nav audit found %d blocking error(s)", len(blockers))
		} else if audit.ExitCode != ExitOK && !slices.ContainsFunc(audit.Envelope.Findings, func(finding report.Finding) bool {
			return finding.Level == "error"
		}) {
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
		inputState := session.stateID
		checkpoint := session.current
		candidate := session.BeginStep()
		runOptions := Options{
			RepoRoot: opts.RepoRoot, CapabilityID: step.capability,
			InputPath: input.path, Args: step.args,
		}
		outcome, runErr := runWithBook(ctx, runOptions, candidate, checkpoint)
		changedEntries, changesErr := session.ModifiedEntries(candidate)
		if changesErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("compare in-memory step changes: %w", changesErr))
		}
		cancelled = cancelled || outcome.Envelope.Status == report.StatusCancelled
		outputState := inputState
		if runErr == nil && outcome.ExitCode == ExitOK {
			session.CommitStep(step.name, candidate)
			outputState = session.stateID
		}
		stepSummaries = append(stepSummaries, cleanStepSummaryFrom(step.name, step.capability, inputState, outputState, changedEntries, outcome.Envelope.Status, outcome.Envelope, runErr))
		allEvents = append(allEvents, outcome.Envelope.Events...)
		allFindings = appendCleanFindings(allFindings, step.name, outcome.Envelope.Findings)
		if step.name == "normalize" && runErr == nil && outcome.ExitCode == ExitOK {
			normalizeReport, marshalErr := MarshalEnvelope(outcome.Envelope)
			if marshalErr != nil {
				failure = fmt.Errorf("serialize normalize path map: %w", marshalErr)
			} else {
				session.SetNormalizeReport(normalizeReport)
			}
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

	if failure == nil && len(steps) > 0 && session.hasCandidate {
		finalAudit, finalAuditErr := runWithBook(ctx, Options{
			RepoRoot: opts.RepoRoot, CapabilityID: "epub.package.nav.audit",
			InputPath: input.path, DryRun: true,
		}, session.current, nil)
		if finalAuditErr != nil {
			failure = finalAuditErr
			allEvents = append(allEvents, report.Event{Step: "audit-final", Status: "failed", Message: finalAuditErr.Error()})
		} else {
			stepSummaries = append(stepSummaries, cleanStepSummaryFrom("audit-final", "epub.package.nav.audit", session.stateID, session.stateID, []string{}, finalAudit.Envelope.Status, finalAudit.Envelope, nil))
			allEvents = append(allEvents, finalAudit.Envelope.Events...)
			allFindings = appendCleanFindings(allFindings, "audit-final", finalAudit.Envelope.Findings)
			blockers := cleanAuditBlockers(finalAudit.Envelope.Findings, nil)
			if len(blockers) > 0 {
				failure = fmt.Errorf("post-transform nav audit found %d blocking error(s)", len(blockers))
			} else if finalAudit.ExitCode != ExitOK {
				failure = fmt.Errorf("post-transform nav audit exited with code %d", finalAudit.ExitCode)
			}
		}
	}

	var redlineSummary cleanRedlineSummary
	if session.hasCandidate {
		redlineAttempted = true
		var redlineErr error
		redlineSummary, redlineErr = compareCleanRedline(session, inputSHA)
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

	reviewCandidateWritten := false
	if opts.Approve && failure == nil && redlineAttempted && session.hasCandidate && ctx.Err() == nil {
		if writeErr := session.current.WriteToContext(ctx, outputPath); writeErr != nil {
			failure = errors.Join(failure, fmt.Errorf("write approved candidate: %w", writeErr))
		} else {
			result.OutputPath = outputPath
			env.Output = &report.Artifact{Path: outputPath}
			if outputSHA, hashErr := book.FileSHA256ContextLimit(ctx, outputPath, 4<<30); hashErr == nil {
				env.Output.SHA256 = outputSHA
			} else {
				allFindings = append(allFindings, report.Finding{Level: "warn", ID: "clean.output-sha256-unavailable", Title: "Output SHA-256 unavailable", Detail: hashErr.Error(), Location: outputPath})
			}
		}
	}
	if opts.RetainReviewCandidate && failure != nil && redlineAttempted && session.hasCandidate && ctx.Err() == nil {
		reviewPath := cleanReviewOutputPath(outputPath)
		if writeErr := session.current.WriteToContext(ctx, reviewPath); writeErr != nil {
			failure = errors.Join(failure, fmt.Errorf("write review-only candidate: %w", writeErr))
		} else {
			result.OutputPath = reviewPath
			env.Output = &report.Artifact{Path: reviewPath}
			if outputSHA, hashErr := book.FileSHA256ContextLimit(ctx, reviewPath, 4<<30); hashErr == nil {
				env.Output.SHA256 = outputSHA
			} else {
				allFindings = append(allFindings, report.Finding{Level: "warn", ID: "clean.output-sha256-unavailable", Title: "Output SHA-256 unavailable", Detail: hashErr.Error(), Location: reviewPath})
			}
			reviewCandidateWritten = true
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
	selectedSteps := make([]string, 0, len(steps))
	for _, step := range steps {
		selectedSteps = append(selectedSteps, step.name)
	}
	facts["pipeline.selectedSteps"] = selectedSteps
	artifactDisposition := "none"
	switch {
	case reviewCandidateWritten:
		artifactDisposition = "review-only"
	case env.Output != nil:
		artifactDisposition = "approved"
	case status == report.StatusPlanned && !session.hasCandidate:
		artifactDisposition = "planned"
	case session.hasCandidate && failure == nil:
		artifactDisposition = "planned"
	case session.hasCandidate:
		artifactDisposition = "withheld"
	}
	facts["pipeline.artifactDisposition"] = artifactDisposition
	for _, step := range stepSummaries {
		if step.Name == "normalize" {
			if mappings, ok := step.Facts["epub.structure.normalize.mappings"]; ok {
				facts["epub.clean.normalize.mappings"] = mappings
			}
			break
		}
	}
	if session.hasCandidate && env.Output == nil {
		facts["epub.clean.previewState"] = session.stateID
	}
	env.Events = allEvents
	env.Findings = append(env.Findings, allFindings...)
	blockers := make([]string, 0)
	for _, finding := range env.Findings {
		if finding.Level == "error" {
			blockers = append(blockers, finding.ID)
		}
	}
	slices.Sort(blockers)
	blockers = slices.Compact(blockers)
	facts["pipeline.blockers"] = blockers
	env.Facts = facts
	result.Envelope = env
	return result
}

func cleanAuditBlockers(findings []report.Finding, steps []cleanStepDefinition) []report.Finding {
	migrateSelected := slices.ContainsFunc(steps, func(step cleanStepDefinition) bool { return step.name == "migrate" })
	blockers := make([]report.Finding, 0)
	for _, finding := range findings {
		if finding.Level != "error" {
			continue
		}
		migratable := migrateSelected && (finding.Title == `MathML XHTML item missing properties="mathml"` ||
			finding.Title == `Inline SVG XHTML item missing properties="svg"`)
		if !migratable {
			blockers = append(blockers, finding)
		}
	}
	return blockers
}

func containsCleanFinding(findings []report.Finding, target report.Finding) bool {
	return slices.ContainsFunc(findings, func(finding report.Finding) bool { return finding.ID == target.ID })
}

func compareCleanRedline(session *cleanSession, inputSHA string) (cleanRedlineSummary, error) {
	pathMap := map[string]string{}
	if len(session.normalizeReport) > 0 {
		var err error
		pathMap, err = redline.LoadPathMap(session.normalizeReport)
		if err != nil {
			return cleanRedlineSummary{}, err
		}
	}
	findings, err := redline.Check(redline.OriginalState(session.original), redline.CurrentState(session.current), nil, redline.Options{PathMap: pathMap})
	if readErr := session.current.ReadError(); readErr != nil && err == nil {
		err = readErr
	}
	result := cleanRedlineSummary{
		Status: report.StatusComplete,
		Facts: map[string]any{
			"epub.redline.check":          "all",
			"epub.redline.before":         session.original.InputPath(),
			"epub.redline.afterState":     session.stateID,
			"epub.redline.beforeSHA256":   inputSHA,
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

func cleanStepSummaryFrom(name, capability, inputState, outputState string, changedEntries []string, status string, env report.Envelope, err error) cleanStepSummary {
	findings := nonNilCleanFindings(env.Findings)
	if err != nil {
		findings = append(findings, report.Finding{Level: "error", ID: "clean.step-run-failed", Title: "Clean stage failed", Detail: err.Error(), Location: name})
	}
	summary := cleanStepSummary{
		Name: name, Capability: capability, Status: status,
		InputState: inputState, OutputState: outputState, ChangedEntries: slices.Clone(changedEntries),
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
