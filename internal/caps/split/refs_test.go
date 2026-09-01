package split

import (
	"reflect"
	"testing"
)

func TestParseSrcsetCandidates(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantURL []string
		wantErr bool
	}{
		{
			name:    "density candidates",
			value:   "../Images/a.webp 1x, ../Images/b.webp 2x",
			wantURL: []string{"../Images/a.webp", "../Images/b.webp"},
		},
		{
			name:    "data URI comma belongs to URL",
			value:   "data:image/svg+xml,%3Csvg%3E 1x, ../Images/b.webp 2x",
			wantURL: []string{"data:image/svg+xml,%3Csvg%3E", "../Images/b.webp"},
		},
		{
			name:    "data URI multiple commas",
			value:   "data:text/plain,a,b,c 1x, ../Images/b.webp 2x",
			wantURL: []string{"data:text/plain,a,b,c", "../Images/b.webp"},
		},
		{
			name:    "malformed trailing comma",
			value:   "../Images/a.webp 1x,",
			wantErr: true,
		},
		{
			name:    "quoted candidate",
			value:   "\"../Images/a.webp\" 1x",
			wantErr: true,
		},
		{
			name:    "unknown descriptor",
			value:   "../Images/a.webp 1q",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSrcsetCandidates(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSrcsetCandidates() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			urls := make([]string, len(got))
			for i, candidate := range got {
				urls[i] = candidate.url
			}
			if !reflect.DeepEqual(urls, tt.wantURL) {
				t.Fatalf("candidate URLs = %#v, want %#v", urls, tt.wantURL)
			}
		})
	}
}

func TestCollectRawURIsIncludesSrcsetCandidates(t *testing.T) {
	text := `<source srcset="data:image/svg+xml,%3Csvg%3E 1x, ../Images/a.webp 2x" src="../Images/fallback.webp">`
	got, err := collectRawURIsStrict(text)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"data:image/svg+xml,%3Csvg%3E", "../Images/a.webp", "../Images/fallback.webp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectRawURIsStrict() = %#v, want %#v", got, want)
	}
}

func TestCollectCSSURIsStrict(t *testing.T) {
	text := `/* url("ignored.png") */ .hero { background: url("../Images/hero.webp"); } @import "../Styles/more.css";`
	got, err := collectCSSURIsStrict(text)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"../Images/hero.webp", "../Styles/more.css"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectCSSURIsStrict() = %#v, want %#v", got, want)
	}
}
