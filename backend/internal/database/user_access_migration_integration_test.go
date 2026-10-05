package database

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"internship-management-system/backend/internal/model"
)

func TestLegacyUserAccessBackfillAndSeedSafety(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	schema := fmt.Sprintf("m22_legacy_%d", time.Now().UnixNano())
	must(tx.Exec("CREATE SCHEMA " + schema).Error)
	must(tx.Exec("SET LOCAL search_path TO " + schema).Error)
	must(MigrateAndSeed(tx))
	var before []model.User
	var assignment model.ProfessorAssignment
	must(tx.Order("id").Find(&before).Error)
	must(tx.First(&assignment).Error)
	// Recreate the exact absence of M22 columns on a previously deployed schema.
	must(tx.Exec("ALTER TABLE users DROP CONSTRAINT chk_user_verification_status").Error)
	for _, column := range []string{"is_active", "verification_status", "verification_reviewed_at", "verification_reviewed_by", "verification_rejection_reason", "verification_resubmitted_at"} {
		must(tx.Exec("ALTER TABLE users DROP COLUMN " + column).Error)
	}
	must(MigrateAndSeed(tx))
	var after []model.User
	must(tx.Order("id").Find(&after).Error)
	if len(before) != len(after) {
		t.Fatal("migration recreated user accounts")
	}
	for i, user := range after {
		if !user.IsActive {
			t.Fatal("legacy account disabled")
		}
		want := model.UserVerificationNotRequired
		if user.Role == model.RoleUniversitySupervisor {
			want = model.UserVerificationApproved
		}
		if user.VerificationStatus != want {
			t.Fatalf("legacy role %s verification %s", user.Role, user.VerificationStatus)
		}
		if !reflect.DeepEqual(user, before[i]) {
			t.Fatal("migration mutated legacy account identity or timestamps")
		}
	}
	var currentAssignment model.ProfessorAssignment
	must(tx.First(&currentAssignment, assignment.ID).Error)
	if !reflect.DeepEqual(assignment, currentAssignment) {
		t.Fatal("legacy relationship changed")
	}
	var university, admin model.User
	must(tx.Where("role = ?", model.RoleUniversitySupervisor).First(&university).Error)
	must(tx.Where("role = ?", model.RoleAdmin).First(&admin).Error)
	for _, status := range []model.UserVerificationStatus{model.UserVerificationRejected, model.UserVerificationPending} {
		now := time.Now().UTC()
		must(tx.Model(&university).Updates(map[string]any{"is_active": false, "verification_status": status, "verification_reviewed_at": now, "verification_reviewed_by": admin.ID, "verification_rejection_reason": "preserved decision"}).Error)
		must(tx.Model(&model.User{}).Where("role <> ?", model.RoleUniversitySupervisor).Update("is_active", false).Error)
		var decisions []model.User
		must(tx.Order("id").Find(&decisions).Error)
		for i := 0; i < 2; i++ {
			must(MigrateAndSeed(tx))
		}
		var unchanged []model.User
		must(tx.Order("id").Find(&unchanged).Error)
		if !reflect.DeepEqual(decisions, unchanged) {
			t.Fatal("repeated startup reset verification or enabled accounts")
		}
	}
	newUser := model.User{FullName: "Self registered", Email: "m22-new@example.test", PasswordHash: "fixture", Role: model.RoleUniversitySupervisor, VerificationStatus: model.UserVerificationPending, IsActive: true}
	must(tx.Create(&newUser).Error)
	must(MigrateAndSeed(tx))
	must(tx.First(&newUser, newUser.ID).Error)
	if newUser.VerificationStatus != model.UserVerificationPending || !newUser.IsActive {
		t.Fatal("new registration backfilled as legacy trusted account")
	}
}
