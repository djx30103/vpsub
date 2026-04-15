package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gocache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/djx30103/vpsub/internal/config"
	"github.com/djx30103/vpsub/internal/middleware"
	"github.com/djx30103/vpsub/pkg/log"
	"github.com/djx30103/vpsub/pkg/provider/base"
)

var subscribeTestModeOnce sync.Once

// setupSubscribeTestMode 用于在订阅处理测试中只初始化一次 Gin 测试模式，避免并发修改全局状态。
// 参数含义：无。
// 返回值：无。
func setupSubscribeTestMode() {
	subscribeTestModeOnce.Do(func() {
		gin.SetMode(gin.TestMode)
	})
}

// TestNormalizeRequestPath_TrimsTrailingSlashes 用于验证路径归一化时会删除末尾连续斜杠，同时保留非末尾内容不变。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestNormalizeRequestPath_TrimsTrailingSlashes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "single trailing slash",
			path: "/test.yaml/",
			want: "/test.yaml",
		},
		{
			name: "multiple trailing slashes",
			path: "/test.yaml//",
			want: "/test.yaml",
		},
		{
			name: "nested path single trailing slash",
			path: "/a/b/c/",
			want: "/a/b/c",
		},
		{
			name: "nested path multiple trailing slashes",
			path: "/a/b/c///",
			want: "/a/b/c",
		},
		{
			name: "only trailing slashes remain empty",
			path: "//",
			want: "",
		},
		{
			name: "middle slash preserved",
			path: "/group//node///",
			want: "/group//node",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeRequestPath(testCase.path)
			if got != testCase.want {
				t.Fatalf("normalizeRequestPath(%q) = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}

// TestGetProviderInfo_UsesCachedAPIWhenProviderFails 用于验证上游接口失败时会回退到最近一次成功的流量缓存。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestGetProviderInfo_UsesCachedAPIWhenProviderFails(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newTestSubscribeHandler(t)
	cachedAPI := &base.APIResponseInfo{
		Upload:   10,
		Download: 20,
		Total:    100,
		Expire:   0,
	}
	handler.cache.Set("shared-provider", cachedAPI, time.Minute)

	apiInfo := handler.getProviderInfo(context.Background(), conf)
	if apiInfo != cachedAPI {
		t.Fatalf("expected cached api info to be reused")
	}

	fileContent, err := handler.readSubscriptionFile(conf)
	if err != nil {
		t.Fatalf("readSubscriptionFile returned error: %v", err)
	}

	updated, err := appendUsageGroups(fileContent, apiInfo, conf.UsageDisplay)
	if err != nil {
		t.Fatalf("appendUsageGroups returned error: %v", err)
	}

	if got := string(updated); got == "" || !strings.Contains(got, "已用流量") {
		t.Fatalf("expected usage group to be appended, got: %s", got)
	}
}

// TestGetProviderInfo_ReusesProviderLevelAPICache 用于验证同一 provider_ref 下的不同路由会复用同一份 API 缓存。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestGetProviderInfo_ReusesProviderLevelAPICache(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newTestSubscribeHandler(t)
	conf.Path = "/second.yaml"
	conf.File = "second.yaml"

	content := []byte("proxy-groups:\n  - name: 自动选择\n    type: select\n    proxies:\n      - REJECT\n")
	if err := os.WriteFile(filepath.Join(handler.appConfig.Global.Storage.SubscriptionDir, conf.File), content, 0o600); err != nil {
		t.Fatalf("failed to write subscription file: %v", err)
	}

	cachedAPI := &base.APIResponseInfo{
		Upload:   30,
		Download: 40,
		Total:    200,
	}
	handler.cache.Set("shared-provider", cachedAPI, time.Minute)

	apiInfo := handler.getProviderInfo(context.Background(), conf)
	if apiInfo != cachedAPI {
		t.Fatalf("expected shared provider cache to be reused")
	}
}

// TestGetProviderInfo_ReturnsNilWhenProviderFailsWithoutCache 用于验证上游接口失败且无缓存时不会返回流量信息。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestGetProviderInfo_ReturnsNilWhenProviderFailsWithoutCache(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newTestSubscribeHandler(t)

	apiInfo := handler.getProviderInfo(context.Background(), conf)
	if apiInfo != nil {
		t.Fatalf("expected api info to be nil when no cache exists")
	}

	fileContent, err := handler.readSubscriptionFile(conf)
	if err != nil {
		t.Fatalf("readSubscriptionFile returned error: %v", err)
	}

	if got := string(fileContent); strings.Contains(got, "已用流量") {
		t.Fatalf("expected raw file content without usage group, got: %s", got)
	}
}

