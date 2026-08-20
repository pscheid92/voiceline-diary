package gemini

import (
	"regexp"
	"strings"

	"google.golang.org/genai"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
)

func heard(msg *genai.LiveServerMessage) (conversation.Event, bool) {
	var ev conversation.Event

	if msg.ToolCall != nil {
		for _, fc := range msg.ToolCall.FunctionCalls {
			if call, ok := asCall(fc); ok {
				ev.Calls = append(ev.Calls, call)
			}
		}
	}

	sc := msg.ServerContent
	if sc == nil {
		return ev, !ev.Empty()
	}

	ev.TurnComplete = sc.TurnComplete
	ev.Interrupted = sc.Interrupted

	if sc.InputTranscription != nil {
		ev.InputTranscript = spoken(sc.InputTranscription.Text)
	}
	if sc.OutputTranscription != nil {
		ev.OutputTranscript = spoken(sc.OutputTranscription.Text)
	}
	if sc.ModelTurn != nil {
		for _, p := range sc.ModelTurn.Parts {
			if p.InlineData != nil {
				ev.Audio = append(ev.Audio, p.InlineData.Data...)
			}
		}
	}

	return ev, !ev.Empty()
}

func asCall(fc *genai.FunctionCall) (conversation.Call, bool) {
	call := conversation.Call{ID: fc.ID}

	switch fc.Name {
	case finishTool:
		call.Kind = conversation.CallFinish

	case ratingTool:
		n, _ := fc.Args["rating"].(float64)
		call.Kind, call.Rating = conversation.CallRating, int(n)

	case emotionTool:
		call.Kind, call.Text = conversation.CallEmotion, text(fc.Args["emotion"])

	case wentWellTool:
		call.Kind, call.Text = conversation.CallWentWell, text(fc.Args["item"])

	case wentBadlyTool:
		call.Kind, call.Text = conversation.CallWentBadly, text(fc.Args["item"])

	case todoTool:
		call.Kind, call.Text = conversation.CallTodo, text(fc.Args["item"])

	default:
		return conversation.Call{}, false
	}

	return call, true
}

func toolFor(kind conversation.CallKind) string {
	switch kind {
	case conversation.CallFinish:
		return finishTool
	case conversation.CallRating:
		return ratingTool
	case conversation.CallEmotion:
		return emotionTool
	case conversation.CallWentWell:
		return wentWellTool
	case conversation.CallWentBadly:
		return wentBadlyTool
	case conversation.CallTodo:
		return todoTool
	}
	return ""
}

func text(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

var annotationRE = regexp.MustCompile(`(?i)<[a-z_ ]+>`)

func spoken(s string) string {
	cleaned := annotationRE.ReplaceAllString(s, "")
	if strings.TrimSpace(cleaned) == "" {
		return ""
	}
	return cleaned
}
