package conversation

type Role string

const (
	User      Role = "user"
	Companion Role = "companion"
)

type Turn struct {
	Index  int    `json:"index"`
	Role   Role   `json:"role"`
	Text   string `json:"text"`
	Closed bool   `json:"closed"`
}

func NewTurn(index int, role Role) Turn {
	return Turn{Index: index, Role: role}
}

type Event struct {
	Audio            []byte
	InputTranscript  string
	OutputTranscript string
	Interrupted      bool
	TurnComplete     bool
	Calls            []Call
}

func (e Event) Empty() bool {
	return e.Audio == nil &&
		e.InputTranscript == "" &&
		e.OutputTranscript == "" &&
		!e.Interrupted &&
		!e.TurnComplete &&
		len(e.Calls) == 0
}

type CallKind string

const (
	CallRating    CallKind = "rating"
	CallEmotion   CallKind = "emotion"
	CallWentWell  CallKind = "went_well"
	CallWentBadly CallKind = "went_badly"
	CallTodo      CallKind = "todo"
	CallFinish    CallKind = "finish"
)

type Call struct {
	ID     string
	Kind   CallKind
	Text   string
	Rating int
}