// TestWriteSubscriptionResponse_OmitsUsageHeadersWithoutAPI 用于验证没有流量信息时仅返回文件内容，不写入流量相关响应头。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestWriteSubscriptionResponse_OmitsUsageHeadersWithoutAPI(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newTestSubscribeHandler(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	handler.writeSubscriptionResponse(c, conf, []byte("mixed-port: 7890\n"), nil)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	if got := recorder.Header().Get("Subscription-Userinfo"); got != "" {
		t.Fatalf("expected Subscription-Userinfo header to be empty, got: %s", got)
	}

	if got := recorder.Header().Get("Content-Disposition"); got != "" {
		t.Fatalf("expected Content-Disposition header to be empty, got: %s", got)
	}

	if got := recorder.Body.String(); got != "mixed-port: 7890\n" {
		t.Fatalf("unexpected body: %s", got)
	}
}

// TestGet_RejectsRequestWhenUserAgentDoesNotMatch 用于验证命中路由但 UA 不匹配时会直接拒绝访问。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestGet_RejectsRequestWhenUserAgentDoesNotMatch(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newTestSubscribeHandler(t)
	conf.AccessControl = &config.AccessControlConfig{
		UserAgent: "ClashX",
	}
	handler.appConfig.PathToConfig = map[string]config.PathConfig{
		"/test.yaml": conf,
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/test.yaml", nil)
	req.Header.Set("User-Agent", "sing-box")
	c.Request = req
	c.Params = gin.Params{{Key: "path", Value: "/test.yaml"}}

	handler.Get(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}

// TestServeHTTP_EmitsInfoLogWhenPathMissing 用于验证未命中路由配置时，完整 Gin 链路仍会输出访问日志。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestServeHTTP_EmitsInfoLogWhenPathMissing(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, _ := newObservedSubscribeHandler(t, zap.InfoLevel)
	engine := newObservedSubscribeEngine(handler)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing.yaml", nil)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}

	entries := handler.recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 info log for missing path, got %d", len(entries))
	}

	fields := entries[0].ContextMap()
	if got := fields["status"]; got != int64(http.StatusNotFound) {
		t.Fatalf("expected status 404 in log, got %#v", got)
	}
	if got := fields["path"]; got != "/missing.yaml" {
		t.Fatalf("expected path /missing.yaml in log, got %#v", got)
	}
}

// TestServeHTTP_EmitsInfoLogWhenUserAgentRejected 用于验证 UA 拒绝场景在完整 Gin 链路下同样会记录访问日志。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestServeHTTP_EmitsInfoLogWhenUserAgentRejected(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newObservedSubscribeHandler(t, zap.InfoLevel)
	conf.AccessControl = &config.AccessControlConfig{
		UserAgent: "ClashX",
	}
	handler.appConfig.PathToConfig = map[string]config.PathConfig{
		"/test.yaml": conf,
	}
	engine := newObservedSubscribeEngine(handler)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test.yaml", nil)
	req.Header.Set("User-Agent", "sing-box")
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}

	entries := handler.recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 info log for rejected user agent, got %d", len(entries))
	}

	fields := entries[0].ContextMap()
	if got := fields["status"]; got != int64(http.StatusNotFound) {
		t.Fatalf("expected status 404 in log, got %#v", got)
	}
	if got := fields["path"]; got != "/test.yaml" {
		t.Fatalf("expected path /test.yaml in log, got %#v", got)
	}
	if got := fields["user_agent"]; got != "sing-box" {
		t.Fatalf("expected user_agent sing-box in log, got %#v", got)
	}
}

// TestReadSubscriptionFile_UsesCachedContentUntilFileChanges 用于验证订阅文件会按 path 缓存，并在文件变更后自动刷新。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestReadSubscriptionFile_UsesCachedContentUntilFileChanges(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newTestSubscribeHandler(t)

	first, err := handler.readSubscriptionFile(conf)
	if err != nil {
		t.Fatalf("readSubscriptionFile returned error: %v", err)
	}

	if got := string(first); !strings.Contains(got, "节点选择") {
		t.Fatalf("unexpected initial file content: %s", got)
	}

	updatedContent := []byte("proxy-groups:\n  - name: 已更新节点\n    type: select\n    proxies:\n      - REJECT\n")
	filePath := filepath.Join(handler.appConfig.Global.Storage.SubscriptionDir, conf.File)
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filePath, updatedContent, 0o600); err != nil {
		t.Fatalf("failed to update subscription file: %v", err)
	}

	second, err := handler.readSubscriptionFile(conf)
	if err != nil {
		t.Fatalf("readSubscriptionFile returned error after update: %v", err)
	}

	if got := string(second); !strings.Contains(got, "已更新节点") {
		t.Fatalf("expected updated file content after file change, got: %s", got)
	}
}

