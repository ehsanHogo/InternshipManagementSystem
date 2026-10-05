package middleware

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func RequireApprovedCompany(db *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID, ok := ctx.Get(ContextUserID)
		id, valid := userID.(uint)
		if !ok || !valid {
			abortUnauthorized(ctx)
			return
		}
		if _, err := service.ApprovedCompanyID(db, id); err != nil {
			if errors.Is(err, service.ErrCompanyRegistrationRequired) || errors.Is(err, service.ErrOpportunityCompanyMissing) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "COMPANY_REGISTRATION_NOT_APPROVED", "error": "ثبت شرکت باید توسط مدیر سیستم تأیید شود."})
			} else {
				log.Printf("company access check failed: %v", err)
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "error": "خطایی در سرور رخ داد."})
			}
			return
		}
		ctx.Next()
	}
}

// Shared business routes keep other roles unchanged while enforcing company access.
func RequireApprovedCompanyIfSupervisor(db *gorm.DB) gin.HandlerFunc {
	guard := RequireApprovedCompany(db)
	return func(ctx *gin.Context) {
		role, _ := ctx.Get(ContextRole)
		if role == model.RoleCompanySupervisor {
			guard(ctx)
			return
		}
		ctx.Next()
	}
}
