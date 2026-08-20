package router

import (
	"net/http"

	ginlogger "github.com/gin-contrib/logger"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func withRequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		l := zerolog.Ctx(ctx).
			With().
			Str("request_id", requestid.Get(c)).
			Logger()

		c.Request = c.Request.WithContext(l.WithContext(ctx))
		c.Next()
	}
}

func logRequests() gin.HandlerFunc {
	return ginlogger.SetLogger(
		ginlogger.WithLogger(func(c *gin.Context, _ zerolog.Logger) zerolog.Logger {
			ctx := c.Request.Context()
			return *zerolog.Ctx(ctx)
		}),
		ginlogger.WithSkipper(func(c *gin.Context) bool {
			isHealthCheck := c.Request.URL.Path == "/healtz"
			isOk := c.Writer.Status() == http.StatusOK
			return isHealthCheck && isOk
		}),
	)
}
