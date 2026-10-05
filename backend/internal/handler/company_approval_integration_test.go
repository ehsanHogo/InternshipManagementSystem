package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"internship-management-system/backend/internal/auth"
	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

// Committed fixtures in an isolated schema allow real row-lock concurrency.
func TestCompanyFacultyApproval(t *testing.T) {
	db := companyApprovalTestDB(t)
	suffix := time.Now().UnixNano()
	professor := createOpportunityTestUser(t, db, suffix, "approval-professor", model.RoleProfessor, nil)
	university := createOpportunityTestUser(t, db, suffix, "approval-university", model.RoleUniversitySupervisor, nil)
	student := createOpportunityTestUser(t, db, suffix, "approval-student", model.RoleStudent, nil)
	admin := createOpportunityTestUser(t, db, suffix, "approval-admin", model.RoleAdmin, nil)
	supervisor := createOpportunityTestUser(t, db, suffix, "approval-supervisor", model.RoleCompanySupervisor, nil)
	const secret = "company-approval-integration-secret"
	tokens := map[model.Role]string{}
	for _, user := range []model.User{professor, university, student, admin, supervisor} {
		token, err := auth.CreateToken(user, secret, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		tokens[user.Role] = token
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authenticated := router.Group("/api", appmiddleware.RequireAuth(secret))
	approval := NewCompanyApprovalHandler(service.NewCompanyApprovalService(db))
	approval.RegisterUniversityRoutes(authenticated.Group("/university"))
	approval.RegisterStudentRoutes(authenticated.Group("/student"))
	opportunities := NewOpportunityHandler(service.NewOpportunityService(db))
	studentRoutes := authenticated.Group("/student", appmiddleware.RequireRole(model.RoleStudent))
	studentRoutes.GET("/opportunities", opportunities.ListStudent)
	studentRoutes.GET("/opportunities/:id", opportunities.GetStudent)
	request := func(method, path string, body any, role model.Role) *httptest.ResponseRecorder {
		return performJSONRequest(t, router, method, path, body, tokens[role])
	}
	read := func(path string, role model.Role, result any) {
		t.Helper()
		response := request(http.MethodGet, path, nil, role)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, response.Code, response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), result); err != nil {
			t.Fatal(err)
		}
	}
	newCompany := func(label string, approved bool) model.Company {
		company := createOpportunityTestCompany(t, db, suffix, label, approved)
		contact := map[string]any{"website": "https://example.test", "phone": "02112345678", "email": label + "@example.test", "address": "تهران"}
		if err := db.Model(&company).Updates(contact).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.First(&company, company.ID).Error; err != nil {
			t.Fatal(err)
		}
		return company
	}
	var counter int
	newCase := func(company model.Company, status model.InternshipCaseStatus, result model.ProfessorFinalResult) (model.InternshipCase, model.InternshipOpportunity) {
		counter++
		caseStudent := createOpportunityTestUser(t, db, suffix, fmt.Sprintf("approval-case-%d", counter), model.RoleStudent, nil)
		opportunity := model.InternshipOpportunity{CompanyID: company.ID, CreatedBy: supervisor.ID, Title: "فرصت منتخب", Description: "شرح", WorkField: "نرم‌افزار", Location: "تهران", Status: model.OpportunityStatusOpen}
		if err := db.Create(&opportunity).Error; err != nil {
			t.Fatal(err)
		}
		file := model.File{OriginalName: "resume.pdf", StoredName: fmt.Sprintf("approval-%d.pdf", counter), Path: "/tmp/resume.pdf", MimeType: "application/pdf", SizeBytes: 128, UploadedBy: caseStudent.ID, UploadedAt: time.Now()}
		if err := db.Create(&file).Error; err != nil {
			t.Fatal(err)
		}
		application := model.OpportunityApplication{OpportunityID: opportunity.ID, StudentID: caseStudent.ID, ResumeFileID: file.ID, Status: model.ApplicationStatusAccepted, AppliedAt: time.Now()}
		if err := db.Create(&application).Error; err != nil {
			t.Fatal(err)
		}
		item := model.InternshipCase{StudentID: caseStudent.ID, ProfessorID: professor.ID, CompanySupervisorID: &supervisor.ID, Status: status}
		if result != "" {
			item.FinalResult = &result
			now := time.Now()
			item.CompletedAt = &now
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		preference := model.InternshipPreference{InternshipCaseID: item.ID, OpportunityApplicationID: application.ID, Priority: 1}
		if err := db.Create(&preference).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&item).Update("selected_preference_id", preference.ID).Error; err != nil {
			t.Fatal(err)
		}
		return item, opportunity
	}
	eligible := newCompany("excellent", false)
	good := newCompany("good", false)
	seedApproved := newCompany("seeded", true) // Approved lists do not require history.
	nonSelected := newCompany("non-selected", false)
	empty := newCompany("no-history", false)
	passedCase, openOpportunity := newCase(eligible, model.InternshipCaseStatusPassed, model.ProfessorFinalResultExcellent)
	newCase(eligible, model.InternshipCaseStatusPassed, model.ProfessorFinalResultGood)
	newCase(good, model.InternshipCaseStatusPassed, model.ProfessorFinalResultGood)
	ineligible := []model.Company{empty, nonSelected}
	for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled, model.InternshipCaseStatusActive} {
		company := newCompany(string(status), false)
		result := model.ProfessorFinalResult("")
		if status == model.InternshipCaseStatusFailed {
			result = model.ProfessorFinalResultFailed
		}
		newCase(company, status, result)
		ineligible = append(ineligible, company)
		// Mixed history must still display only successful evidence.
		newCase(eligible, status, result)
	}
	// Company A accepted the student and is priority 1; only selected B qualifies.
	priorityCase, _ := newCase(nonSelected, model.InternshipCaseStatusPassed, model.ProfessorFinalResultGood)
	var oldPreference model.InternshipPreference
	if err := db.Where("internship_case_id = ?", priorityCase.ID).First(&oldPreference).Error; err != nil {
		t.Fatal(err)
	}
	var originalApplication model.OpportunityApplication
	if err := db.First(&originalApplication, oldPreference.OpportunityApplicationID).Error; err != nil {
		t.Fatal(err)
	}
	secondOpportunity := model.InternshipOpportunity{CompanyID: good.ID, CreatedBy: supervisor.ID, Title: "اولویت دوم منتخب", Description: "شرح", WorkField: "نرم‌افزار", Location: "تهران", Status: model.OpportunityStatusClosed}
	if err := db.Create(&secondOpportunity).Error; err != nil {
		t.Fatal(err)
	}
	secondFile := model.File{OriginalName: "resume.pdf", StoredName: "second-preference.pdf", Path: "/tmp/second.pdf", MimeType: "application/pdf", SizeBytes: 128, UploadedBy: priorityCase.StudentID, UploadedAt: time.Now()}
	if err := db.Create(&secondFile).Error; err != nil {
		t.Fatal(err)
	}
	secondApplication := model.OpportunityApplication{StudentID: priorityCase.StudentID, OpportunityID: secondOpportunity.ID, ResumeFileID: secondFile.ID, Status: model.ApplicationStatusAccepted, AppliedAt: time.Now()}
	if err := db.Create(&secondApplication).Error; err != nil {
		t.Fatal(err)
	}
	secondPreference := model.InternshipPreference{InternshipCaseID: priorityCase.ID, OpportunityApplicationID: secondApplication.ID, Priority: 2}
	if err := db.Create(&secondPreference).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&priorityCase).Update("selected_preference_id", secondPreference.ID).Error; err != nil {
		t.Fatal(err)
	}
	// Add reporting data to make the invariance snapshot meaningful.
	now := time.Now()
	report := model.WeeklyReport{InternshipCaseID: passedCase.ID, WeekNumber: 1, StartDate: now, EndDate: now, ActivityDescription: "فعالیت", SubmittedAt: &now, CompanyReviewStatus: model.WeeklyReviewApproved, ProfessorReviewStatus: model.WeeklyReviewApproved}
	if err := db.Create(&report).Error; err != nil {
		t.Fatal(err)
	}
	evaluation := model.CompanyEvaluation{InternshipCaseID: passedCase.ID, CompanySupervisorID: supervisor.ID, AttendanceRating: model.EvaluationRatingGood, ParticipationRating: model.EvaluationRatingGood, LearningRating: model.EvaluationRatingGood, InterestRating: model.EvaluationRatingGood, PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingGood, ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingGood, ProjectPerformanceRating: model.EvaluationRatingGood, SubmittedAt: now}
	if err := db.Create(&evaluation).Error; err != nil {
		t.Fatal(err)
	}
	final := model.FinalReport{InternshipCaseID: passedCase.ID, CurrentFileID: originalApplication.ResumeFileID, Status: model.FinalReportApproved, SubmittedAt: now, ReviewedAt: &now}
	if err := db.Create(&final).Error; err != nil {
		t.Fatal(err)
	}
	approvePath := func(id uint) string { return fmt.Sprintf("/api/university/companies/%d/approve", id) }
	detailPath := func(id uint) string { return fmt.Sprintf("/api/university/companies/%d", id) }

	t.Run("selected PASSED only, distinct counts and private evidence", func(t *testing.T) {
		var companies []service.ApprovalCompanyView
		read("/api/university/companies/eligible-for-approval", model.RoleUniversitySupervisor, &companies)
		if len(companies) != 2 {
			t.Fatalf("eligible companies = %+v", companies)
		}
		counts := map[uint]int64{}
		for _, company := range companies {
			counts[company.ID] = company.PassedInternshipCount
		}
		if counts[eligible.ID] != 2 || counts[good.ID] != 2 {
			t.Fatalf("counts = %v", counts)
		}
		for _, company := range ineligible {
			if counts[company.ID] != 0 {
				t.Fatalf("ineligible company %d listed", company.ID)
			}
		}
		var detail service.ApprovalCompanyDetail
		read(detailPath(eligible.ID), model.RoleUniversitySupervisor, &detail)
		if detail.IsApproved || detail.PassedInternshipCount != 2 || len(detail.SuccessfulInternships) != 2 {
			t.Fatalf("detail = %+v", detail)
		}
		for _, evidence := range detail.SuccessfulInternships {
			if evidence.FinalResult == nil || (*evidence.FinalResult != model.ProfessorFinalResultExcellent && *evidence.FinalResult != model.ProfessorFinalResultGood) {
				t.Fatalf("non-successful evidence: %+v", evidence)
			}
		}
		var approved []model.Company
		read("/api/university/companies/approved", model.RoleUniversitySupervisor, &approved)
		if len(approved) != 1 || approved[0].ID != seedApproved.ID {
			t.Fatalf("approved without history = %+v", approved)
		}
	})

	t.Run("roles and unauthenticated access", func(t *testing.T) {
		before := companyApprovalSnapshot(t, db)
		for _, role := range []model.Role{model.RoleStudent, model.RoleProfessor, model.RoleCompanySupervisor, model.RoleAdmin, ""} {
			want := http.StatusForbidden
			if role == "" {
				want = http.StatusUnauthorized
			}
			for _, path := range []string{"/api/university/companies/eligible-for-approval", "/api/university/companies/approved", detailPath(eligible.ID)} {
				if response := request(http.MethodGet, path, nil, role); response.Code != want {
					t.Fatalf("%s GET %s = %d", role, path, response.Code)
				}
			}
			if response := request(http.MethodPost, approvePath(eligible.ID), nil, role); response.Code != want {
				t.Fatalf("%s approval = %d", role, response.Code)
			}
		}
		for _, role := range []model.Role{model.RoleProfessor, model.RoleCompanySupervisor, model.RoleUniversitySupervisor, model.RoleAdmin, ""} {
			want := http.StatusForbidden
			if role == "" {
				want = http.StatusUnauthorized
			}
			if response := request(http.MethodGet, "/api/student/approved-companies", nil, role); response.Code != want {
				t.Fatalf("%s student list = %d", role, response.Code)
			}
		}
		if !reflect.DeepEqual(before, companyApprovalSnapshot(t, db)) {
			t.Fatal("unauthorized access mutated data")
		}
	})

	t.Run("ineligible, missing company and body spoofing rejected without mutation", func(t *testing.T) {
		before := companyApprovalSnapshot(t, db)
		for _, company := range ineligible {
			assertAPIError(t, request(http.MethodPost, approvePath(company.ID), nil, model.RoleUniversitySupervisor), http.StatusConflict, "COMPANY_NOT_ELIGIBLE_FOR_APPROVAL")
		}
		assertAPIError(t, request(http.MethodPost, approvePath(seedApproved.ID), nil, model.RoleUniversitySupervisor), http.StatusConflict, "COMPANY_ALREADY_APPROVED")
		assertAPIError(t, request(http.MethodPost, approvePath(999999), nil, model.RoleUniversitySupervisor), http.StatusNotFound, "COMPANY_NOT_FOUND")
		assertAPIError(t, request(http.MethodGet, detailPath(999999), nil, model.RoleUniversitySupervisor), http.StatusNotFound, "COMPANY_NOT_FOUND")
		assertAPIError(t, request(http.MethodPost, "/api/university/companies/0/approve", nil, model.RoleUniversitySupervisor), http.StatusBadRequest, "INVALID_COMPANY_ID")
		for _, body := range []any{map[string]any{"companyId": seedApproved.ID, "isApproved": true, "approvedBy": university.ID, "eligibility": true}, map[string]any{}} {
			assertAPIError(t, request(http.MethodPost, approvePath(eligible.ID), body, model.RoleUniversitySupervisor), http.StatusBadRequest, "INVALID_COMPANY_APPROVAL")
		}
		oversized := httptest.NewRequest(http.MethodPost, approvePath(eligible.ID), strings.NewReader(strings.Repeat(" ", 1024)+`{"isApproved":true}`))
		oversized.Header.Set("Authorization", "Bearer "+tokens[model.RoleUniversitySupervisor])
		oversizedResponse := httptest.NewRecorder()
		router.ServeHTTP(oversizedResponse, oversized)
		assertAPIError(t, oversizedResponse, http.StatusBadRequest, "INVALID_COMPANY_APPROVAL")
		if !reflect.DeepEqual(before, companyApprovalSnapshot(t, db)) {
			t.Fatal("rejected requests mutated data")
		}
	})

	t.Run("approval revalidates history after list fetch", func(t *testing.T) {
		stale := newCompany("stale", false)
		item, _ := newCase(stale, model.InternshipCaseStatusPassed, model.ProfessorFinalResultGood)
		var list []service.ApprovalCompanyView
		read("/api/university/companies/eligible-for-approval", model.RoleUniversitySupervisor, &list)
		found := false
		for _, company := range list {
			found = found || company.ID == stale.ID
		}
		if !found {
			t.Fatal("fixture missing from eligible list")
		}
		if err := db.Model(&item).UpdateColumn("status", model.InternshipCaseStatusFailed).Error; err != nil {
			t.Fatal(err)
		}
		before := companyApprovalSnapshot(t, db)
		assertAPIError(t, request(http.MethodPost, approvePath(stale.ID), nil, model.RoleUniversitySupervisor), http.StatusConflict, "COMPANY_NOT_ELIGIBLE_FOR_APPROVAL")
		if !reflect.DeepEqual(before, companyApprovalSnapshot(t, db)) {
			t.Fatal("stale approval mutated data")
		}
	})

	t.Run("explicit approval changes only flag and badge follows company", func(t *testing.T) {
		var catalogBefore studentOpportunityResponse
		read(fmt.Sprintf("/api/student/opportunities/%d", openOpportunity.ID), model.RoleStudent, &catalogBefore)
		if catalogBefore.Company.IsApproved {
			t.Fatal("PASSED history auto-approved company")
		}
		before := companyApprovalSnapshot(t, db)
		response := request(http.MethodPost, approvePath(eligible.ID), nil, model.RoleUniversitySupervisor)
		if response.Code != http.StatusOK {
			t.Fatalf("approval = %d %s", response.Code, response.Body.String())
		}
		var afterCompany model.Company
		if err := db.First(&afterCompany, eligible.ID).Error; err != nil {
			t.Fatal(err)
		}
		if !afterCompany.IsApproved {
			t.Fatal("approval flag not persisted")
		}
		afterCompany.IsApproved = false
		if !reflect.DeepEqual(eligible, afterCompany) {
			t.Fatal("approval changed company fields or timestamps")
		}
		assertOnlyCompanyApprovalChanged(t, before, companyApprovalSnapshot(t, db), eligible.ID)
		beforeRepeat := companyApprovalSnapshot(t, db)
		assertAPIError(t, request(http.MethodPost, approvePath(eligible.ID), nil, model.RoleUniversitySupervisor), http.StatusConflict, "COMPANY_ALREADY_APPROVED")
		if !reflect.DeepEqual(beforeRepeat, companyApprovalSnapshot(t, db)) {
			t.Fatal("repeat approval mutated data")
		}
		var remaining []service.ApprovalCompanyView
		read("/api/university/companies/eligible-for-approval", model.RoleUniversitySupervisor, &remaining)
		for _, company := range remaining {
			if company.ID == eligible.ID {
				t.Fatal("approved company still eligible")
			}
		}
		var approved []model.Company
		read("/api/university/companies/approved", model.RoleUniversitySupervisor, &approved)
		if len(approved) != 2 {
			t.Fatalf("approved list = %+v", approved)
		}
		var detail studentOpportunityResponse
		read(fmt.Sprintf("/api/student/opportunities/%d", openOpportunity.ID), model.RoleStudent, &detail)
		if !detail.Company.IsApproved {
			t.Fatal("opportunity detail badge stale")
		}
		var catalog []studentOpportunityResponse
		read("/api/student/opportunities", model.RoleStudent, &catalog)
		assertCatalogContains(t, catalog, openOpportunity.ID, eligible.ID, true)
		var public []map[string]any
		read("/api/student/approved-companies", model.RoleStudent, &public)
		if len(public) != 2 {
			t.Fatalf("student approved companies = %v", public)
		}
		for _, company := range public {
			if len(company) != 6 {
				t.Fatalf("unexpected public fields = %v", company)
			}
			for _, field := range []string{"id", "name", "website", "phone", "email", "address"} {
				if _, ok := company[field]; !ok {
					t.Fatalf("missing public field %s", field)
				}
			}
			if id := uint(company["id"].(float64)); id != eligible.ID && id != seedApproved.ID {
				t.Fatal("unapproved company exposed to student")
			}
		}
	})

	t.Run("simultaneous approval has one winner", func(t *testing.T) {
		concurrent := newCompany("concurrent", false)
		newCase(concurrent, model.InternshipCaseStatusPassed, model.ProfessorFinalResultGood)
		before := companyApprovalSnapshot(t, db)
		start := make(chan struct{})
		responses := make(chan *httptest.ResponseRecorder, 2)
		for i := 0; i < 2; i++ {
			go func() {
				<-start
				response := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, approvePath(concurrent.ID), nil)
				req.Header.Set("Authorization", "Bearer "+tokens[model.RoleUniversitySupervisor])
				router.ServeHTTP(response, req)
				responses <- response
			}()
		}
		close(start)
		wins, conflicts := 0, 0
		for i := 0; i < 2; i++ {
			select {
			case response := <-responses:
				if response.Code == http.StatusOK {
					wins++
				} else {
					assertAPIError(t, response, http.StatusConflict, "COMPANY_ALREADY_APPROVED")
					conflicts++
				}
			case <-time.After(10 * time.Second):
				t.Fatal("concurrent approvals timed out")
			}
		}
		if wins != 1 || conflicts != 1 {
			t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
		}
		var company model.Company
		if err := db.First(&company, concurrent.ID).Error; err != nil {
			t.Fatal(err)
		}
		if !company.IsApproved {
			t.Fatal("winner not persisted")
		}
		company.IsApproved = false
		if !reflect.DeepEqual(company, concurrent) {
			t.Fatal("concurrent approval changed company profile")
		}
		assertOnlyCompanyApprovalChanged(t, before, companyApprovalSnapshot(t, db), concurrent.ID)
	})
}

