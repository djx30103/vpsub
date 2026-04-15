package config

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/djx30103/vpsub/pkg/bytesize"
)

// RootConfig 保存完整的配置结构。
type RootConfig struct {
	AppMode   string         `mapstructure:"app_mode"`
	Server    ServerConfig   `mapstructure:"server"`
	Log       LogConfig      `mapstructure:"log"`
	Global    GlobalConfig   `mapstructure:"global"`
	Defaults  DefaultsConfig `mapstructure:"defaults"`
	Providers ProviderMap    `mapstructure:"providers"`
	Routes    []RouteItem    `mapstructure:"routes"`
}

// ServerConfig 表示 HTTP 服务配置。
type ServerConfig struct {
	ListenAddr string        `mapstructure:"listen_addr"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

// LogConfig 表示日志配置。
type LogConfig struct {
	Level string `mapstructure:"level"`
}

// GlobalConfig 表示全局配置。
type GlobalConfig struct {
	Storage StorageConfig `mapstructure:"storage"`
}

// DefaultsConfig 表示全局默认配置。
type DefaultsConfig struct {
	Provider     *ProviderConfigOverride `mapstructure:"provider"`
	UsageDisplay *UsageDisplayConfig     `mapstructure:"usage_display"`
}

// ProviderConfig 是解析合并后的运行时服务商配置，字段均为值类型，调用方可直接使用。
type ProviderConfig struct {
	APITTL         time.Duration
	RequestTimeout time.Duration
	UpdateInterval time.Duration
}

// ProviderConfigOverride 对应配置文件中的服务商参数，指针字段表示"未配置"，用于与默认值合并。
type ProviderConfigOverride struct {
	APITTL         *time.Duration `mapstructure:"api_ttl"`
	RequestTimeout *time.Duration `mapstructure:"request_timeout"`
	UpdateInterval *time.Duration `mapstructure:"update_interval"`
}

// UsageDisplayConfig 表示订阅中追加的流量展示配置。
type UsageDisplayConfig struct {
	Enable          bool   `mapstructure:"enable"`
	Prepend         bool   `mapstructure:"prepend"`
	TrafficFormat   string `mapstructure:"traffic_format"`
	ResetTimeFormat string `mapstructure:"reset_time_format"`
	TrafficUnit     string `mapstructure:"traffic_unit"`
}

// UsageDisplayOverride 用于表达路由级的显式覆盖，支持将布尔值覆盖为 false。
type UsageDisplayOverride struct {
	Enable          *bool   `mapstructure:"enable"`
	Prepend         *bool   `mapstructure:"prepend"`
	TrafficFormat   *string `mapstructure:"traffic_format"`
	ResetTimeFormat *string `mapstructure:"reset_time_format"`
	TrafficUnit     *string `mapstructure:"traffic_unit"`
}

// StorageConfig 表示文件存储配置。
type StorageConfig struct {
	SubscriptionDir string `mapstructure:"subscription_dir"`
}

// ProviderMap 表示账号名到服务商账号配置的映射。
type ProviderMap map[string]ProviderItem

// ProviderItem 表示单个服务商账号配置。
// Overrides 直接映射配置文件中的 overrides 字段，nil 表示无覆盖。
type ProviderItem struct {
	Type      string                  `mapstructure:"type"`
	APIID     string                  `mapstructure:"api_id"`
	APIKey    string                  `mapstructure:"api_key"`
	Overrides *ProviderConfigOverride `mapstructure:"overrides"`
}

// RouteItem 表示对外暴露的订阅路由配置。
type RouteItem struct {
	Path          string                `mapstructure:"path"`
	File          string                `mapstructure:"file"`
	ProviderRef   string                `mapstructure:"provider_ref"`
	AccessControl *AccessControlConfig  `mapstructure:"access_control"`
	UsageDisplay  *UsageDisplayOverride `mapstructure:"usage_display"`
}

// AccessControlConfig 表示路由级访问约束配置。
type AccessControlConfig struct {
	UserAgent string `mapstructure:"user_agent"`
}

// initDefault 用于补齐根配置中的默认值。
func (r *RootConfig) initDefault() {
	if r.AppMode == "" {
		r.AppMode = "release"
	}

	if r.Server.Timeout == 0 {
		r.Server.Timeout = 30 * time.Second
	}

	if r.Server.ListenAddr == "" {
		r.Server.ListenAddr = ":30103"
	}

	if r.Log.Level == "" {
		r.Log.Level = "warn"
	}

	// 统一标准化 provider 类型，避免配置层把大小写兼容逻辑散落到校验和运行时阶段。
	r.normalizeProviders()
	r.Global.initDefault()
	r.Defaults.initDefault()
}

// normalizeProviders 用于标准化 provider 配置中的类型字段，统一收敛大小写和首尾空白差异。
// 参数含义：无。
// 返回值：无。
func (r *RootConfig) normalizeProviders() {
	for name, providerItem := range r.Providers {
		providerItem.Type = strings.ToLower(strings.TrimSpace(providerItem.Type))
		r.Providers[name] = providerItem
	}
}

// initDefault 用于补齐全局配置中的默认值。
func (r *GlobalConfig) initDefault() {
	if r.Storage.SubscriptionDir == "" {
		r.Storage.SubscriptionDir = "./subscriptions"
	}
}

// initDefault 用于补齐默认配置中的默认值。
func (r *DefaultsConfig) initDefault() {
	if r.Provider == nil {
		r.Provider = &ProviderConfigOverride{}
	}

	if r.UsageDisplay == nil {
		r.UsageDisplay = &UsageDisplayConfig{}
	}

	r.Provider.initDefault()
	r.UsageDisplay.initDefault()
}

// initDefault 用于补齐服务商覆盖配置中的默认值。
// APITTL 仅在 nil 时填入 0（关闭缓存）；其余字段在 nil 或 0 时都填入内置默认值。
func (r *ProviderConfigOverride) initDefault() {
	if r.APITTL == nil {
		// 0 是有效值（关闭缓存），仅在用户未配置时才填入该默认值。
		r.APITTL = new(time.Duration)
		*r.APITTL = 300 * time.Second
	}

	if r.RequestTimeout == nil {
		r.RequestTimeout = new(time.Duration)
		*r.RequestTimeout = 10 * time.Second
	}

	if r.UpdateInterval == nil {
		r.UpdateInterval = new(time.Duration)
		*r.UpdateInterval = 24 * time.Hour
	}
}

// initDefault 用于补齐流量展示配置中的默认值。
func (r *UsageDisplayConfig) initDefault() {
	if r.TrafficUnit == "" {
		r.TrafficUnit = "G"
	}

	if r.TrafficFormat == "" {
		r.TrafficFormat = "⛽ 已用流量 {{.used}} / {{.total}}"
	}

	if r.ResetTimeFormat == "" {
		r.ResetTimeFormat = "📅 重置日期 {{.year}}-{{.month}}-{{.day}}"
	}
}

// validate 用于校验根配置的静态合法性，避免明显错误拖到请求期。
func (r *RootConfig) validate() error {
	if err := r.Defaults.validate(); err != nil {
		return err
	}

	for name, providerItem := range r.Providers {
		if err := providerItem.validate(); err != nil {
			return fmt.Errorf("provider %q: %w", name, err)
		}
	}

	for i, route := range r.Routes {
		if err := route.validate(); err != nil {
			return fmt.Errorf("route[%d] %q: %w", i, route.Path, err)
		}
	}

	return nil
}

// validate 用于校验默认配置是否合法。
func (r *DefaultsConfig) validate() error {
	if r.Provider != nil {
		if err := r.Provider.validate(); err != nil {
			return fmt.Errorf("defaults.provider: %w", err)
		}
	}

	if r.UsageDisplay != nil {
		if err := r.UsageDisplay.validate(); err != nil {
			return fmt.Errorf("defaults.usage_display: %w", err)
		}
	}

	return nil
}

// validate 用于校验服务商覆盖配置是否合法。
func (r *ProviderConfigOverride) validate() error {
	if r.APITTL != nil && *r.APITTL < 0 {
		return errors.New("api_ttl must be >= 0")
	}

	if r.RequestTimeout != nil && *r.RequestTimeout < 0 {
		return errors.New("request_timeout must be >= 0")
	}

	if r.UpdateInterval != nil && *r.UpdateInterval < 0 {
		return errors.New("update_interval must be >= 0")
	}

	return nil
}

// validate 用于校验流量展示配置是否合法。
func (r *UsageDisplayConfig) validate() error {
	if !bytesize.IsValidUnit(r.TrafficUnit) {
		return errors.New("traffic_unit is invalid")
	}

	if err := validateTrafficTemplate(r.TrafficFormat); err != nil {
		return err
	}

	if err := validateResetTimeTemplate(r.ResetTimeFormat); err != nil {
		return err
	}

	return nil
}

// validate 用于校验路由级覆盖配置是否合法。
func (r *UsageDisplayOverride) validate() error {
	if r.TrafficUnit != nil && !bytesize.IsValidUnit(*r.TrafficUnit) {
		return errors.New("traffic_unit is invalid")
	}

	if r.TrafficFormat != nil {
		if err := validateTrafficTemplate(*r.TrafficFormat); err != nil {
			return err
		}
	}

	if r.ResetTimeFormat != nil {
		if err := validateResetTimeTemplate(*r.ResetTimeFormat); err != nil {
			return err
		}
	}

	return nil
}

// validate 用于校验账号配置是否合法。
func (r *ProviderItem) validate() error {
	if strings.TrimSpace(r.Type) == "" {
		return errors.New("type is required")
	}

	// passthrough 不调用任何外部 API，无需 api_id 和 api_key。
	if r.Type != "passthrough" {
		if strings.TrimSpace(r.APIID) == "" {
			return errors.New("api_id is required")
		}

		if strings.TrimSpace(r.APIKey) == "" {
			return errors.New("api_key is required")
		}
	}

	if r.Overrides != nil {
		if err := r.Overrides.validate(); err != nil {
			return fmt.Errorf("overrides: %w", err)
		}
	}

	return nil
}

// validate 用于校验路由配置是否合法。
func (r *RouteItem) validate() error {
	if strings.TrimSpace(r.Path) == "" {
		return errors.New("path is required")
	}

	if !strings.HasPrefix(r.Path, "/") {
		return errors.New("path must start with /")
	}

	if strings.TrimSpace(r.File) == "" {
		return errors.New("file is required")
	}

	if strings.TrimSpace(r.ProviderRef) == "" {
		return errors.New("provider_ref is required")
	}

	if !isSafeRoutePath(r.Path) {
		return errors.New("path is invalid")
	}

	if !isSafeSubscriptionName(r.File) {
		return errors.New("file is invalid")
	}

	if r.AccessControl != nil {
		if err := r.AccessControl.validate(); err != nil {
			return fmt.Errorf("access_control: %w", err)
		}
	}

	if r.UsageDisplay != nil {
		if err := r.UsageDisplay.validate(); err != nil {
			return fmt.Errorf("usage_display: %w", err)
		}
	}

	return nil
}

// validate 用于校验访问约束配置是否合法。
func (r *AccessControlConfig) validate() error {
	if strings.TrimSpace(r.UserAgent) == "" {
		return errors.New("user_agent must not be empty")
	}

	return nil
}

// validateTrafficTemplate 用于校验流量模板是否语法正确且同时引用 used 与 total。
func validateTrafficTemplate(format string) error {
	// 使用不会出现在正常文本中的哨兵值，避免将模板外的字面量误判为占位符。
	const (
		usedSentinel  = "TRAFFIC_USED_SENTINEL"
		totalSentinel = "TRAFFIC_TOTAL_SENTINEL"
	)

	result, err := ExecuteTemplate(format, map[string]string{
		"used":  usedSentinel,
		"total": totalSentinel,
	})
	if err != nil {
		return errors.New("traffic_format is invalid")
	}

	if !strings.Contains(result, usedSentinel) || !strings.Contains(result, totalSentinel) {
		return errors.New("traffic_format must contain {{.used}} and {{.total}}")
	}

	return nil
}

// validateResetTimeTemplate 用于校验重置时间模板是否语法正确，且只能引用年月日占位符。
func validateResetTimeTemplate(format string) error {
	// 使用不会出现在正常文本中的哨兵值，避免将模板外的字面量误判为占位符。
	const (
		yearSentinel  = "RESET_TIME_YEAR_SENTINEL"
		monthSentinel = "RESET_TIME_MONTH_SENTINEL"
		daySentinel   = "RESET_TIME_DAY_SENTINEL"
	)

	result, err := ExecuteTemplate(format, map[string]string{
		"year":  yearSentinel,
		"month": monthSentinel,
		"day":   daySentinel,
	})
	if err != nil {
		return errors.New("reset_time_format is invalid")
	}

	if !strings.Contains(result, yearSentinel) &&
		!strings.Contains(result, monthSentinel) &&
		!strings.Contains(result, daySentinel) {
		return errors.New("reset_time_format must contain at least one date placeholder")
	}

	return nil
}

// ExecuteTemplate 用于按统一的 Go 模板规则渲染字符串，借助 missingkey=error 拦截未知字段。
// 参数含义：format 为待渲染模板；data 为模板可用字段。
// 返回值：返回渲染后的字符串和模板执行错误。
func ExecuteTemplate(format string, data map[string]string) (string, error) {
	tpl, err := template.New("config").Option("missingkey=error").Parse(format)
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	if err := tpl.Execute(&buffer, data); err != nil {
		return "", err
	}

	return buffer.String(), nil
}

// isSafeSubscriptionName 用于校验订阅文件名是否仍处于订阅目录内，避免路径越界。
func isSafeSubscriptionName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}

	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return false
	}

	cleanName := strings.ReplaceAll(name, "\\", "/")
	for _, segment := range strings.Split(cleanName, "/") {
		if segment == ".." || segment == "" {
			return false
		}
	}

	return true
}

// isSafeRoutePath 用于校验对外路由路径在去除前导斜杠后仍能安全映射到订阅目录内部。
// 参数含义：path 为配置中的对外路由路径。
// 返回值：返回是否为安全路径。
func isSafeRoutePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return false
	}

	return isSafeSubscriptionName(strings.TrimPrefix(path, "/"))
}
