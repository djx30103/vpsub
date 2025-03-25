package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"vpsub/pkg/provider"
	"vpsub/pkg/provider/base"
	"vpsub/pkg/xemoji"

	"github.com/gin-gonic/gin"
	gocache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"gopkg.in/yaml.v2"

	"vpsub/pkg/bytesize"
	"vpsub/pkg/config"
)

type SubscribeHandler struct {
	*Handler
	sfGroup   *singleflight.Group
	appConfig *config.AppConfig

	cache *gocache.Cache
}

type ResponseCacheInfo struct {
	CacheFile []byte
	CacheAPI  *base.APIResponseInfo
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

func createProxyGroup(name string) any {
	return map[string]any{
		"name": name,
		"type": "select",
		"proxies": []string{
			"REJECT",
		},
	}
}

func newAppendGroup(apiInfo *base.APIResponseInfo, usageDisplay config.UsageDisplayConfig) []any {
	groupList := make([]any, 0, 2)
	if apiInfo.Expire > 0 {
		// "📅 重置日期 {{.year}}-{{.month}}-{{.day}}"
		t := time.Unix(apiInfo.Expire, 0)
		expireFormat := strings.ReplaceAll(usageDisplay.ExpireFormat, "{{.year}}", t.Format("2006"))
		expireFormat = strings.ReplaceAll(expireFormat, "{{.month}}", t.Format("01"))
		expireFormat = strings.ReplaceAll(expireFormat, "{{.day}}", t.Format("02"))
		expireFormat = strings.ReplaceAll(expireFormat, "{{.hour}}", t.Format("15"))
		expireFormat = strings.ReplaceAll(expireFormat, "{{.minute}}", t.Format("04"))
		expireFormat = strings.ReplaceAll(expireFormat, "{{.second}}", t.Format("05"))
		groupList = append(groupList, createProxyGroup(expireFormat))
	}

	if apiInfo.Upload > 0 || apiInfo.Download > 0 || apiInfo.Total > 0 {
		// "⛽ 已用流量 {{.used}} / {{.total}}"
		trafficFormat := strings.ReplaceAll(usageDisplay.TrafficFormat, "{{.used}}", bytesize.Format(apiInfo.Download+apiInfo.Upload, usageDisplay.TrafficUnit))
		trafficFormat = strings.ReplaceAll(trafficFormat, "{{.total}}", bytesize.Format(apiInfo.Total, usageDisplay.TrafficUnit))
		groupList = append(groupList, createProxyGroup(trafficFormat))
	}

	return groupList
}

func (h *SubscribeHandler) appendUsageGroups(info *ResponseCacheInfo) error {
	if !h.appConfig.Global.UsageDisplay.Enable {
		return nil
	}

	v := viper.New()
	v.SetConfigType("yaml")
	err := v.ReadConfig(bytes.NewReader(info.CacheFile))
	if err != nil {
		return errors.Wrap(err, "failed to read yaml config")
	}

	groupList, ok := v.Get("proxy-groups").([]any)
	if !ok {
		return errors.New("no proxy-groups found in config")
	}

	if len(groupList) == 0 {
		return errors.New("no proxy-groups found in config")
	}

	appendGroupList := newAppendGroup(info.CacheAPI, h.appConfig.Global.UsageDisplay)
	if len(appendGroupList) == 0 {
		return errors.New("no usage groups found in config")
	}

	// 插入开头
	if h.appConfig.Global.UsageDisplay.Prepend {
		groupList = slices.Insert(groupList, 0, appendGroupList...)
	} else {
		groupList = append(groupList, appendGroupList...)
	}

	v.Set("proxy-groups", groupList)

	allSettings := v.AllSettings()
	emojiKv, err := xemoji.EncodeEmojiToID(allSettings)
	if err != nil {
		return errors.Wrap(err, "failed to emoji2ID")
	}

	by, err := yaml.Marshal(allSettings)
	if err != nil {
		return errors.Wrap(err, "failed to marshal allSettings")
	}

	info.CacheFile = xemoji.DecodeEmojiFromID(by, emojiKv)

	return nil
}

func (h *SubscribeHandler) fetchProviderInfo(ctx context.Context, conf config.PathConfig, cacheKey string) (*base.APIResponseInfo, error) {
	if *conf.Cache.APITTL != 0 {
		cacheAPIAny, ok := h.cache.Get(cacheKey)
		if ok {
			return cacheAPIAny.(*base.APIResponseInfo), nil
		}
	}

	client, err := provider.NewProvider(base.APIRequestInfo{
		APIID:          conf.APIID,
		APIKey:         conf.APIKey,
		ProviderType:   conf.ProviderType,
		RequestTimeout: *conf.Provider.RequestTimeout,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to new provider")
	}

	res, err := client.GetServiceInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service info")
	}

	return res, nil
}

func (h *SubscribeHandler) readSubscriptionFile(_ context.Context, conf config.PathConfig, cacheKey string) ([]byte, error) {
	if *conf.Cache.FileTTL != 0 {
		cacheFileAny, ok := h.cache.Get(cacheKey)
		if ok {
			return cacheFileAny.([]byte), nil
		}
	}

	res, err := os.ReadFile(filepath.Join(h.appConfig.Global.Storage.SubscriptionDir, conf.Filename))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	return res, nil
}

func (h *SubscribeHandler) prepareSubscriptionResponse(ctx context.Context, conf config.PathConfig, path string) (any, error) {

	responseCacheKey := "response:" + path
	apiCacheKey := "api:" + path
	fileKey := "file:" + path

	// 从缓存中读取
	if *conf.Cache.ResponseTTL != 0 {
		cacheResponse, ok := h.cache.Get(responseCacheKey)
		if ok {
			h.logger.WithContext(ctx).Debug("cache hit", zap.String("key", responseCacheKey))
			return cacheResponse, nil
		}
	}

	var (
		err error
		res = new(ResponseCacheInfo)
	)

	// 获取文件信息
	res.CacheFile, err = h.readSubscriptionFile(ctx, conf, fileKey)
	if err != nil {
		h.logger.WithContext(ctx).Error("failed to read file", zap.Error(err))
		return nil, errors.Wrap(err, "failed to read file")
	}

	res.CacheAPI, err = h.fetchProviderInfo(ctx, conf, apiCacheKey)
	if err != nil {
		h.logger.WithContext(ctx).Error("failed to get service info", zap.Error(err))
		return nil, errors.Wrap(err, "failed to get service info")
	}

	// 构建使用情况，忽略错误
	err = h.appendUsageGroups(res)
	if err != nil {
		h.logger.WithContext(ctx).Error("failed to build usage group", zap.Error(err))
		//return nil, errors.Wrap(err, "failed to build usage group")
	}

	if *conf.Cache.FileTTL != 0 {
		h.cache.Set(fileKey, res.CacheFile, *conf.Cache.FileTTL)
	}

	if *conf.Cache.APITTL != 0 {
		h.cache.Set(apiCacheKey, res.CacheAPI, *conf.Cache.APITTL)
	}

	if *conf.Cache.ResponseTTL != 0 {
		h.cache.Set(responseCacheKey, res, *conf.Cache.ResponseTTL)
	}

	return res, nil
}

func (h *SubscribeHandler) writeSubscriptionResponse(c *gin.Context, conf config.PathConfig, info *ResponseCacheInfo) {
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
		return h.prepareSubscriptionResponse(c, conf, path)
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
	h.writeSubscriptionResponse(c, conf, info)
}
