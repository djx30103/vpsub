package middleware

import (
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"vpsub/pkg/log"
)

func Logger(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var reqBody []byte
		if c.Request.Body != nil {
			reqBody, _ = c.GetRawData()
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			if len(reqBody) == 0 {
				reqBody = nil
			}
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
			zap.Any("header", c.Request.Header),
			zap.Any("bytes_in", len(reqBody)),
			zap.Any("bytes_out", c.Writer.Size()),
			zap.Error(c.Errors.Last()),
		)
	}
}
