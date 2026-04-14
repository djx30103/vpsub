package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/djx30103/vpsub/pkg/log"
)

// TestLogger_OnlyRecordsUserAgent 用于验证请求日志只记录 User-Agent，不再输出完整请求头。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestLogger_OnlyRecordsUserAgent(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	core, recorded := observer.New(zap.InfoLevel)
	logger := &log.Logger{Logger: zap.New(core)}

	engine := gin.New()
	engine.Use(Logger(logger))
	engine.GET("/subscriptions/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/subscriptions/test", nil)
	req.Header.Set("User-Agent", "Clash.Meta/1.0")
	req.Header.Set("Authorization", "Bearer secret-token")

	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	fields := entries[0].ContextMap()
	if _, ok := fields["header"]; ok {
		t.Fatalf("expected header field to be absent, got %#v", fields["header"])
	}

	if got := fields["user_agent"]; got != "Clash.Meta/1.0" {
		t.Fatalf("expected user_agent Clash.Meta/1.0, got %#v", got)
	}
}

// TestLogger_KeepsRequestBodyReadable 用于验证日志中间件不会吞掉请求体，后续业务处理仍可读取原始 body。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestLogger_KeepsRequestBodyReadable(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	core, recorded := observer.New(zap.InfoLevel)
	logger := &log.Logger{Logger: zap.New(core)}

	engine := gin.New()
	engine.Use(Logger(logger))
	engine.POST("/subscriptions/test", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("failed to read request body in handler: %v", err)
		}
		c.String(http.StatusOK, string(body))
	})

	req := httptest.NewRequest(http.MethodPost, "/subscriptions/test", strings.NewReader("payload"))
	req.Header.Set("User-Agent", "Clash.Meta/1.0")

	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}

	if got := resp.Body.String(); got != "payload" {
		t.Fatalf("expected body payload, got %q", got)
	}

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	fields := entries[0].ContextMap()
	switch got := fields["bytes_in"].(type) {
	case int:
		if got != 7 {
			t.Fatalf("expected bytes_in 7, got %#v", got)
		}
	case int64:
		if got != 7 {
			t.Fatalf("expected bytes_in 7, got %#v", got)
		}
	default:
		t.Fatalf("expected bytes_in to be integer, got %#v", fields["bytes_in"])
	}
}
