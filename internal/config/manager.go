package config

import (
	"flag"
	"fmt"

	"github.com/spf13/viper"

	"github.com/djx30103/vpsub/pkg/pathutil"
	"github.com/djx30103/vpsub/pkg/provider"
)

const defaultConfigPath = "config/config.yml"

// AppConfig 保存应用配置和预处理后的路由映射。
type AppConfig struct {
	RootConfig
	PathToConfig map[string]PathConfig // 完整请求路径到配置的映射
}

// PathConfig 保存单个订阅请求路径对应的运行时配置。
// ProviderConfig 在构建阶段已完成默认值合并，字段均可直接使用。
type PathConfig struct {
	Path         string
	File         string
	ProviderRef  string
	ProviderType string
	APIID        string
	APIKey       string

	ProviderConfig ProviderConfig
	AccessControl  *AccessControlConfig
	UsageDisplay   *UsageDisplayConfig
}

// Load 用于读取并反序列化配置文件，同时补齐默认值并执行静态校验。
// 参数含义：path 为配置文件路径。
// 返回值：返回根配置和加载错误。
func Load(path string) (RootConfig, error) {
	var conf RootConfig
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return conf, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := v.Unmarshal(&conf); err != nil {
		return conf, fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	conf.initDefault()
	if err := conf.validate(); err != nil {
		return conf, fmt.Errorf("invalid config: %w", err)
	}

	return conf, nil
}

// BuildRuntime 用于将静态根配置编译为运行时配置索引。
// 参数含义：root 为已完成静态校验的根配置。
// 返回值：返回运行时配置和编译错误。
func BuildRuntime(root RootConfig) (*AppConfig, error) {
	root.initDefault()

	appConfig := &AppConfig{
		RootConfig:   root,
		PathToConfig: make(map[string]PathConfig, len(root.Routes)),
	}

	if err := appConfig.preprocessConfig(); err != nil {
		return nil, err
	}

	return appConfig, nil
}

// ResolveConfigPath 用于统一解析配置文件路径，优先级依次为命令行、环境变量、默认值。
// 参数含义：flagSet 为命令行参数集；args 为命令行参数；getenv 为环境变量读取函数。
// 返回值：返回最终配置文件路径；解析失败时返回错误。
func ResolveConfigPath(flagSet *flag.FlagSet, args []string, getenv func(string) string) (string, error) {
	// 先读取环境变量，作为命令行参数未显式传入时的默认值来源。
	defaultPath := defaultConfigPath
	if env := getenv("VPSUB_CONF_PATH"); env != "" {
		defaultPath = env
	}

	// 统一通过 flag 解析，确保命令行参数可以覆盖环境变量。
	var configPath string
	flagSet.StringVar(&configPath, "conf", defaultPath, "config path, eg: -conf config/config.yml")
	if err := flagSet.Parse(args); err != nil {
		return "", fmt.Errorf("failed to parse flags: %w", err)
	}

	// 显式传入空字符串时，仍然回退到内置默认值，避免读取空路径。
	if configPath == "" {
		return defaultConfigPath, nil
	}

	return configPath, nil
}

// preprocessConfig 用于将 providers 与 routes 解析成请求路径到运行时配置的映射。
// 参数含义：无。
// 返回值：返回预处理错误。
func (a *AppConfig) preprocessConfig() error {
	if len(a.Providers) == 0 {
		return fmt.Errorf("providers is required")
	}

	if len(a.Routes) == 0 {
		return fmt.Errorf("routes is required")
	}

	for _, route := range a.Routes {
		if err := a.buildPathForRoute(route); err != nil {
			return err
		}
	}

	return nil
}

// buildPathForRoute 用于解析单个 route，并建立请求路径到运行时配置的映射。
// 参数含义：route 为单条路由配置。
// 返回值：返回构建错误。
func (a *AppConfig) buildPathForRoute(route RouteItem) error {
	providerItem, ok := a.Providers[route.ProviderRef]
	if !ok {
		return fmt.Errorf("provider_ref not found: %s", route.ProviderRef)
	}

	if !provider.IsValidProvider(providerItem.Type) {
		return fmt.Errorf("unknown provider type: %s", providerItem.Type)
	}

	reqPath, err := pathutil.NormalizeRoutePath(route.Path)
	if err != nil {
		return fmt.Errorf("normalize path %q: %w", route.Path, err)
	}

	filePath, err := pathutil.NormalizeSubscriptionFilePath(route.File)
	if err != nil {
		return fmt.Errorf("normalize file %q: %w", route.File, err)
	}

	if _, exist := a.PathToConfig[reqPath]; exist {
		return fmt.Errorf("duplicate request path: %s", reqPath)
	}

	usageDisplay := a.resolveUsageDisplay(route)
	if err := usageDisplay.validate(); err != nil {
		return fmt.Errorf("route %q usage_display: %w", reqPath, err)
	}

	a.PathToConfig[reqPath] = PathConfig{
		Path:           reqPath,
		File:           filePath,
		ProviderRef:    route.ProviderRef,
		ProviderType:   providerItem.Type,
		APIID:          providerItem.APIID,
		APIKey:         providerItem.APIKey,
		ProviderConfig: a.resolveProviderConfig(providerItem),
		AccessControl:  route.AccessControl,
		UsageDisplay:   usageDisplay,
	}

	return nil
}

// resolveProviderConfig 将默认值与账号级覆盖合并，返回完全填充的运行时配置。
// 参数含义：providerItem 为服务商账号配置。
// 返回值：返回解析完成后的运行时服务商配置。
func (a *AppConfig) resolveProviderConfig(providerItem ProviderItem) ProviderConfig {
	resolved := ProviderConfig{
		APITTL:         *a.Defaults.Provider.APITTL,
		RequestTimeout: *a.Defaults.Provider.RequestTimeout,
		UpdateInterval: *a.Defaults.Provider.UpdateInterval,
	}

	if providerItem.Overrides == nil {
		return resolved
	}

	o := providerItem.Overrides
	if o.APITTL != nil {
		// APITTL=0 是有效配置（关闭缓存），因此只判断 nil，不过滤零值。
		resolved.APITTL = *o.APITTL
	}
	if o.RequestTimeout != nil {
		resolved.RequestTimeout = *o.RequestTimeout
	}
	if o.UpdateInterval != nil {
		resolved.UpdateInterval = *o.UpdateInterval
	}

	return resolved
}

// resolveUsageDisplay 用于合并默认展示配置与路由级覆盖配置。
// 参数含义：route 为单条路由配置。
// 返回值：返回解析完成后的展示配置。
func (a *AppConfig) resolveUsageDisplay(route RouteItem) *UsageDisplayConfig {
	resolved := &UsageDisplayConfig{}
	*resolved = *a.Defaults.UsageDisplay

	if route.UsageDisplay == nil {
		return resolved
	}

	applyUsageDisplayOverrides(resolved, route.UsageDisplay)
	return resolved
}

// applyUsageDisplayOverrides 用于将路由级展示覆盖配置合并到目标配置中。
// 参数含义：dst 为目标配置；src 为路由级覆盖配置。
// 返回值：无。
func applyUsageDisplayOverrides(dst *UsageDisplayConfig, src *UsageDisplayOverride) {
	if src == nil {
		return
	}

	if src.Enable != nil {
		dst.Enable = *src.Enable
	}

	if src.Prepend != nil {
		dst.Prepend = *src.Prepend
	}

	if src.TrafficFormat != nil {
		dst.TrafficFormat = *src.TrafficFormat
	}

	if src.TrafficUnit != nil {
		dst.TrafficUnit = *src.TrafficUnit
	}

	if src.ResetTimeFormat != nil {
		dst.ResetTimeFormat = *src.ResetTimeFormat
	}
}
