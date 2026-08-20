package router

import (
	"net/http"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"

	"github.com/pscheid92/voiceline-diary/web"
)

func New(talking http.Handler, diaryURL string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()
	engine.Use(
		requestid.New(),
		withRequestLogger(),
		logRequests(),
		gin.Recovery(),
	)

	engine.GET("/healthz", liveness)

	v1 := engine.Group("/api/v1")
	{
		v1.GET("/config", clientConfig(diaryURL))
		v1.GET("/voice", gin.WrapH(talking))
	}

	engine.NoRoute(gin.WrapH(http.FileServer(http.FS(web.FS))))

	return engine
}

func liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func clientConfig(diaryURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"diary_url": diaryURL})
	}
}
