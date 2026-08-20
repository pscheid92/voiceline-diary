package session

import (
	"fmt"
	"strings"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
)

type notes struct {
	Turns []conversation.Turn
}

func (n *notes) Append(role conversation.Role, fragment string) (conversation.Turn, bool) {
	if strings.TrimSpace(fragment) == "" {
		return conversation.Turn{}, false
	}

	turn := n.turnFor(role)
	turn.Text = strings.TrimSpace(turn.Text + fragment)

	return *turn, true
}

func (n *notes) CloseTurn() {
	if last := n.getLastTurn(); last != nil {
		last.Closed = true
	}
}

func (n *notes) Transcript() string {
	var lines strings.Builder

	for _, turn := range n.Turns {
		fmt.Fprintf(&lines, "%s: %s\n", speaker(turn.Role), turn.Text)
	}

	return lines.String()
}

func (n *notes) turnFor(role conversation.Role) *conversation.Turn {
	if last := n.getLastTurn(); last != nil && last.Role == role && !last.Closed {
		return last
	}

	newTurn := conversation.NewTurn(len(n.Turns), role)
	n.Turns = append(n.Turns, newTurn)

	return n.getLastTurn()
}

func (n *notes) getLastTurn() *conversation.Turn {
	length := len(n.Turns)

	if length == 0 {
		return nil
	}

	return &n.Turns[length-1]
}

func speaker(role conversation.Role) string {
	if role == conversation.User {
		return "Me"
	}
	return "Companion"
}
