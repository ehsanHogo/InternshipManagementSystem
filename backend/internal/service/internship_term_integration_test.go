package service

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"internship-management-system/backend/internal/database"
	"internship-management-system/backend/internal/model"
)

func termTestDatabase(t *testing.T) *gorm.DB {
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
	schema := fmt.Sprintf("internship_term_%d", time.Now().UnixNano())
	if err := root.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		rootSQL.Close()
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
	sqlDB.SetMaxOpenConns(6)
	t.Cleanup(func() { sqlDB.Close() })
	// Exercise the production migration and seeding, including repeated startup.
	if err := database.MigrateAndSeed(db); err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateAndSeed(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func createOpenTermFixture(t *testing.T, db *gorm.DB, suffix int64) {
	t.Helper()
	university := applicationTestUser(t, db, suffix, "term-fixture-university", model.RoleUniversitySupervisor, nil)
	if _, err := NewInternshipTermService(db).Create(university.ID, 1405, model.InternshipTermTypeSummer); err != nil {
		t.Fatal(err)
	}
}

func TestAcademicInternshipTerms(t *testing.T) {
	db := termTestDatabase(t)
	suffix := time.Now().UnixNano()
	university := applicationTestUser(t, db, suffix, "term-university", model.RoleUniversitySupervisor, nil)
	professor := applicationTestUser(t, db, suffix, "term-professor", model.RoleProfessor, nil)
	terms := NewInternshipTermService(db)
	workflow := NewInternshipService(db)
	studentCounter := 0
	student := func() model.User {
		studentCounter++
		user := applicationTestUser(t, db, suffix, fmt.Sprintf("term-student-%d", studentCounter), model.RoleStudent, nil)
		if err := db.Create(&model.ProfessorAssignment{StudentID: user.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}).Error; err != nil {
			t.Fatal(err)
		}
		return user
	}
	newStudent := student()
	if _, created, err := workflow.CreateOrGetCase(newStudent.ID); !errors.Is(err, ErrNoOpenInternshipTerm) || created {
		t.Fatalf("no open term: created=%v err=%v", created, err)
	}
	if current, err := terms.Current(); err != nil || current != nil {
		t.Fatalf("current before open: %+v %v", current, err)
	}
	for _, role := range []model.Role{model.RoleStudent, model.RoleProfessor, model.RoleCompanySupervisor, model.RoleAdmin} {
		actor := applicationTestUser(t, db, suffix, "term-role-"+string(role), role, nil)
		if _, err := terms.Create(actor.ID, 1405, model.InternshipTermTypeSummer); !errors.Is(err, ErrTermAccessDenied) {
			t.Fatalf("create as %s: %v", role, err)
		}
		if _, err := terms.List(actor.ID); !errors.Is(err, ErrTermAccessDenied) {
			t.Fatalf("list as %s: %v", role, err)
		}
		if _, _, err := terms.Detail(actor.ID, 1); !errors.Is(err, ErrTermAccessDenied) {
			t.Fatalf("detail as %s: %v", role, err)
		}
		if _, err := terms.Close(actor.ID, 1); !errors.Is(err, ErrTermAccessDenied) {
			t.Fatalf("close as %s: %v", role, err)
		}
	}
	for _, input := range []struct {
		year int
		kind model.InternshipTermType
	}{{0, model.InternshipTermTypeFirst}, {10000, model.InternshipTermTypeFirst}, {1405, "OTHER"}} {
		if _, err := terms.Create(university.ID, input.year, input.kind); !errors.Is(err, ErrInvalidTerm) {
			t.Fatalf("invalid input: %v", err)
		}
	}
	term, err := terms.Create(university.ID, 1405, model.InternshipTermTypeSummer)
	if err != nil || term.Status != model.InternshipTermStatusOpen || term.OpenedAt.IsZero() || term.CreatedBy != university.ID || term.ClosedAt != nil {
		t.Fatalf("create term: %+v %v", term, err)
	}
	if _, err := terms.Create(university.ID, 1405, model.InternshipTermTypeSummer); !errors.Is(err, ErrDuplicateTerm) {
		t.Fatalf("duplicate term: %v", err)
	}
	if _, err := terms.Create(university.ID, 1406, model.InternshipTermTypeFirst); !errors.Is(err, ErrTermAlreadyOpen) {
		t.Fatalf("second open term: %v", err)
	}
	if list, err := terms.List(university.ID); err != nil || len(list) != 1 {
		t.Fatalf("term list: %+v %v", list, err)
	}
	createdCase, created, err := workflow.CreateOrGetCase(newStudent.ID)
	if err != nil || !created || createdCase.TermID == nil || *createdCase.TermID != term.ID || createdCase.Term == nil {
		t.Fatalf("automatic assignment: %+v %v %v", createdCase, created, err)
	}
	for _, status := range nonTerminalCaseStatuses() {
		t.Run("reuses "+string(status), func(t *testing.T) {
			user := student()
			item := model.InternshipCase{TermID: &term.ID, StudentID: user.ID, ProfessorID: professor.ID, Status: status}
			if err := db.Create(&item).Error; err != nil {
				t.Fatal(err)
			}
			reused, created, err := workflow.CreateOrGetCase(user.ID)
			if err != nil || created || reused.ID != item.ID {
				t.Fatalf("existing %s: %+v %v %v", status, reused, created, err)
			}
		})
	}
	for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
		t.Run("same term retry "+string(status), func(t *testing.T) {
			user := student()
			old := model.InternshipCase{TermID: &term.ID, StudentID: user.ID, ProfessorID: professor.ID, Status: status}
			if err := db.Create(&old).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(&old, old.ID).Error; err != nil {
				t.Fatal(err)
			}
			retry, created, err := workflow.CreateOrGetCase(user.ID)
			if err != nil || !created || retry.ID == old.ID || retry.TermID == nil || *retry.TermID != term.ID {
				t.Fatalf("retry: %+v %v %v", retry, created, err)
			}
			var persisted model.InternshipCase
			if err := db.First(&persisted, old.ID).Error; err != nil || !reflect.DeepEqual(old, persisted) {
				t.Fatalf("history changed: %v", err)
			}
		})
	}

	// All current statuses, plus another term and a nullable legacy case.
	closedCases := []model.InternshipCase{}
	for _, status := range studentReadableCaseStatuses() {
		user := student()
		now := time.Now().UTC().Truncate(time.Microsecond)
		comment := "existing cancellation"
		item := model.InternshipCase{TermID: &term.ID, StudentID: user.ID, ProfessorID: professor.ID, Status: status}
		if status == model.InternshipCaseStatusCancelled {
			item.CancellationComment, item.CancelledAt = &comment, &now
		}
		if status == model.InternshipCaseStatusPassed || status == model.InternshipCaseStatusFailed {
			item.CompletedAt = &now
		}
		if status == model.InternshipCaseStatusActive {
			item.ActivatedAt = &now
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.First(&item, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		closedCases = append(closedCases, item)
	}
	legacyUser := student()
	legacy := model.InternshipCase{StudentID: legacyUser.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusFailed}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	otherTerm := model.InternshipTerm{AcademicYear: 1404, TermType: model.InternshipTermTypeSummer, Status: model.InternshipTermStatusClosed, OpenedAt: time.Now(), CreatedBy: university.ID}
	if err := db.Create(&otherTerm).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&legacy, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	otherUser := student()
	otherCase := model.InternshipCase{TermID: &otherTerm.ID, StudentID: otherUser.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusDraft}
	if err := db.Create(&otherCase).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&otherCase, otherCase.ID).Error; err != nil {
		t.Fatal(err)
	}
	_, termCases, err := terms.Detail(university.ID, term.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundDraft := false
	for _, item := range termCases {
		if item.Status == model.InternshipCaseStatusDraft {
			foundDraft = true
		}
		if item.TermID == nil || *item.TermID != term.ID {
			t.Fatal("term detail leaked other cases")
		}
	}
	if !foundDraft {
		t.Fatal("term detail omitted drafts")
	}
	closed, err := terms.Close(university.ID, term.ID)
	if err != nil || closed.Status != model.InternshipTermStatusClosed || closed.ClosedAt == nil || closed.ClosedBy == nil || *closed.ClosedBy != university.ID {
		t.Fatalf("close: %+v %v", closed, err)
	}
	if err := db.First(closed, closed.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, before := range closedCases {
		t.Run("closure "+string(before.Status), func(t *testing.T) {
			var after model.InternshipCase
			if err := db.First(&after, before.ID).Error; err != nil {
				t.Fatal(err)
			}
			if containsStatus(terminalCaseStatuses(), before.Status) {
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("terminal history changed: before=%+v after=%+v", before, after)
				}
			} else if after.Status != model.InternshipCaseStatusCancelled || after.CancelledAt == nil || !after.CancelledAt.Equal(*closed.ClosedAt) || after.CancellationComment == nil || *after.CancellationComment != TermClosureCancellationComment || *after.TermID != term.ID {
				t.Fatalf("automatic cancellation: %+v", after)
			}
		})
	}
	for _, before := range []model.InternshipCase{legacy, otherCase} {
		var after model.InternshipCase
		if err := db.First(&after, before.ID).Error; err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("unrelated/legacy case changed: %v", err)
		}
	}
	if old, err := workflow.GetStudentHistoricalCase(legacyUser.ID, legacy.ID); err != nil || old.TermID != nil || old.Term != nil {
		t.Fatalf("read legacy: %+v %v", old, err)
	}
	if _, err := terms.Close(university.ID, term.ID); !errors.Is(err, ErrTermClosed) {
		t.Fatalf("close twice: %v", err)
	}
	if _, err := terms.Create(university.ID, 1405, model.InternshipTermTypeSummer); !errors.Is(err, ErrDuplicateTerm) {
		t.Fatalf("duplicate closed identity: %v", err)
	}
	if _, _, err := workflow.CreateOrGetCase(newStudent.ID); !errors.Is(err, ErrNoOpenInternshipTerm) {
		t.Fatalf("retry without term: %v", err)
	}
	nextTerm, err := terms.Create(university.ID, 1406, model.InternshipTermTypeFirst)
	if err != nil {
		t.Fatal(err)
	}
	for _, before := range closedCases {
		if before.Status == model.InternshipCaseStatusPassed {
			if _, _, err := workflow.CreateOrGetCase(before.StudentID); !errors.Is(err, ErrInternshipPassed) {
				t.Fatalf("passed case retry after next term: %v", err)
			}
			company := applicationTestCompany(t, db, suffix, "pt")
			supervisor := applicationTestUser(t, db, suffix, "passed-term-supervisor", model.RoleCompanySupervisor, &company.ID)
			opportunity := applicationTestOpportunity(t, db, suffix, "passed-term-opportunity", company.ID, supervisor.ID, model.OpportunityStatusOpen)
			resume := applicationTestResume(suffix, "passed-term-resume")
			if _, err := NewOpportunityApplicationService(db).Apply(before.StudentID, opportunity.ID, resume); !errors.Is(err, ErrInternshipAlreadyPassed) {
				t.Fatalf("passed application after next term: %v", err)
			}
			assertNoApplicationOrFile(t, db, before.StudentID, opportunity.ID, resume)
			continue
		}
		retry, created, err := workflow.CreateOrGetCase(before.StudentID)
		if err != nil || !created || retry.ID == before.ID || retry.TermID == nil || *retry.TermID != nextTerm.ID {
			t.Fatalf("next term retry: %+v %v %v", retry, created, err)
		}
	}
}

func TestTermClosureRollsBack(t *testing.T) {
	db := termTestDatabase(t)
	suffix := time.Now().UnixNano()
	university := applicationTestUser(t, db, suffix, "rollback-university", model.RoleUniversitySupervisor, nil)
	professor := applicationTestUser(t, db, suffix, "rollback-professor", model.RoleProfessor, nil)
	terms := NewInternshipTermService(db)
	term, err := terms.Create(university.ID, 1405, model.InternshipTermTypeSummer)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.First(term, term.ID).Error; err != nil {
		t.Fatal(err)
	}
	cases := []model.InternshipCase{}
	for index := 0; index < 2; index++ {
		user := applicationTestUser(t, db, suffix, fmt.Sprintf("rollback-student-%d", index), model.RoleStudent, nil)
		item := model.InternshipCase{TermID: &term.ID, StudentID: user.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusDraft}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.First(&item, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		cases = append(cases, item)
	}
	assertUnchanged := func() {
		t.Helper()
		var persisted model.InternshipTerm
		if err := db.First(&persisted, term.ID).Error; err != nil || !reflect.DeepEqual(*term, persisted) {
			t.Fatalf("term was changed on failure: %+v %v", persisted, err)
		}
		for _, before := range cases {
			var after model.InternshipCase
			if err := db.First(&after, before.ID).Error; err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("case was changed on failure: %+v %v", after, err)
			}
		}
	}
	// A real PostgreSQL failure during case updates must roll back the whole close.
	if err := db.Exec(fmt.Sprintf(`CREATE FUNCTION reject_case_cancel() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id = %d THEN RAISE EXCEPTION 'test cancellation failure'; END IF; RETURN NEW; END $$`, cases[1].ID)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER reject_case_cancel BEFORE UPDATE ON internship_cases FOR EACH ROW EXECUTE FUNCTION reject_case_cancel()`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := terms.Close(university.ID, term.ID); err == nil {
		t.Fatal("expected case update failure")
	}
	assertUnchanged()
	if err := db.Exec(`DROP TRIGGER reject_case_cancel ON internship_cases`).Error; err != nil {
		t.Fatal(err)
	}
	// Failure in the final term update also rolls back successful cancellations.
	if err := db.Exec(`CREATE FUNCTION reject_term_close() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test term close failure'; END $$`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER reject_term_close BEFORE UPDATE ON internship_terms FOR EACH ROW EXECUTE FUNCTION reject_term_close()`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := terms.Close(university.ID, term.ID); err == nil {
		t.Fatal("expected term update failure")
	}
	assertUnchanged()
	if err := db.Exec(`DROP TRIGGER reject_term_close ON internship_terms`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := terms.Close(university.ID, term.ID); err != nil {
		t.Fatalf("close after failures: %v", err)
	}
}

func TestInternshipTermConcurrency(t *testing.T) {
	db := termTestDatabase(t)
	suffix := time.Now().UnixNano()
	university := applicationTestUser(t, db, suffix, "concurrent-university", model.RoleUniversitySupervisor, nil)
	professor := applicationTestUser(t, db, suffix, "concurrent-professor", model.RoleProfessor, nil)
	terms := NewInternshipTermService(db)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, kind := range []model.InternshipTermType{model.InternshipTermTypeFirst, model.InternshipTermTypeSummer} {
		go func(kind model.InternshipTermType) {
			<-start
			_, err := terms.Create(university.ID, 1405, kind)
			results <- err
		}(kind)
	}
	close(start)
	successes, conflicts := 0, 0
	for index := 0; index < 2; index++ {
		select {
		case err := <-results:
			if err == nil {
				successes++
			} else if errors.Is(err, ErrTermAlreadyOpen) {
				conflicts++
			} else {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent term creation timed out")
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("creation: %d successes %d conflicts", successes, conflicts)
	}
	term, err := terms.Current()
	if err != nil || term == nil {
		t.Fatalf("current: %+v %v", term, err)
	}
	// Also prove the database constraint works without service prechecks.
	extra := model.InternshipTerm{AcademicYear: 1406, TermType: model.InternshipTermTypeSummer, Status: model.InternshipTermStatusOpen, OpenedAt: time.Now(), CreatedBy: university.ID}
	if err := db.Create(&extra).Error; !isUniqueViolation(err) {
		t.Fatalf("one-open database index: %v", err)
	}
	duplicate := model.InternshipTerm{AcademicYear: term.AcademicYear, TermType: term.TermType, Status: model.InternshipTermStatusClosed, OpenedAt: time.Now(), CreatedBy: university.ID}
	if err := db.Create(&duplicate).Error; !isUniqueViolation(err) {
		t.Fatalf("identity database index: %v", err)
	}
	createStudent := func(label string) model.User {
		user := applicationTestUser(t, db, suffix, label, model.RoleStudent, nil)
		if err := db.Create(&model.ProfessorAssignment{StudentID: user.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}).Error; err != nil {
			t.Fatal(err)
		}
		return user
	}
	t.Run("close waits for case creation then cancels it", func(t *testing.T) {
		user := createStudent("creation-first")
		tx := db.Begin()
		if tx.Error != nil {
			t.Fatal(tx.Error)
		}
		defer tx.Rollback()
		item, created, err := NewInternshipService(tx).CreateOrGetCase(user.ID)
		if err != nil || !created {
			t.Fatalf("creation: %+v %v %v", item, created, err)
		}
		started, finished := make(chan struct{}), make(chan error, 1)
		go func() { close(started); _, err := terms.Close(university.ID, term.ID); finished <- err }()
		<-started
		select {
		case err := <-finished:
			t.Fatalf("closure passed held SHARE lock: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		if err := tx.Commit().Error; err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-finished:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("close timed out")
		}
		var persisted model.InternshipCase
		if err := db.First(&persisted, item.ID).Error; err != nil || persisted.Status != model.InternshipCaseStatusCancelled {
			t.Fatalf("concurrent case left unfinished: %+v %v", persisted, err)
		}
	})
	t.Run("creation waits for closure then rejects closed term", func(t *testing.T) {
		term, err := terms.Create(university.ID, 1406, model.InternshipTermTypeFirst)
		if err != nil {
			t.Fatal(err)
		}
		user := createStudent("closure-first")
		tx := db.Begin()
		if tx.Error != nil {
			t.Fatal(tx.Error)
		}
		defer tx.Rollback()
		if _, err := NewInternshipTermService(tx).Close(university.ID, term.ID); err != nil {
			t.Fatal(err)
		}
		started, finished := make(chan struct{}), make(chan error, 1)
		go func() {
			close(started)
			_, _, err := NewInternshipService(db).CreateOrGetCase(user.ID)
			finished <- err
		}()
		<-started
		select {
		case err := <-finished:
			t.Fatalf("creation passed held UPDATE lock: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		if err := tx.Commit().Error; err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-finished:
			if !errors.Is(err, ErrNoOpenInternshipTerm) {
				t.Fatalf("creation after close: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("creation timed out")
		}
		var count int64
		if err := db.Model(&model.InternshipCase{}).Where("student_id = ?", user.ID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("closed term received a case: %d %v", count, err)
		}
	})
}

func TestTermClosurePreservesHistoricalAuthorization(t *testing.T) {
	db := termTestDatabase(t)
	suffix := time.Now().UnixNano()
	university := applicationTestUser(t, db, suffix, "access-university", model.RoleUniversitySupervisor, nil)
	professor := applicationTestUser(t, db, suffix, "access-professor", model.RoleProfessor, nil)
	otherProfessor := applicationTestUser(t, db, suffix, "access-other-professor", model.RoleProfessor, nil)
	company := applicationTestCompany(t, db, suffix, "ac")
	supervisor := applicationTestUser(t, db, suffix, "access-supervisor", model.RoleCompanySupervisor, &company.ID)
	otherSupervisor := applicationTestUser(t, db, suffix, "access-other-supervisor", model.RoleCompanySupervisor, &company.ID)
	terms := NewInternshipTermService(db)
	term, err := terms.Create(university.ID, 1405, model.InternshipTermTypeSummer)
	if err != nil {
		t.Fatal(err)
	}
	workflow := NewInternshipService(db)
	student := applicationTestUser(t, db, suffix, "access-student", model.RoleStudent, nil)
	now := time.Now()
	item := model.InternshipCase{TermID: &term.ID, StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusActive, ActivatedAt: &now, CompanySupervisorID: &supervisor.ID}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	opportunity := applicationTestOpportunity(t, db, suffix, "access-opportunity", company.ID, supervisor.ID, model.OpportunityStatusOpen)
	resume := applicationTestResume(suffix, "access-resume")
	if err := db.Create(resume).Error; err != nil {
		t.Fatal(err)
	}
	application := model.OpportunityApplication{StudentID: student.ID, OpportunityID: opportunity.ID, ResumeFileID: resume.ID, Status: model.ApplicationStatusAccepted}
	if err := db.Create(&application).Error; err != nil {
		t.Fatal(err)
	}
	preference := model.InternshipPreference{InternshipCaseID: item.ID, OpportunityApplicationID: application.ID, Priority: 1}
	if err := db.Create(&preference).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&item).Update("selected_preference_id", preference.ID).Error; err != nil {
		t.Fatal(err)
	}
	draftUser := applicationTestUser(t, db, suffix, "access-draft", model.RoleStudent, nil)
	draft := model.InternshipCase{TermID: &term.ID, StudentID: draftUser.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusDraft}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.GetProfessorCase(professor.ID, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.GetCompanyCase(supervisor.ID, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := terms.Close(university.ID, term.ID); err != nil {
		t.Fatal(err)
	}
	if historical, err := workflow.GetProfessorCase(professor.ID, item.ID); err != nil || historical.Status != model.InternshipCaseStatusCancelled {
		t.Fatalf("professor history: %+v %v", historical, err)
	}
	if historical, err := workflow.GetCompanyCase(supervisor.ID, item.ID); err != nil || historical.Status != model.InternshipCaseStatusCancelled {
		t.Fatalf("company history: %+v %v", historical, err)
	}
	if _, err := workflow.GetProfessorCase(otherProfessor.ID, item.ID); !errors.Is(err, ErrCaseAccessDenied) {
		t.Fatalf("other professor access: %v", err)
	}
	if _, err := workflow.GetCompanyCase(otherSupervisor.ID, item.ID); !errors.Is(err, ErrCaseNotAssignedToCompany) {
		t.Fatalf("other supervisor access: %v", err)
	}
	if _, err := workflow.GetProfessorCase(professor.ID, draft.ID); !errors.Is(err, ErrCaseAccessDenied) {
		t.Fatalf("cancelled draft exposed to professor: %v", err)
	}
	if _, err := workflow.GetCompanyCase(supervisor.ID, draft.ID); !errors.Is(err, ErrCaseNotAssignedToCompany) {
		t.Fatalf("cancelled draft exposed to company: %v", err)
	}
	if items, err := workflow.ListProfessorCases(professor.ID); err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("professor list: %+v %v", items, err)
	}
	if items, err := workflow.ListCompanyCases(supervisor.ID); err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("company list: %+v %v", items, err)
	}
	if _, err := workflow.CompleteProfessorCase(professor.ID, item.ID, ProfessorCompletionInput{Result: model.ProfessorFinalResultGood}); !errors.Is(err, ErrProfessorCaseNotActive) {
		t.Fatalf("cancelled case mutation: %v", err)
	}
}

func TestTermMigrationPreservesLegacyCases(t *testing.T) {
	db := termTestDatabase(t)
	suffix := time.Now().UnixNano()
	student := applicationTestUser(t, db, suffix, "migration-student", model.RoleStudent, nil)
	professor := applicationTestUser(t, db, suffix, "migration-professor", model.RoleProfessor, nil)
	// Reproduce the pre-milestone schema in this disposable test schema.
	if err := db.Exec("ALTER TABLE internship_cases DROP COLUMN term_id").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO internship_cases (student_id, professor_id, status, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())", student.ID, professor.ID, model.InternshipCaseStatusFailed).Error; err != nil {
		t.Fatal(err)
	}
	var before struct {
		ID          uint
		StudentID   uint
		ProfessorID uint
		Status      model.InternshipCaseStatus
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}
	if err := db.Table("internship_cases").First(&before).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateAndSeed(db); err != nil {
		t.Fatal(err)
	}
	item, err := NewInternshipService(db).GetStudentHistoricalCase(student.ID, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.TermID != nil || item.Term != nil || item.Status != before.Status || item.StudentID != before.StudentID || item.ProfessorID != before.ProfessorID || !item.CreatedAt.Equal(before.CreatedAt) || !item.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("migration rewrote legacy case: %+v", item)
	}
	if current, err := NewInternshipTermService(db).Current(); err != nil || current != nil {
		t.Fatalf("migration invented an open term: %+v %v", current, err)
	}
}
