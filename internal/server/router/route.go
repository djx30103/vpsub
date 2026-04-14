package router

import (
	"github.com/gin-gonic/gin"

	"github.com/djx30103/vpsub/internal/config"
	"github.com/djx30103/vpsub/internal/handler"
	"github.com/djx30103/vpsub/pkg/log"
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
