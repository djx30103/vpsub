package racknerd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestParseServiceInfoResponse_QuotedNumbers 用于验证带引号的数值字段也能被正确解析。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestParseServiceInfoResponse_QuotedNumbers(t *testing.T) {
	t.Parallel()

	resp := "\"3221225472000\",\"3212876925\",4291754419075,0successracknerd-58c7b7xx.xx.xx.xx"

	info, err := parseServiceInfoResponse(resp)
	if err != nil {
		t.Fatalf("parseServiceInfoResponse returned error: %v", err)
	}

	if info.Total != 3221225472000 {
		t.Fatalf("unexpected total: %d", info.Total)
	}

	if info.Upload != 1606438462 {
		t.Fatalf("unexpected upload: %d", info.Upload)
	}

	if info.Download != 1606438462 {
		t.Fatalf("unexpected download: %d", info.Download)
	}
}

// TestClientGetServiceInfo_QueriesRackNerdAPI 用于验证查询服务信息时会携带正确参数，并能解析响应结果。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestClientGetServiceInfo_QueriesRackNerdAPI(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotKey string
	var gotHash string
	var gotAction string
	var gotBW string
	var gotUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 这里记录请求关键字段，用于验证客户端是否按 RackNerd 约定拼装请求。
		gotPath = r.URL.Path
		gotKey = r.URL.Query().Get("key")
		gotHash = r.URL.Query().Get("hash")
		gotAction = r.URL.Query().Get("action")
		gotBW = r.URL.Query().Get("bw")
		gotUserAgent = r.Header.Get("User-Agent")

		_, _ = w.Write([]byte("\"100\",\"40\""))
	}))
	defer server.Close()

	client := &Client{
		apiKey:  "test-key",
		apiHash: "test-hash",
		baseURL: server.URL,
		httpCli: server.Client(),
	}

	info, err := client.GetServiceInfo(context.Background())
	if err != nil {
		t.Fatalf("GetServiceInfo returned error: %v", err)
	}

	if gotPath != "/api/client/command.php" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotKey != "test-key" {
		t.Fatalf("unexpected key: %s", gotKey)
	}
	if gotHash != "test-hash" {
		t.Fatalf("unexpected hash: %s", gotHash)
	}
	if gotAction != "info" {
		t.Fatalf("unexpected action: %s", gotAction)
	}
	if gotBW != "true" {
		t.Fatalf("unexpected bw: %s", gotBW)
	}
	if gotUserAgent == "" {
		t.Fatal("expected user agent to be set")
	}
	if info.Total != 100 {
		t.Fatalf("unexpected total: %d", info.Total)
	}
	if info.Upload != 20 {
		t.Fatalf("unexpected upload: %d", info.Upload)
	}
	if info.Download != 20 {
		t.Fatalf("unexpected download: %d", info.Download)
	}
	if info.Expire <= 0 {
		t.Fatalf("unexpected expire: %d", info.Expire)
	}
}

// TestNextResetUnix_BeforeResetDay 验证当前日期在重置日之前时，返回本月重置日的时间戳。
func TestNextResetUnix_BeforeResetDay(t *testing.T) {
	t.Parallel()

	// 构造一个"今天是 15 日，重置日是 20 日"的场景
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.Local)
	resetDay := 20

	got := nextResetUnix(now, resetDay, time.Local)
	want := time.Date(2026, 5, 20, 0, 0, 0, 0, time.Local).Unix()

	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

// TestNextResetUnix_OnResetDay 验证当前日期恰好等于重置日时，返回下月重置日的时间戳。
func TestNextResetUnix_OnResetDay(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.Local)
	resetDay := 20

	got := nextResetUnix(now, resetDay, time.Local)
	want := time.Date(2026, 6, 20, 0, 0, 0, 0, time.Local).Unix()

	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

// TestNextResetUnix_AfterResetDay 验证当前日期在重置日之后时，返回下月重置日的时间戳。
func TestNextResetUnix_AfterResetDay(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.Local)
	resetDay := 20

	got := nextResetUnix(now, resetDay, time.Local)
	want := time.Date(2026, 6, 20, 0, 0, 0, 0, time.Local).Unix()

	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

// TestNextResetUnix_YearRollover 验证 12 月时能正确跨年到次年 1 月。
func TestNextResetUnix_YearRollover(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 12, 15, 10, 0, 0, 0, time.Local)
	resetDay := 10

	got := nextResetUnix(now, resetDay, time.Local)
	want := time.Date(2027, 1, 10, 0, 0, 0, 0, time.Local).Unix()

	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

// TestNextResetUnix_UsesResetTimezone 验证重置时间必须按供应商重置时区计算，不能跟随服务本地时区。
func TestNextResetUnix_UsesResetTimezone(t *testing.T) {
	t.Parallel()

	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	losAngeles := time.FixedZone("America/Los_Angeles", -7*60*60)

	// 服务在上海时间 5 月 1 日凌晨，但此时洛杉矶仍处于 4 月 30 日白天，下一次重置应为洛杉矶 5 月 1 日零点。
	now := time.Date(2026, 5, 1, 0, 30, 0, 0, shanghai)
	resetDay := 1

	got := nextResetUnix(now, resetDay, losAngeles)
	want := time.Date(2026, 5, 1, 0, 0, 0, 0, losAngeles).Unix()

	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

// TestNextResetUnix_UsesLosAngelesDST 用于验证重置时间按洛杉矶时区计算时，会自动跟随夏令时切换。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestNextResetUnix_UsesLosAngelesDST(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}

	now := time.Date(2026, 3, 8, 1, 0, 0, 0, loc)
	got := nextResetUnix(now, 1, loc)
	want := time.Date(2026, 4, 1, 0, 0, 0, 0, loc).Unix()

	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}
