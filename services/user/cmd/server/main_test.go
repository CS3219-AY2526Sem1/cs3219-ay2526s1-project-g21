package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"peerprep/user/internal/models"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareServerGlobals(t *testing.T) {
	t.Helper()
	resetServerGlobals()
	t.Cleanup(resetServerGlobals)
}

func TestConnectWithRetrySuccess(t *testing.T) {
	prepareServerGlobals(t)
	origOpen := gormOpen
	defer func() { gormOpen = origOpen }()

	var calls int32
	gormOpen = func(dsn string) (*gorm.DB, error) {
		atomic.AddInt32(&calls, 1)
		db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	logger := zap.NewNop()
	db, err := connectWithRetry("dsn", time.Second, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected single connection attempt, got %d", calls)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql DB: %v", err)
	}
	sqlDB.Close()
}

func TestConnectWithRetryFailure(t *testing.T) {
	prepareServerGlobals(t)
	origOpen := gormOpen
	defer func() { gormOpen = origOpen }()

	gormOpen = func(string) (*gorm.DB, error) {
		return nil, errors.New("connect failed")
	}

	logger := zap.NewNop()
	_, err := connectWithRetry("dsn", 200*time.Millisecond, logger)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
}

func TestConnectWithRetryPingFailure(t *testing.T) {
	prepareServerGlobals(t)
	origOpen := gormOpen
	defer func() { gormOpen = origOpen }()

	gormOpen = func(string) (*gorm.DB, error) {
		db, err := gorm.Open(sqlite.Open("file:ping-fail?mode=memory&cache=shared"), &gorm.Config{})
		if err != nil {
			return nil, err
		}
		sqlDB, err := db.DB()
		if err != nil {
			return nil, err
		}
		sqlDB.Close()
		return db, nil
	}

	logger := zap.NewNop()
	if _, err := connectWithRetry("dsn", 600*time.Millisecond, logger); err == nil {
		t.Fatalf("expected error due to ping failure")
	}
}

func TestRunSuccess(t *testing.T) {
	prepareServerGlobals(t)
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "pass")
	t.Setenv("POSTGRES_DB", "db")
	t.Setenv("PORT", "")
	t.Setenv("SKIP_REDIS_SUBSCRIBER", "true")

	origOpen := gormOpen
	defer func() { gormOpen = origOpen }()
	origListen := httpListenServe
	defer func() { httpListenServe = origListen }()
	origTimeout := dbConnectTimeout
	defer func() { dbConnectTimeout = origTimeout }()
	dbConnectTimeout = 100 * time.Millisecond

	var listenedAddr string
	servedHealth := false
	httpListenServe = func(addr string, handler http.Handler) error {
		listenedAddr = addr
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		servedHealth = rec.Body.String() == "ok"
		return nil
	}

	gormOpen = func(dsn string) (*gorm.DB, error) {
		db, err := gorm.Open(sqlite.Open("file:run-success?mode=memory&cache=shared"), &gorm.Config{})
		if err != nil {
			return nil, err
		}
		if err := db.AutoMigrate(&models.User{}); err != nil {
			return nil, err
		}
		return db, nil
	}

	if err := run(); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if listenedAddr != ":8080" {
		t.Fatalf("expected listen addr :8080, got %s", listenedAddr)
	}
	if !servedHealth {
		t.Fatalf("expected health endpoint to respond")
	}
}

func TestRunConnectFailure(t *testing.T) {
	prepareServerGlobals(t)
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "pass")
	t.Setenv("POSTGRES_DB", "db")
	t.Setenv("SKIP_REDIS_SUBSCRIBER", "true")

	origOpen := gormOpen
	defer func() { gormOpen = origOpen }()
	origTimeout := dbConnectTimeout
	defer func() { dbConnectTimeout = origTimeout }()
	dbConnectTimeout = 50 * time.Millisecond

	gormOpen = func(string) (*gorm.DB, error) {
		return nil, errors.New("connect failed")
	}

	if err := run(); err == nil {
		t.Fatalf("expected error from run when connection fails")
	}
}

func TestRunListenFailure(t *testing.T) {
	prepareServerGlobals(t)
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "pass")
	t.Setenv("POSTGRES_DB", "db")
	t.Setenv("SKIP_REDIS_SUBSCRIBER", "true")

	origOpen := gormOpen
	defer func() { gormOpen = origOpen }()
	origListen := httpListenServe
	defer func() { httpListenServe = origListen }()
	origTimeout := dbConnectTimeout
	defer func() { dbConnectTimeout = origTimeout }()
	dbConnectTimeout = 100 * time.Millisecond

	gormOpen = func(dsn string) (*gorm.DB, error) {
		db, err := gorm.Open(sqlite.Open("file:run-listen?mode=memory&cache=shared"), &gorm.Config{})
		if err != nil {
			return nil, err
		}
		if err := db.AutoMigrate(&models.User{}); err != nil {
			return nil, err
		}
		return db, nil
	}
	httpListenServe = func(string, http.Handler) error {
		return errors.New("listen failed")
	}

	if err := run(); err == nil {
		t.Fatalf("expected listen error from run")
	}
}

