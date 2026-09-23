package typography

import (
	"context"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
)

// typographyPageCache lazily shares source bytes and parsed XHTML between the
// coverage, font-mode, and stylesheet-link passes. Loading remains lazy so the
// first error and cancellation point stay in the same order as the pipeline.
type typographyPageCache struct {
	ctx   context.Context
	book  *book.Book
	pages map[string]*typographyPage
}

type typographyPage struct {
	data      []byte
	dataErr   error
	dataReady bool

	tree      *opf.SpanNode
	treeErr   error
	treeReady bool

	regions      []xhtml.Region
	regionStop   int
	regionsReady bool
}

func newTypographyPageCache(ctx context.Context, b *book.Book) *typographyPageCache {
	return &typographyPageCache{ctx: ctx, book: b, pages: make(map[string]*typographyPage)}
}

func (c *typographyPageCache) page(path string) *typographyPage {
	page := c.pages[path]
	if page == nil {
		page = &typographyPage{}
		c.pages[path] = page
	}
	return page
}

func (c *typographyPageCache) data(path string) ([]byte, error) {
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	page := c.page(path)
	if !page.dataReady {
		page.data, page.dataErr = c.book.CurrentContext(c.ctx, path)
		page.dataReady = true
	}
	return page.data, page.dataErr
}

func (c *typographyPageCache) tree(path string) (*opf.SpanNode, error) {
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	page := c.page(path)
	if !page.treeReady {
		data, err := c.data(path)
		if err != nil {
			page.treeErr = err
		} else {
			page.tree, page.treeErr = opf.ScanXHTMLSpanTree(data)
		}
		page.treeReady = true
	}
	return page.tree, page.treeErr
}

func (c *typographyPageCache) markupRegions(path, text string) ([]xhtml.Region, int) {
	page := c.page(path)
	if !page.regionsReady {
		page.regions, page.regionStop = xhtml.ScanRegions(text)
		page.regionsReady = true
	}
	return page.regions, page.regionStop
}
