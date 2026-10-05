package middleware

import (
	"errors"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
)

const (
	ContextUserID = "auth_user_id"
	ContextRole   = "auth_role"
)

func RequireAuth(secret string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			abortUnauthorized(ctx)
			return
		}

		claims, err := auth.ParseToken(parts[1], secret)
		if err != nil {
			abortUnauthorized(ctx)
			return
		}

		ctx.Set(ContextUserID, claims.UserID)
		ctx.Set(ContextRole, claims.Role)
		ctx.Next()
	}
}

func RequireRole(allowedRoles ...model.Role) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		role, exists := ctx.Get(ContextRole)
		if !exists {
			abortUnauthorized(ctx)
			return
		}

		currentRole, ok := role.(model.Role)
		if !ok {
			abortUnauthorized(ctx)
			return
		}

		for _, allowedRole := range allowedRoles {
			if currentRole == allowedRole {
				ctx.Next()
				return
			}
		}

		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "اجازه دسترسی به این بخش را ندارید."})
	}
}

func abortUnauthorized(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "ورود به سامانه الزامی است."})
}

// JWTs identify the user; the current database row authorizes every request.
func RequireActiveUser(db *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id, exists := ctx.Get(ContextUserID)
		if !exists {
			abortUnauthorized(ctx)
			return
		}
		var user model.User
		err := db.First(&user, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !user.IsActive) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "حساب کاربری غیرفعال است یا در دسترس نیست."})
			return
		}
		if err != nil {
			log.Printf("current account lookup failed: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "خطایی در سرور رخ داد."})
			return
		}
		ctx.Set(ContextRole, user.Role)
		ctx.Set("auth_current_user", user)
		ctx.Next()
	}
}

// Register personal/status endpoints before this middleware. All subsequent
// business routes, including shared company/file routes, require verification.
func RequireVerifiedUniversity() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		value, exists := ctx.Get("auth_current_user")
		user, ok := value.(model.User)
		if !exists || !ok {
			abortUnauthorized(ctx)
			return
		}
		if user.Role == model.RoleUniversitySupervisor && user.VerificationStatus != model.UserVerificationApproved {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "UNIVERSITY_VERIFICATION_REQUIRED", "error": "دسترسی به امکانات دانشگاه نیازمند تأیید احراز هویت مدیر سیستم است."})
			return
		}
		ctx.Next()
	}
}
