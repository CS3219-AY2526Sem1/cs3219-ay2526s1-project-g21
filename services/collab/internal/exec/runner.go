package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"collab/internal/models"
)

type Runner struct {
	client  *http.Client
	baseURL string
}

func NewRunner() *Runner {
	base := strings.TrimSpace(os.Getenv("SANDBOX_URL"))
	if base == "" {
		base = "http://localhost:8090"
	}
	base = strings.TrimRight(base, "/")
	return &Runner{
		client:  &http.Client{},
		baseURL: base,
	}
}

type RunOutput struct {
	Stdout   string
	Stderr   string
	Exit     int
	TimedOut bool
}

type SandboxLimits struct {
	WallTime time.Duration
	MemoryB  int64
	NanoCPUs int64
}

type sandboxRequest struct {
	Language string        `json:"language"`
	Code     string        `json:"code"`
	Limits   sandboxLimits `json:"limits"`
}

type sandboxLimits struct {
	WallTimeMs  int64 `json:"wallTimeMs"`
	MemoryBytes int64 `json:"memoryBytes"`
	NanoCPUs    int64 `json:"nanoCPUs"`
}

type sandboxResponse struct {
	Stdout string         `json:"stdout"`
	Stderr string         `json:"stderr"`
	Exit   runExit        `json:"exit"`
	Events []sandboxEvent `json:"events"`
	Error  string         `json:"error,omitempty"`
}

type runExit struct {
	Code     int  `json:"code"`
	TimedOut bool `json:"timedOut"`
}

type sandboxEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

var ErrDockerUnavailable = errors.New("docker daemon unreachable")

func (r *Runner) RunOnce(ctx context.Context, lang models.Language, code string, limits SandboxLimits) (RunOutput, error) {
	resp, err := r.invokeSandbox(ctx, lang, code, limits)
	if err != nil {
		return RunOutput{}, err
	}
	if mapped := mapSandboxError(resp.Error); mapped != nil {
		return RunOutput{}, mapped
	}
	return RunOutput{
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
		Exit:     resp.Exit.Code,
		TimedOut: resp.Exit.TimedOut,
	}, nil
}

func (r *Runner) RunStream(ctx context.Context, lang models.Language, code string, limits SandboxLimits) ([]models.WSFrame, error) {
	resp, err := r.invokeSandbox(ctx, lang, code, limits)
	if err != nil {
		return nil, err
	}

	frames := make([]models.WSFrame, 0, len(resp.Events))
	for _, evt := range resp.Events {
		switch evt.Type {
		case "stdout", "stderr", "error":
			var msg string
			if err := json.Unmarshal(evt.Data, &msg); err != nil {
				continue
			}
			frames = append(frames, models.WSFrame{Type: evt.Type, Data: msg})
		case "exit":
			var exitData runExit
			if err := json.Unmarshal(evt.Data, &exitData); err != nil {
				continue
			}
			frames = append(frames, models.WSFrame{
				Type: "exit",
				Data: map[string]any{"code": exitData.Code, "timedOut": exitData.TimedOut},
			})
		}
	}
	if resp.Error != "" && !hasErrorFrame(frames) {
		frames = append(frames, models.WSFrame{Type: "error", Data: resp.Error})
	}
	return frames, mapSandboxError(resp.Error)
}

func hasErrorFrame(frames []models.WSFrame) bool {
	for _, f := range frames {
		if f.Type == "error" {
			return true
		}
	}
	return false
}

func (r *Runner) invokeSandbox(ctx context.Context, lang models.Language, code string, limits SandboxLimits) (sandboxResponse, error) {
	reqPayload := sandboxRequest{
		Language: string(lang),
		Code:     code,
		Limits: sandboxLimits{
			WallTimeMs:  limitsMillis(limits.WallTime, 10*time.Second),
			MemoryBytes: limits.MemoryB,
			NanoCPUs:    limits.NanoCPUs,
		},
	}
	if reqPayload.Limits.MemoryBytes == 0 {
		reqPayload.Limits.MemoryBytes = 512 * 1024 * 1024
	}
	if reqPayload.Limits.NanoCPUs == 0 {
		reqPayload.Limits.NanoCPUs = 1_000_000_000
	}

	body, _ := json.Marshal(reqPayload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/run", bytes.NewReader(body))
	if err != nil {
		return sandboxResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return sandboxResponse{}, err
	}
	defer resp.Body.Close()

	var sr sandboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return sandboxResponse{}, err
	}
	if resp.StatusCode >= 400 {
		if sr.Error == "" {
			sr.Error = resp.Status
		}
		return sandboxResponse{}, mapSandboxError(sr.Error)
	}
	return sr, nil
}

func limitsMillis(d time.Duration, fallback time.Duration) int64 {
	if d <= 0 {
		d = fallback
	}
	return int64(d / time.Millisecond)
}

func mapSandboxError(code string) error {
	switch {
	case code == "" || code == "success":
		return nil
	case code == "sandbox_unavailable":
		return ErrDockerUnavailable
	case code == "unsupported_language":
		return errors.New("unsupported language")
	}
	return errors.New(code)
}

func (r *Runner) LangSpecPublic(lang models.Language) (spec models.LanguageSpec, image string, fileName string, cmds [][]string, err error) {
	return r.langSpec(lang)
}

func (r *Runner) langSpec(lang models.Language) (models.LanguageSpec, string, string, [][]string, error) {
	switch lang {
	case models.LangPython:
		return models.LanguageSpec{
				Name:            lang,
				FileName:        "main.py",
				RunCmd:          []string{"python3", "main.py"},
				DefaultTabSize:  4,
				Formatter:       []string{"black"},
				ExampleTemplate: "print(\"Hello from Python!\")\n",
			},
			"python:3.11-slim",
			"main.py",
			[][]string{{"python3", "main.py"}},
			nil

	case models.LangJava:
		return models.LanguageSpec{
				Name:            lang,
				FileName:        "Main.java",
				CompileCmd:      []string{"javac", "Main.java"},
				ExecCmd:         []string{"/bin/sh", "-c", "java Main"},
				DefaultTabSize:  4,
				Formatter:       []string{"google-java-format"},
				ExampleTemplate: "public class Main {\n    public static void main(String[] args) {\n        System.out.println(\"Hello from Java!\");\n    }\n}\n",
			},
			"eclipse-temurin:17-jdk",
			"Main.java",
			[][]string{{"javac", "Main.java"}, {"/bin/sh", "-c", "java Main"}},
			nil

	case models.LangCPP:
		return models.LanguageSpec{
				Name:            lang,
				FileName:        "main.cpp",
				CompileCmd:      []string{"g++", "-O2", "-std=c++17", "main.cpp", "-o", "main"},
				ExecCmd:         []string{"./main"},
				DefaultTabSize:  2,
				Formatter:       []string{"clang-format"},
				ExampleTemplate: "#include <iostream>\n\nint main() {\n    std::cout << \"Hello from C++!\" << std::endl;\n    return 0;\n}\n",
			},
			"gcc:13",
			"main.cpp",
			[][]string{{"g++", "-O2", "-std=c++17", "main.cpp", "-o", "main"}, {"./main"}},
			nil
	default:
		return models.LanguageSpec{}, "", "", nil, errors.New("unsupported language")
	}
}
