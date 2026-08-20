package notion

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jomei/notionapi"

	"github.com/pscheid92/voiceline-diary/internal/diary"
)

const timeout = 15 * time.Second

type pages interface {
	Create(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error)
}

type Client struct {
	pages      pages
	databaseID string
}

func New(token, databaseID string) *Client {
	api := notionapi.NewClient(notionapi.Token(token))
	return &Client{pages: api.Page, databaseID: databaseID}
}

func (c *Client) DatabaseURL() string {
	return "https://www.notion.so/" + strings.ReplaceAll(c.databaseID, "-", "")
}

func (c *Client) File(ctx context.Context, e diary.Entry, now time.Time) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	p := render(e, now, c.databaseID)

	created, err := c.pages.Create(ctx, p)
	if err != nil {
		return "", fmt.Errorf("notion: %w: %v", diary.ErrProvider, err)
	}

	return created.URL, nil
}
