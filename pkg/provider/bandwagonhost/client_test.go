package bandwagonhost

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/djx30103/vpsub/pkg/provider/base"
)

// TestGetServiceInfo_AllowsZeroUsage 用于验证流量总量有效但当前用量为 0 时仍返回正常结果。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestGetServiceInfo_AllowsZeroUsage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"monthly_data_multiplier":1073741824,"plan_monthly_data":500,"data_counter":0,"data_next_reset":1711929600}`))
	}))
	defer server.Close()

	client := New(base.APIRequestInfo{
		APIID:          "veid-1",
		APIKey:         "key-1",
		RequestTimeout: time.Second,
	})
	client.baseURL = server.URL

	info, err := client.GetServiceInfo(context.Background())
	if err != nil {
		t.Fatalf("GetServiceInfo returned error: %v", err)
	}

	if info.Total != 500*1073741824 {
		t.Fatalf("unexpected total: %d", info.Total)
	}

	if info.Upload != 0 || info.Download != 0 {
		t.Fatalf("expected zero usage, got upload=%d download=%d", info.Upload, info.Download)
	}
}
