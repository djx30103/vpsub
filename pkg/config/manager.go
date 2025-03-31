package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"

	"vpsub/pkg/provider"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// AppConfig 保存应用配置和预处理的映射
type AppConfig struct {
	RootConfig
	// 预处理的映射
	PathToConfig map[string]PathConfig // 完整请求路径到配置的映射
}

// PathConfig 保存每个请求路径对应的配置
type PathConfig struct {
	ProviderType string // 提供商类型 (bandwagonhost, vultr等)
	APIID        string // API ID
	APIKey       string // API Key
	Filename     string // 文件名

	DefaultConfig *DefaultConfig
}

func NewConfig() *AppConfig {
	envConf := os.Getenv("VPSUB_CONF_PATH")
	if envConf == "" {
		flag.StringVar(&envConf, "conf", "config/config.yml", "config path, eg: -conf config/config.yml")
		flag.Parse()
	}

	if envConf == "" {
		envConf = "config/config.yml"
	}

	fmt.Println("load conf file:", envConf)

	conf, err := getConfig(envConf)
	if err != nil {
		panic(err)
	}

	// 创建并初始化AppConfig
	appConfig := &AppConfig{
		RootConfig:   conf,
		PathToConfig: make(map[string]PathConfig),
	}

	// 预处理配置
	err = appConfig.preprocessConfig()
	if err != nil {
		panic(err)
	}

	return appConfig
}

func getConfig(path string) (RootConfig, error) {
	var conf RootConfig
	v := viper.New()
	v.SetConfigFile(path)
	err := v.ReadInConfig()
	if err != nil {
		return conf, errors.Wrap(err, "failed to read config file")
	}

	err = v.Unmarshal(&conf)
	if err != nil {
		return conf, errors.Wrap(err, "failed to unmarshal config file")
	}

	conf.initDefault()

	return conf, nil
}

// preprocessConfig 预处理配置，建立映射关系
func (a *AppConfig) preprocessConfig() error {
	if len(a.Providers) == 0 {
		return errors.New("providers is required")
	}

	for providerType, itemList := range a.Providers {
		if !provider.IsValidProvider(providerType) {
			return errors.Errorf("unknown provider type: %s", providerType)
		}

		if len(itemList) == 0 {
			return errors.Errorf("providers is required, provider type: %s", providerType)
		}

		err := a.buildPathForProvider(providerType, itemList)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *AppConfig) buildPathForProvider(providerType string, itemList []ProviderItem) error {
	for _, item := range itemList {
		if item.APIID == "" {
			return errors.Errorf("api_id is required, provider type: %s, route_prefix:%s", providerType, item.RoutePrefix)
		}

		if item.APIKey == "" {
			return errors.Errorf("api_key is required, provider type: %s, route_prefix:%s", providerType, item.RoutePrefix)
		}

		if len(item.Subscriptions) == 0 {
			return errors.Errorf("subscriptions is required, provider type: %s, route_prefix:%s", providerType, item.RoutePrefix)
		}

		defaultConfig := a.loadNowConfig(item)
		for _, sub := range item.Subscriptions {
			reqPath, err := url.JoinPath(item.RoutePrefix, sub)
			if err != nil {
				return errors.Wrap(err, "failed to join path")
			}

			_, exist := a.PathToConfig[reqPath]
			if exist {
				return errors.Errorf("duplicate route prefix: %s", item.RoutePrefix)
			}

			a.PathToConfig[reqPath] = PathConfig{
				ProviderType:  providerType,
				APIID:         item.APIID,
				APIKey:        item.APIKey,
				Filename:      sub,
				DefaultConfig: defaultConfig,
			}
		}
	}

	return nil
}

// 处理缓存配置的覆盖
func applyCacheOverrides(dst *CacheConfig, src *CacheConfig) {
	if src == nil {
		return
	}

	if src.ResponseTTL != nil {
		dst.ResponseTTL = src.ResponseTTL
	}

	if src.FileTTL != nil {
		dst.FileTTL = src.FileTTL
	}

	if src.APITTL != nil {
		dst.APITTL = src.APITTL
	}
}

// 处理提供商配置的覆盖
func applyProviderOverrides(dst *ProviderConfig, src *ProviderConfig) {
	if src == nil {
		return
	}

	if src.UpdateInterval != nil && *src.UpdateInterval != 0 {
		dst.UpdateInterval = src.UpdateInterval
	}

	if src.RequestTimeout != nil && *src.RequestTimeout != 0 {
		dst.RequestTimeout = src.RequestTimeout
	}
}

// 处理使用显示配置的覆盖
func applyUsageDisplayOverrides(dst *UsageDisplayConfig, src *UsageDisplayConfig) {
	if src == nil || !src.Enable {
		return
	}

	dst.Enable = src.Enable
	dst.Prepend = src.Prepend

	if src.TrafficFormat != "" {
		dst.TrafficFormat = src.TrafficFormat
	}

	if src.TrafficUnit != "" {
		dst.TrafficUnit = src.TrafficUnit
	}

	if src.ExpireFormat != "" {
		dst.ExpireFormat = src.ExpireFormat
	}
}

func (a *AppConfig) loadNowConfig(item ProviderItem) *DefaultConfig {
	res := &DefaultConfig{
		Cache:        new(CacheConfig),
		Provider:     new(ProviderConfig),
		UsageDisplay: new(UsageDisplayConfig),
	}

	*res.Cache = *a.Defaults.Cache
	*res.Provider = *a.Defaults.Provider
	*res.UsageDisplay = *a.Defaults.UsageDisplay

	if item.Overrides == nil {
		return res
	}

	applyCacheOverrides(res.Cache, item.Overrides.Cache)
	applyProviderOverrides(res.Provider, item.Overrides.Provider)
	applyUsageDisplayOverrides(res.UsageDisplay, item.Overrides.UsageDisplay)
	return res
}
