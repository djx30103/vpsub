package router

import (
	"github.com/gin-gonic/gin"

	"github.com/djx30103/vpsub/internal/handler"
)

type Router struct {
	subscribeHandler *handler.SubscribeHandler
}

func NewRouter(
	subscribeHandler *handler.SubscribeHandler,
) *Router {
	return &Router{
		subscribeHandler: subscribeHandler,
	}
}

func (r *Router) Register(engine *gin.Engine) {
	{
		engine.GET("/*path", r.subscribeHandler.Get)
	}
}
