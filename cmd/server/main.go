package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/djx30103/vpsub/internal/config"
	"github.com/djx30103/vpsub/internal/server"
	"github.com/djx30103/vpsub/pkg/app"
	"github.com/djx30103/vpsub/pkg/log"
)

var (
	appName = "subscribe"
	version = "v1.0.0"
	id, _   = os.Hostname()
)

func newApp(
	hs *server.HTTPServer,
) *app.App {
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
	configPath, err := config.ResolveConfigPath(flag.CommandLine, os.Args[1:], os.Getenv)
	if err != nil {
		panic(err)
	}
	printConfigPath(os.Stderr, configPath)

	rootConfig, err := config.Load(configPath)
	if err != nil {
		panic(err)
	}

	conf, err := config.BuildRuntime(rootConfig)
	if err != nil {
		panic(err)
	}

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

// printConfigPath 用于在日志系统初始化前输出实际使用的配置文件路径。
// 参数含义：writer 为输出目标；configPath 为最终使用的配置文件路径。
// 返回值：无。
func printConfigPath(writer io.Writer, configPath string) {
	_, _ = fmt.Fprintf(writer, "using config path: %s\n", configPath)
}
