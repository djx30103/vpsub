package router

import (
	"vpsub/pkg/config"

	"vpsub/internal/handler"
	"vpsub/pkg/log"

	"github.com/gin-gonic/gin"
)

type Router struct {
	logger           *log.Logger
	subscribeHandler *handler.SubscribeHandler
	conf             *config.AppConfig
}

func NewRouter(
	logger *log.Logger,
	subscribeHandler *handler.SubscribeHandler,
	conf *config.AppConfig,
) *Router {
	return &Router{
		logger:           logger,
		subscribeHandler: subscribeHandler,
		conf:             conf,
	}
}

func (r *Router) Register(engine *gin.Engine) {
	{
		engine.GET("/*path", r.subscribeHandler.Get)
	}
}
