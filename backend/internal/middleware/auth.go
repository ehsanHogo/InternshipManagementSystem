package middleware

import (
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

		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}

func abortUnauthorized(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
}
