package pipeline

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

// Cancel at a deterministic scan checkpoint, not with a wall-clock race.
// The wrapped context's Done and Err remain consistent after cancellation.
type checkpointContext struct {
	context.Context
	cancel    context.CancelFunc
	remaining atomic.Int32
}

func (c *checkpointContext) Err() error {
	if c.remaining.Add(-1) == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func cancelAtCheckpoint(t *testing.T, n int32) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	c := &checkpointContext{Context: ctx, cancel: cancel}
	c.remaining.Store(n)
	return c
}

func TestRealScannersStopInsideWorkWithoutPartialResults(t *testing.T) {
	for _, id := range []string{"epub.package.nav.audit", "epub.layout.audit", "epub.text.content.analyze", "epub.image.layout.optimize", "epub.css.layering.optimize"} {
		t.Run(id, func(t *testing.T) {
			// This is a real registered capability, not a pipeline runner stub.
			b, err := book.Open(buildEpubWithOPF(t))
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			for _, checkpoint := range []int32{1, 4} {
				res, err := registry[id](cancelAtCheckpoint(t, checkpoint), b, Args{}, nil)
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("checkpoint %d: err=%v status=%s", checkpoint, err, res.Status)
				}
				if res.Status != "" || len(res.Findings) != 0 || len(res.Facts) != 0 {
					t.Fatalf("cancelled scan leaked a partial success/error report: %+v", res)
				}
				if len(b.ModifiedNames()) != 0 {
					t.Fatalf("cancelled scan applied edits: %v", b.ModifiedNames())
				}
			}
		})
	}
}

func TestSourceAnalysisCancellationRemainsAnError(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"a.xhtml", "<html><body><p>one</p><p>two</p></body></html>"},
		{"a.html", "<p>one</p><p>two</p>"},
		{"a.md", "# One\n\nTwo\n\nThree"},
		{"a.txt", "One\n\nTwo\n\nThree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := registry["epub.text.content.analyze"](cancelAtCheckpoint(t, 5), nil, Args{"source_name": tc.name, "source_content": tc.content}, nil)
			if !errors.Is(err, context.Canceled) || res.Status != "" {
				t.Fatalf("cancel classified as book finding: %+v %v", res, err)
			}
		})
	}
}
