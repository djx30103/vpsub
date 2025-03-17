package handler

import (
	"vpsub/pkg/config"

	"vpsub/pkg/log"
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
