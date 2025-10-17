package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"sandbox/internal/runtime"
)

var (
	executeFn      = runtime.Execute
	listenAndServe = http.ListenAndServe
	logFatalf      = log.Fatalf
)

type runRequest struct {
	Language string        `json:"language"`
	Code     string        `json:"code"`
	Limits   *limitsConfig `json:"limits,omitempty"`
}

type limitsConfig struct {
	WallTimeMs  int64 `json:"wallTimeMs"`
	MemoryBytes int64 `json:"memoryBytes"`
	NanoCPUs    int64 `json:"nanoCPUs"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	addr := ":8090"
	if v := os.Getenv("SANDBOX_HTTP_ADDR"); v != "" {
		addr = v
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/run", runHandler)

	log.Printf("sandbox service listening on %s", addr)
	if err := listenAndServe(addr, mux); err != nil {
		logFatalf("sandbox server failed: %v", err)
	}
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(errorResponse{Error: "method_not_allowed"})
		return
	}

	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(errorResponse{Error: "invalid_request"})
		return
	}

	lang := runtime.Language(req.Language)
	limits := runtime.Limits{}
	if req.Limits != nil {
		if req.Limits.WallTimeMs > 0 {
			limits.WallTime = time.Duration(req.Limits.WallTimeMs) * time.Millisecond
		}
		limits.MemoryB = req.Limits.MemoryBytes
		limits.NanoCPUs = req.Limits.NanoCPUs
	}

	ctx := r.Context()
	result, err := executeFn(ctx, lang, req.Code, limits)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(errorResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if result.Error != "" {
		w.WriteHeader(http.StatusOK)
	}
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
