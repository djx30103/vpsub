package handler

import (
	"github.com/djx30103/vpsub/internal/config"
	"github.com/djx30103/vpsub/pkg/log"
)

type Handler struct {
	logger *log.Logger
	conf   *config.AppConfig
}

func NewHandler(logger *log.Logger, conf *config.AppConfig) *Handler {
	return &Handler{
		logger: logger,
		conf:   conf,
	}
}
