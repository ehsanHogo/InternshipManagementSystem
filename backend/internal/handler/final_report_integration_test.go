package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/auth"
	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestFinalReportV2API(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.ProfessorAssignment{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}, &model.FinalReport{}); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	suffix := time.Now().UnixNano()
	createOpenTermFixture(t, tx, suffix)
	companyA := createOpportunityTestCompany(t, tx, suffix, "fra", true)
	companyB := createOpportunityTestCompany(t, tx, suffix, "frb", true)
	supervisor := createOpportunityTestUser(t, tx, suffix, "frsupervisor", model.RoleCompanySupervisor, &companyA.ID)
	unrelated := createOpportunityTestUser(t, tx, suffix, "frunrelated", model.RoleCompanySupervisor, &companyB.ID)
	professor := createOpportunityTestUser(t, tx, suffix, "frprofessor", model.RoleProfessor, nil)
	otherProfessor := createOpportunityTestUser(t, tx, suffix, "frotherprofessor", model.RoleProfessor, nil)
	university := createOpportunityTestUser(t, tx, suffix, "fruniversity", model.RoleUniversitySupervisor, nil)
	admin := createOpportunityTestUser(t, tx, suffix, "fradmin", model.RoleAdmin, nil)
	uploadDir := t.TempDir()
	workflow := service.NewInternshipService(tx)
	handler := NewInternshipHandler(workflow, uploadDir)
	const secret = "final-report-v2-integration"
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api", appmiddleware.RequireAuth(secret))
	students := api.Group("/student", appmiddleware.RequireRole(model.RoleStudent))
	handler.RegisterStudentFinalReportRoutes(students)
	students.GET("/internship-case", handler.GetCurrentCase)
	students.GET("/internship-cases/history", handler.ListStudentHistoricalCases)
	professors := api.Group("/professor", appmiddleware.RequireRole(model.RoleProfessor))
	handler.RegisterProfessorFinalReportRoutes(professors)
	professors.GET("/internship-cases/:id", handler.GetProfessorCase)
	professors.POST("/internship-cases/:id/complete", handler.CompleteProfessorCase)
	api.GET("/files/:id/download", handler.DownloadFile)
	token := func(user model.User) string {
		value, err := auth.CreateToken(user, secret, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	request := func(t *testing.T, method, path string, user model.User, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		bearer := ""
		if user.ID != 0 {
			bearer = token(user)
		}
		rec := performJSONRequest(t, router, method, path, body, bearer)
		if rec.Code != want {
			t.Fatalf("%s %s = %d want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec
	}
	const studentPath = "/api/student/internship-case/final-report"
	professorPath := func(item model.InternshipCase) string {
		return fmt.Sprintf("/api/professor/internship-cases/%d/final-report", item.ID)
	}
	upload := func(t *testing.T, student model.User, name, mime string, content []byte, extra map[string]string, want int) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for key, value := range extra {
			if err := writer.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, name))
		header.Set("Content-Type", mime)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, studentPath, &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if student.ID != 0 {
			req.Header.Set("Authorization", "Bearer "+token(student))
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("upload = %d want %d: %s", rec.Code, want, rec.Body.String())
		}
		return rec
	}
	decode := func(t *testing.T, rec *httptest.ResponseRecorder, want model.FinalReportStatus) model.FinalReport {
		t.Helper()
		var report model.FinalReport
		if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.Status != want || report.ID == 0 || report.CurrentFileID == 0 || report.CurrentFile.ID != report.CurrentFileID {
			t.Fatalf("unexpected report: %+v", report)
		}
		return report
	}
	var counter int
	fixture := func(t *testing.T, status model.InternshipCaseStatus) (model.User, model.InternshipCase) {
		t.Helper()
		counter++
		student := createOpportunityTestUser(t, tx, suffix, fmt.Sprintf("frstudent%d", counter), model.RoleStudent, nil)
		item := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, CompanySupervisorID: &supervisor.ID, Status: status}
		if err := tx.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		return student, item
	}
	validPDF := []byte("%PDF-1.4\nfinal report\n%%EOF")
	firstUpload := func(t *testing.T, student model.User) model.FinalReport {
		t.Helper()
		return decode(t, upload(t, student, "final.pdf", "application/pdf", validPDF, nil, 201), model.FinalReportSubmitted)
	}
	stored := func(t *testing.T, id uint) model.FinalReport {
		t.Helper()
		var report model.FinalReport
		if err := tx.Preload("CurrentFile").First(&report, id).Error; err != nil {
			t.Fatal(err)
		}
		return report
	}
	count := func(t *testing.T, entity any) int64 {
		t.Helper()
		var n int64
		if err := tx.Model(entity).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	diskCount := func(t *testing.T) int {
		t.Helper()
		entries, err := os.ReadDir(uploadDir)
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	noOrphans := func(t *testing.T, action func()) {
		t.Helper()
		files, reports, disks := count(t, &model.File{}), count(t, &model.FinalReport{}), diskCount(t)
		action()
		if count(t, &model.File{}) != files || count(t, &model.FinalReport{}) != reports || diskCount(t) != disks {
			t.Fatal("rejected upload left orphan metadata or disk files")
		}
	}
	review := func(t *testing.T, item model.InternshipCase, action string, comment any, want int) *httptest.ResponseRecorder {
		t.Helper()
		return request(t, "POST", professorPath(item)+"/"+action, professor, comment, want)
	}

	t.Run("first upload without weekly reports and server controlled fields", func(t *testing.T) {
		student, item := fixture(t, model.InternshipCaseStatusActive)
		rec := request(t, "GET", studentPath, student, nil, 200)
		if rec.Body.String() != "null" {
			t.Fatal("report exists before first PDF")
		}
		report := decode(t, upload(t, student, "final.PDF", "application/pdf", validPDF, map[string]string{
			"student_id": fmt.Sprint(university.ID), "internship_case_id": "999999", "status": "APPROVED", "review_comment": "fake", "reviewed_at": "2030-01-01",
		}, 201), model.FinalReportSubmitted)
		if report.InternshipCaseID != item.ID || report.ReviewedAt != nil || report.ReviewComment != nil || report.SubmittedAt.IsZero() || report.CreatedAt.IsZero() || report.UpdatedAt.IsZero() || report.CurrentFile.UploadedBy != student.ID {
			t.Fatalf("wrong first submission: %+v", report)
		}
		persisted := stored(t, report.ID)
		data, err := os.ReadFile(persisted.CurrentFile.Path)
		if err != nil || !bytes.Equal(data, validPDF) {
			t.Fatalf("PDF storage mismatch: %v", err)
		}
		decode(t, request(t, "GET", studentPath, student, nil, 200), model.FinalReportSubmitted)
		decode(t, request(t, "GET", professorPath(item), professor, nil, 200), model.FinalReportSubmitted)
		noOrphans(t, func() { upload(t, student, "duplicate.pdf", "application/pdf", validPDF, nil, 409) })
		if stored(t, report.ID).CurrentFileID != report.CurrentFileID {
			t.Fatal("duplicate replaced submitted content")
		}
		review(t, item, "approve", map[string]string{"comment": "  accepted  "}, 200)
		approved := stored(t, report.ID)
		if approved.Status != model.FinalReportApproved || approved.ReviewedAt == nil || approved.ReviewComment == nil || *approved.ReviewComment != "accepted" {
			t.Fatalf("invalid approval: %+v", approved)
		}
		// Exercise each database constraint independently so another constraint
		// cannot mask a missing File FK or status check.
		_, emptyCase := fixture(t, model.InternshipCaseStatusActive)
		constraints := []struct {
			name     string
			sqlState string
			mutate   func(*model.FinalReport)
		}{
			{"idx_final_reports_internship_case_id", "23505", func(r *model.FinalReport) {}},
			{"fk_internship_cases_final_report", "23503", func(r *model.FinalReport) { r.InternshipCaseID = 999999999 }},
			{"fk_final_reports_current_file", "23503", func(r *model.FinalReport) { r.InternshipCaseID = emptyCase.ID; r.CurrentFileID = 999999999 }},
			{"chk_final_reports_status", "23514", func(r *model.FinalReport) { r.InternshipCaseID = emptyCase.ID; r.Status = "DRAFT" }},
		}
		for i, constraint := range constraints {
			point := fmt.Sprintf("final_report_constraint_%d", i)
			if err := tx.SavePoint(point).Error; err != nil {
				t.Fatal(err)
			}
			row := approved
			row.ID = 0
			constraint.mutate(&row)
			err := tx.Omit("CurrentFile").Create(&row).Error
			if rollbackErr := tx.RollbackTo(point).Error; rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != constraint.sqlState || pgErr.ConstraintName != constraint.name {
				t.Fatalf("expected constraint %s (%s), got %v", constraint.name, constraint.sqlState, err)
			}
		}
	})

	t.Run("revision cycle cleanup and terminal approval", func(t *testing.T) {
		student, item := fixture(t, model.InternshipCaseStatusActive)
		report := firstUpload(t, student)
		for _, body := range []any{nil, map[string]string{"comment": " \n\t "}} {
			review(t, item, "request-revision", body, 400)
		}
		if got := stored(t, report.ID); got.Status != model.FinalReportSubmitted || got.ReviewedAt != nil {
			t.Fatal("blank feedback changed report")
		}
		report = decode(t, review(t, item, "request-revision", map[string]string{"comment": "  correct section two  "}, 200), model.FinalReportRevisionRequested)
		if report.ReviewComment == nil || *report.ReviewComment != "correct section two" || report.ReviewedAt == nil {
			t.Fatal("revision feedback missing")
		}
		for _, action := range []string{"approve", "request-revision"} {
			review(t, item, action, map[string]string{"comment": "change"}, 409)
		}
		previous := stored(t, report.ID)
		files, disks := count(t, &model.File{}), diskCount(t)
		corrected := []byte("%PDF-1.7\ncorrected final report\n%%EOF")
		report = decode(t, upload(t, student, "corrected.pdf", "application/pdf", corrected, nil, 201), model.FinalReportSubmitted)
		if report.ID != previous.ID || report.CurrentFileID == previous.CurrentFileID || !report.SubmittedAt.After(previous.SubmittedAt) || report.ReviewedAt != nil || report.ReviewComment == nil || *report.ReviewComment != *previous.ReviewComment {
			t.Fatal("correction did not reset submission and preserve latest feedback")
		}
		if err := tx.First(&model.File{}, previous.CurrentFileID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("superseded metadata remains: %v", err)
		}
		if _, err := os.Stat(previous.CurrentFile.Path); !os.IsNotExist(err) {
			t.Fatalf("superseded disk file remains: %v", err)
		}
		if count(t, &model.File{}) != files || diskCount(t) != disks {
			t.Fatal("correction left orphan files")
		}
		rec := request(t, "GET", fmt.Sprintf("/api/files/%d/download", report.CurrentFileID), professor, nil, 200)
		if !bytes.Equal(rec.Body.Bytes(), corrected) {
			t.Fatal("professor received old content")
		}
		noOrphans(t, func() { upload(t, student, "blocked.pdf", "application/pdf", validPDF, nil, 409) })
		report = decode(t, review(t, item, "approve", nil, 200), model.FinalReportApproved)
		if report.ReviewComment != nil || report.ReviewedAt == nil {
			t.Fatal("approval without comment retained obsolete revision feedback")
		}
		before := stored(t, report.ID)
		for _, action := range []string{"approve", "request-revision"} {
			review(t, item, action, map[string]string{"comment": "change"}, 409)
		}
		noOrphans(t, func() { upload(t, student, "blocked.pdf", "application/pdf", validPDF, nil, 409) })
		after := stored(t, report.ID)
		if !after.ReviewedAt.Equal(*before.ReviewedAt) || !after.UpdatedAt.Equal(before.UpdatedAt) || after.ReviewComment != nil || after.CurrentFileID != before.CurrentFileID {
			t.Fatal("terminal report mutated")
		}
		request(t, "DELETE", studentPath, student, nil, 404)
	})

	t.Run("PDF validation size and role restrictions", func(t *testing.T) {
		student, _ := fixture(t, model.InternshipCaseStatusActive)
		invalid := []struct {
			name, mime string
			data       []byte
			status     int
		}{
			{"final.doc", "application/pdf", validPDF, 400},
			{"final.pdf", "application/zip", validPDF, 400},
			{"final.pdf", "application/pdf", []byte("fake PDF"), 400},
			{"final.pdf", "application/pdf", nil, 400},
			{"final.pdf", "application/pdf", append([]byte("%PDF-"), bytes.Repeat([]byte("x"), maxFinalReportSize)...), 413},
		}
		for _, input := range invalid {
			noOrphans(t, func() {
				rec := upload(t, student, input.name, input.mime, input.data, nil, input.status)
				if input.status == 413 {
					assertAPIError(t, rec, 413, "FINAL_REPORT_TOO_LARGE")
				}
			})
		}
		huge := httptest.NewRequest("POST", studentPath, bytes.NewReader(nil))
		huge.ContentLength = maxFinalReportSize + (1 << 20) + 1
		huge.Header.Set("Authorization", "Bearer "+token(student))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, huge)
		assertAPIError(t, rec, 413, "FINAL_REPORT_TOO_LARGE")
		for _, wrong := range []model.User{professor, supervisor, university, admin} {
			noOrphans(t, func() { upload(t, wrong, "final.pdf", "application/pdf", validPDF, nil, 403) })
		}
		noOrphans(t, func() { upload(t, model.User{}, "final.pdf", "application/pdf", validPDF, nil, 401) })
	})

	t.Run("ownership case snapshot protected PDFs and resume regression", func(t *testing.T) {
		student, item := fixture(t, model.InternshipCaseStatusActive)
		otherStudent, otherCase := fixture(t, model.InternshipCaseStatusActive)
		report := firstUpload(t, student)
		// A changed current assignment cannot transfer the historical case's review permission.
		assignment := model.ProfessorAssignment{StudentID: student.ID, ProfessorID: otherProfessor.ID, AssignedAt: time.Now()}
		if err := tx.Create(&assignment).Error; err != nil {
			t.Fatal(err)
		}
		request(t, "GET", professorPath(item), otherProfessor, nil, 403)
		for _, action := range []string{"approve", "request-revision"} {
			request(t, "POST", professorPath(item)+"/"+action, otherProfessor, map[string]string{"comment": "change"}, 403)
			for _, wrong := range []model.User{student, supervisor, university, admin} {
				request(t, "POST", professorPath(item)+"/"+action, wrong, nil, 403)
			}
			request(t, "POST", professorPath(item)+"/"+action, model.User{}, nil, 401)
		}
		own := request(t, "GET", studentPath+"?internship_case_id="+fmt.Sprint(item.ID), otherStudent, nil, 200)
		if own.Body.String() != "null" {
			t.Fatal("student read another student's report by ID")
		}
		manipulated := decode(t, upload(t, otherStudent, "mine.pdf", "application/pdf", validPDF, map[string]string{"internship_case_id": fmt.Sprint(item.ID)}, 201), model.FinalReportSubmitted)
		if manipulated.InternshipCaseID != otherCase.ID || stored(t, report.ID).CurrentFileID != report.CurrentFileID {
			t.Fatal("student replaced another case's report")
		}
		path := fmt.Sprintf("/api/files/%d/download", report.CurrentFileID)
		for _, owner := range []model.User{student, professor, supervisor, university} {
			request(t, "GET", path, owner, nil, 200)
		}
		// Assigned company and University retain the explicit pre-existing read-only policy.
		for _, wrong := range []model.User{otherStudent, otherProfessor, unrelated, admin} {
			request(t, "GET", path, wrong, nil, 403)
		}
		request(t, "GET", path, model.User{}, nil, 401)
		decode(t, review(t, item, "approve", nil, 200), model.FinalReportApproved)
		// Resume permissions remain relationship-based and exclude Professor/University.
		opportunity := model.InternshipOpportunity{CompanyID: companyA.ID, CreatedBy: supervisor.ID, Title: "resume regression", Description: "test", WorkField: "software", Location: "Tehran", Status: model.OpportunityStatusOpen}
		if err := tx.Create(&opportunity).Error; err != nil {
			t.Fatal(err)
		}
		resume := model.File{OriginalName: "resume.pdf", StoredName: fmt.Sprintf("fr-resume-%d.pdf", suffix), Path: uploadDir + "/resume.pdf", MimeType: "application/pdf", SizeBytes: int64(len(validPDF)), UploadedBy: student.ID, UploadedAt: time.Now()}
		if err := os.WriteFile(resume.Path, validPDF, 0o640); err != nil {
			t.Fatal(err)
		}
		if err := tx.Create(&resume).Error; err != nil {
			t.Fatal(err)
		}
		application := model.OpportunityApplication{StudentID: student.ID, OpportunityID: opportunity.ID, ResumeFileID: resume.ID, Status: model.ApplicationStatusAccepted, AppliedAt: time.Now()}
		if err := tx.Create(&application).Error; err != nil {
			t.Fatal(err)
		}
		path = fmt.Sprintf("/api/files/%d/download", resume.ID)
		for _, owner := range []model.User{student, supervisor} {
			request(t, "GET", path, owner, nil, 200)
		}
		for _, wrong := range []model.User{otherStudent, professor, otherProfessor, university, unrelated, admin} {
			request(t, "GET", path, wrong, nil, 403)
		}
		request(t, "GET", path, model.User{}, nil, 401)
	})

	for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingCompanyDetails, model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusReadyToStart, model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
		t.Run("inactive "+string(status), func(t *testing.T) {
			student, item := fixture(t, status)
			noOrphans(t, func() { upload(t, student, "final.pdf", "application/pdf", validPDF, nil, 400) })
			if err := tx.Model(&item).Update("status", model.InternshipCaseStatusActive).Error; err != nil {
				t.Fatal(err)
			}
			report := firstUpload(t, student)
			if err := tx.Model(&item).Update("status", status).Error; err != nil {
				t.Fatal(err)
			}
			for _, action := range []string{"approve", "request-revision"} {
				review(t, item, action, map[string]string{"comment": "change"}, 409)
			}
			if err := tx.Model(&model.FinalReport{}).Where("id = ?", report.ID).Update("status", model.FinalReportRevisionRequested).Error; err != nil {
				t.Fatal(err)
			}
			noOrphans(t, func() { upload(t, student, "corrected.pdf", "application/pdf", validPDF, nil, 400) })
			if status == model.InternshipCaseStatusPassed || status == model.InternshipCaseStatusFailed {
				decode(t, request(t, "GET", studentPath, student, nil, 200), model.FinalReportRevisionRequested)
				decode(t, request(t, "GET", professorPath(item), professor, nil, 200), model.FinalReportRevisionRequested)
				request(t, "GET", fmt.Sprintf("/api/files/%d/download", report.CurrentFileID), student, nil, 200)
				request(t, "GET", fmt.Sprintf("/api/files/%d/download", report.CurrentFileID), professor, nil, 200)
			}
		})
	}

	t.Run("failed first upload rolls back metadata and cleans disk", func(t *testing.T) {
		student, _ := fixture(t, model.InternshipCaseStatusActive)
		name := "test:fail_final_report_creation"
		if err := db.Callback().Create().Before("gorm:create").Register(name, func(db *gorm.DB) {
			if db.Statement.Schema != nil && db.Statement.Schema.Table == "final_reports" {
				db.AddError(errors.New("simulated final report DB failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		defer db.Callback().Create().Remove(name)
		noOrphans(t, func() { upload(t, student, "final.pdf", "application/pdf", validPDF, nil, 500) })
	})

	t.Run("failed correction retains usable old PDF", func(t *testing.T) {
		student, item := fixture(t, model.InternshipCaseStatusActive)
		report := firstUpload(t, student)
		review(t, item, "request-revision", map[string]string{"comment": "correct"}, 200)
		previous := stored(t, report.ID)
		name := "test:fail_final_report_update"
		if err := db.Callback().Update().Before("gorm:update").Register(name, func(db *gorm.DB) {
			if db.Statement.Schema != nil && db.Statement.Schema.Table == "final_reports" {
				db.AddError(errors.New("simulated correction DB failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		defer db.Callback().Update().Remove(name)
		noOrphans(t, func() { upload(t, student, "corrected.pdf", "application/pdf", validPDF, nil, 500) })
		after := stored(t, report.ID)
		if after.CurrentFileID != previous.CurrentFileID || after.Status != model.FinalReportRevisionRequested {
			t.Fatal("failed correction changed current report")
		}
		request(t, "GET", fmt.Sprintf("/api/files/%d/download", previous.CurrentFileID), professor, nil, 200)
	})

	t.Run("completion requires both weekly approvals company evaluation and final approval", func(t *testing.T) {
		student, item := fixture(t, model.InternshipCaseStatusActive)
		report := firstUpload(t, student)
		finalPath := fmt.Sprintf("/api/professor/internship-cases/%d/complete", item.ID)
		detailPath := fmt.Sprintf("/api/professor/internship-cases/%d", item.ID)
		assertReady := func(want bool) {
			rec := request(t, "GET", detailPath, professor, nil, 200)
			var detail professorCaseDetailResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			if detail.CanProfessorComplete != want {
				t.Fatalf("readiness=%v want %v", detail.CanProfessorComplete, want)
			}
		}
		completion := map[string]string{"result": "GOOD", "comment": " final comment "}
		assertReady(false)
		request(t, "POST", finalPath, professor, completion, 409)
		reports := []model.WeeklyReport{}
		now := time.Now()
		for week := 1; week <= 8; week++ {
			row := model.WeeklyReport{InternshipCaseID: item.ID, WeekNumber: week, StartDate: now, EndDate: now, ActivityDescription: "activity", SubmittedAt: &now, CompanyReviewStatus: model.WeeklyReviewApproved, ProfessorReviewStatus: model.WeeklyReviewApproved}
			if err := tx.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			reports = append(reports, row)
		}
		eval := model.CompanyEvaluation{InternshipCaseID: item.ID, CompanySupervisorID: supervisor.ID, AttendanceRating: model.EvaluationRatingGood, ParticipationRating: model.EvaluationRatingGood, LearningRating: model.EvaluationRatingGood, InterestRating: model.EvaluationRatingGood, PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingGood, ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingGood, ProjectPerformanceRating: model.EvaluationRatingGood, SubmittedAt: now}
		if err := tx.Create(&eval).Error; err != nil {
			t.Fatal(err)
		}
		// Merely submitted and revision-requested reports cannot complete a case.
		for _, status := range []model.FinalReportStatus{model.FinalReportSubmitted, model.FinalReportRevisionRequested} {
			if err := tx.Model(&report).Update("status", status).Error; err != nil {
				t.Fatal(err)
			}
			assertReady(false)
			assertAPIError(t, request(t, "POST", finalPath, professor, completion, 409), 409, "FINAL_REPORT_NOT_APPROVED")
		}
		if err := tx.Model(&report).Update("status", model.FinalReportSubmitted).Error; err != nil {
			t.Fatal(err)
		}
		review(t, item, "approve", nil, 200)
		assertReady(true)
		for _, field := range []string{"company_review_status", "professor_review_status"} {
			if err := tx.Model(&reports[0]).Update(field, model.WeeklyReviewPending).Error; err != nil {
				t.Fatal(err)
			}
			assertReady(false)
			request(t, "POST", finalPath, professor, completion, 409)
			if err := tx.Model(&reports[0]).Update(field, model.WeeklyReviewApproved).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Delete(&reports[7]).Error; err != nil {
			t.Fatal(err)
		}
		assertReady(false)
		request(t, "POST", finalPath, professor, completion, 409)
		if err := tx.Create(&reports[7]).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.Delete(&eval).Error; err != nil {
			t.Fatal(err)
		}
		assertReady(false)
		request(t, "POST", finalPath, professor, completion, 409)
		if err := tx.Create(&eval).Error; err != nil {
			t.Fatal(err)
		}
		assertReady(true)
		rec := request(t, "POST", finalPath, professor, completion, 200)
		var detail professorCaseDetailResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
			t.Fatal(err)
		}
		if detail.Internship.Status != model.InternshipCaseStatusPassed || detail.FinalResult == nil || *detail.FinalResult != model.ProfessorFinalResultGood || detail.ProfessorComment == nil || *detail.ProfessorComment != "final comment" || detail.CompletedAt == nil || detail.CanProfessorComplete {
			t.Fatal("existing completion contract changed")
		}
		noOrphans(t, func() { upload(t, student, "blocked.pdf", "application/pdf", validPDF, nil, 400) })
		for _, action := range []string{"approve", "request-revision"} {
			review(t, item, action, map[string]string{"comment": "change"}, 409)
		}
		request(t, "GET", fmt.Sprintf("/api/files/%d/download", report.CurrentFileID), professor, nil, 200)
	})
	t.Run("student terminal history remains owned and readable after retry", func(t *testing.T) {
		student, item := fixture(t, model.InternshipCaseStatusActive)
		report := firstUpload(t, student)
		review(t, item, "approve", nil, 200)
		now := time.Now()
		for week := 1; week <= 8; week++ {
			row := model.WeeklyReport{InternshipCaseID: item.ID, WeekNumber: week, StartDate: now, EndDate: now, ActivityDescription: "historical activity", SubmittedAt: &now, CompanyReviewStatus: model.WeeklyReviewApproved, ProfessorReviewStatus: model.WeeklyReviewApproved}
			if err := tx.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
		}
		evaluation := model.CompanyEvaluation{InternshipCaseID: item.ID, CompanySupervisorID: supervisor.ID, AttendanceRating: model.EvaluationRatingGood, ParticipationRating: model.EvaluationRatingGood, LearningRating: model.EvaluationRatingGood, InterestRating: model.EvaluationRatingGood, PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingGood, ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingGood, ProjectPerformanceRating: model.EvaluationRatingGood, SubmittedAt: now}
		if err := tx.Create(&evaluation).Error; err != nil {
			t.Fatal(err)
		}
		finalPath := fmt.Sprintf("/api/professor/internship-cases/%d/complete", item.ID)
		assertAPIError(t, request(t, "POST", finalPath, otherProfessor, map[string]string{"result": "GOOD"}, 403), 403, "INTERNSHIP_CASE_NOT_ASSIGNED_TO_PROFESSOR")
		assertAPIError(t, request(t, "POST", finalPath, professor, map[string]string{"result": ""}, 400), 400, "INVALID_FINAL_RESULT")
		finalized := request(t, "POST", finalPath, professor, map[string]string{"result": "FAILED", "comment": " final unsuccessful result "}, 200)
		var outcome professorCaseDetailResponse
		if err := json.Unmarshal(finalized.Body.Bytes(), &outcome); err != nil {
			t.Fatal(err)
		}
		if outcome.Internship.Status != model.InternshipCaseStatusFailed || outcome.FinalResult == nil || *outcome.FinalResult != model.ProfessorFinalResultFailed || outcome.CompletedAt == nil || outcome.ProfessorComment == nil || *outcome.ProfessorComment != "final unsuccessful result" || outcome.CanProfessorComplete {
			t.Fatalf("incorrect failed response: %+v", outcome)
		}
		assertAPIError(t, request(t, "POST", finalPath, professor, map[string]string{"result": "GOOD"}, 409), 409, "INTERNSHIP_CASE_NOT_ACTIVE")
		assignment := model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: now}
		if err := tx.Create(&assignment).Error; err != nil {
			t.Fatal(err)
		}
		draft, created, err := service.NewInternshipService(tx).CreateOrGetCase(student.ID)
		if err != nil || !created || draft.ID == item.ID {
			t.Fatalf("retry case: %v", err)
		}
		const path = "/api/student/internship-cases/history"
		var history []internshipCaseResponse
		if err := json.Unmarshal(request(t, "GET", path, student, nil, 200).Body.Bytes(), &history); err != nil {
			t.Fatal(err)
		}
		if len(history) != 1 || history[0].ID != item.ID || history[0].Status != model.InternshipCaseStatusFailed || len(history[0].WeeklyReports) != 8 || history[0].FinalReport == nil || history[0].FinalReport.CurrentFileID != report.CurrentFileID || history[0].CompanyEvaluation == nil || history[0].CompletedAt == nil || !history[0].CompletedAt.Equal(*outcome.CompletedAt) {
			t.Fatalf("lost historical case/report data: %+v", history)
		}
		otherStudent, _ := fixture(t, model.InternshipCaseStatusDraft)
		response := request(t, "GET", path+"?student_id="+fmt.Sprint(student.ID), otherStudent, nil, 200)
		if response.Body.String() != "[]" {
			t.Fatal("history endpoint exposed another student's cases")
		}
		request(t, "GET", path, model.User{}, nil, 401)
		request(t, "GET", path, supervisor, nil, 403)
		request(t, "GET", fmt.Sprintf("/api/files/%d/download", report.CurrentFileID), student, nil, 200)
	})

}
