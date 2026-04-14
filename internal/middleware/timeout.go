package middleware

import (
	"net/http"

	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"

	"github.com/djx30103/vpsub/internal/config"
)

// timeoutResponse 用于返回统一的超时响应体。
// 参数含义：c 为 Gin 上下文。
// 返回值：无。
func timeoutResponse(c *gin.Context) {
	c.String(http.StatusRequestTimeout, "timeout")
}

// TimeoutMiddleware 用于为整个请求链路设置超时，并在超时时返回固定响应。
// 参数含义：conf 为服务端配置，内部使用其中的超时时间。
// 返回值：返回可注册到 Gin 的超时中间件。
func TimeoutMiddleware(conf config.ServerConfig) gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(conf.Timeout),
		timeout.WithResponse(timeoutResponse),
	)
}
