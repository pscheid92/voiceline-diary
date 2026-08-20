package session

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
)

func TestSession_TheLoop(t *testing.T) {
	c := begin(t)
	c.book.EXPECT().File(mock.Anything, mock.Anything, mock.Anything).Return("https://notion.so/fake-page", nil)

	saidAt := time.Date(2026, time.August, 20, 23, 12, 0, 0, time.UTC)

	c.says(NewSpeakCommand([]byte{1, 2, 3}))
	assert.Equal(t, [][]byte{{1, 2, 3}}, spoken(t, c, 1))

	aDayWorthFiling(c)
	c.hears(conversation.Event{Calls: []conversation.Call{goodbye}, TurnComplete: true})
	await[Concluded](t, c)

	c.says(NewSaveCommand(saidAt))
	c.over(t)

	c.companion.AssertNumberOfCalls(t, "Greet", 1)

	concluded := told[Concluded](c)
	require.Len(t, concluded, 1)
	assert.Empty(t, concluded[0].Caveat, "a day talked through in full is not salvage")

	entries := told[Filed](c)
	require.Len(t, entries, 1)
	assert.Equal(t, "https://notion.so/fake-page", entries[0].Entry.URL)
	c.user.AssertCalled(t, "Close", "saved")

	require.Len(t, filed(c), 1, "want exactly one entry filed")
	assert.Equal(t, []string{"a run"}, filed(c)[0].WentWell)
	assert.NotEmpty(t, filed(c)[0].Transcript,
		"the entry must reach the diary with the conversation it came from")

	c.book.AssertCalled(t, "File", mock.Anything, mock.Anything, saidAt)
}

func TestSession_CarriesAudioBothWaysAndDropsWhatWasInterrupted(t *testing.T) {
	c := begin(t)

	c.says(NewSpeakCommand([]byte{9}))
	assert.Equal(t, [][]byte{{9}}, spoken(t, c, 1))

	c.hears(conversation.Event{Audio: []byte{7}})
	c.hears(conversation.Event{Interrupted: true})
	c.hears(said(conversation.Companion, "Still here."))
	await[TurnGrew](t, c)

	c.leaves()
	c.over(t)

	c.user.AssertCalled(t, "Play", mock.Anything, []byte{7})
	c.user.AssertNumberOfCalls(t, "Flush", 1)
}

func TestSession_FinishingWaitsForWhatTheDayOwes(t *testing.T) {
	c := begin(t)

	c.says(NewCommand(CmdFinish))
	c.says(NewCommand(CmdFinish))
	c.leaves()
	c.over(t)

	asking := told[Asking](c)
	require.Len(t, asking, 2, "every press is answered")
	assert.Equal(t, []string{"rating", "emotion"}, asking[0].Missing)

	c.companion.AssertNumberOfCalls(t, "AskFor", 1)
	c.companion.AssertCalled(t, "AskFor", mock.Anything, []string{"rating", "emotion"})
}

func TestSession_ANewAnswerLetsTheCompanionBeSentAgain(t *testing.T) {
	c := begin(t)

	c.says(NewCommand(CmdFinish))
	await[Asking](t, c)

	c.hears(calls(rating(7)))
	await[DraftChanged](t, c)

	c.says(NewCommand(CmdFinish))
	c.leaves()
	c.over(t)

	c.companion.AssertNumberOfCalls(t, "AskFor", 2)
	c.companion.AssertCalled(t, "AskFor", mock.Anything, []string{"emotion"})
}

func TestSession_TheCompanionCannotEndAnIncompleteDay(t *testing.T) {
	c := begin(t)

	c.hears(said(conversation.User, "I'm off, good night."))
	c.hears(conversation.Event{Calls: []conversation.Call{goodbye}, TurnComplete: true})
	refused := refusals(settled(t, c, 1))

	c.leaves()
	c.over(t)

	require.Len(t, refused, 1, "a goodbye over an unfinished day is refused")
	assert.Equal(t, conversation.CallFinish, refused[0].call.Kind)
	assert.Contains(t, refused[0].refusal, "rating")
	assert.NotContains(t, refused[0].refusal, "emotion",
		"a day owing both is asked for one of them; the other comes after it is answered")

	assert.Empty(t, told[Concluded](c), "the conversation is still going")
	assert.NotEmpty(t, told[Asking](c), "and the person is told why")
}

func TestSession_TheCompanionMayFinishSayingGoodnight(t *testing.T) {
	c := begin(t)

	aDayWorthFiling(c)
	c.hears(calls(goodbye))
	c.hears(conversation.Event{Audio: []byte{1}, OutputTranscript: "Sleep well."})
	c.hears(conversation.Event{TurnComplete: true})
	await[Concluded](t, c)

	c.says(NewCommand(CmdDiscard))
	c.over(t)

	c.user.AssertCalled(t, "Play", mock.Anything, []byte{1})
	require.Len(t, told[Concluded](c), 1, "and only then did it end")
}

