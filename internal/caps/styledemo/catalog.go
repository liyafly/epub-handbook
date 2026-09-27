package styledemo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// Build the catalog from manifest/spine and XHTML; no second scene inventory
// to drift away from fixtures. Catalog mode is discovery, not validation.
func describeScenes(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	var read func(string) ([]byte, error)
	resourceSource := "filesystem"
	sourcePathBase := "demo source tree"
	if b != nil {
		read = b.Current
		resourceSource = "artifact"
		sourcePathBase = "EPUB archive path"
	} else if p.DemoDir != "" {
		root, err := os.OpenRoot(p.DemoDir)
		if err != nil {
			return report.Result{}, err
		}
		defer root.Close()
		read = root.ReadFile
	} else if p.DemoFS != nil {
		read = func(name string) ([]byte, error) { return fs.ReadFile(p.DemoFS, name) }
		resourceSource = "embedded"
		sourcePathBase = "embedded demo resource path"
	} else {
		return report.Result{}, fmt.Errorf("styledemo: no demo source is available; set demo_dir or run a binary with embedded catalog resources")
	}
	scenes, findings, err := scanScenes(ctx, read, p.Query)
	if err != nil {
		return report.Result{}, err
	}
	return report.Result{Capability: CapabilityID, Status: report.StatusComplete, Facts: map[string]any{
		"mode": "catalog", "collection": "scenes", "query": p.Query, "scenes": scenes, "sceneCount": len(scenes),
		"resourceSource": resourceSource, "sourcePathBase": sourcePathBase,
		"previewStatus": "not-rendered", "readerStatus": "not-verified",
		"evidenceSource": "docs/final/reader-matrix.yaml",
	}, Findings: findings}, nil
}

func scanScenes(ctx context.Context, read func(string) ([]byte, error), query string) ([]report.StyleScene, []report.Finding, error) {
	container, err := read(opf.ContainerPath)
	if err != nil {
		return nil, nil, err
	}
	root, err := opf.ScanSpanTree(container)
	if err != nil {
		return nil, nil, err
	}
	packagePath := ""
	for _, node := range root.Walk() {
		if node.Name.Local == "rootfile" {
			packagePath, _ = node.AttrByLocal("", "full-path")
			break
		}
	}
	packagePath, err = pypath.ValidateArchivePath(packagePath, "container rootfile")
	if err != nil {
		return nil, nil, err
	}
	data, err := read(packagePath)
	if err != nil {
		return nil, nil, err
	}
	pkg, err := opf.Parse(packagePath, data)
	if err != nil {
		return nil, nil, err
	}
	scenes := []report.StyleScene{}
	findings := []report.Finding{}
	stylesheetErrors := map[string]error{}
	stylesheetHashes := map[string]string{}
	query = strings.ToLower(strings.TrimSpace(query))
	for _, ref := range pkg.Spine {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		item, ok := pkg.ItemByID(ref.IDRef)
		if !ok {
			findings = append(findings, report.Finding{
				Level: "warn", ID: "styledemo.catalog-missing-spine-item", Title: ref.IDRef,
				Detail: "catalog spine idref does not resolve to a manifest item", Location: packagePath,
			})
			continue
		}
		if item.MediaType != "application/xhtml+xml" || pypath.HasNavProp(item.Properties) {
			continue
		}
		name, err := pypath.ValidateArchivePath(item.ArchivePath, "scene path")
		if err != nil {
			return nil, nil, err
		}
		data, err := read(name)
		if err != nil {
			return nil, nil, err
		}
		doc, err := opf.ScanXHTMLSpanTree(data)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		scene := report.StyleScene{ID: ref.IDRef, Path: name, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Classes: []string{}, Stylesheets: []string{}, StylesheetSHA256: map[string]string{}}
		for _, node := range doc.Walk() {
			if node.Name.Local == "title" && scene.Title == "" {
				scene.Title = strings.TrimSpace(node.IterText())
			}
			class, _ := node.AttrByLocal("", "class")
			for _, token := range strings.Fields(class) {
				if !slices.Contains(scene.Classes, token) {
					scene.Classes = append(scene.Classes, token)
				}
			}
			if node.Name.Local != "link" {
				continue
			}
			rel, _ := node.AttrByLocal("", "rel")
			if !slices.Contains(strings.Fields(strings.ToLower(rel)), "stylesheet") {
				continue
			}
			href, _ := node.AttrByLocal("", "href")
			if href == "" || pypath.IsExternalURI(href) {
				continue
			}
			cssPath, err := pypath.ResolveRelativePath(name, pypath.URLSplit(href).Path)
			if err != nil {
				return nil, nil, err
			}
			cssErr, checked := stylesheetErrors[cssPath]
			if !checked {
				var cssData []byte
				cssData, cssErr = read(cssPath)
				stylesheetErrors[cssPath] = cssErr
				if cssErr == nil {
					stylesheetHashes[cssPath] = fmt.Sprintf("%x", sha256.Sum256(cssData))
				}
			}
			if cssErr != nil {
				findings = append(findings, report.Finding{
					Level: "warn", ID: "styledemo.catalog-missing-stylesheet", Title: cssPath,
					Detail: cssErr.Error(), Location: name,
				})
				continue
			}
			if !slices.Contains(scene.Stylesheets, cssPath) {
				scene.Stylesheets = append(scene.Stylesheets, cssPath)
				scene.StylesheetSHA256[cssPath] = stylesheetHashes[cssPath]
			}
		}
		slices.Sort(scene.Classes)
		slices.Sort(scene.Stylesheets)
		if query == "" || strings.Contains(strings.ToLower(scene.ID+" "+scene.Title+" "+scene.Path+" "+strings.Join(scene.Classes, " ")), query) {
			scenes = append(scenes, scene)
		}
	}
	return scenes, findings, nil
}

