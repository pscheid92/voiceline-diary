package notion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jomei/notionapi"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/pscheid92/voiceline-diary/internal/diary"
)

func TestDatabaseURL_IsSomewhereABrowserCanGo(t *testing.T) {
	url := New("token", "1429989f-e8ac-4eff-bc8f-57f56486db54").DatabaseURL()

	assert.Equal(t, "https://www.notion.so/1429989fe8ac4effbc8f57f56486db54", url)
}

func TestFile_AnswersWithWhereTheEntryNowLives(t *testing.T) {
	at := time.Date(2026, time.August, 19, 21, 30, 0, 0, time.UTC)
	entry := diary.NewEntry(diary.Day{Rating: 7, Emotion: "content"}, "Me: a good day\n")

	pages := newMockPages(t)
	pages.EXPECT().Create(mock.Anything, mock.Anything).
		Return(&notionapi.Page{URL: "https://notion.so/the-page"}, nil)

	url, err := (&Client{pages: pages, databaseID: "db"}).File(context.Background(), entry, at)

	require.NoError(t, err)
	assert.Equal(t, "https://notion.so/the-page", url,
		"the person is sent to the page, not told it worked")

	req := pages.Calls[0].Arguments.Get(1).(*notionapi.PageCreateRequest)
	assert.Equal(t, notionapi.DatabaseID("db"), req.Parent.DatabaseID)
	assert.NotEmpty(t, req.Children, "an entry with nothing in it is not what was filed")
}

func TestFile_SaysWhoFailedWhenTheDiaryRefusesIt(t *testing.T) {
	pages := newMockPages(t)
	pages.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errors.New("401 unauthorized"))

	url, err := (&Client{pages: pages, databaseID: "db"}).File(
		context.Background(), diary.Entry{}, time.Now())

	assert.Empty(t, url)
	assert.ErrorIs(t, err, diary.ErrProvider,
		"the session tells a person their entry could not be saved by asking who broke")
	assert.Contains(t, err.Error(), "401 unauthorized", "and what the provider actually said")
}

func TestFile_DoesNotWaitOnNotionForever(t *testing.T) {
	var deadline time.Time

	pages := newMockPages(t)
	pages.EXPECT().Create(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ *notionapi.PageCreateRequest) (*notionapi.Page, error) {
			deadline, _ = ctx.Deadline()
			return &notionapi.Page{}, nil
		})

	_, err := (&Client{pages: pages, databaseID: "db"}).File(
		context.Background(), diary.Entry{}, time.Now())

	require.NoError(t, err)
	assert.False(t, deadline.IsZero(),
		"a person waiting on a save cannot be left there by a provider that never answers")
}