// TestReadSubscriptionFile_UsesConfiguredRelativeFile 用于验证订阅文件读取会基于 route.file，而不是对外 path 推导磁盘路径。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestReadSubscriptionFile_UsesConfiguredRelativeFile(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newTestSubscribeHandler(t)
	conf.Path = "/masked-route"
	conf.File = "nested/b.yaml"

	filePath := filepath.Join(handler.appConfig.Global.Storage.SubscriptionDir, conf.File)
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create subscription directory: %v", err)
	}

	content := []byte("proxy-groups:\n  - name: 独立文件\n    type: select\n    proxies:\n      - REJECT\n")
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		t.Fatalf("failed to write configured subscription file: %v", err)
	}

	fileContent, err := handler.readSubscriptionFile(conf)
	if err != nil {
		t.Fatalf("readSubscriptionFile returned error: %v", err)
	}

	if got := string(fileContent); !strings.Contains(got, "独立文件") {
		t.Fatalf("expected content from configured file, got: %s", got)
	}
}

// TestAppendUsageGroups_ExecutesTemplateSyntax 用于验证流量展示文案在运行时会按 Go 模板语义渲染。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_ExecutesTemplateSyntax(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newTestSubscribeHandler(t)
	fileContent, err := handler.readSubscriptionFile(conf)
	if err != nil {
		t.Fatalf("readSubscriptionFile returned error: %v", err)
	}

	conf.UsageDisplay.TrafficFormat = `{{printf "⛽ 已用流量 %s / %s" .used .total}}`
	conf.UsageDisplay.ResetTimeFormat = `{{if .year}}📅 重置日期 {{.year}}-{{.month}}-{{.day}}{{end}}`

	apiInfo := &base.APIResponseInfo{
		Upload:   0,
		Download: 2 * 1024 * 1024 * 1024,
		Total:    5 * 1024 * 1024 * 1024,
		Expire:   time.Date(2026, 4, 13, 8, 30, 0, 0, time.Local).Unix(),
	}

	updated, err := appendUsageGroups(fileContent, apiInfo, conf.UsageDisplay)
	if err != nil {
		t.Fatalf("appendUsageGroups returned error: %v", err)
	}

	got := string(updated)
	if !strings.Contains(got, "⛽ 已用流量 2G / 5G") {
		t.Fatalf("expected traffic template output, got: %s", got)
	}

	if !strings.Contains(got, "📅 重置日期 2026-04-13") {
		t.Fatalf("expected expire template output, got: %s", got)
	}
}

// TestAppendUsageGroups_PreservesCommentsAndAnchors 用于验证追加流量展示分组时不会破坏原 YAML 中的注释与 anchor/alias 结构。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_PreservesCommentsAndAnchors(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	fileContent := []byte(`proxy-groups:
  - name: &main 节点选择
    type: select
    proxies:
      - *main
# 用户注释
rules:
  - MATCH,DIRECT
`)

	apiInfo := &base.APIResponseInfo{
		Upload:   0,
		Download: 2 * 1024 * 1024 * 1024,
		Total:    5 * 1024 * 1024 * 1024,
	}

	updated, err := appendUsageGroups(fileContent, apiInfo, newTestUsageDisplayConfig())
	if err != nil {
		t.Fatalf("appendUsageGroups returned error: %v", err)
	}

	got := string(updated)
	if !strings.Contains(got, "# 用户注释") {
		t.Fatalf("expected comment to be preserved, got: %s", got)
	}

	if !strings.Contains(got, "&main") {
		t.Fatalf("expected anchor to be preserved, got: %s", got)
	}

	if !strings.Contains(got, "*main") {
		t.Fatalf("expected alias to be preserved, got: %s", got)
	}
}

// TestAppendUsageGroups_AppendsToTailWhenPrependDisabled 用于验证关闭 prepend 时展示分组会追加到原有 proxy-groups 末尾。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_AppendsToTailWhenPrependDisabled(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	fileContent := []byte(`proxy-groups:
  - name: 主分组
    type: select
    proxies:
      - REJECT
