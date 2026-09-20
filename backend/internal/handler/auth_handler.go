package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/auth"
	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
)

type AuthHandler struct {
	db            *gorm.DB
	jwtSecret     string
	tokenLifetime time.Duration
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func NewAuthHandler(db *gorm.DB, jwtSecret string, tokenLifetime time.Duration) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, tokenLifetime: tokenLifetime}
}

func (handler *AuthHandler) Login(ctx *gin.Context) {
	var request loginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
		return
	}

	var user model.User
	result := handler.db.Where("email = ?", strings.ToLower(strings.TrimSpace(request.Email))).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) || (result.Error == nil && !auth.CheckPassword(user.PasswordHash, request.Password)) {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "ایمیل یا رمز عبور نادرست است."})
		return
	}
	if result.Error != nil {
		log.Printf("login user lookup failed: %v", result.Error)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "خطایی در سرور رخ داد."})
		return
	}

	token, err := auth.CreateToken(user, handler.jwtSecret, handler.tokenLifetime)
	if err != nil {
		log.Printf("create login token: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "خطایی در سرور رخ داد."})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"token": token, "user": user.Public()})
}

func (handler *AuthHandler) Me(ctx *gin.Context) {
	userID, exists := ctx.Get(appmiddleware.ContextUserID)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "ورود به سامانه الزامی است."})
		return
	}

	var user model.User
	if err := handler.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "ورود به سامانه الزامی است."})
			return
		}
		log.Printf("current user lookup failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "خطایی در سرور رخ داد."})
		return
	}

	ctx.JSON(http.StatusOK, user.Public())
}

func (handler *AuthHandler) Protected(ctx *gin.Context) {
	userID, _ := ctx.Get(appmiddleware.ContextUserID)
	role, _ := ctx.Get(appmiddleware.ContextRole)
	ctx.JSON(http.StatusOK, gin.H{"message": "احراز هویت انجام شده است.", "userId": userID, "role": role})
}
