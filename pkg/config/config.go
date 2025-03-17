package config

import (
	"strings"
	"time"

	"vpsub/pkg/bytesize"
)

// RootConfig 保存完整的配置结构
type RootConfig struct {
	AppMode   string        `mapstructure:"app_mode"`
	Server    ServerConfig  `mapstructure:"server"`
	Log       LogConfig     `mapstructure:"log"`
	Global    GlobalConfig  `mapstructure:"global"`
	Defaults  DefaultConfig `mapstructure:"defaults"`
	Providers ProviderMap   `mapstructure:"providers"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	ListenAddr string        `mapstructure:"listen_addr"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `mapstructure:"level"`
}

type GlobalConfig struct {
	Storage      StorageConfig      `mapstructure:"storage"`
	UsageDisplay UsageDisplayConfig `mapstructure:"usage_display"`
}

// DefaultConfig 默认配置
type DefaultConfig struct {
	Cache    *CacheConfig    `mapstructure:"cache"`
	Provider *ProviderConfig `mapstructure:"provider"`
}

type UsageDisplayConfig struct {
	Enable        bool   `mapstructure:"enable"`
	Prepend       bool   `mapstructure:"prepend"`
	TrafficFormat string `mapstructure:"traffic_format"`
	ExpireFormat  string `mapstructure:"expire_format"`
	TrafficUnit   string `mapstructure:"traffic_unit"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	FileTTL     *time.Duration `mapstructure:"file_ttl"`
	APITTL      *time.Duration `mapstructure:"api_ttl"`
	ResponseTTL *time.Duration `mapstructure:"response_ttl"`
}

// ProviderConfig 服务商通用配置
type ProviderConfig struct {
	RequestTimeout *time.Duration `mapstructure:"request_timeout"`
	UpdateInterval *time.Duration `mapstructure:"update_interval"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	SubscriptionDir string `mapstructure:"subscription_dir"`
}

// ProviderMap 提供商配置映射
type ProviderMap map[string][]ProviderItem

// ProviderItem 单个提供商配置项
type ProviderItem struct {
	RoutePrefix   string         `mapstructure:"route_prefix"`
	APIID         string         `mapstructure:"api_id"`
	APIKey        string         `mapstructure:"api_key"`
	Subscriptions []string       `mapstructure:"subscriptions"`
	Overrides     *DefaultConfig `mapstructure:"overrides"`
}

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

	r.Global.initDefault()
	r.Defaults.initDefault()
}

func (r *GlobalConfig) initDefault() {
	if r.Storage.SubscriptionDir == "" {
		r.Storage.SubscriptionDir = "./subscriptions"
	}

	if r.UsageDisplay.Enable {
		trafficFormat := r.UsageDisplay.TrafficFormat
		expireFormat := r.UsageDisplay.ExpireFormat
		if !bytesize.IsValidUnit(r.UsageDisplay.TrafficUnit) {
			r.UsageDisplay.TrafficUnit = "G"
		}

		if !strings.Contains(trafficFormat, "{{.total}}") && !strings.Contains(trafficFormat, "{{.used}}") {
			r.UsageDisplay.TrafficFormat = "⛽ 已用流量 {{.used}} / {{.total}}"
		}

		// year, month, day, hour, minute, second

		if !strings.Contains(expireFormat, "{{.year}}") && !strings.Contains(expireFormat, "{{month}}") &&
			!strings.Contains(expireFormat, "{{.day}}") && !strings.Contains(expireFormat, "{{.hour}}") &&
			!strings.Contains(expireFormat, "{{.minute}}") && !strings.Contains(expireFormat, "{{.second}}") {
			r.UsageDisplay.ExpireFormat = "📅 重置日期 {{.year}}-{{.month}}-{{.day}}"
		}
	}
}

func (r *DefaultConfig) initDefault() {
	if r.Cache == nil {
		zero := time.Duration(0)
		responseTTL := time.Minute
		r.Cache = &CacheConfig{
			FileTTL:     &zero,
			APITTL:      &zero,
			ResponseTTL: &responseTTL,
		}
	}

	if r.Provider == nil {
		r.Provider = new(ProviderConfig)
	}
	if r.Provider.RequestTimeout == nil || *r.Provider.RequestTimeout == 0 {
		requestTimeout := 10 * time.Second
		r.Provider.RequestTimeout = &requestTimeout
	}

	if r.Provider.UpdateInterval == nil || *r.Provider.UpdateInterval == 0 {
		updateInterval := 24 * time.Hour
		r.Provider.UpdateInterval = &updateInterval
	}
}
