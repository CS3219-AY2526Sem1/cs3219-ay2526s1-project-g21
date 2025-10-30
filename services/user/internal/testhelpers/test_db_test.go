package testhelpers

import (
	"errors"
	"testing"

	"peerprep/user/internal/models"

	"gorm.io/gorm"
)

func TestSetupTestDBCreatesSchema(t *testing.T) {
	db := SetupTestDB(t)
	if !db.Migrator().HasTable(&models.User{}) {
		t.Fatalf("expected users table to exist")
	}
}

func TestDropUserTableRemovesTable(t *testing.T) {
	db := SetupTestDB(t)
	DropUserTable(t, db)
	if db.Migrator().HasTable(&models.User{}) {
		t.Fatalf("expected users table to be dropped")
	}
}

func TestSetupTestDBPanicsOnOpenFailure(t *testing.T) {
	orig := openSQLite
	defer func() { openSQLite = orig }()
	openSQLite = func(string) (*gorm.DB, error) { return nil, errors.New("boom") }

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on open failure")
		}
	}()

	SetupTestDB(t)
}

func TestSetupTestDBPanicsOnMigrateFailure(t *testing.T) {
	origOpen := openSQLite
	defer func() { openSQLite = origOpen }()
	origMigrate := migrateSchema
	defer func() { migrateSchema = origMigrate }()

	openSQLite = func(dsn string) (*gorm.DB, error) { return origOpen(dsn) }
	migrateSchema = func(*gorm.DB) error { return errors.New("migrate boom") }

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on migrate failure")
		}
	}()

	SetupTestDB(t)
}

func TestDropUserTablePanicsOnFailure(t *testing.T) {
	db := SetupTestDB(t)
	orig := dropUserTableFn
	defer func() { dropUserTableFn = orig }()
	dropUserTableFn = func(*gorm.DB) error { return errors.New("drop fail") }

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on drop failure")
		}
	}()

	DropUserTable(t, db)
}
