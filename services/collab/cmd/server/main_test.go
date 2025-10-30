package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestRunReturnsListenError(t *testing.T) {
	origListen := listenAndServe
	origExit := exitFunc
	t.Cleanup(func() {
		listenAndServe = origListen
		exitFunc = origExit
	})

	listenAndServe = func(addr string, handler http.Handler) error {
		if handler == nil {
			t.Fatalf("expected handler")
		}
		if addr != ":9090" {
			t.Fatalf("expected addr :9090, got %s", addr)
		}
		return errors.New("boom")
	}
	exitFunc = func(error) {}

	t.Setenv("PORT", "9090")
	t.Setenv("REDIS_ADDR", "localhost:0")
	t.Setenv("QUESTION_SERVICE_URL", "http://localhost")

	if err := run(context.TODO()); err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestMainCompletes(t *testing.T) {
	origListen := listenAndServe
	origExit := exitFunc
	t.Cleanup(func() {
		listenAndServe = origListen
		exitFunc = origExit
	})

	listenAndServe = func(string, http.Handler) error { return nil }
	exitFunc = func(error) { t.Fatal("exitFunc should not be called") }

	t.Setenv("PORT", "9091")
	t.Setenv("REDIS_ADDR", "localhost:0")
	t.Setenv("QUESTION_SERVICE_URL", "http://localhost")

	main()
}

func TestRunUsesDefaults(t *testing.T) {
	origListen := listenAndServe
	origDefaults := [3]string{defaultRedisAddr, defaultQuestionURL, defaultPort}
	t.Cleanup(func() {
		listenAndServe = origListen
		defaultRedisAddr = origDefaults[0]
		defaultQuestionURL = origDefaults[1]
		defaultPort = origDefaults[2]
	})

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	defaultRedisAddr = mr.Addr()
	defaultQuestionURL = "http://localhost" // unused but non-empty
	defaultPort = "8080"

	listenAndServe = func(addr string, handler http.Handler) error {
		if addr != ":8080" {
			t.Fatalf("expected default port, got %s", addr)
		}
		if handler == nil {
			t.Fatalf("handler nil")
		}
		return nil
	}

	t.Setenv("PORT", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("QUESTION_SERVICE_URL", "")

	if err := run(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestMainHandlesError(t *testing.T) {
	origListen := listenAndServe
	origExit := exitFunc
	t.Cleanup(func() {
		listenAndServe = origListen
		exitFunc = origExit
	})

	listenAndServe = func(string, http.Handler) error { return errors.New("main boom") }
	var got error
	exitFunc = func(err error) { got = err }

	t.Setenv("PORT", "9092")
	t.Setenv("REDIS_ADDR", "localhost:0")
	t.Setenv("QUESTION_SERVICE_URL", "http://localhost")

	main()

	if got == nil || got.Error() != "main boom" {
		t.Fatalf("expected exitFunc to capture error, got %v", got)
	}
}

func TestHealthHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	healthHandler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/collab/healthz", nil))
	if rec.Body.String() != "ok" {
		t.Fatalf("expected ok, got %q", rec.Body.String())
	}
}

func TestDefaultExit(t *testing.T) {
	origExit := exit
	origWriter := log.Writer()
	t.Cleanup(func() {
		exit = origExit
		log.SetOutput(origWriter)
	})

	var gotCode int
	exit = func(code int) { gotCode = code }
	var buf bytes.Buffer
	log.SetOutput(&buf)

	defaultExit(errors.New("boom"))
	if gotCode != 1 {
		t.Fatalf("expected exit code 1, got %d", gotCode)
	}
	if !bytes.Contains(buf.Bytes(), []byte("boom")) {
		t.Fatalf("expected log to contain boom, got %q", buf.String())
	}
}