`)

	usageDisplay := newTestUsageDisplayConfig()
	usageDisplay.Prepend = false

	apiInfo := &base.APIResponseInfo{
		Upload:   0,
		Download: 2 * 1024 * 1024 * 1024,
		Total:    5 * 1024 * 1024 * 1024,
	}

	updated, err := appendUsageGroups(fileContent, apiInfo, usageDisplay)
	if err != nil {
		t.Fatalf("appendUsageGroups returned error: %v", err)
	}

	got := string(updated)
	originIndex := strings.Index(got, "name: 主分组")
	usageIndex := strings.Index(got, "已用流量 2G / 5G")
	if originIndex == -1 || usageIndex == -1 {
		t.Fatalf("expected both original and usage groups, got: %s", got)
	}

	if usageIndex < originIndex {
		t.Fatalf("expected usage group appended after original group, got: %s", got)
	}
}

// TestAppendUsageGroups_PrependsToHeadWhenPrependEnabled 用于验证开启 prepend 时展示分组会插入到原有 proxy-groups 前面。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_PrependsToHeadWhenPrependEnabled(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	fileContent := []byte(`proxy-groups:
  - name: 原分组
    type: select
    proxies:
      - REJECT
`)

	usageDisplay := newTestUsageDisplayConfig()
	usageDisplay.Prepend = true

	apiInfo := &base.APIResponseInfo{
		Upload:   0,
		Download: 2 * 1024 * 1024 * 1024,
		Total:    5 * 1024 * 1024 * 1024,
	}

	updated, err := appendUsageGroups(fileContent, apiInfo, usageDisplay)
	if err != nil {
		t.Fatalf("appendUsageGroups returned error: %v", err)
	}

	got := string(updated)
	originIndex := strings.Index(got, "name: 原分组")
	usageIndex := strings.Index(got, "已用流量 2G / 5G")
	if originIndex == -1 || usageIndex == -1 {
		t.Fatalf("expected both original and usage groups, got: %s", got)
	}

	if usageIndex > originIndex {
		t.Fatalf("expected usage group prepended before original group, got: %s", got)
	}
}

// TestAppendUsageGroups_ReturnsErrorWhenYAMLInvalid 用于验证原始内容不是合法 YAML 时会直接返回解析错误。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_ReturnsErrorWhenYAMLInvalid(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	_, err := appendUsageGroups([]byte("proxy-groups: ["), &base.APIResponseInfo{Total: 1}, newTestUsageDisplayConfig())
	if err == nil {
		t.Fatalf("expected appendUsageGroups to fail on invalid yaml")
	}
}

// TestAppendUsageGroups_ReturnsErrorWhenProxyGroupsMissing 用于验证缺少 proxy-groups 时会返回明确错误，避免静默改写其他字段。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_ReturnsErrorWhenProxyGroupsMissing(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	fileContent := []byte("rules:\n  - MATCH,DIRECT\n")
	_, err := appendUsageGroups(fileContent, &base.APIResponseInfo{Total: 1}, newTestUsageDisplayConfig())
	if err == nil || !strings.Contains(err.Error(), "no proxy-groups found in config") {
		t.Fatalf("expected missing proxy-groups error, got: %v", err)
	}
}

// TestAppendUsageGroups_ReturnsErrorWhenProxyGroupsIsNotSequence 用于验证 proxy-groups 结构非法时会直接报错。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_ReturnsErrorWhenProxyGroupsIsNotSequence(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	fileContent := []byte("proxy-groups:\n  name: 非法结构\n")
	_, err := appendUsageGroups(fileContent, &base.APIResponseInfo{Total: 1}, newTestUsageDisplayConfig())
	if err == nil || !strings.Contains(err.Error(), "proxy-groups must be sequence") {
		t.Fatalf("expected proxy-groups sequence error, got: %v", err)
	}
}

// TestAppendUsageGroups_AppendsGroupsWhenUsageDataIsZero 用于验证流量和到期时间都为零时也会追加展示分组。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_AppendsGroupsWhenUsageDataIsZero(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	fileContent := []byte(`proxy-groups:
  - name: 原分组
    type: select
    proxies:
      - REJECT
