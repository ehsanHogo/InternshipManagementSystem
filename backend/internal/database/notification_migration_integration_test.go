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

func TestNotificationMigrationDoesNotBackfillAndPreservesExistingRows(t *testing.T) {
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
	schema := fmt.Sprintf("m23_migration_%d", time.Now().UnixNano())
	must(tx.Exec("CREATE SCHEMA " + schema).Error)
	must(tx.Exec("SET LOCAL search_path TO " + schema).Error)
	must(MigrateAndSeed(tx))
	// Simulate M22's existing users with no notifications table.
	must(tx.Migrator().DropTable(&model.Notification{}))
	var before []model.User
	must(tx.Order("id").Find(&before).Error)
	must(MigrateAndSeed(tx))
	var count int64
	must(tx.Model(&model.Notification{}).Count(&count).Error)
	if count != 0 {
		t.Fatal("historical notifications were backfilled")
	}
	var after []model.User
	must(tx.Order("id").Find(&after).Error)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("notification migration changed existing accounts")
	}
	for _, index := range []string{"idx_notifications_user_created", "idx_notifications_user_read", "idx_notifications_unread_action"} {
		if !tx.Migrator().HasIndex(&model.Notification{}, index) {
			t.Fatalf("missing index %s", index)
		}
	}
	// Simulate an already deployed M23 table and preserve its generic history.
	must(tx.Exec("ALTER TABLE notifications ALTER COLUMN type TYPE varchar(32)").Error)
	legacy := model.Notification{UserID: before[0].ID, Type: "COMPANY_ACTION_REQUIRED", Title: "قدیمی", Message: "محفوظ"}
	must(tx.Create(&legacy).Error)
	must(tx.First(&legacy, legacy.ID).Error)
	must(MigrateAndSeed(tx))
	var old model.Notification
	must(tx.First(&old, legacy.ID).Error)
	if !reflect.DeepEqual(legacy, old) {
		t.Fatal("category migration rewrote legacy notification")
	}
	// The longest category must fit after upgrading the original varchar(32).
	n := model.Notification{UserID: before[0].ID, Type: model.NotificationAdminUniversityVerification, Title: "اعلان", Message: "محفوظ", IsRead: true}
	must(tx.Create(&n).Error)
	// Compare persisted timestamps at PostgreSQL precision.
	must(tx.First(&n, n.ID).Error)
	must(MigrateAndSeed(tx))
	must(MigrateAndSeed(tx))
	var stored model.Notification
	must(tx.First(&stored, n.ID).Error)
	if !reflect.DeepEqual(n, stored) {
		t.Fatal("startup changed existing notification")
	}
}
