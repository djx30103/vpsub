package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadAndBuildRuntime_LoadsPathRoutes 用于验证单 path 路由配置会被加载并编译为运行时查询映射。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestLoadAndBuildRuntime_LoadsPathRoutes(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
defaults:
  provider:
    api_ttl: 2m
    request_timeout: 5s
    update_interval: 12h
  usage_display:
    enable: true
    prepend: false
    traffic_format: "已用 {{.used}} / {{.total}}"
    traffic_unit: "G"
    reset_time_format: "重置 {{.year}}-{{.month}}-{{.day}}"
providers:
  hk-bwh:
    type: bandwagonhost
    api_id: "veid-1"
    api_key: "key-1"
routes:
  - path: "/bwh/a.yaml"
    file: "a.yaml"
    provider_ref: "hk-bwh"
    access_control:
      user_agent: "ClashX"
`)

	root, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	appConf, err := BuildRuntime(root)
	if err != nil {
		t.Fatalf("BuildRuntime returned error: %v", err)
	}

	if len(appConf.PathToConfig) != 1 {
		t.Fatalf("expected 1 path config, got %d", len(appConf.PathToConfig))
	}

	pathConf, ok := appConf.PathToConfig["/bwh/a.yaml"]
	if !ok {
		t.Fatalf("expected path config for /bwh/a.yaml")
	}

	if pathConf.Path != "/bwh/a.yaml" {
		t.Fatalf("expected path /bwh/a.yaml, got %s", pathConf.Path)
	}

	if pathConf.File != "a.yaml" {
		t.Fatalf("expected file a.yaml, got %s", pathConf.File)
	}

	if pathConf.ProviderRef != "hk-bwh" {
		t.Fatalf("expected provider ref hk-bwh, got %s", pathConf.ProviderRef)
	}

	if pathConf.ProviderType != "bandwagonhost" {
		t.Fatalf("expected provider type bandwagonhost, got %s", pathConf.ProviderType)
	}

	if got := pathConf.ProviderConfig.APITTL; got != 2*time.Minute {
		t.Fatalf("expected api_ttl 2m, got %s", got)
	}

	if pathConf.AccessControl == nil {
		t.Fatalf("expected access control to be loaded")
	}

	if pathConf.AccessControl.UserAgent != "ClashX" {
		t.Fatalf("expected user agent ClashX, got %s", pathConf.AccessControl.UserAgent)
	}
}

// TestBuildRuntime_RejectsUnknownProviderRef 用于验证运行时编译阶段会拒绝不存在的 provider_ref。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestBuildRuntime_RejectsUnknownProviderRef(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
providers:
  hk-bwh:
    type: bandwagonhost
    api_id: "veid-1"
    api_key: "key-1"
routes:
  - path: "/bwh/a.yaml"
    file: "a.yaml"
    provider_ref: "missing-provider"
`)

	root, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if _, err := BuildRuntime(root); err == nil {
		t.Fatalf("expected BuildRuntime to fail for unknown provider_ref")
	}
}

// TestLoad_RejectsUnsafeRoutePath 用于验证配置加载阶段会拒绝越界或非法的路由路径。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestLoad_RejectsUnsafeRoutePath(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
providers:
  hk-bwh:
    type: bandwagonhost
    api_id: "veid-1"
    api_key: "key-1"
routes:
  - path: "../secret.yaml"
    file: "a.yaml"
    provider_ref: "hk-bwh"
