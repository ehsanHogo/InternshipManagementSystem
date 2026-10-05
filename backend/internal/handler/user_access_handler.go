package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type UserAccessHandler struct{ service *service.UserAccessService }

func NewUserAccessHandler(s *service.UserAccessService) *UserAccessHandler {
	return &UserAccessHandler{service: s}
}
func (h *UserAccessHandler) RegisterAdminRoutes(group *gin.RouterGroup) {
	group.GET("/user-verifications", h.ListVerifications)
	group.GET("/user-verifications/:id", h.VerificationDetail)
	group.POST("/user-verifications/:id/approve", h.Approve)
	group.POST("/user-verifications/:id/reject", h.Reject)
	group.GET("/users", h.ListUsers)
	group.GET("/users/:id", h.UserDetail)
	group.POST("/users/:id/enable", h.Enable)
	group.POST("/users/:id/disable", h.Disable)
}
func (h *UserAccessHandler) Register(ctx *gin.Context) {
	var input service.UniversityRegistrationInput
	if !decodeProfileRequest(ctx, &input) {
		writeUserAccessError(ctx, service.ErrInvalidUserAccess)
		return
	}
	user, err := h.service.Register(input)
	if err != nil {
		writeUserAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"user": user, "message": "ثبت‌نام انجام شد. برای مشاهده وضعیت احراز هویت وارد شوید."})
}
func (h *UserAccessHandler) Resubmit(ctx *gin.Context) {
	id, ok := currentUserID(ctx)
	if !ok {
		return
	}
	user, err := h.service.Resubmit(id)
	if err != nil {
		writeUserAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, user)
}
func (h *UserAccessHandler) ListVerifications(ctx *gin.Context) { h.list(ctx, true) }
func (h *UserAccessHandler) ListUsers(ctx *gin.Context)         { h.list(ctx, false) }
func (h *UserAccessHandler) list(ctx *gin.Context, verificationOnly bool) {
	var active *bool
	role := model.Role("")
	status := model.UserVerificationStatus("")
	if verificationOnly {
		status = model.UserVerificationStatus(ctx.Query("status"))
	} else {
		role = model.Role(ctx.Query("role"))
		if value := ctx.Query("isActive"); value != "" {
			if value != "true" && value != "false" {
				writeUserAccessError(ctx, service.ErrInvalidUserAccess)
				return
			}
			enabled := value == "true"
			active = &enabled
		}
	}
	users, err := h.service.List(verificationOnly, status, ctx.Query("search"), role, active)
	if err != nil {
		writeUserAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, users)
}
func (h *UserAccessHandler) VerificationDetail(ctx *gin.Context) { h.detail(ctx, true) }
func (h *UserAccessHandler) UserDetail(ctx *gin.Context)         { h.detail(ctx, false) }
func (h *UserAccessHandler) detail(ctx *gin.Context, verificationOnly bool) {
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		writeUserAccessError(ctx, service.ErrInvalidUserAccess)
		return
	}
	user, err := h.service.Detail(id, verificationOnly)
	if err != nil {
		writeUserAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, user)
}
func (h *UserAccessHandler) Approve(ctx *gin.Context) {
	h.review(ctx, model.UserVerificationApproved, "")
}
func (h *UserAccessHandler) Reject(ctx *gin.Context) {
	var input struct {
		Reason string `json:"reason"`
	}
	if !decodeUserReviewRequest(ctx, &input) {
		writeUserAccessError(ctx, service.ErrVerificationReason)
		return
	}
	h.review(ctx, model.UserVerificationRejected, input.Reason)
}
func (h *UserAccessHandler) review(ctx *gin.Context, status model.UserVerificationStatus, reason string) {
	adminID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		writeUserAccessError(ctx, service.ErrInvalidUserAccess)
		return
	}
	user, err := h.service.Review(adminID, id, status, reason)
	if err != nil {
		writeUserAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, user)
}
func (h *UserAccessHandler) Enable(ctx *gin.Context)  { h.setActive(ctx, true) }
func (h *UserAccessHandler) Disable(ctx *gin.Context) { h.setActive(ctx, false) }
func (h *UserAccessHandler) setActive(ctx *gin.Context, active bool) {
	adminID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		writeUserAccessError(ctx, service.ErrInvalidUserAccess)
		return
	}
	user, err := h.service.SetActive(adminID, id, active)
	if err != nil {
		writeUserAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, user)
}
func writeUserAccessError(ctx *gin.Context, err error) {
	status, message := http.StatusBadRequest, "اطلاعات درخواست معتبر نیست. نام، ایمیل معتبر و رمز عبور غیرخالی تا ۷۲ بایت الزامی است."
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		status, message = http.StatusNotFound, "کاربر مورد نظر یافت نشد."
	case errors.Is(err, service.ErrEmailAlreadyExists):
		status, message = http.StatusConflict, "این ایمیل قبلاً در سامانه ثبت شده است."
	case errors.Is(err, service.ErrVerificationTransition):
		status, message = http.StatusConflict, "این اقدام در وضعیت فعلی احراز هویت مجاز نیست."
	case errors.Is(err, service.ErrVerificationReason):
		message = "دلیل رد باید غیرخالی و حداکثر ۵۰۰۰ نویسه باشد."
	case errors.Is(err, service.ErrAdminSelfDisable):
		status, message = http.StatusConflict, "نمی‌توانید حساب خود را غیرفعال کنید."
	case errors.Is(err, service.ErrLastActiveAdmin):
		status, message = http.StatusConflict, "حداقل یک مدیر فعال باید در سامانه باقی بماند."
	case errors.Is(err, service.ErrUserAccessForbidden):
		status, message = http.StatusForbidden, "اجازه انجام این اقدام را ندارید."
	case errors.Is(err, service.ErrInvalidUserAccess):
	default:
		log.Printf("user access operation failed: %v", err)
		status, message = http.StatusInternalServerError, "خطایی در سرور رخ داد."
	}
	ctx.JSON(status, gin.H{"error": message})
}

func decodeUserReviewRequest(ctx *gin.Context, request any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil {
		return false
	}
	return decoder.Decode(new(any)) == io.EOF
}