func TestSession_ARatingOffTheScaleIsRefused(t *testing.T) {
	c := begin(t)

	c.hears(calls(rating(47)))
	refused := refusals(settled(t, c, 1))

	c.leaves()
	c.over(t)

	require.Len(t, refused, 1)
	assert.Contains(t, refused[0].refusal, "1 to 10")
	assert.Empty(t, told[DraftChanged](c), "a refused note changes nothing, so it announces nothing")
}

func TestSession_AFourthGoodThingIsRefusedAndTheFirstThreeSurvive(t *testing.T) {
	c := begin(t)

	c.hears(calls(wentWell("a run"), wentWell("a nap"), wentWell("a walk"), wentWell("a swim")))
	refused := refusals(settled(t, c, 4))

	c.leaves()
	c.over(t)

	require.Len(t, refused, 1, "only the fourth is refused")
	assert.Contains(t, refused[0].refusal, "a swim")

	changes := told[DraftChanged](c)
	require.Len(t, changes, 3, "one announcement per accepted note, none for the refusal")
	assert.Equal(t, []string{"a run", "a nap", "a walk"}, changes[2].Day.WentWell)
	assert.NoError(t, changes[2].Day.Validate(),
		"what is recorded must never break the rules a day is filed under")
}

func TestSession_TomorrowHoldsAsManyThingsAsTheySay(t *testing.T) {
	c := begin(t)

	c.hears(calls(todo("errands"), todo("call the bank"), todo("book a table"), todo("water the plants")))
	answers := settled(t, c, 4)

	c.leaves()
	c.over(t)

	assert.Empty(t, refusals(answers), "the cap belongs to what a day was, not to what is still ahead")

	changes := told[DraftChanged](c)
	require.Len(t, changes, 4, "every one of them is written down")
	assert.Equal(t, []string{"errands", "call the bank", "book a table", "water the plants"},
		changes[3].Day.Todos)
	assert.NoError(t, changes[3].Day.Validate())
}

func TestSession_EveryReadingCarriesWhetherTheDayIsComplete(t *testing.T) {
	c := begin(t)

	c.hears(calls(rating(7)))
	c.hears(calls(emotion("content")))
	settled(t, c, 2)

	c.leaves()
	c.over(t)

	changes := told[DraftChanged](c)
	require.Len(t, changes, 2)
	assert.False(t, changes[0].Complete, "a rating alone does not finish a day")
	assert.True(t, changes[1].Complete, "both answers do")
}

func TestSession_ARatingReplacesAndAListGrows(t *testing.T) {
	c := begin(t)

	c.hears(calls(rating(7), emotion("fine"), wentWell("a run")))
	c.hears(calls(rating(6), wentWell("a nap")))
	settled(t, c, 5)

	c.leaves()
	c.over(t)

	changes := told[DraftChanged](c)
	day := changes[len(changes)-1].Day

	assert.Equal(t, 6, day.Rating, "the newer number wins")
	assert.Equal(t, "fine", day.Emotion, "an untouched answer stays")
	assert.Equal(t, []string{"a run", "a nap"}, day.WentWell)
}

func TestSession_SalvagesWhatIsLeftWhenTheCompanionIsLost(t *testing.T) {
	c := begin(t)

	c.hears(said(conversation.User, "A good day."))
	c.hears(calls(rating(8), emotion("glad")))
	c.loses()
	await[Concluded](t, c)

	c.says(NewCommand(CmdDiscard))
	c.over(t)

	concluded := told[Concluded](c)
	require.Len(t, concluded, 1)
	assert.Equal(t, CaveatSalvaged, concluded[0].Caveat)
	assert.Equal(t, 8, concluded[0].Draft.Rating, "what was written down before the drop survives")
}

func TestSession_AbandoningLeavesADraftMarkedIncomplete(t *testing.T) {
	c := begin(t)

	c.hears(said(conversation.User, "Not now."))
	await[TurnGrew](t, c)

	c.says(NewCommand(CmdAbandon))
	c.says(NewCommand(CmdDiscard))
	c.over(t)

	concluded := told[Concluded](c)
	require.Len(t, concluded, 1)
	assert.Equal(t, CaveatIncomplete, concluded[0].Caveat)
	c.book.AssertNumberOfCalls(t, "File", 0)
}

func TestSession_ABrowserThatLeftGetsNoReview(t *testing.T) {
	c := begin(t)

	c.hears(said(conversation.User, "A good day."))
	c.hears(calls(rating(8), emotion("glad")))
	settled(t, c, 2)

	c.leaves()
	c.over(t)

	assert.Empty(t, told[Concluded](c), "there is nobody to show it to")
	c.book.AssertNumberOfCalls(t, "File", 0)
}

func TestSession_NothingSaidIsAnErrorRatherThanADraft(t *testing.T) {
	c := begin(t)

	c.says(NewCommand(CmdAbandon))
	c.leaves()
	c.over(t)

	failed := told[Failed](c)
	require.Len(t, failed, 1)
	assert.Equal(t, ReasonNothingSaid, failed[0].Reason)
	assert.Empty(t, told[Concluded](c))
}