`)

	if _, err := Load(configPath); err == nil {
		t.Fatalf("expected Load to fail for unsafe route path")
	}
}

// TestResolveConfigPath_UsesDefaultWhenEnvAndFlagAreEmpty 用于验证未提供环境变量和命令行时会使用默认配置路径。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestResolveConfigPath_UsesDefaultWhenEnvAndFlagAreEmpty(t *testing.T) {
	t.Parallel()

	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	got, err := ResolveConfigPath(flagSet, nil, func(string) string {
		return ""
	})
	if err != nil {
		t.Fatalf("resolveConfigPath returned error: %v", err)
	}
	if got != "config/config.yml" {
		t.Fatalf("expected default config path, got %s", got)
	}
}

// TestResolveConfigPath_UsesEnvWhenFlagIsAbsent 用于验证未显式传参时会读取环境变量作为配置路径。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestResolveConfigPath_UsesEnvWhenFlagIsAbsent(t *testing.T) {
	t.Parallel()

	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	got, err := ResolveConfigPath(flagSet, nil, func(key string) string {
		if key == "VPSUB_CONF_PATH" {
			return "/tmp/from-env.yml"
		}

		return ""
	})
	if err != nil {
		t.Fatalf("resolveConfigPath returned error: %v", err)
	}
	if got != "/tmp/from-env.yml" {
		t.Fatalf("expected env config path, got %s", got)
	}
}

// TestResolveConfigPath_FlagOverridesEnv 用于验证命令行参数会覆盖环境变量配置路径。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestResolveConfigPath_FlagOverridesEnv(t *testing.T) {
	t.Parallel()

	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	got, err := ResolveConfigPath(flagSet, []string{"-conf", "/tmp/from-flag.yml"}, func(key string) string {
		if key == "VPSUB_CONF_PATH" {
			return "/tmp/from-env.yml"
		}

		return ""
	})
	if err != nil {
		t.Fatalf("resolveConfigPath returned error: %v", err)
	}
	if got != "/tmp/from-flag.yml" {
		t.Fatalf("expected flag config path, got %s", got)
	}
}

// TestResolveConfigPath_ReturnsErrorOnInvalidFlag 用于验证命令行参数非法时不会被静默吞掉。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestResolveConfigPath_ReturnsErrorOnInvalidFlag(t *testing.T) {
	t.Parallel()

	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	if _, err := ResolveConfigPath(flagSet, []string{"-unknown"}, func(string) string {
		return ""
	}); err == nil {
		t.Fatalf("expected resolveConfigPath to return error on invalid flag")
	}
}

// TestBuildRuntime_AppliesDefaultsForDirectRootConfig 用于验证直接传入手工 RootConfig 时也能先补齐默认值再构建运行时配置。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestBuildRuntime_AppliesDefaultsForDirectRootConfig(t *testing.T) {
	t.Parallel()

	appConfig, err := BuildRuntime(RootConfig{
		Providers: ProviderMap{
			"hk-bwh": {
				Type:   "bandwagonhost",
				APIID:  "veid-1",
				APIKey: "key-1",
			},
		},
		Routes: []RouteItem{
			{
				Path:        "/bwh/a.yaml",
				File:        "a.yaml",
				ProviderRef: "hk-bwh",
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildRuntime returned error: %v", err)
	}

	pathConf, ok := appConfig.PathToConfig["/bwh/a.yaml"]
	if !ok {
		t.Fatalf("expected path config for /bwh/a.yaml")
	}

	if pathConf.ProviderConfig.RequestTimeout != 10*time.Second {
		t.Fatalf("expected default request timeout 10s, got %s", pathConf.ProviderConfig.RequestTimeout)
	}

	if pathConf.ProviderConfig.APITTL != 300*time.Second {
		t.Fatalf("expected default api_ttl 300s, got %s", pathConf.ProviderConfig.APITTL)
	}

	if pathConf.UsageDisplay.Enable || pathConf.UsageDisplay.TrafficUnit != "G" {
		t.Fatalf("expected usage display defaults to be initialized")
	}
}

// TestLoad_RejectsUnsafeRouteFile 用于验证配置加载阶段会拒绝越界的订阅文件相对路径。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestLoad_RejectsUnsafeRouteFile(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
providers:
  hk-bwh:
    type: bandwagonhost
    api_id: "veid-1"
    api_key: "key-1"
routes:
  - path: "/masked"
    file: "../secret.yaml"
    provider_ref: "hk-bwh"
`)

	if _, err := Load(configPath); err == nil {
		t.Fatalf("expected Load to fail for unsafe route file")
	}
}

// writeTestConfig 用于在临时目录中写入测试配置文件。
// 参数含义：t 为测试上下文；content 为配置文件内容。
// 返回值：返回写入后的配置文件绝对路径。
func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	return configPath
}
