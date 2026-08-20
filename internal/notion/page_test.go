package notion

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jomei/notionapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pscheid92/voiceline-diary/internal/diary"
)

func blockText(t *testing.T, b notionapi.Block) string {
	t.Helper()

	var rich []notionapi.RichText
	switch v := b.(type) {
	case notionapi.Heading2Block:
		rich = v.Heading2.RichText
	case notionapi.ParagraphBlock:
		rich = v.Paragraph.RichText
	case notionapi.BulletedListItemBlock:
		rich = v.BulletedListItem.RichText
	default:
		t.Fatalf("unexpected block type %T", b)
	}

	require.NotEmpty(t, rich, "block %T has no rich text", b)
	return rich[0].Text.Content
}

func TestRender_TitlesTheEntryWithThePersonsClock(t *testing.T) {
	at := time.Date(2026, time.August, 19, 0, 30, 0, 0, time.FixedZone("CEST", 2*3600))

	req := render(diary.Entry{Day: diary.Day{Rating: 7, Emotion: "ruhig"}}, at, "db")

	title, ok := req.Properties["title"].(notionapi.TitleProperty)
	require.True(t, ok, "the title property must be a TitleProperty, got %T", req.Properties["title"])
	require.Len(t, title.Title, 1)
	assert.Equal(t, "Wed, Aug 19 2026, 00:30 — 7/10, ruhig", title.Title[0].Text.Content)
	assert.Equal(t, notionapi.DatabaseID("db"), req.Parent.DatabaseID)
}

func TestAddSection_RendersItemsAsBullets(t *testing.T) {
	var p page

	p.addSection("What went well", []string{"a walk", "a nap"})

	require.Len(t, p.children, 3, "want heading plus two bullets")
	assert.Equal(t, notionapi.BlockTypeHeading2, p.children[0].GetType())
	for i, want := range []string{"a walk", "a nap"} {
		assert.Equal(t, notionapi.BlockTypeBulletedListItem, p.children[i+1].GetType())
		assert.Equal(t, want, blockText(t, p.children[i+1]))
	}
}

func TestAddSection_EmptyListShowsADash(t *testing.T) {
	var p page

	p.addSection("Todos", nil)

	require.Len(t, p.children, 2, "want heading plus dash")
	assert.Equal(t, "—", blockText(t, p.children[1]))
}

func TestAddTranscript_SplitsUnderNotionsCap(t *testing.T) {
	var p page
	said := strings.Repeat("ä", 5000)

	p.addTranscript(said)

	var rejoined string
	for _, block := range p.children[1:] {
		part := blockText(t, block)
		assert.LessOrEqual(t, utf8.RuneCountInString(part), 2000, "a paragraph is over Notion's cap")
		assert.True(t, utf8.ValidString(part), "a paragraph was cut mid-rune")
		rejoined += part
	}
	assert.Equal(t, said, rejoined, "the paragraphs must join back into the transcript")
}