`)

	updated, err := appendUsageGroups(fileContent, &base.APIResponseInfo{}, newTestUsageDisplayConfig())
	if err != nil {
		t.Fatalf("appendUsageGroups returned error: %v", err)
	}

	got := string(updated)
	if !strings.Contains(got, "已用流量 0G / 0G") {
		t.Fatalf("expected zero usage group, got: %s", got)
	}

	if !strings.Contains(got, "重置日期 1970-01-01") {
		t.Fatalf("expected zero expire group, got: %s", got)
	}
}

// TestGet_AcceptsRequestPathWithMultipleTrailingSlashes 用于验证请求路径带连续尾斜杠时仍能命中已配置路由。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestGet_AcceptsRequestPathWithMultipleTrailingSlashes(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	handler, conf := newTestSubscribeHandler(t)
	handler.appConfig.PathToConfig = map[string]config.PathConfig{
		"/test.yaml": conf,
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/test.yaml//", nil)
	c.Params = gin.Params{{Key: "path", Value: "/test.yaml//"}}

	handler.Get(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

// TestAppendUsageGroups_PreservesTopLevelFieldOrder 用于验证追加展示分组后，已有顶层字段顺序仍保持不变。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_PreservesTopLevelFieldOrder(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	fileContent := []byte(`mixed-port: 7890
proxy-groups:
  - name: 原分组
    type: select
    proxies:
      - REJECT
rules:
  - MATCH,DIRECT
`)

	updated, err := appendUsageGroups(fileContent, &base.APIResponseInfo{Total: 1, Download: 1}, newTestUsageDisplayConfig())
	if err != nil {
		t.Fatalf("appendUsageGroups returned error: %v", err)
	}

	got := string(updated)
	mixedPortIndex := strings.Index(got, "mixed-port: 7890")
	proxyGroupsIndex := strings.Index(got, "proxy-groups:")
	rulesIndex := strings.Index(got, "rules:")
	if mixedPortIndex == -1 || proxyGroupsIndex == -1 || rulesIndex == -1 {
		t.Fatalf("expected top level fields to exist, got: %s", got)
	}

	if mixedPortIndex >= proxyGroupsIndex || proxyGroupsIndex >= rulesIndex {
		t.Fatalf("expected top level field order preserved, got: %s", got)
	}
}

// TestAppendUsageGroups_PreservesInlineAndFootComments 用于验证追加展示分组后，行尾注释和脚注注释仍能保留。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_PreservesInlineAndFootComments(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	fileContent := []byte(`proxy-groups: # 顶层行尾注释
  - name: 原分组 # 分组行尾注释
    type: select
    proxies:
      - REJECT
    # 分组脚注注释
rules:
  - MATCH,DIRECT # 规则行尾注释
`)

	updated, err := appendUsageGroups(fileContent, &base.APIResponseInfo{Total: 1, Download: 1}, newTestUsageDisplayConfig())
	if err != nil {
		t.Fatalf("appendUsageGroups returned error: %v", err)
	}

	got := string(updated)
	if !strings.Contains(got, "# 顶层行尾注释") {
		t.Fatalf("expected top-level inline comment preserved, got: %s", got)
	}
	if !strings.Contains(got, "# 分组行尾注释") {
		t.Fatalf("expected group inline comment preserved, got: %s", got)
	}
	if !strings.Contains(got, "# 分组脚注注释") {
		t.Fatalf("expected group foot comment preserved, got: %s", got)
	}
	if !strings.Contains(got, "# 规则行尾注释") {
		t.Fatalf("expected rule inline comment preserved, got: %s", got)
	}
}

// TestAppendUsageGroups_KeepsExpireGroupBeforeTrafficGroup 用于验证同时生成两个展示分组时，重置时间分组始终位于流量分组之前。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestAppendUsageGroups_KeepsExpireGroupBeforeTrafficGroup(t *testing.T) {
	t.Parallel()

	setupSubscribeTestMode()

	fileContent := []byte(`proxy-groups:
  - name: 原分组
    type: select
    proxies:
      - REJECT
