package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/djx30103/vpsub/internal/config"
)

var timeoutTestModeOnce atomic.Bool

// setupTimeoutTestMode 用于在超时中间件测试中只初始化一次 Gin 测试模式，避免并发修改全局状态。
// 参数含义：无。
// 返回值：无。
func setupTimeoutTestMode() {
	if timeoutTestModeOnce.CompareAndSwap(false, true) {
		gin.SetMode(gin.TestMode)
	}
}

// TestTimeoutMiddleware_CancelsRequestContext 用于验证请求超时后，下游可以从请求上下文感知取消信号。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestTimeoutMiddleware_CancelsRequestContext(t *testing.T) {
	setupTimeoutTestMode()

	engine := gin.New()
	var canceled atomic.Bool

	engine.Use(TimeoutMiddleware(config.ServerConfig{Timeout: 30 * time.Millisecond}))
	engine.GET("/timeout", func(c *gin.Context) {
		select {
		case <-c.Request.Context().Done():
			canceled.Store(true)
			return
		case <-time.After(200 * time.Millisecond):
			c.String(http.StatusOK, "late")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/timeout", nil)
	resp := httptest.NewRecorder()

	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusRequestTimeout {
		t.Fatalf("expected status 408, got %d", resp.Code)
	}

	deadline := time.Now().Add(time.Second)
	for !canceled.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if !canceled.Load() {
		t.Fatalf("expected request context to be canceled after timeout")
	}
}

// TestTimeoutMiddleware_ReturnsTimeoutBody 用于验证请求超时时返回固定的超时响应体。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestTimeoutMiddleware_ReturnsTimeoutBody(t *testing.T) {
	setupTimeoutTestMode()

	engine := gin.New()
	engine.Use(TimeoutMiddleware(config.ServerConfig{Timeout: 10 * time.Millisecond}))
	engine.GET("/timeout", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/timeout", nil)
	resp := httptest.NewRecorder()

	engine.ServeHTTP(resp, req)

	body, err := io.ReadAll(resp.Result().Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if string(body) != "timeout" {
		t.Fatalf("expected body timeout, got %s", string(body))
	}
}
