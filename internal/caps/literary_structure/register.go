package literarystructure

// classVocabulary returns the class tokens named by SPEC-实现约束 §7 and
// docs/how-to/classical-modern-layout.md. A fresh map keeps package state
// immutable while making the provenance explicit in this register file.
func classVocabulary() map[string]bool {
	return map[string]bool{
		"dialog":                     true,
		"poetry":                     true,
		"letter":                     true,
		"scene-break":                true,
		"chapter-head":               true,
		"chapter-head-art":           true,
		"chapter-head-banner":        true,
		"chapter-header":             true,
		"epigraph":                   true,
		"copyright-page":             true,
		"dedication":                 true,
		"epigraph-page":              true,
		"english-fiction":            true,
		"classical-modern":           true,
		"parallel-entry":             true,
		"parallel-pair":              true,
		"parallel-float-pair":        true,
		"parallel-stack-pair":        true,
		"parallel-entry-title":       true,
		"parallel-source":            true,
		"classical-text":             true,
		"modern-text":                true,
		"parallel-return":            true,
		"parallel-ratio-balanced":    true,
		"parallel-ratio-source-wide": true,
		"parallel-clear":             true,
	}
}
