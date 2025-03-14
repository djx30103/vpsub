package middleware

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {

	return cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "OPTIONS", "PUT", "PATCH", "DELETE"},
		AllowHeaders: []string{
			"Content-Type",
			"X-Requested-With",
			"Access-Control-Allow-Credentials",
			"User-Agent",
			"Content-Length",
			"Authorization",
		},
		ExposeHeaders: []string{
			"Content-Type",
			"X-Requested-With",
			"Access-Control-Allow-Credentials",
			"User-Agent",
			"Content-Length",
			"Authorization",
		},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		MaxAge:                    24 * time.Hour,
		OptionsResponseStatusCode: http.StatusNoContent,
	})
}
