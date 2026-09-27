package report

// StyleScene points to real fixture bytes, not a generated compatibility claim.
type StyleScene struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Path             string            `json:"path"`
	SHA256           string            `json:"sha256"`
	Stylesheets      []string          `json:"stylesheets"`
	StylesheetSHA256 map[string]string `json:"stylesheetSHA256"`
	Classes          []string          `json:"classes"`
}

// StylePreset describes a discovered bundled preset. Discovery is not application or reader acceptance.
type StylePreset struct {
	ID           string   `json:"id"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Layers       []string `json:"layers"`
	Notes        string   `json:"notes"`
	Source       string   `json:"source"`
	SourcePath   string   `json:"sourcePath"`
	ReaderStatus string   `json:"readerStatus"`
}
