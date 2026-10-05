package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/service"
)

type ProfileHandler struct{ service *service.ProfileService }

type profileUpdateRequest struct {
	FullName string  `json:"fullName"`
	Phone    string  `json:"phone"`
	JobTitle *string `json:"jobTitle"`
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func NewProfileHandler(profileService *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{service: profileService}
}

// Follow the strict request pattern used for workflow/rating mutations.
func decodeProfileRequest(ctx *gin.Context, request any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil {
		return false
	}
	return decoder.Decode(new(any)) == io.EOF
}

func (handler *ProfileHandler) Update(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	var request profileUpdateRequest
	if !decodeProfileRequest(ctx, &request) {
		writeProfileError(ctx, service.ErrInvalidProfile)
		return
	}
	user, err := handler.service.Update(userID, service.ProfileInput{FullName: request.FullName, Phone: request.Phone, JobTitle: request.JobTitle})
	if err != nil {
		writeProfileError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, user)
}

func (handler *ProfileHandler) ChangePassword(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	var request passwordChangeRequest
	if !decodeProfileRequest(ctx, &request) {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_PASSWORD_REQUEST", "error": "فقط رمز عبور فعلی و رمز عبور جدید را ارسال کنید."})
		return
	}
	if err := handler.service.ChangePassword(userID, request.CurrentPassword, request.NewPassword); err != nil {
		writeProfileError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "رمز عبور با موفقیت تغییر کرد."})
}

func writeProfileError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "ورود به سامانه الزامی است."})
	case errors.Is(err, service.ErrInvalidProfile):
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_PROFILE", "error": "نام و نام خانوادگی الزامی و حداکثر ۲۰۰ نویسه است. شماره تماس حداکثر ۵۰ نویسه و عنوان شغلی فقط برای سرپرست شرکت و حداکثر ۲۰۰ نویسه است. فقط فیلدهای قابل ویرایش را ارسال کنید."})
	case errors.Is(err, service.ErrInvalidNewPassword):
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_NEW_PASSWORD", "error": "رمز عبور جدید نباید خالی باشد و باید حداکثر ۷۲ بایت باشد."})
	case errors.Is(err, service.ErrWrongCurrentPassword):
		// A business error keeps the valid JWT session intact on the frontend.
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "WRONG_CURRENT_PASSWORD", "error": "رمز عبور فعلی نادرست است."})
	case errors.Is(err, service.ErrPasswordUnchanged):
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "PASSWORD_UNCHANGED", "error": "رمز عبور جدید باید با رمز عبور فعلی متفاوت باشد."})
	default:
		log.Printf("profile operation failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "خطایی در سرور رخ داد."})
	}
}
