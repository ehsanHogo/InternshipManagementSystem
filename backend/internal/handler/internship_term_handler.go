package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type InternshipTermHandler struct {
	service *service.InternshipTermService
}

func NewInternshipTermHandler(termService *service.InternshipTermService) *InternshipTermHandler {
	return &InternshipTermHandler{service: termService}
}

func (handler *InternshipTermHandler) RegisterUniversityRoutes(group *gin.RouterGroup) {
	group.GET("/internship-terms", handler.List)
	group.GET("/internship-terms/:id", handler.Detail)
	group.POST("/internship-terms", handler.Create)
	group.POST("/internship-terms/:id/close", handler.Close)
}

func (handler *InternshipTermHandler) Current(ctx *gin.Context) {
	term, err := handler.service.Current()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, term)
}

func (handler *InternshipTermHandler) List(ctx *gin.Context) {
	actorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	terms, err := handler.service.List(actorID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, terms)
}

func (handler *InternshipTermHandler) Detail(ctx *gin.Context) {
	actorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	termID, ok := termIDFromContext(ctx)
	if !ok {
		return
	}
	term, cases, err := handler.service.Detail(actorID, termID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]internshipCaseResponse, 0, len(cases))
	for i := range cases {
		response = append(response, caseResponse(&cases[i]))
	}
	ctx.JSON(http.StatusOK, gin.H{"term": term, "cases": response})
}

func (handler *InternshipTermHandler) Create(ctx *gin.Context) {
	actorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	var request struct {
		AcademicYear int                      `json:"academicYear"`
		TermType     model.InternshipTermType `json:"termType"`
	}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		handler.writeError(ctx, service.ErrInvalidTerm)
		return
	}
	term, err := handler.service.Create(actorID, request.AcademicYear, request.TermType)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, term)
}

func (handler *InternshipTermHandler) Close(ctx *gin.Context) {
	actorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	termID, ok := termIDFromContext(ctx)
	if !ok {
		return
	}
	term, err := handler.service.Close(actorID, termID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, term)
}

func termIDFromContext(ctx *gin.Context) (uint, bool) {
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_INTERNSHIP_TERM", "error": "شناسه ترم کارآموزی معتبر نیست."})
		return 0, false
	}
	return id, true
}

func (handler *InternshipTermHandler) writeError(ctx *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "INTERNSHIP_TERM_REQUEST_FAILED", "انجام عملیات ترم کارآموزی امکان‌پذیر نیست."
	switch {
	case errors.Is(err, service.ErrTermNotFound):
		status, code, message = http.StatusNotFound, "INTERNSHIP_TERM_NOT_FOUND", "ترم کارآموزی یافت نشد."
	case errors.Is(err, service.ErrInvalidTerm):
		status, code, message = http.StatusBadRequest, "INVALID_INTERNSHIP_TERM", "سال تحصیلی و نوع ترم کارآموزی معتبر وارد کنید."
	case errors.Is(err, service.ErrDuplicateTerm):
		status, code, message = http.StatusConflict, "INTERNSHIP_TERM_ALREADY_EXISTS", "این ترم برای سال تحصیلی انتخاب‌شده قبلاً ثبت شده است."
	case errors.Is(err, service.ErrTermAlreadyOpen):
		status, code, message = http.StatusConflict, "INTERNSHIP_TERM_ALREADY_OPEN", "یک ترم کارآموزی باز است. ابتدا ترم فعلی را ببندید."
	case errors.Is(err, service.ErrTermClosed):
		status, code, message = http.StatusConflict, "INTERNSHIP_TERM_CLOSED", "این ترم کارآموزی قبلاً بسته شده است."
	case errors.Is(err, service.ErrTermAccessDenied):
		status, code, message = http.StatusForbidden, "INTERNSHIP_TERM_ACCESS_DENIED", "مدیریت ترم کارآموزی فقط برای مسئول آموزش مجاز است."
	default:
		log.Printf("internship term request failed: %v", err)
	}
	ctx.JSON(status, gin.H{"code": code, "error": message})
}
