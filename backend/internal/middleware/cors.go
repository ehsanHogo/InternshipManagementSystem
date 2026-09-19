package middleware

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS(frontendOrigin string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{frontendOrigin},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	})
}
