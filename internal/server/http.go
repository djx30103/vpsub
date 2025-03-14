package server

import (
	"context"
	"errors"
	"net"
	"net/http"

	"vpsub/pkg/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"vpsub/internal/middleware"
	"vpsub/internal/server/router"
	"vpsub/pkg/log"
)

type HTTPServer struct {
	*http.Server
	logger *log.Logger
}

func (s *HTTPServer) Start(ctx context.Context) error {
	s.BaseContext = func(net.Listener) context.Context {
		return ctx
	}
	s.logger.Info("[HTTP] server start", zap.String("addr", s.Addr))
	err := s.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	s.logger.Info("[HTTP] server stop")
	return s.Shutdown(ctx)
}

func NewHTTPServer(
	logger *log.Logger,
	conf *config.AppConfig,
	router *router.Router,
) *HTTPServer {

	gin.SetMode(conf.AppMode)
	r := gin.New()
	r.Use(
		gin.Recovery(),
		middleware.Logger(logger),                 // 日志
		middleware.CORS(),                         // 跨域
		middleware.TimeoutMiddleware(conf.Server), // 超时
	)

	router.Register(r)

	server := &http.Server{
		Addr:    conf.Server.ListenAddr,
		Handler: r,
	}

	return &HTTPServer{
		Server: server,
		logger: logger,
	}
}
