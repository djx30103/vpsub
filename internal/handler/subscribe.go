package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	gocache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/djx30103/vpsub/internal/config"
	"github.com/djx30103/vpsub/pkg/pathutil"
	"github.com/djx30103/vpsub/pkg/provider"
	"github.com/djx30103/vpsub/pkg/provider/base"
)

type SubscribeHandler struct {
	*Handler
	sfGroup   *singleflight.Group
	appConfig *config.AppConfig

	cache     *gocache.Cache
	fileCache map[string]cachedSubscriptionFile
	fileMu    sync.RWMutex
}

// cachedSubscriptionFile 用于缓存订阅文件内容及其文件元信息，避免每次请求都重复读取未变化的文件。
// 字段含义：content 为文件内容；modTime 为文件最后修改时间；size 为文件大小。
type cachedSubscriptionFile struct {
	content []byte
	modTime time.Time
	size    int64
}

// NewSubscribeHandler 用于构造订阅处理器，并初始化去重与内存缓存组件。
// 参数含义：handler 为基础处理器依赖；appConfig 为应用配置。
// 返回值：返回可直接注册到路由上的订阅处理器。
func NewSubscribeHandler(
	handler *Handler,
	appConfig *config.AppConfig,
) *SubscribeHandler {
	return &SubscribeHandler{
		Handler:   handler,
		appConfig: appConfig,
		sfGroup:   new(singleflight.Group),
		cache:     gocache.New(gocache.NoExpiration, time.Second),
		fileCache: make(map[string]cachedSubscriptionFile),
	}
}

// writeSubscriptionResponse 用于写入最终订阅响应和相关响应头。
// 参数含义：c 为 Gin 上下文；conf 为当前路径配置；fileContent 为响应内容；apiInfo 为流量信息。
// 返回值：无。
func (h *SubscribeHandler) writeSubscriptionResponse(c *gin.Context, conf config.PathConfig, fileContent []byte, apiInfo *base.APIResponseInfo) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	c.Header("Subscription-Updated-At", strconv.FormatInt(time.Now().Unix(), 10))
	c.Header("Profile-Update-Interval", strconv.FormatFloat(conf.ProviderConfig.UpdateInterval.Hours(), 'f', 2, 64))
	// c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=utf-8''%s", url.PathEscape(path.Base(conf.File))))

	if apiInfo != nil {
		subInfo := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", apiInfo.Upload, apiInfo.Download, apiInfo.Total, apiInfo.Expire)
		c.Header("Subscription-Userinfo", subInfo)
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", fileContent)
}

// Get 用于处理订阅下载请求，并在路径命中后返回附带流量信息的订阅内容。
// 参数含义：c 为 Gin 上下文。
// 返回值：无。
func (h *SubscribeHandler) Get(c *gin.Context) {
	requestPath := pathutil.NormalizeRequestPath(c.Param("path"))
	if requestPath == "" {
		h.logger.Debug("path not found", zap.String("path", requestPath))
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	conf, ok := h.appConfig.PathToConfig[requestPath]
	if !ok {
		h.logger.Debug("path not found", zap.String("path", requestPath))
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// 配置了精确 UA 约束时，仅允许匹配的客户端访问，未匹配时直接按不存在处理。
	if conf.AccessControl != nil && conf.AccessControl.UserAgent != "" && c.GetHeader("User-Agent") != conf.AccessControl.UserAgent {
		h.logger.Debug("user agent not allowed", zap.String("path", requestPath), zap.String("user_agent", c.GetHeader("User-Agent")))
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	fileContent, err := h.readSubscriptionFile(conf)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		h.logger.WithContext(c).Error("failed to read subscription file", zap.Error(err))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var apiInfo *base.APIResponseInfo
	if conf.ProviderType != provider.ProviderType_Passthrough {
		apiInfo = h.getProviderInfo(c, conf)
	}

	if conf.UsageDisplay.Enable && apiInfo != nil {
		updated, appendErr := appendUsageGroups(fileContent, apiInfo, conf.UsageDisplay)
		if appendErr != nil {
			h.logger.WithContext(c).Error("failed to append usage groups", zap.Error(appendErr))
		} else {
			fileContent = updated
		}
	}

	h.writeSubscriptionResponse(c, conf, fileContent, apiInfo)
}
