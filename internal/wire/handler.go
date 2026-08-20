package wire

import (
	"net/http"

	"github.com/coder/websocket"
	"github.com/rs/zerolog"

	"github.com/pscheid92/voiceline-diary/internal/session"
)

type Handler struct {
	diary session.Diary
	dial  session.Dial
}

func NewHandler(book session.Diary, dial session.Dial) *Handler {
	return &Handler{diary: book, dial: dial}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	ws, err := websocket.Accept(w, req, nil)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("wire: websocket accept failed")
		return
	}
	defer func() { _ = ws.CloseNow() }()

	conn := New(ws)
	now := instant(req.URL.Query().Get("now"))

	companion, err := h.dial(ctx, now)
	if err != nil {
		zerolog.Ctx(ctx).
			Error().
			Err(err).
			Msg("wire: companion dial failed")

		ev := session.Failed{Reason: session.ReasonDialFailed}
		_ = conn.Emit(ctx, ev)
		return
	}
	defer companion.Close()

	session.
		New(conn, companion, h.diary).
		Run(ctx)
}
