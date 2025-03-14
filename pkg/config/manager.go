package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"

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

	Provider ProviderConfig // 提供商相关配置
	Cache    CacheConfig    // 缓存相关配置
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

	err = conf.Validate()
	if err != nil {
		return conf, errors.Wrap(err, "failed to validate config file")
	}

	return conf, nil
}

// preprocessConfig 预处理配置，建立映射关系
func (a *AppConfig) preprocessConfig() error {

	for providerType, itemList := range a.Providers {
		switch providerType {
		case "bandwagonhost":
		default:
			return errors.New("unknown provider type")
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
		defaultConfig := a.loadDefaultConfig(item)
		for _, sub := range item.Subscriptions {
			reqPath, err := url.JoinPath(item.RoutePrefix, sub)
			if err != nil {
				return errors.Wrap(err, "failed to join path")
			}

			_, exist := a.PathToConfig[reqPath]
			if exist {
				return errors.New("duplicate subscription path")
			}

			a.PathToConfig[reqPath] = PathConfig{
				ProviderType: providerType,
				APIID:        item.APIID,
				APIKey:       item.APIKey,
				Filename:     sub,
				Provider:     *defaultConfig.Provider,
				Cache:        *defaultConfig.Cache,
			}
		}
	}

	return nil
}

func (a *AppConfig) loadDefaultConfig(item ProviderItem) DefaultConfig {
	if item.Overrides == nil {
		return a.Defaults
	}

	overrides := item.Overrides
	res := a.Defaults
	if overrides.Cache != nil {
		if overrides.Cache.ResponseTTL != nil {
			res.Cache.ResponseTTL = overrides.Cache.ResponseTTL
		}

		if overrides.Cache.FileTTL != nil {
			res.Cache.FileTTL = overrides.Cache.FileTTL
		}

		if overrides.Cache.APITTL != nil {
			res.Cache.APITTL = overrides.Cache.APITTL
		}
	}

	if overrides.Provider != nil {
		if overrides.Provider.UpdateInterval != nil {
			res.Provider.UpdateInterval = overrides.Provider.UpdateInterval
		}

		if overrides.Provider.RequestTimeout != nil {
			res.Provider.RequestTimeout = overrides.Provider.RequestTimeout
		}
	}

	return res
}
