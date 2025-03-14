package main

import (
	"os"

	"vpsub/internal/server"
	"vpsub/pkg/app"
	"vpsub/pkg/config"
	"vpsub/pkg/log"
)

var (
	appName = "subscribe"
	version = "v1.0.0"
	id, _   = os.Hostname()
)

func newApp(
	hs *server.HTTPServer) *app.App {
	return app.New(
		app.ID(id),
		app.Name(appName),
		app.Version(version),
		app.Servers(
			hs,
		),
	)
}

//	@title		swagger 接口文档
//	@version	2.0
//	@description

//	@license.name	MIT

//	@securityDefinitions.apikey	Authorization
//	@in							header
//	@name						Authorization

// @BasePath	/
func main() {
	conf := config.NewConfig()

	logger := log.NewLog(conf.Log)

	application, cleanup, err := NewApp(conf, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	err = application.Run()
	if err != nil {
		panic(err)
	}
}
