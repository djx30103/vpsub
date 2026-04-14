package base

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestDoGetRequest_SetsUserAgentAndReturnsBody 用于验证公共 GET 请求会设置统一 User-Agent，并返回响应体内容。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestDoGetRequest_SetsUserAgentAndReturnsBody(t *testing.T) {
	t.Parallel()

	var gotUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 这里记录请求头，确保各供应商共用的请求行为一致。
		gotUserAgent = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	httpCli := &http.Client{Timeout: time.Second}

	body, err := DoGetRequest(context.Background(), httpCli, server.URL)
	if err != nil {
		t.Fatalf("DoGetRequest returned error: %v", err)
	}

	if string(body) != "ok" {
		t.Fatalf("unexpected body: %s", string(body))
	}
	if gotUserAgent == "" {
		t.Fatal("expected user agent to be set")
	}
}

// TestDoGetRequest_ReturnsStatusCodeError 用于验证非 200 响应会返回明确错误。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestDoGetRequest_ReturnsStatusCodeError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	httpCli := &http.Client{Timeout: time.Second}

	_, err := DoGetRequest(context.Background(), httpCli, server.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "failed to get service info, status code: 502" {
		t.Fatalf("unexpected error: %v", err)
	}
}
