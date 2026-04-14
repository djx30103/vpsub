package handler

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/djx30103/vpsub/internal/config"
	"github.com/djx30103/vpsub/pkg/provider"
	"github.com/djx30103/vpsub/pkg/provider/base"
)

// getProviderInfo 用于获取服务商流量信息，内部集成 singleflight 去重、API 缓存写入和失败降级。
// 参数含义：ctx 为请求上下文；conf 为当前路径配置。
// 返回值：返回流量信息，API 失败且无缓存时返回 nil。
func (h *SubscribeHandler) getProviderInfo(ctx context.Context, conf config.PathConfig) *base.APIResponseInfo {
	res, err, _ := h.sfGroup.Do(conf.ProviderRef, func() (interface{}, error) {
		if conf.ProviderConfig.APITTL != 0 {
			if cached, ok := h.cache.Get(conf.ProviderRef); ok {
				return cached.(*base.APIResponseInfo), nil
			}
		}

		client, err := provider.NewProvider(base.APIRequestInfo{
			APIID:          conf.APIID,
			APIKey:         conf.APIKey,
			ProviderType:   conf.ProviderType,
			RequestTimeout: conf.ProviderConfig.RequestTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to new provider: %w", err)
		}

		info, err := client.GetServiceInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get service info: %w", err)
		}

		if conf.ProviderConfig.APITTL != 0 {
			h.cache.Set(conf.ProviderRef, info, conf.ProviderConfig.APITTL)
		}

		return info, nil
	})

	if err != nil {
		// 上游接口失败时保留原始订阅文件返回，只记录降级原因，方便排查问题。
		h.logger.WithContext(ctx).Warn("failed to get provider info, fallback to raw subscription file", zap.String("path", conf.Path), zap.String("provider_ref", conf.ProviderRef), zap.Error(err))
	}

	if res != nil {
		return res.(*base.APIResponseInfo)
	}

	// API 失败时降级为最近一次成功缓存，尽量保证订阅接口继续可用。
	if cached, ok := h.cache.Get(conf.ProviderRef); ok {
		return cached.(*base.APIResponseInfo)
	}

	return nil
}
