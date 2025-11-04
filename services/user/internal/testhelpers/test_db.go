package testhelpers

import (
	"fmt"
	"testing"

	"peerprep/user/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	openSQLite      = func(dsn string) (*gorm.DB, error) { return gorm.Open(sqlite.Open(dsn), &gorm.Config{}) }
    migrateSchema   = func(db *gorm.DB) error { return db.AutoMigrate(&models.User{}, &models.Token{}) }
	dropUserTableFn = func(db *gorm.DB) error { return db.Migrator().DropTable(&models.User{}) }
)

// SetupTestDB creates an isolated in-memory SQLite database for tests.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := openSQLite(dsn)
	if err != nil {
		panic(fmt.Sprintf("failed to open test database: %v", err))
	}
	if err := migrateSchema(db); err != nil {
		panic(fmt.Sprintf("failed to migrate test database: %v", err))
	}
	return db
}

// DropUserTable removes the users table to force repository errors.
func DropUserTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := dropUserTableFn(db); err != nil {
		panic(fmt.Sprintf("failed to drop user table: %v", err))
	}
}
