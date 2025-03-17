//go:build wireinject
// +build wireinject

package main

import (
	"vpsub/internal/handler"
	"vpsub/internal/server"
	"vpsub/internal/server/router"
	"vpsub/pkg/app"
	"vpsub/pkg/config"
	"vpsub/pkg/log"

	"github.com/google/wire"
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
		//DataSet,
		ServerSet,
		RouterSet,
		UserHandlerSet,
		newApp,
	))
}