func TestRunAutoMigrateFailure(t *testing.T) {
	prepareServerGlobals(t)
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "pass")
	t.Setenv("POSTGRES_DB", "db")
	t.Setenv("SKIP_REDIS_SUBSCRIBER", "true")

	origOpen := gormOpen
	defer func() { gormOpen = origOpen }()
	origAuto := runAutoMigrate
	defer func() { runAutoMigrate = origAuto }()
	origTimeout := dbConnectTimeout
	defer func() { dbConnectTimeout = origTimeout }()
	dbConnectTimeout = 100 * time.Millisecond

	gormOpen = func(dsn string) (*gorm.DB, error) {
		return gorm.Open(sqlite.Open("file:run-migrate?mode=memory&cache=shared"), &gorm.Config{})
	}
	runAutoMigrate = func(*gorm.DB, ...interface{}) error {
		return errors.New("migrate failed")
	}

	if err := run(); err == nil {
		t.Fatalf("expected migrate error from run")
	}
}

func TestRunLoggerFailure(t *testing.T) {
	prepareServerGlobals(t)
	origLogger := newLogger
	defer func() { newLogger = origLogger }()

	newLogger = func(...zap.Option) (*zap.Logger, error) { return nil, errors.New("logger boom") }

	if err := run(); err == nil {
		t.Fatalf("expected logger error from run")
	}
}

func TestMainFunction(t *testing.T) {
	prepareServerGlobals(t)
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "pass")
	t.Setenv("POSTGRES_DB", "db")
	t.Setenv("PORT", "9090")
	t.Setenv("SKIP_REDIS_SUBSCRIBER", "true")

	origOpen := gormOpen
	defer func() { gormOpen = origOpen }()
	origListen := httpListenServe
	defer func() { httpListenServe = origListen }()
	origTimeout := dbConnectTimeout
	defer func() { dbConnectTimeout = origTimeout }()
	dbConnectTimeout = 100 * time.Millisecond

	gormOpen = func(dsn string) (*gorm.DB, error) {
		db, err := gorm.Open(sqlite.Open("file:main-test?mode=memory&cache=shared"), &gorm.Config{})
		if err != nil {
			return nil, err
		}
		if err := db.AutoMigrate(&models.User{}); err != nil {
			return nil, err
		}
		return db, nil
	}

	var listened string
	servedHealth := false
	httpListenServe = func(addr string, handler http.Handler) error {
		listened = addr
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		servedHealth = rec.Body.String() == "ok"
		return nil
	}

	main()

	if listened != ":9090" {
		t.Fatalf("expected main to listen on :9090, got %s", listened)
	}
	if !servedHealth {
		t.Fatalf("expected health endpoint to be served")
	}
}

func TestMainHandlesError(t *testing.T) {
	prepareServerGlobals(t)
	t.Setenv("SKIP_REDIS_SUBSCRIBER", "true")
	origOpen := gormOpen
	defer func() { gormOpen = origOpen }()
	origTimeout := dbConnectTimeout
	defer func() { dbConnectTimeout = origTimeout }()
	origLogger := newLogger
	defer func() { newLogger = origLogger }()
	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	origLogFatal := logFatalFn
	defer func() { logFatalFn = origLogFatal }()

	dbConnectTimeout = 50 * time.Millisecond
	newLogger = func(...zap.Option) (*zap.Logger, error) { return zap.NewNop(), nil }
	gormOpen = func(string) (*gorm.DB, error) { return nil, errors.New("connect failed") }

	var captured error
	exitCalled := false
	exitFunc = func(int) { exitCalled = true }
	logFatalFn = func(err error) {
		captured = err
		exitFunc(1)
	}

	main()

	if captured == nil {
		t.Fatalf("expected logFatalFn to capture error")
	}
	if !exitCalled {
		t.Fatalf("expected exitFunc to be invoked")
	}
}

func TestDefaultLogFatal(t *testing.T) {
	prepareServerGlobals(t)
	origExit := exitFunc
	defer func() { exitFunc = origExit }()

	var code int
	exitFunc = func(c int) { code = c }

	defaultLogFatal(errors.New("boom"))

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestDefaultGormOpen(t *testing.T) {
	prepareServerGlobals(t)
	origDialector := newDialector
	defer func() { newDialector = origDialector }()

	newDialector = func(string) gorm.Dialector { return sqlite.Open("file:default-gorm?mode=memory&cache=shared") }

	db, err := defaultGormOpen("ignored")
	if err != nil {
		t.Fatalf("defaultGormOpen returned error: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql DB: %v", err)
	}
	sqlDB.Close()
}
