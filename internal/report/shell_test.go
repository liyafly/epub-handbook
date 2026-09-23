package report

import "testing"

func TestShellQuote(t *testing.T) {
	tests := []struct{ input, want string }{
		{"", "''"},
		{"/a/b/book.epub", "/a/b/book.epub"},
		{"/a/with space/book.epub", "'/a/with space/book.epub'"},
		{"/a/<draft>.epub", "'/a/<draft>.epub'"},
		{"/a/指南/书.epub", "'/a/指南/书.epub'"},
		{"/a/it's/book.epub", `'/a/it'"'"'s/book.epub'`},
		{"~/book.epub", "'~/book.epub'"},
	}
	for _, tt := range tests {
		if got := ShellQuote(tt.input); got != tt.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