func TestSession_SavingRefusesADayThatIsNotAnEntry(t *testing.T) {
	c := begin(t)

	c.hears(said(conversation.User, "A quiet day."))
	await[TurnGrew](t, c)

	c.says(NewCommand(CmdAbandon))
	c.says(NewSaveCommand(time.Now()))
	c.leaves()
	c.over(t)

	failed := told[Failed](c)
	require.Len(t, failed, 1)
	assert.Equal(t, ReasonIncomplete, failed[0].Reason)
	c.book.AssertNumberOfCalls(t, "File", 0)
}

func TestSession_AFailedFilingIsReportedAsOne(t *testing.T) {
	c := begin(t)
	c.book.EXPECT().File(mock.Anything, mock.Anything, mock.Anything).
		Return("", errors.New("notion is down"))

	aDayWorthFiling(c)
	c.hears(conversation.Event{Calls: []conversation.Call{goodbye}, TurnComplete: true})
	await[Concluded](t, c)

	c.says(NewSaveCommand(time.Now()))
	c.over(t)

	failed := told[Failed](c)
	require.Len(t, failed, 1)
	assert.Equal(t, ReasonSaveFailed, failed[0].Reason)
	assert.Empty(t, told[Filed](c))
	c.user.AssertNotCalled(t, "Close", mock.Anything)
}

func TestSession_DiscardingFilesNothing(t *testing.T) {
	c := begin(t)

	aDayWorthFiling(c)
	c.hears(conversation.Event{Calls: []conversation.Call{goodbye}, TurnComplete: true})
	await[Concluded](t, c)

	c.says(NewCommand(CmdDiscard))
	c.over(t)

	require.Len(t, told[Discarded](c), 1)
	c.book.AssertNumberOfCalls(t, "File", 0)
	c.user.AssertCalled(t, "Close", "discarded")
}

func TestSession_WritesBothSidesDownAsTurns(t *testing.T) {
	c := begin(t)

	c.hears(said(conversation.User, "It rained all day."))
	c.hears(said(conversation.Companion, "That sounds grey."))
	await[TurnGrew](t, c)

	c.leaves()
	c.over(t)

	turns := told[TurnGrew](c)
	require.Len(t, turns, 2)
	assert.Equal(t, conversation.User, turns[0].Turn.Role)
	assert.Equal(t, "It rained all day.", turns[0].Turn.Text)
	assert.Equal(t, conversation.Companion, turns[1].Turn.Role)
	assert.Equal(t, 1, turns[1].Turn.Index)
}

func TestSession_GivesUpWhenTheOpeningCannotBeDelivered(t *testing.T) {
	user := newMockUser(t)
	user.EXPECT().Emit(mock.Anything, Opened{}).Return(errors.New("socket gone"))
	companion := newMockCompanion(t)

	New(user, companion, newMockDiary(t)).Run(t.Context())

	companion.AssertNotCalled(t, "Greet", mock.Anything)
}

func TestSession_CountsTheAudioTheCompanionNeverHeard(t *testing.T) {
	c := beginWith(t, func(c *talk) { c.deaf = errors.New("socket rebuilding") })

	c.says(NewSpeakCommand([]byte{9}))
	spoken(t, c, 1)

	c.leaves()
	c.over(t)

	assert.Equal(t, 1, c.unheard,
		"a chunk the companion never got is dropped on purpose, not silently")
}

func TestSession_CountsWhatTheBrowserNeverSaw(t *testing.T) {
	c := begin(t)
	await[Opened](t, c)
	c.blind.Store(true)

	c.hears(said(conversation.User, "A good day."))
	await[TurnGrew](t, c)

	c.leaves()
	c.over(t)

	assert.Positive(t, c.unseen, "an event the browser never got is dropped on purpose, not silently")
}

func TestSession_EndingATurnHandsTheFloorOverWithoutWaitingForSilence(t *testing.T) {
	c := begin(t)

	c.says(NewCommand(CmdEndTurn))
	c.leaves()
	c.over(t)

	c.companion.AssertNumberOfCalls(t, "EndTurn", 1)
}

func TestSession_WritesDownWhatWentBadlyToo(t *testing.T) {
	c := begin(t)

	c.hears(calls(wentBadly("overslept"), wentBadly("late to an appointment")))
	settled(t, c, 2)

	c.leaves()
	c.over(t)

	changes := told[DraftChanged](c)
	require.Len(t, changes, 2)
	assert.Equal(t, []string{"overslept", "late to an appointment"}, changes[1].Day.WentBadly)
}

func TestSession_RefusesANoteWithNothingInIt(t *testing.T) {
	c := begin(t)

	c.hears(calls(emotion("  "), wentWell(""), wentBadly("\t"), todo("")))
	refused := refusals(settled(t, c, 4))

	c.leaves()
	c.over(t)

	assert.Len(t, refused, 4, "a call carrying no words is not something the day can hold")
	assert.Empty(t, told[DraftChanged](c), "and nothing changed, so nothing was announced")
}
