package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"internship-management-system/backend/internal/service"
)

type NotificationHandler struct{ service *service.NotificationService }

func NewNotificationHandler(s *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: s}
}
func (h *NotificationHandler) List(ctx *gin.Context) {
	id, ok := currentUserID(ctx)
	if !ok {
		return
	}
	notifications, err := h.service.List(id)
	if err != nil {
		writeNotificationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, notifications)
}
func (h *NotificationHandler) UnreadCount(ctx *gin.Context) {
	id, ok := currentUserID(ctx)
	if !ok {
		return
	}
	count, err := h.service.UnreadCount(id)
	if err != nil {
		writeNotificationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"count": count})
}
func (h *NotificationHandler) MarkViewed(ctx *gin.Context) {
	id, ok := currentUserID(ctx)
	if !ok {
		return
	}
	var request struct {
		NotificationIDs []uint `json:"notificationIds"`
	}
	if !decodeProfileRequest(ctx, &request) || request.NotificationIDs == nil {
		writeNotificationError(ctx, service.ErrInvalidNotificationIDs)
		return
	}
	if err := h.service.MarkViewed(id, request.NotificationIDs); err != nil {
		writeNotificationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "اعلان‌های نمایش‌داده‌شده خوانده شدند."})
}
func writeNotificationError(ctx *gin.Context, err error) {
	if errors.Is(err, service.ErrInvalidNotificationIDs) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه اعلان‌های نمایش‌داده‌شده را ارسال کنید (حداکثر ۱۰۰ مورد)."})
		return
	}
	log.Printf("notification operation failed: %v", err)
	ctx.JSON(http.StatusInternalServerError, gin.H{"error": "خطایی در سرور رخ داد."})
}
