package styledemo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
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
	if b != nil {
		read = b.Current
	} else {
		root, err := os.OpenRoot(p.DemoDir)
		if err != nil {
			return report.Result{}, err
		}
		defer root.Close()
		read = root.ReadFile
	}
	scenes, err := scanScenes(ctx, read, p.Query)
	if err != nil {
		return report.Result{}, err
	}
	return report.Result{Capability: CapabilityID, Status: report.StatusComplete, Facts: map[string]any{
		"mode": "catalog", "query": p.Query, "scenes": scenes, "sceneCount": len(scenes),
		"previewStatus": "not-rendered", "readerStatus": "not-verified",
		"evidenceSource": "docs/final/reader-matrix.yaml",
	}}, nil
}

func scanScenes(ctx context.Context, read func(string) ([]byte, error), query string) ([]report.StyleScene, error) {
	container, err := read(opf.ContainerPath)
	if err != nil {
		return nil, err
	}
	root, err := opf.ScanSpanTree(container)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	data, err := read(packagePath)
	if err != nil {
		return nil, err
	}
	pkg, err := opf.Parse(packagePath, data)
	if err != nil {
		return nil, err
	}
	scenes := []report.StyleScene{}
	query = strings.ToLower(strings.TrimSpace(query))
	for _, ref := range pkg.Spine {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item, ok := pkg.ItemByID(ref.IDRef)
		if !ok {
			return nil, fmt.Errorf("catalog: missing spine item %s", ref.IDRef)
		}
		if item.MediaType != "application/xhtml+xml" || pypath.HasNavProp(item.Properties) {
			continue
		}
		name, err := pypath.ValidateArchivePath(item.ArchivePath, "scene path")
		if err != nil {
			return nil, err
		}
		data, err := read(name)
		if err != nil {
			return nil, err
		}
		doc, err := opf.ScanSpanTree(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		scene := report.StyleScene{ID: ref.IDRef, Path: name, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Classes: []string{}, Stylesheets: []string{}}
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
				return nil, err
			}
			if _, err := read(cssPath); err != nil {
				return nil, fmt.Errorf("catalog stylesheet %s: %w", cssPath, err)
			}
			if !slices.Contains(scene.Stylesheets, cssPath) {
				scene.Stylesheets = append(scene.Stylesheets, cssPath)
			}
		}
		slices.Sort(scene.Classes)
		if query == "" || strings.Contains(strings.ToLower(scene.ID+" "+scene.Title+" "+scene.Path+" "+strings.Join(scene.Classes, " ")), query) {
			scenes = append(scenes, scene)
		}
	}
	return scenes, nil
}