`)

	apiInfo := &base.APIResponseInfo{
		Upload:   0,
		Download: 2 * 1024 * 1024 * 1024,
		Total:    5 * 1024 * 1024 * 1024,
		Expire:   time.Date(2026, 4, 13, 8, 30, 0, 0, time.Local).Unix(),
	}

	updated, err := appendUsageGroups(fileContent, apiInfo, newTestUsageDisplayConfig())
	if err != nil {
		t.Fatalf("appendUsageGroups returned error: %v", err)
	}

	got := string(updated)
	expireIndex := strings.Index(got, "重置日期 2026-04-13")
	trafficIndex := strings.Index(got, "已用流量 2G / 5G")
	originIndex := strings.Index(got, "name: 原分组")
	if expireIndex == -1 || trafficIndex == -1 || originIndex == -1 {
		t.Fatalf("expected expire, traffic and original groups, got: %s", got)
	}

	if expireIndex >= trafficIndex {
		t.Fatalf("expected expire group before traffic group, got: %s", got)
	}
	if trafficIndex >= originIndex {
		t.Fatalf("expected display groups before original group when prepend enabled, got: %s", got)
	}
}

// newTestSubscribeHandler 用于构造订阅处理器测试所需的最小运行环境。
// 参数含义：t 为测试上下文。
// 返回值：返回测试处理器和对应的路径配置。
func newTestSubscribeHandler(t *testing.T) (*SubscribeHandler, config.PathConfig) {
	t.Helper()

	subscriptionDir := t.TempDir()
	filename := "test.yaml"
	content := []byte("proxy-groups:\n  - name: 节点选择\n    type: select\n    proxies:\n      - REJECT\n")
	if err := os.WriteFile(filepath.Join(subscriptionDir, filename), content, 0o600); err != nil {
		t.Fatalf("failed to write subscription file: %v", err)
	}

	appConf := &config.AppConfig{
		RootConfig: config.RootConfig{
			Global: config.GlobalConfig{
				Storage: config.StorageConfig{
					SubscriptionDir: subscriptionDir,
				},
			},
		},
	}

	handler := NewSubscribeHandler(&Handler{
		logger: newTestHandlerLogger(),
		conf:   appConf,
	}, appConf)
	handler.cache = gocache.New(gocache.NoExpiration, time.Second)

	conf := config.PathConfig{
		Path:           "/test.yaml",
		File:           filename,
		ProviderRef:    "shared-provider",
		ProviderConfig: newTestProviderConfig(),
		UsageDisplay:   newTestUsageDisplayConfig(),
	}

	return handler, conf
}

// newTestProviderConfig 用于生成测试使用的 provider 配置。
// 参数含义：无。
// 返回值：返回启用 API 缓存的 provider 配置。
func newTestProviderConfig() config.ProviderConfig {
	return config.ProviderConfig{
		APITTL:         time.Minute,
		RequestTimeout: time.Second,
		UpdateInterval: time.Hour,
	}
}

// newTestUsageDisplayConfig 用于生成测试使用的流量展示配置。
// 参数含义：无。
// 返回值：返回启用流量展示的配置。
func newTestUsageDisplayConfig() *config.UsageDisplayConfig {
	return &config.UsageDisplayConfig{
		Enable:          true,
		Prepend:         true,
		TrafficFormat:   "⛽ 已用流量 {{.used}} / {{.total}}",
		ResetTimeFormat: "📅 重置日期 {{.year}}-{{.month}}-{{.day}}",
		TrafficUnit:     "G",
	}
}

// newTestHandlerLogger 用于创建测试场景下的空日志实例。
// 参数含义：无。
// 返回值：返回不会输出内容的日志实例。
func newTestHandlerLogger() *log.Logger {
	return &log.Logger{Logger: zap.NewNop()}
}

type observedSubscribeHandler struct {
	*SubscribeHandler
	recorded *observer.ObservedLogs
}

// newObservedSubscribeHandler 用于创建带可观测日志收集器的订阅处理器，便于断言日志级别是否符合预期。
// 参数含义：t 为测试上下文；level 为日志捕获级别。
// 返回值：返回带观测器的处理器和对应路径配置。
func newObservedSubscribeHandler(t *testing.T, level zapcore.Level) (*observedSubscribeHandler, config.PathConfig) {
	t.Helper()

	core, recorded := observer.New(level)

	subscribeHandler, conf := newTestSubscribeHandler(t)
	subscribeHandler.logger = &log.Logger{Logger: zap.New(core)}

	return &observedSubscribeHandler{
		SubscribeHandler: subscribeHandler,
		recorded:         recorded,
	}, conf
}

// newObservedSubscribeEngine 用于构造挂载真实日志中间件和订阅路由的 Gin 引擎，便于验证完整请求链路行为。
// 参数含义：handler 为待测试的订阅处理器。
// 返回值：返回可直接接收 HTTP 请求的 Gin 引擎。
func newObservedSubscribeEngine(handler *observedSubscribeHandler) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.Logger(handler.logger))
	engine.GET("/*path", handler.Get)
	return engine
}
