package report

// StyleScene points to real fixture bytes, not a generated compatibility claim.
type StyleScene struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Path        string   `json:"path"`
	SHA256      string   `json:"sha256"`
	Stylesheets []string `json:"stylesheets"`
	Classes     []string `json:"classes"`
}