func describePresets(ctx context.Context, p Params) (report.Result, error) {
	var sourceFS fs.FS
	source := "filesystem"
	if p.PresetDir != "" {
		sourceFS = os.DirFS(p.PresetDir)
	} else if p.PresetFS != nil {
		sourceFS = p.PresetFS
		source = "embedded"
	} else {
		return report.Result{}, fmt.Errorf("styledemo: no preset catalog is available; run from a checkout or use a binary with embedded catalog resources")
	}
	entries, err := fs.ReadDir(sourceFS, ".")
	if err != nil {
		return report.Result{}, fmt.Errorf("styledemo: read preset catalog: %w", err)
	}
	query := strings.ToLower(strings.TrimSpace(p.Query))
	presets := []report.StylePreset{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return report.Result{}, err
		}
		if !entry.IsDir() {
			continue
		}
		configPath := path.Join(entry.Name(), "preset.json")
		data, err := fs.ReadFile(sourceFS, configPath)
		if err != nil {
			return report.Result{}, fmt.Errorf("styledemo: read %s: %w", configPath, err)
		}
		var config struct {
			Name        string   `json:"name"`
			Version     string   `json:"version"`
			Description string   `json:"description"`
			Layers      []string `json:"layers"`
			Notes       string   `json:"notes"`
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return report.Result{}, fmt.Errorf("styledemo: parse %s: %w", configPath, err)
		}
		if config.Name == "" || config.Version == "" || len(config.Layers) == 0 {
			return report.Result{}, fmt.Errorf("styledemo: %s is missing name, version, or layers", configPath)
		}
		if config.Description == "" {
			config.Description = config.Name
		}
		if query != "" && !strings.Contains(strings.ToLower(config.Name+" "+config.Description+" "+strings.Join(config.Layers, " ")+" "+config.Notes), query) {
			continue
		}
		presets = append(presets, report.StylePreset{
			ID: entry.Name(), Version: config.Version, Description: config.Description,
			Layers: append([]string{}, config.Layers...), Notes: config.Notes,
			Source: source, SourcePath: path.Join(entry.Name(), "preset.json"), ReaderStatus: "not-verified",
		})
	}
	slices.SortFunc(presets, func(a, b report.StylePreset) int { return strings.Compare(a.ID, b.ID) })
	return report.Result{Capability: CapabilityID, Status: report.StatusComplete, Facts: map[string]any{
		"mode": "catalog", "collection": "presets", "query": p.Query, "presets": presets, "presetCount": len(presets),
		"resourceSource": source, "readerStatus": "not-verified", "evidenceSource": "docs/final/reader-matrix.yaml",
	}}, nil
}
