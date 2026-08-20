package notion

import (
	"fmt"
	"slices"
	"time"

	"github.com/jomei/notionapi"

	"github.com/pscheid92/voiceline-diary/internal/diary"
)

const maxTextRunes = 1900

func render(e diary.Entry, now time.Time, databaseID string) *notionapi.PageCreateRequest {
	var p page

	t := title(e, now)

	p.addTitle(t)
	p.addLine(t)

	p.addSection("What went well", e.WentWell)
	p.addSection("What went badly", e.WentBadly)
	p.addSection("Todos", e.Todos)

	p.addTranscript(e.Transcript)

	return p.request(databaseID)
}

type page struct {
	title    string
	children []notionapi.Block
}

func (p *page) addTitle(s string) {
	p.title = s
}

func (p *page) addLine(s string) {
	p.children = append(p.children, paragraph(s))
}

func (p *page) addSection(name string, items []string) {
	p.children = append(p.children, heading(name))

	if len(items) == 0 {
		p.children = append(p.children, paragraph("—"))
		return
	}

	for _, item := range items {
		p.children = append(p.children, bullet(item))
	}
}

func (p *page) addTranscript(s string) {
	p.children = append(p.children, heading("Transcript"))

	for part := range slices.Chunk([]rune(s), maxTextRunes) {
		p.children = append(p.children, paragraph(string(part)))
	}
}

func (p *page) request(databaseID string) *notionapi.PageCreateRequest {
	return &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			Type:       notionapi.ParentTypeDatabaseID,
			DatabaseID: notionapi.DatabaseID(databaseID),
		},
		Properties: notionapi.Properties{
			"title": notionapi.TitleProperty{Title: text(p.title)},
		},
		Children: p.children,
	}
}

func title(e diary.Entry, now time.Time) string {
	ts := now.Format("Mon, Jan 2 2006, 15:04")
	return fmt.Sprintf("%s — %d/10, %s", ts, e.Rating, e.Emotion)
}

func text(s string) []notionapi.RichText {
	return []notionapi.RichText{{Text: &notionapi.Text{Content: s}}}
}

func basic(t notionapi.BlockType) notionapi.BasicBlock {
	return notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: t}
}

func heading(s string) notionapi.Block {
	return notionapi.Heading2Block{
		BasicBlock: basic(notionapi.BlockTypeHeading2),
		Heading2:   notionapi.Heading{RichText: text(s)},
	}
}

func paragraph(s string) notionapi.Block {
	return notionapi.ParagraphBlock{
		BasicBlock: basic(notionapi.BlockTypeParagraph),
		Paragraph:  notionapi.Paragraph{RichText: text(s)},
	}
}

func bullet(s string) notionapi.Block {
	return notionapi.BulletedListItemBlock{
		BasicBlock:       basic(notionapi.BlockTypeBulletedListItem),
		BulletedListItem: notionapi.ListItem{RichText: text(s)},
	}
}
