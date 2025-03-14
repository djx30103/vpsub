package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"vpsub/pkg/provider"

	gocache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"vpsub/pkg/config"
	"vpsub/pkg/provider/bandwagonhost"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
)

type SubscribeHandler struct {
	*Handler
	sfGroup   *singleflight.Group
	appConfig *config.AppConfig

	cache *gocache.Cache
}

type ResponseCacheInfo struct {
	CacheFile []byte
	CacheAPI  *provider.APIResponseInfo
}

func NewSubscribeHandler(
	handler *Handler,
	appConfig *config.AppConfig,
) *SubscribeHandler {
	return &SubscribeHandler{
		Handler:   handler,
		appConfig: appConfig,
		sfGroup:   new(singleflight.Group),
		cache:     gocache.New(gocache.NoExpiration, time.Second),
	}
}

func (h *SubscribeHandler) Get(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		h.logger.Debug("path not found", zap.String("path", path))
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	conf, ok := h.appConfig.PathToConfig[path]
	if !ok {
		h.logger.Debug("path not found", zap.String("path", path))
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	res, err, _ := h.sfGroup.Do(path, func() (interface{}, error) {
		needCacheResponse := *conf.Cache.ResponseTTL != 0
		needCacheFile := *conf.Cache.FileTTL != 0
		needCacheAPI := *conf.Cache.APITTL != 0
		responseKey := "response:" + path
		fileKey := "file:" + path
		apiKey := "api:" + path

		if needCacheResponse {
			cacheResponse, ok := h.cache.Get(responseKey)
			if ok {
				h.logger.Debug("cache hit", zap.String("key", responseKey))
				return cacheResponse, nil
			}
		}

		newResponse := new(ResponseCacheInfo)

		if needCacheFile {
			cacheFileAny, ok := h.cache.Get(fileKey)
			if ok {
				h.logger.Debug("cache hit", zap.String("key", fileKey))
				newResponse.CacheFile = cacheFileAny.([]byte)
			}
		}

		if needCacheAPI {
			cacheAPIAny, ok := h.cache.Get(apiKey)
			if ok {
				h.logger.Debug("cache hit", zap.String("key", apiKey))
				newResponse.CacheAPI = cacheAPIAny.(*provider.APIResponseInfo)
			}
		}

		var err error
		if newResponse.CacheFile == nil {
			newResponse.CacheFile, err = os.ReadFile(filepath.Join(h.appConfig.Storage.SubscriptionDir, conf.Filename))
			if err != nil {
				h.logger.Error("failed to read file", zap.Error(err))
				return nil, errors.Wrap(err, "failed to read file")
			}

			if needCacheFile {
				h.cache.Set(fileKey, newResponse.CacheFile, *conf.Cache.FileTTL)
			}
		}

		if newResponse.CacheAPI == nil {
			switch conf.ProviderType {
			case "bandwagonhost":
				client := bandwagonhost.New(conf)
				newResponse.CacheAPI, err = client.GetServiceInfo(c)
			}

			if err != nil {
				h.logger.Error("failed to get service info", zap.Error(err))
				return nil, errors.Wrap(err, "failed to get service info")
			}

			if needCacheAPI {
				h.cache.Set(apiKey, newResponse.CacheAPI, *conf.Cache.APITTL)
			}
		}

		if needCacheResponse {
			h.cache.Set(responseKey, newResponse, *conf.Cache.ResponseTTL)
		}

		return newResponse, nil
	})

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	info := res.(*ResponseCacheInfo)
	cacheAPI := info.CacheAPI
	subInfo := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", cacheAPI.Upload, cacheAPI.Download, cacheAPI.Total, cacheAPI.Expire)

	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	c.Header("Subscription-Updated-At", strconv.FormatInt(time.Now().Unix(), 10))
	c.Header("Subscription-Userinfo", subInfo)
	c.Header("Profile-Update-Interval", strconv.FormatFloat(conf.Provider.UpdateInterval.Hours(), 'f', 2, 64))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=utf-8''%s", url.PathEscape(conf.Filename)))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", info.CacheFile)
}
