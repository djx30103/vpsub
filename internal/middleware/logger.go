package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/djx30103/vpsub/pkg/log"
)

func Logger(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		bytesIn := 0
		if c.Request.ContentLength > 0 {
			// 这里直接使用请求头声明的长度，避免仅为日志统计而额外读取并复制整个请求体。
			bytesIn = int(c.Request.ContentLength)
		}

		start := time.Now()

		c.Next()

		latency := time.Since(start)
		if latency > time.Minute {
			latency = latency.Truncate(time.Second)
		}

		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		if query != "" {
			path = path + "?" + query
		}

		logger.WithContext(c).Info("request Log",
			zap.String("method", c.Request.Method),
			zap.Int("status", c.Writer.Status()),
			zap.String("latency", latency.String()),
			zap.String("ip", c.ClientIP()),
			zap.String("path", path),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Any("bytes_in", bytesIn),
			zap.Any("bytes_out", c.Writer.Size()),
			zap.Error(c.Errors.Last()),
		)
	}
}
