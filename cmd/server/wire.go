//go:build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/djx30103/vpsub/internal/config"
	"github.com/djx30103/vpsub/internal/handler"
	"github.com/djx30103/vpsub/internal/server"
	"github.com/djx30103/vpsub/internal/server/router"
	"github.com/djx30103/vpsub/pkg/app"
	"github.com/djx30103/vpsub/pkg/log"
)

var DataSet = wire.NewSet()

var ServerSet = wire.NewSet(
	server.NewHTTPServer,
)

var RouterSet = wire.NewSet(
	router.NewRouter,
)

var UserHandlerSet = wire.NewSet(
	handler.NewHandler,
	handler.NewSubscribeHandler,
)

func NewApp(*config.AppConfig, *log.Logger) (*app.App, func(), error) {
	panic(wire.Build(
		// DataSet,
		ServerSet,
		RouterSet,
		UserHandlerSet,
		newApp,
	))
}
