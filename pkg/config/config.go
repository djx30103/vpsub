package config

import (
	"time"

	"github.com/pkg/errors"
)

// RootConfig 保存完整的配置结构
type RootConfig struct {
	AppMode   string         `mapstructure:"app_mode"`
	Server    ServerConfig   `mapstructure:"server"`
	Log       LogConfig      `mapstructure:"log"`
	Storage   *StorageConfig `mapstructure:"storage"`
	Defaults  DefaultConfig  `mapstructure:"defaults"`
	Providers ProviderMap    `mapstructure:"providers"`
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

// DefaultConfig 默认配置
type DefaultConfig struct {
	Cache    *CacheConfig    `mapstructure:"cache"`
	Provider *ProviderConfig `mapstructure:"provider"`
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

func (r *RootConfig) Validate() error {
	if r.AppMode == "" {
		r.AppMode = "release"
	}

	if len(r.Providers) == 0 {
		return errors.New("providers is required")
	}

	err := r.Storage.Validate()
	if err != nil {
		return err
	}

	err = r.Providers.Validate()
	if err != nil {
		return err
	}

	err = r.Defaults.Provider.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (p *ProviderConfig) Validate() error {
	if p.RequestTimeout == nil || *p.RequestTimeout == 0 {
		return errors.New("request_timeout is required")
	}

	if p.UpdateInterval == nil || *p.UpdateInterval == 0 {
		return errors.New("update_interval is required")
	}

	return nil
}

func (s *StorageConfig) Validate() error {
	if s.SubscriptionDir == "" {
		return errors.New("subscription_dir is required")
	}

	return nil
}

func (p ProviderMap) Validate() error {
	for _, itemList := range p {
		if len(itemList) == 0 {
			return errors.New("providers is required")
		}

		for _, item := range itemList {
			err := item.Validate()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *ProviderItem) Validate() error {
	if p.RoutePrefix == "" {
		return errors.New("route_prefix is required")
	}

	if p.APIKey == "" {
		return errors.New("api_key is required")
	}

	if len(p.Subscriptions) == 0 {
		return errors.New("subscriptions is required")
	}

	return nil

}
