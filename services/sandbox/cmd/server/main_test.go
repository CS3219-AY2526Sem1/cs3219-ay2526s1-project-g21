package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"sandbox/internal/runtime"
)

func TestRunHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/run", nil)
	rec := httptest.NewRecorder()

	runHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error != "method_not_allowed" {
		t.Fatalf("unexpected error message %q", resp.Error)
	}
}

func TestRunHandlerInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()

	runHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "invalid_request" {
		t.Fatalf("expected invalid_request, got %q", resp.Error)
	}
}

func TestRunHandlerExecuteError(t *testing.T) {
	orig := executeFn
	defer func() { executeFn = orig }()

	var capturedLang runtime.Language
	var capturedCode string
	var capturedLimits runtime.Limits

	executeFn = func(ctx context.Context, lang runtime.Language, code string, limits runtime.Limits) (runtime.Result, error) {
		capturedLang = lang
		capturedCode = code
		capturedLimits = limits
		return runtime.Result{}, errors.New("boom")
	}

	payload := `{"language":"python","code":"print('hi')","limits":{"wallTimeMs":1000,"memoryBytes":2048,"nanoCPUs":2}}`
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(payload))
	rec := httptest.NewRecorder()

	runHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "boom" {
		t.Fatalf("expected boom error, got %q", resp.Error)
	}
	if capturedLang != runtime.LangPython {
		t.Fatalf("expected python lang, got %s", capturedLang)
	}
	if capturedCode != "print('hi')" {
		t.Fatalf("wrong code captured: %q", capturedCode)
	}
	if capturedLimits.WallTime != 1000*time.Millisecond || capturedLimits.MemoryB != 2048 || capturedLimits.NanoCPUs != 2 {
		t.Fatalf("unexpected limits: %+v", capturedLimits)
	}
}

func TestRunHandlerSuccessWithErrorMessage(t *testing.T) {
	orig := executeFn
	defer func() { executeFn = orig }()

	executeFn = func(ctx context.Context, lang runtime.Language, code string, limits runtime.Limits) (runtime.Result, error) {
		return runtime.Result{
			Stdout: "out",
			Error:  "compile_error",
			Exit:   runtime.ExitInfo{Code: 1, TimedOut: false},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(`{"language":"python","code":"print()"}`))
	rec := httptest.NewRecorder()

	runHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected json content-type, got %q", ct)
	}
	var res runtime.Result
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if res.Error != "compile_error" || res.Exit.Code != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestRunHandlerSuccessNoError(t *testing.T) {
	orig := executeFn
	defer func() { executeFn = orig }()

	executeFn = func(ctx context.Context, lang runtime.Language, code string, limits runtime.Limits) (runtime.Result, error) {
		return runtime.Result{
			Stdout: "out",
			Stderr: "",
			Exit:   runtime.ExitInfo{Code: 0, TimedOut: false},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(`{"language":"python","code":"print(1)"}`))
	rec := httptest.NewRecorder()

	runHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var res runtime.Result
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if res.Exit.Code != 0 || res.Error != "" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestRunHandlerEncodeFailure(t *testing.T) {
	orig := executeFn
	defer func() { executeFn = orig }()

	executeFn = func(ctx context.Context, lang runtime.Language, code string, limits runtime.Limits) (runtime.Result, error) {
		return runtime.Result{Exit: runtime.ExitInfo{Code: 0}}, nil
	}

	logOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(logOutput)

	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(`{"language":"python","code":"print(1)"}`))
	w := &failingWriter{header: make(http.Header)}

	runHandler(w, req)

	if w.writes == 0 {
		t.Fatalf("expected encoder to attempt writing")
	}
}

func TestMainFunction(t *testing.T) {
	origExec := executeFn
	origListen := listenAndServe
	origFatal := logFatalf
	defer func() {
		executeFn = origExec
		listenAndServe = origListen
		logFatalf = origFatal
		os.Unsetenv("SANDBOX_HTTP_ADDR")
	}()

	executeFn = func(ctx context.Context, lang runtime.Language, code string, limits runtime.Limits) (runtime.Result, error) {
		return runtime.Result{}, nil
	}

	var addrs []string
	var served http.Handler
	var fatalMessages []string

	listenAndServe = func(addr string, handler http.Handler) error {
		addrs = append(addrs, addr)
		served = handler
		return nil
	}

	logFatalf = func(format string, args ...interface{}) {
		fatalMessages = append(fatalMessages, fmt.Sprintf(format, args...))
	}

	os.Setenv("SANDBOX_HTTP_ADDR", ":9999")

	main()

	if len(addrs) == 0 || addrs[0] != ":9999" {
		t.Fatalf("expected first addr :9999, got %v", addrs)
	}
	if served == nil {
		t.Fatalf("expected handler to be registered")
	}

	// Basic sanity: handler responds using runHandler logic
	req := httptest.NewRequest(http.MethodGet, "/run", nil)
	rec := httptest.NewRecorder()
	served.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", rec.Code)
	}

	os.Unsetenv("SANDBOX_HTTP_ADDR")
	served = nil

	main()

	if len(addrs) < 2 || addrs[1] != ":8090" {
		t.Fatalf("expected default addr :8090, got %v", addrs)
	}

	listenAndServe = func(string, http.Handler) error {
		return errors.New("boom")
	}

	main()

	if len(fatalMessages) == 0 || fatalMessages[len(fatalMessages)-1] != "sandbox server failed: boom" {
		t.Fatalf("expected fatal message, got %v", fatalMessages)
	}
}

type failingWriter struct {
	header http.Header
	status int
	writes int
}

func (f *failingWriter) Header() http.Header {
	return f.header
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.writes++
	return 0, errors.New("write failure")
}

func (f *failingWriter) WriteHeader(status int) {
	f.status = status
}
