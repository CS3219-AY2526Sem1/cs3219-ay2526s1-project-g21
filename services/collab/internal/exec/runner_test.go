package exec

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"collab/internal/models"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRunOnceSuccess(t *testing.T) {
	var got sandboxRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/run" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed decoding request: %v", err)
		}
		resp := sandboxResponse{
			Stdout: "out",
			Stderr: "err",
			Exit:   runExit{Code: 0, TimedOut: false},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	output, err := runner.RunOnce(context.Background(), models.LangPython, "print('hi')", SandboxLimits{
		WallTime: 500 * time.Millisecond,
		MemoryB:  128,
		NanoCPUs: 250,
	})
	if err != nil {
		t.Fatalf("run once error: %v", err)
	}
	if output.Stdout != "out" || output.Stderr != "err" || output.Exit != 0 || output.TimedOut {
		t.Fatalf("unexpected output: %#v", output)
	}
	if got.Language != string(models.LangPython) {
		t.Fatalf("unexpected language sent: %#v", got)
	}
	if got.Limits.WallTimeMs == 0 {
		t.Fatalf("expected wall time to be set: %#v", got.Limits)
	}
}

func TestRunOnceSandboxUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(sandboxResponse{Error: "sandbox_unavailable"})
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	_, err := runner.RunOnce(context.Background(), models.LangPython, "code", SandboxLimits{})
	if !errors.Is(err, ErrDockerUnavailable) {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestRunStreamConvertsEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := sandboxResponse{
			Events: []sandboxEvent{
				{Type: "stdout", Data: json.RawMessage(`"hello"`)},
				{Type: "stderr", Data: json.RawMessage(`"oops"`)},
				{Type: "error", Data: json.RawMessage(`"bad"`)},
				{Type: "exit", Data: json.RawMessage(`{"code":123,"timedOut":true}`)},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	frames, err := runner.RunStream(context.Background(), models.LangPython, "code", SandboxLimits{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frames) != 4 {
		t.Fatalf("expected 4 frames, got %d", len(frames))
	}
	if frames[0].Type != "stdout" || frames[1].Type != "stderr" || frames[2].Type != "error" {
		t.Fatalf("unexpected frames: %#v", frames)
	}
	exitData := frames[3].Data.(map[string]any)
	codeVal, ok := exitData["code"]
	if !ok {
		t.Fatalf("missing code in exit frame: %#v", exitData)
	}
	var codeInt int
	switch v := codeVal.(type) {
	case int:
		codeInt = v
	case float64:
		codeInt = int(v)
	default:
		t.Fatalf("unexpected code type %T", v)
	}
	timedOut, ok := exitData["timedOut"].(bool)
	if !ok {
		t.Fatalf("unexpected timedOut value: %#v", exitData["timedOut"])
	}
	if codeInt != 123 || !timedOut {
		t.Fatalf("unexpected exit data: %#v", exitData)
	}
}

func TestRunStreamPropagatesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := sandboxResponse{Error: "unsupported_language"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	frames, err := runner.RunStream(context.Background(), models.LangPython, "code", SandboxLimits{})
	if err == nil || err.Error() != "unsupported language" {
		t.Fatalf("expected unsupported language error, got %v", err)
	}
	if len(frames) != 1 || frames[0].Type != "error" {
		t.Fatalf("expected appended error frame, got %#v", frames)
	}
}

func TestInvokeSandboxAppliesDefaults(t *testing.T) {
	var got sandboxRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(sandboxResponse{})
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	resp, err := runner.invokeSandbox(context.Background(), models.LangPython, "code", SandboxLimits{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Stdout != "" || resp.Stderr != "" || len(resp.Events) != 0 {
		t.Fatalf("unexpected non-empty sandbox response: %#v", resp)
	}
	if got.Limits.WallTimeMs != 10000 {
		t.Fatalf("expected fallback wall time 10000, got %d", got.Limits.WallTimeMs)
	}
	if got.Limits.MemoryBytes != 512*1024*1024 {
		t.Fatalf("expected fallback memory, got %d", got.Limits.MemoryBytes)
	}
	if got.Limits.NanoCPUs != 1_000_000_000 {
		t.Fatalf("expected fallback nano cpus, got %d", got.Limits.NanoCPUs)
	}
}

func TestInvokeSandboxHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(sandboxResponse{})
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	_, err := runner.invokeSandbox(context.Background(), models.LangPython, "code", SandboxLimits{})
	if err == nil || err.Error() != "500 Internal Server Error" {
		t.Fatalf("expected http error, got %v", err)
	}
}

func TestInvokeSandboxJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{invalid"))
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	if _, err := runner.invokeSandbox(context.Background(), models.LangPython, "code", SandboxLimits{}); err == nil {
		t.Fatalf("expected JSON decode error")
	}
}

func TestHasErrorFrame(t *testing.T) {
	if hasErrorFrame(nil) {
		t.Fatalf("expected false for nil frames")
	}
	if !hasErrorFrame([]models.WSFrame{{Type: "error"}}) {
		t.Fatalf("expected true when error frame present")
	}
}

func TestLimitsMillis(t *testing.T) {
	if got := limitsMillis(-1, time.Second); got != 1000 {
		t.Fatalf("expected fallback 1000, got %d", got)
	}
	if got := limitsMillis(1500*time.Millisecond, time.Second); got != 1500 {
		t.Fatalf("unexpected millis: %d", got)
	}
}

func TestMapSandboxError(t *testing.T) {
	if err := mapSandboxError(""); err != nil {
		t.Fatalf("expected nil for success")
	}
	if err := mapSandboxError("success"); err != nil {
		t.Fatalf("expected nil for success code")
	}
	if err := mapSandboxError("sandbox_unavailable"); !errors.Is(err, ErrDockerUnavailable) {
		t.Fatalf("expected docker error, got %v", err)
	}
	if err := mapSandboxError("unsupported_language"); err == nil || err.Error() != "unsupported language" {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mapSandboxError("other"); err == nil || err.Error() != "other" {
		t.Fatalf("expected passthrough error, got %v", err)
	}
}

func TestLangSpecPublic(t *testing.T) {
	runner := &Runner{}
	spec, _, _, _, err := runner.LangSpecPublic(models.LangPython)
	if err != nil || spec.FileName != "main.py" {
		t.Fatalf("unexpected python spec: %#v err=%v", spec, err)
	}
	spec, _, _, _, err = runner.LangSpecPublic(models.LangJava)
	if err != nil || spec.FileName != "Main.java" {
		t.Fatalf("unexpected java spec: %#v err=%v", spec, err)
	}
	spec, _, _, _, err = runner.LangSpecPublic(models.LangCPP)
	if err != nil || spec.FileName != "main.cpp" {
		t.Fatalf("unexpected cpp spec: %#v err=%v", spec, err)
	}
	if _, _, _, _, err := runner.LangSpecPublic(models.Language("unknown")); err == nil {
		t.Fatalf("expected error for unsupported language")
	}
}

func TestNewRunnerDefaults(t *testing.T) {
	t.Setenv("SANDBOX_URL", "")
	r := NewRunner()
	if r.baseURL != "http://localhost:8090" {
		t.Fatalf("expected default base url, got %s", r.baseURL)
	}

	t.Setenv("SANDBOX_URL", " http://example.com/sandbox/ ")
	r = NewRunner()
	if r.baseURL != "http://example.com/sandbox" {
		t.Fatalf("expected trimmed base url, got %s", r.baseURL)
	}
}

func TestRunOnceMapsSandboxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := sandboxResponse{Error: "unsupported_language"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	if _, err := runner.RunOnce(context.Background(), models.LangPython, "code", SandboxLimits{}); err == nil || err.Error() != "unsupported language" {
		t.Fatalf("expected unsupported language error, got %v", err)
	}
}

func TestRunStreamIgnoresUnknownEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := sandboxResponse{
			Events: []sandboxEvent{
				{Type: "unknown", Data: json.RawMessage(`"data"`)},
				{Type: "error", Data: json.RawMessage(`"oops"`)},
			},
			Error: "ignored",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	frames, err := runner.RunStream(context.Background(), models.LangPython, "code", SandboxLimits{})
	if err == nil || err.Error() != "ignored" {
		t.Fatalf("expected propagated error, got %v", err)
	}
	if len(frames) != 1 || frames[0].Type != "error" {
		t.Fatalf("expected single error frame, got %#v", frames)
	}
}

func TestRunStreamInvokeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(sandboxResponse{})
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	if _, err := runner.RunStream(context.Background(), models.LangPython, "code", SandboxLimits{}); err == nil {
		t.Fatalf("expected invoke error")
	}
}

func TestInvokeSandboxClientError(t *testing.T) {
	runner := &Runner{client: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial")
	})}, baseURL: "http://sandbox"}

	if _, err := runner.invokeSandbox(context.Background(), models.LangPython, "code", SandboxLimits{}); err == nil || !strings.Contains(err.Error(), "dial") {
		t.Fatalf("expected client error, got %v", err)
	}
}

func TestInvokeSandboxBadURL(t *testing.T) {
	runner := &Runner{client: &http.Client{}, baseURL: "://bad"}
	if _, err := runner.invokeSandbox(context.Background(), models.LangPython, "code", SandboxLimits{}); err == nil {
		t.Fatalf("expected request error")
	}
}

func TestRunStreamSkipsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := sandboxResponse{
			Events: []sandboxEvent{
				{Type: "stdout", Data: json.RawMessage(`{"not":"a string"}`)},
				{Type: "exit", Data: json.RawMessage(`"oops"`)},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	runner := &Runner{client: server.Client(), baseURL: server.URL}
	frames, err := runner.RunStream(context.Background(), models.LangPython, "code", SandboxLimits{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frames) != 0 {
		t.Fatalf("expected frames to be skipped, got %#v", frames)
	}
}
