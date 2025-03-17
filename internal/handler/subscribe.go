package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	emoji "github.com/Andrew-M-C/go.emoji"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gocache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"gopkg.in/yaml.v2"

	"vpsub/pkg/bytesize"
	"vpsub/pkg/config"
	"vpsub/pkg/provider"
	"vpsub/pkg/provider/bandwagonhost"
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

// 返回：emoji:id 的映射
func marshalEmoji(allSettings map[string]any) (map[string]string, error) {
	by, err := json.Marshal(allSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal allSettings")
	}

	data := string(by)

	emojiKv := make(map[string]string)
	final := emoji.ReplaceAllEmojiFunc(data, func(emoji string) string {
		id, ok := emojiKv[emoji]
		if ok {
			return id
		}

		id = "{{.%EMOJI%}}" + uuid.NewString() + "{{.%EMOJI%}}"

		emojiKv[emoji] = id

		return id
	})

	err = json.Unmarshal([]byte(final), &allSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal allSettings")
	}

	return emojiKv, nil
}

func unmarshalEmoji(by []byte, emojiKv map[string]string) []byte {
	data := string(by)

	for em, id := range emojiKv {
		data = strings.ReplaceAll(data, id, em)
	}

	return []byte(data)
}

func buildGroupInfo(name string) any {
	kv := map[string]any{
		"name": name,
		"type": "select",
		"proxies": []string{
			"REJECT",
		},
	}

	//by, err := yaml.Marshal(kv)
	//if err != nil {
	//	return ""
	//}
	return kv
}

func (h *SubscribeHandler) buildUsageGroup(info *ResponseCacheInfo) error {
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

	// "📅 重置日期 {{.year}}-{{.month}}-{{.day}}"
	t := time.Unix(info.CacheAPI.Expire, 0)
	expireFormat := h.appConfig.Global.UsageDisplay.ExpireFormat
	expireFormat = strings.ReplaceAll(expireFormat, "{{.year}}", t.Format("2006"))
	expireFormat = strings.ReplaceAll(expireFormat, "{{.month}}", t.Format("01"))
	expireFormat = strings.ReplaceAll(expireFormat, "{{.day}}", t.Format("02"))
	expireFormat = strings.ReplaceAll(expireFormat, "{{.hour}}", t.Format("15"))
	expireFormat = strings.ReplaceAll(expireFormat, "{{.minute}}", t.Format("04"))
	expireFormat = strings.ReplaceAll(expireFormat, "{{.second}}", t.Format("05"))

	// "⛽ 已用流量 {{.used}} / {{.total}}"
	trafficFormat := h.appConfig.Global.UsageDisplay.TrafficFormat
	trafficUnit := h.appConfig.Global.UsageDisplay.TrafficUnit
	used := bytesize.Format(info.CacheAPI.Download+info.CacheAPI.Upload, trafficUnit)
	total := bytesize.Format(info.CacheAPI.Total, trafficUnit)
	trafficFormat = strings.ReplaceAll(trafficFormat, "{{.used}}", used)
	trafficFormat = strings.ReplaceAll(trafficFormat, "{{.total}}", total)

	// 插入开头
	if h.appConfig.Global.UsageDisplay.Prepend {
		groupList = slices.Insert(groupList, 0, buildGroupInfo(expireFormat))
		groupList = slices.Insert(groupList, 0, buildGroupInfo(trafficFormat))
	} else {
		groupList = append(groupList, buildGroupInfo(trafficFormat))
		groupList = append(groupList, buildGroupInfo(expireFormat))
	}

	v.Set("proxy-groups", groupList)

	allSettings := v.AllSettings()
	emojiKv, err := marshalEmoji(allSettings)
	if err != nil {
		return errors.Wrap(err, "failed to emoji2ID")
	}

	by, err := yaml.Marshal(allSettings)
	if err != nil {
		return errors.Wrap(err, "failed to marshal allSettings")
	}

	info.CacheFile = unmarshalEmoji(by, emojiKv)

	return nil
}

func (h *SubscribeHandler) loadResponse(ctx context.Context, conf config.PathConfig, path string) (any, error) {
	needCacheResponse := *conf.Cache.ResponseTTL != 0
	needCacheFile := *conf.Cache.FileTTL != 0
	needCacheAPI := *conf.Cache.APITTL != 0
	responseKey := "response:" + path
	fileKey := "file:" + path
	apiKey := "api:" + path

	if needCacheResponse {
		cacheResponse, ok := h.cache.Get(responseKey)
		if ok {
			h.logger.WithContext(ctx).Debug("cache hit", zap.String("key", responseKey))
			return cacheResponse, nil
		}
	}

	newResponse := new(ResponseCacheInfo)

	if needCacheFile {
		cacheFileAny, ok := h.cache.Get(fileKey)
		if ok {
			h.logger.WithContext(ctx).Debug("cache hit", zap.String("key", fileKey))
			newResponse.CacheFile = cacheFileAny.([]byte)
		}
	}

	if needCacheAPI {
		cacheAPIAny, ok := h.cache.Get(apiKey)
		if ok {
			h.logger.WithContext(ctx).Debug("cache hit", zap.String("key", apiKey))
			newResponse.CacheAPI = cacheAPIAny.(*provider.APIResponseInfo)
		}
	}

	var err error
	if newResponse.CacheAPI == nil {
		switch conf.ProviderType {
		case "bandwagonhost":
			client := bandwagonhost.New(conf)
			newResponse.CacheAPI, err = client.GetServiceInfo(ctx)
		}

		if err != nil {
			h.logger.WithContext(ctx).Error("failed to get service info", zap.Error(err))
			return nil, errors.Wrap(err, "failed to get service info")
		}

		if needCacheAPI {
			h.cache.Set(apiKey, newResponse.CacheAPI, *conf.Cache.APITTL)
		}
	}

	if newResponse.CacheFile == nil {
		newResponse.CacheFile, err = os.ReadFile(filepath.Join(h.appConfig.Global.Storage.SubscriptionDir, conf.Filename))
		if err != nil {
			h.logger.WithContext(ctx).Error("failed to read file", zap.Error(err))
			return nil, errors.Wrap(err, "failed to read file")
		}

		// 添加失败的话忽略掉返回错误
		err = h.buildUsageGroup(newResponse)
		if err != nil {
			h.logger.WithContext(ctx).Error("failed to build usage group", zap.Error(err))
			//return nil, errors.Wrap(err, "failed to build usage group")
		}

		if needCacheFile {
			h.cache.Set(fileKey, newResponse.CacheFile, *conf.Cache.FileTTL)
		}
	}

	if needCacheResponse {
		h.cache.Set(responseKey, newResponse, *conf.Cache.ResponseTTL)
	}

	return newResponse, nil
}

func (h *SubscribeHandler) buildResponse(c *gin.Context, conf config.PathConfig, info *ResponseCacheInfo) {
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
		return h.loadResponse(c, conf, path)
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
	h.buildResponse(c, conf, info)
}
