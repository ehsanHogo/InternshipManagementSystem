package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/service"
)

type HealthHandler struct {
	service *service.HealthService
}

func NewHealthHandler(service *service.HealthService) *HealthHandler {
	return &HealthHandler{service: service}
}

func (handler *HealthHandler) Get(ctx *gin.Context) {
	if err := handler.service.Check(ctx.Request.Context()); err != nil {
		log.Printf("health check failed: %v", err)
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
