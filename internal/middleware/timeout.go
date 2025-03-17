package middleware

import (
	"net/http"

	"vpsub/pkg/config"

	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
)

func timeoutResponse(c *gin.Context) {
	c.String(http.StatusRequestTimeout, "timeout")
}

func TimeoutMiddleware(conf config.ServerConfig) gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(conf.Timeout),
		timeout.WithHandler(func(c *gin.Context) {
			c.Next()
		}),
		timeout.WithResponse(timeoutResponse),
	)
}
