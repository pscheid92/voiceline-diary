package router

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCorrelation_ReachesLinesWrittenBelowTheHTTPLayer(t *testing.T) {
	logs := captureLogs(t)
	engine := newTestRouter()

	res := call(engine, http.NoBody, nil)

	id := res.Header().Get("X-Request-ID")
	require.NotEmpty(t, id, "every response must name the request it answers")
	assert.Contains(t, logs(), `"request_id":"`+id+`"`,
		"the line logged for this request must carry its id")
}

func TestCorrelation_KeepsAnIDAProxyAlreadySet(t *testing.T) {
	logs := captureLogs(t)
	engine := newTestRouter()

	res := call(engine, http.NoBody, http.Header{"X-Request-Id": []string{"from-the-proxy"}})

	assert.Equal(t, "from-the-proxy", res.Header().Get("X-Request-ID"))
	assert.Contains(t, logs(), `"request_id":"from-the-proxy"`)
}

func newTestRouter() *gin.Engine {
	return New(http.NotFoundHandler(), "https://www.notion.so/somewhere")
}

func call(engine *gin.Engine, body io.Reader, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", body)
	for k, v := range header {
		req.Header[k] = v
	}
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	return res
}

func captureLogs(t *testing.T) func() string {
	t.Helper()

	var buf bytes.Buffer
	previous := zerolog.DefaultContextLogger
	captured := zerolog.New(&buf)
	zerolog.DefaultContextLogger = &captured
	t.Cleanup(func() { zerolog.DefaultContextLogger = previous })

	return buf.String
}

func TestLiveness_AnswersWithoutTouchingAnything(t *testing.T) {
	res := httptest.NewRecorder()
	newTestRouter().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))

	assert.Equal(t, http.StatusOK, res.Code)
	assert.JSONEq(t, `{"status":"ok"}`, res.Body.String())
}