func assertOnlyCompanyApprovalChanged(t *testing.T, before, after map[string]string, companyID uint) {
	t.Helper()
	var expectedCompanies, actualCompanies []map[string]any
	if err := json.Unmarshal([]byte(before["companies"]), &expectedCompanies); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(after["companies"]), &actualCompanies); err != nil {
		t.Fatal(err)
	}
	for _, company := range expectedCompanies {
		if uint(company["id"].(float64)) == companyID {
			company["is_approved"] = true
		}
	}
	if !reflect.DeepEqual(expectedCompanies, actualCompanies) {
		t.Fatal("approval changed company data beyond the target is_approved flag")
	}
	for table, rows := range before {
		if table != "companies" && after[table] != rows {
			t.Fatalf("approval changed unrelated %s data", table)
		}
	}
}

func companyApprovalSnapshot(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	for _, table := range []string{"companies", "users", "internship_cases", "internship_preferences", "opportunity_applications", "internship_opportunities", "weekly_reports", "company_evaluations", "final_reports", "files"} {
		var rows string
		if err := db.Raw("SELECT COALESCE(jsonb_agg(to_jsonb(rows) ORDER BY rows.id), '[]'::jsonb)::text FROM " + table + " AS rows").Scan(&rows).Error; err != nil {
			t.Fatal(err)
		}
		snapshot[table] = rows
	}
	return snapshot
}

func companyApprovalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	rootSQL, err := root.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rootSQL.Close() })
	schema := fmt.Sprintf("company_approval_%d", time.Now().UnixNano())
	if err := root.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	scopedDSN := dsn + " search_path=" + schema
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		scopedDSN = parsed.String()
	}
	db, err := gorm.Open(postgres.Open(scopedDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.StudentInternshipRating{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}, &model.FinalReport{}); err != nil {
		t.Fatal(err)
	}
	return db
}
