package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type ratedCaseResponse struct {
	internshipCaseResponse
	service.CompanyRating
	StudentRating     *int `json:"studentRating"`
	CanRateInternship bool `json:"canRateInternship"`
}

func (handler *InternshipHandler) RegisterStudentRatingRoutes(group *gin.RouterGroup) {
	group.POST("/internship-cases/:id/rating", appmiddleware.RequireRole(model.RoleStudent), handler.CreateStudentRating)
}

func (handler *InternshipHandler) CreateStudentRating(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	var request struct {
		Rating int `json:"rating"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		handler.writeRatingError(ctx, service.ErrInvalidStudentRating)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		handler.writeRatingError(ctx, service.ErrInvalidStudentRating)
		return
	}
	_, err := handler.service.CreateStudentRating(studentID, caseID, request.Rating)
	if err != nil {
		handler.writeRatingError(ctx, err)
		return
	}
	item, err := handler.service.GetStudentHistoricalCase(studentID, caseID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	handler.writeRatedCase(ctx, http.StatusCreated, item)
}

func (handler *InternshipHandler) writeRatingError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidStudentRating):
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_STUDENT_RATING", "error": "فقط یک امتیاز صحیح از ۱ تا ۵ ارسال کنید."})
	case errors.Is(err, service.ErrCaseNotRateable):
		ctx.JSON(http.StatusConflict, gin.H{"code": "INTERNSHIP_CASE_NOT_RATEABLE", "error": "فقط شرکت محل کارآموزی پرونده قبول‌شده قابل ارزیابی است."})
	case errors.Is(err, service.ErrStudentRatingExists):
		ctx.JSON(http.StatusConflict, gin.H{"code": "STUDENT_RATING_ALREADY_EXISTS", "error": "امتیاز این پرونده قبلاً ثبت شده و قابل تغییر نیست."})
	default:
		handler.writeError(ctx, err)
	}
}

func selectedCaseCompanyID(item *model.InternshipCase) uint {
	if item.SelectedPreference == nil || item.SelectedPreference.InternshipCaseID != item.ID ||
		item.SelectedPreference.OpportunityApplication.StudentID != item.StudentID {
		return 0
	}
	return item.SelectedPreference.OpportunityApplication.Opportunity.CompanyID
}

func (handler *InternshipHandler) writeRatedCase(ctx *gin.Context, status int, item *model.InternshipCase) {
	views, err := handler.ratedCaseViews([]model.InternshipCase{*item})
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(status, views[0])
}

func (handler *InternshipHandler) ratedCaseViews(items []model.InternshipCase) ([]ratedCaseResponse, error) {
	ids := make([]uint, 0, len(items))
	for i := range items {
		if id := selectedCaseCompanyID(&items[i]); id != 0 {
			ids = append(ids, id)
		}
	}
	ratings, err := handler.service.CompanyRatings(ids)
	if err != nil {
		return nil, err
	}
	views := make([]ratedCaseResponse, 0, len(items))
	for i := range items {
		item := &items[i]
		view := ratedCaseResponse{internshipCaseResponse: studentCaseResponse(item), CompanyRating: ratings[selectedCaseCompanyID(item)]}
		if item.StudentRating != nil {
			score := item.StudentRating.Rating
			view.StudentRating = &score
		}
		view.CanRateInternship = item.Status == model.InternshipCaseStatusPassed && item.StudentRating == nil && selectedCaseCompanyID(item) != 0
		views = append(views, view)
	}
	return views, nil
}

// University case details receive only company aggregates, with no student controls.
func (handler *InternshipHandler) writeUniversityRatedCase(ctx *gin.Context, item *model.InternshipCase) {
	ids := []uint{selectedCaseCompanyID(item)}
	for _, preference := range item.Preferences {
		ids = append(ids, preference.OpportunityApplication.Opportunity.CompanyID)
	}
	ratings, err := handler.service.CompanyRatings(ids)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	type ratedPreferenceResponse struct {
		internshipPreferenceResponse
		service.CompanyRating
	}
	preferences := make([]ratedPreferenceResponse, 0, len(item.Preferences))
	for _, preference := range item.Preferences {
		preferences = append(preferences, ratedPreferenceResponse{preferenceResponse(preference), ratings[preference.OpportunityApplication.Opportunity.CompanyID]})
	}
	ctx.JSON(http.StatusOK, struct {
		internshipCaseResponse
		service.CompanyRating
		Preferences []ratedPreferenceResponse `json:"preferences"`
	}{caseResponse(item), ratings[selectedCaseCompanyID(item)], preferences})
}
