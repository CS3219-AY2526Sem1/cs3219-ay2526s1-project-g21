# AI Service

## Overview

The AI Service is a microservice that provides intelligent code related capabilities forPeerPrep. It uses LLMs to generate contextual explanations of code snippets in various programming languages and difficulty levels. (for now). More modes to be added later

## Features

- **Code Explanation**: Generate detailed explanations of code snippets with different levels of detail (beginner, intermediate, advanced)
- **Multi-Language Support**: Currently supports Python, Java, C++, and JavaScript
- **Provider Abstraction**: Pluggable LLM provider system (currently implemented with Google Gemini)
- **Template-Based Prompts**: YAML-based prompt templates for consistent and maintainable AI interactions
- **Request Validation**: Comprehensive input validation and error handling
- **Health Monitoring**: Built-in health check endpoints

## Directory Overview

```
ai/
├── cmd/
│   └── server/
│       └── main.go                  # Application entry point and server setup
├── internal/
│   ├── config/
│   │   └── config.go               # Configuration management and validation
│   ├── handlers/
│   │   ├── ai_handler.go           # AI explanation endpoint handlers
│   │   └── health_handler.go       # Health check handlers
│   ├── llm/
│   │   ├── provider.go             # LLM provider interface and error definitions
│   │   ├── registry.go             # Provider registry and factory pattern
│   │   └── gemini/
│   │       ├── client.go           # Gemini API client implementation
│   │       ├── config.go           # Gemini-specific configuration
│   │       └── init.go             # Provider registration
│   ├── middleware/
│   │   └── validation.go           # Generic request validation middleware
│   ├── models/
│   │   ├── request.go              # Request structures and validation
│   │   └── response.go             # Response structures and metadata
│   ├── prompts/
│   │   ├── manager.go              # Prompt template management
│   │   └── templates/
│   │       └── explain.yaml        # Code explanation prompt templates
│   ├── routers/
│   │   ├── ai_routes.go            # AI service route definitions
│   │   └── health_routes.go        # Health check route definitions
│   └── utils/
│       ├── logging.go              # Logging utilities
│       └── response.go             # HTTP response utilities
├── go.mod                          # Go module dependencies
├── go.sum                          # Dependency checksums
└── Dockerfile                      # Container configuration
```

## Architecture

### 1. Application Bootstrap (`cmd/server/main.go`)

**Responsibility**: Service initialization, dependency injection, and lifecycle management

**Key Components**:

```go
func main() {
    // 1. Logger initialization (Zap production logger)
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // 2. Configuration loading and validation
    cfg, err := config.LoadConfig()

    // 3. Prompt manager initialization
    promptManager, err := prompts.NewPromptManager()

    // 4. AI provider initialization (factory pattern)
    aiProvider, err := llm.NewProvider(cfg.Provider)

    // 5. Handler creation with dependency injection
    aiHandler := handlers.NewAIHandler(aiProvider, promptManager, logger)
    healthHandler := handlers.NewHealthHandler()

    // 6. Router setup with middleware
    router := chi.NewRouter()
    router.Use(cors.Handler(...), middleware.RequestID, ...)

    // 7. Graceful shutdown implementation
    server := &http.Server{...}
    go server.ListenAndServe()
    <-shutdownChan
    server.Shutdown(ctx)
}
```

**Architecture Patterns**:

- **Dependency Injection**: All dependencies passed explicitly to constructors
- **Factory Pattern**: Provider creation through registry
- **Graceful Shutdown**: Signal handling with 30-second timeout
- **Resource Management**: Proper cleanup with defer statements

### 2. Configuration System (`internal/config/`)

**File**: `config.go`

**Structure**:

```go
type Config struct {
    Provider string // AI provider name (currently "gemini")
}

func LoadConfig() (*Config, error) {
    config := &Config{
        Provider: getEnvOrDefault("AI_PROVIDER", "gemini"),
    }
    return config, validateConfig(config)
}
```

**Design Patterns**:

- **Environment-First Configuration**: All config from environment variables
- **Validation at Load Time**: Immediate validation with descriptive errors
- **Default Values**: Sensible defaults for optional configurations
- **Provider Extensibility**: Easy addition of new providers

### 3. LLM Provider System (`internal/llm/`)

#### Provider Interface (`provider.go`)

```go
type Provider interface {
    GenerateExplanation(ctx context.Context, prompt string, requestID string, detailLevel string) (*models.ExplainResponse, error)
    GetProviderName() string
}

type Closer interface {
    Close() error
}

type ProviderError struct {
    Provider string
    Code     string  // Standardized error codes
    Message  string
    Err      error
}
```

**Design Principles**:

- **Interface Segregation**: Minimal interface for core functionality
- **Optional Interfaces**: `Closer` for providers needing cleanup
- **Standardized Errors**: Common error codes across providers
- **Context Propagation**: Timeout and cancellation support

#### Provider Registry (`registry.go`)

```go
type ProviderFactory func() (Provider, error)

var providers = make(map[string]ProviderFactory)

func RegisterProvider(name string, factory ProviderFactory) {
    providers[name] = factory
}

func NewProvider(name string) (Provider, error) {
    factory, exists := providers[name]
    if !exists {
        return nil, fmt.Errorf("unsupported provider: %s", name)
    }
    return factory()
}
```

**Architecture Benefits**:

- **Plugin Architecture**: Easy addition of new providers
- **Lazy Initialization**: Providers created only when needed
- **Type Safety**: Compile-time interface verification
- **Global Registry**: Single source of truth for available providers

#### Gemini Implementation (`internal/llm/gemini/`)

**Client Structure** (`client.go`):

```go
type Client struct {
    client *genai.Client  // Google GenAI client
    config *Config        // Provider-specific configuration
}

func (c *Client) GenerateExplanation(ctx context.Context, prompt string, requestID string, detailLevel string) (*models.ExplainResponse, error) {
    startTime := time.Now()

    // API call with context
    result, err := c.client.Models.GenerateContent(ctx, c.config.Model, genai.Text(prompt), nil)

    // Error handling with provider-specific errors
    if err != nil {
        return nil, &llm.ProviderError{
            Provider: "gemini",
            Code:     llm.ErrCodeServiceDown,
            Message:  "Failed to generate explanation",
            Err:      err,
        }
    }

    // Response processing and metadata collection
    explanation, err := result.Text()
    processingTime := time.Since(startTime).Milliseconds()

    return &models.ExplainResponse{
        Explanation: explanation,
        RequestID:   requestID,
        Metadata: models.ExplanationMetadata{
            ProcessingTime: int(processingTime),
            DetailLevel:    detailLevel,
            Provider:       "gemini",
            Model:          c.config.Model,
        },
    }, nil
}
```

**Provider Registration** (`init.go`):

```go
func init() {
    llm.RegisterProvider("gemini", func() (llm.Provider, error) {
        config, err := NewConfig()
        if err != nil {
            return nil, err
        }
        return NewClient(config)
    })
}
```

**Configuration** (`config.go`):

```go
type Config struct {
    APIKey string
    Model  string
}

func NewConfig() (*Config, error) {
    apiKey := os.Getenv("GEMINI_API_KEY")
    if apiKey == "" {
        return nil, errors.New("GEMINI_API_KEY environment variable is required")
    }

    model := os.Getenv("GEMINI_MODEL")
    if model == "" {
        model = "gemini-2.5-flash" // Default model
    }

    return &Config{APIKey: apiKey, Model: model}, nil
}
```

### 4. Prompt Management System (`internal/prompts/`)

**Manager Structure** (`manager.go`):

```go
type PromptManager struct {
    prompts map[string]map[string]string // mode -> detailLevel -> complete prompt
}

type PromptTemplate struct {
    BasePrompt   string            `yaml:"base_prompt"`
    DetailLevels map[string]string `yaml:"detail_levels"`
}

//go:embed templates/*.yaml
var templateFS embed.FS
```

**Template Loading Process**:

```go
func (pm *PromptManager) loadPrompts() error {
    entries, err := templateFS.ReadDir("templates")

    for _, entry := range entries {
        if !strings.HasSuffix(entry.Name(), ".yaml") {
            continue
        }

        // Read and parse YAML
        data, err := templateFS.ReadFile("templates/" + entry.Name())
        var promptTemplate PromptTemplate
        yaml.Unmarshal(data, &promptTemplate)

        // Combine base prompt with detail levels
        name := strings.TrimSuffix(entry.Name(), ".yaml")
        pm.prompts[name] = make(map[string]string)

        for detailLevel, detailPrompt := range promptTemplate.DetailLevels {
            var fullPrompt strings.Builder
            if promptTemplate.BasePrompt != "" {
                fullPrompt.WriteString(promptTemplate.BasePrompt)
                fullPrompt.WriteString("\n\n")
            }
            fullPrompt.WriteString(detailPrompt)
            pm.prompts[name][detailLevel] = fullPrompt.String()
        }
    }
    return nil
}
```

**Prompt Building**:

```go
func (pm *PromptManager) BuildPrompt(mode, code, language, detailLevel string) (string, error) {
    promptTemplate := pm.prompts[mode][detailLevel]

    // Simple string replacement (no complex templating)
    result := strings.ReplaceAll(promptTemplate, "{{.Language}}", language)
    result = strings.ReplaceAll(result, "{{.Code}}", code)

    return result, nil
}
```

**Template Structure** (`templates/explain.yaml`):

````yaml
base_prompt: |
  You are a helpful programming tutor. Your job is to explain code snippets clearly and accurately.

  Guidelines:
  - Focus on explaining what the code does, not how to improve it
  - Use clear, accessible language appropriate for the detail level
  - Be concise but thorough

detail_levels:
  beginner: |
    Explain this {{.Language}} code in simple terms...

    Code to explain:
    ```{{.Language}}
    {{.Code}}
    ```

  intermediate: |
    Explain this {{.Language}} code for someone with some programming experience...

  advanced: |
    Provide a detailed technical explanation of this {{.Language}} code...
````

**Architecture Benefits**:

- **Compile-Time Embedding**: Templates embedded in binary
- **Hot-Swappable**: Easy template updates without code changes
- **Structured Prompts**: Consistent prompt structure across modes
- **Template Inheritance**: Base prompts with level-specific additions

### 5. Request/Response Pipeline

#### Request Models (`internal/models/request.go`)

```go
type ExplainRequest struct {
    Code        string `json:"code"`
    Language    string `json:"language"`
    DetailLevel string `json:"detail_level"`
    RequestID   string `json:"request_id"`
}

func (r *ExplainRequest) Validate() error {
    // Required field validation
    if r.Code == "" {
        return &ErrorResponse{Code: "missing_code", Message: "Code field is required"}
    }

    // Language validation
    supportedLanguages := map[string]bool{
        "python": true, "java": true, "cpp": true, "javascript": true,
    }
    if !supportedLanguages[r.Language] {
        return &ErrorResponse{
            Code: "unsupported_language",
            Message: "Language not supported. Supported languages: python, java, cpp, javascript",
        }
    }

    // Detail level validation with default
    if r.DetailLevel == "" {
        r.DetailLevel = "intermediate"
    }
    validDetailLevels := map[string]bool{
        "beginner": true, "intermediate": true, "advanced": true,
    }
    if !validDetailLevels[r.DetailLevel] {
        return &ErrorResponse{
            Code: "invalid_detail_level",
            Message: "Detail level must be one of: beginner, intermediate, advanced",
        }
    }

    return nil
}
```

#### Response Models (`internal/models/response.go`)

```go
type ExplainResponse struct {
    Explanation string              `json:"explanation"`
    RequestID   string              `json:"request_id"`
    Metadata    ExplanationMetadata `json:"metadata"`
}

type ExplanationMetadata struct {
    ProcessingTime int    `json:"processing_time_ms"`
    DetailLevel    string `json:"detail_level"`
    Provider       string `json:"provider,omitempty"`
    Model          string `json:"model,omitempty"`
}

type ErrorResponse struct {
    Code    string                  `json:"code"`
    Message string                  `json:"message"`
    Details []ValidationErrorDetail `json:"details,omitempty"`
}

func (e *ErrorResponse) Error() string {
    return e.Message
}
```

#### Validation Middleware (`internal/middleware/validation.go`)

**Generic Validation with Go Generics**:

```go
type Validator interface {
    Validate() error
}

func ValidateRequest[T Validator]() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Create instance using reflection
            var req T
            reqType := reflect.TypeOf(req)
            if reqType.Kind() == reflect.Ptr {
                req = reflect.New(reqType.Elem()).Interface().(T)
            } else {
                req = reflect.New(reqType).Interface().(T)
            }

            // JSON deserialization
            if err := json.NewDecoder(r.Body).Decode(req); err != nil {
                utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
                    Code: "invalid_json",
                    Message: "Invalid JSON in request body",
                })
                return
            }

            // Model validation
            if err := req.Validate(); err != nil {
                if errResp, ok := err.(*models.ErrorResponse); ok {
                    utils.JSON(w, http.StatusBadRequest, *errResp)
                } else {
                    utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
                        Code: "validation_error",
                        Message: err.Error(),
                    })
                }
                return
            }

            // Store in context for handler
            ctx := context.WithValue(r.Context(), "validated_request", req)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

**Benefits**:

- **Type Safety**: Compile-time type checking with generics
- **Reusability**: Works with any struct implementing Validator
- **Context Propagation**: Validated request available in handlers
- **Error Standardization**: Consistent error response format

### 6. HTTP Handlers (`internal/handlers/`)

#### AI Handler (`ai_handler.go`)

```go
type AIHandler struct {
    provider      llm.Provider
    promptManager *prompts.PromptManager
    logger        *zap.Logger
}

func (h *AIHandler) ExplainHandler(w http.ResponseWriter, r *http.Request) {
    // Extract validated request from middleware
    req := r.Context().Value("validated_request").(*models.ExplainRequest)

    // Generate request ID if not provided
    if req.RequestID == "" {
        req.RequestID = generateRequestID()
    }

    // Build prompt using template manager
    prompt, err := h.promptManager.BuildPrompt("explain", req.Code, req.Language, req.DetailLevel)
    if err != nil {
        h.logger.Error("Failed to build prompt", zap.Error(err), zap.String("request_id", req.RequestID))
        utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
            Code: "prompt_error",
            Message: "Failed to build AI prompt",
        })
        return
    }

    // Call AI provider
    response, err := h.provider.GenerateExplanation(r.Context(), prompt, req.RequestID, req.DetailLevel)
    if err != nil {
        h.logger.Error("AI provider error", zap.Error(err), zap.String("request_id", req.RequestID))
        utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
            Code: "ai_error",
            Message: "Failed to generate explanation",
        })
        return
    }

    // Success logging and response
    h.logger.Info("Explanation generated successfully",
        zap.String("request_id", req.RequestID),
        zap.String("provider", h.provider.GetProviderName()),
        zap.Int("processing_time_ms", response.Metadata.ProcessingTime))

    utils.JSON(w, http.StatusOK, response)
}
```

#### Health Handler (`health_handler.go`)

```go
type HealthHandler struct{}

func (h *HealthHandler) HealthzHandler(w http.ResponseWriter, r *http.Request) {
    utils.JSON(w, http.StatusOK, map[string]string{
        "status":  "ok",
        "service": "ai",
        "version": "1.0.0",
    })
}

func (h *HealthHandler) ReadyzHandler(w http.ResponseWriter, r *http.Request) {
    utils.JSON(w, http.StatusOK, map[string]string{
        "status":  "ready",
        "service": "ai",
    })
}
```

### 7. Routing System (`internal/routers/`)

#### AI Routes (`ai_routes.go`)

```go
func AIRoutes(router *chi.Mux, aiHandler *handlers.AIHandler) {
    router.Route("/ai", func(r chi.Router) {
        r.With(middleware.ValidateRequest[*models.ExplainRequest]()).Post("/explain", aiHandler.ExplainHandler)
        // Future routes prepared
        // r.Post("/hint", aiHandler.HintHandler)
    })
}
```

#### Health Routes (`health_routes.go`)

```go
func HealthRoutes(router *chi.Mux, healthHandler *handlers.HealthHandler) {
    router.Get("/healthz", healthHandler.HealthzHandler)
    router.Get("/readyz", healthHandler.ReadyzHandler)
}
```

### 8. Utility Layer (`internal/utils/`)

#### Response Utilities (`response.go`)

```go
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(data)
}

func JSONError(w http.ResponseWriter, statusCode int, code, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "code":    code,
        "message": message,
    })
}
```

#### Logging Utilities (`logging.go`)

```go
var Logger *zap.Logger

func InitLogger() {
    var err error
    Logger, err = zap.NewProduction()
    if err != nil {
        panic("Failed to initialize logger: " + err.Error())
    }
}

func GetLogger() *zap.Logger {
    if Logger == nil {
        InitLogger()
    }
    return Logger
}
```

## Error Handling Architecture

### 1. Error Types and Hierarchy

```go
// Base error interface
type error interface {
    Error() string
}

// Provider-specific errors (provider.go:19-32)
type ProviderError struct {
    Provider string
    Code     string  // Standardized codes
    Message  string
    Err      error   // Wrapped error
}

// Application errors (response.go:18-28)
type ErrorResponse struct {
    Code    string                  `json:"code"`
    Message string                  `json:"message"`
    Details []ValidationErrorDetail `json:"details,omitempty"`
}
```

### 2. Error Propagation Chain

```
Gemini API Error → ProviderError → HTTP Error Response
Validation Error → ErrorResponse → HTTP Error Response
Template Error → Generic Error → HTTP Error Response
```

### 3. Error Handling Patterns

```go
// Provider Error Creation (client.go:28-34)
return nil, &llm.ProviderError{
    Provider: "gemini",
    Code:     llm.ErrCodeAPIKey,
    Message:  "Failed to create Gemini client",
    Err:      err,
}

// Handler Error Processing (ai_handler.go:50-57)
if err != nil {
    h.logger.Error("AI provider error", zap.Error(err), zap.String("request_id", req.RequestID))
    utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
        Code:    "ai_error",
        Message: "Failed to generate explanation",
    })
    return
}

// Middleware Error Handling (validation.go:51-60)
if errResp, ok := err.(*models.ErrorResponse); ok {
    utils.JSON(w, http.StatusBadRequest, *errResp)
} else {
    utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
        Code:    "validation_error",
        Message: err.Error(),
    })
}
```

## Dependency Management

### Dependency Injection Pattern

```go
// Constructor Pattern (main.go:61)
aiHandler := handlers.NewAIHandler(aiProvider, promptManager, logger)

// Handler Constructor (ai_handler.go:20-26)
func NewAIHandler(provider llm.Provider, promptManager *prompts.PromptManager, logger *zap.Logger) *AIHandler {
    return &AIHandler{
        provider:      provider,
        promptManager: promptManager,
        logger:        logger,
    }
}
```

### Interface Dependencies

```go
// Handler depends on interfaces, not concrete types
type AIHandler struct {
    provider      llm.Provider           // Interface
    promptManager *prompts.PromptManager // Concrete (template system)
    logger        *zap.Logger           // Concrete (logging)
}
```

## Configuration

### Environment Variables

| Variable         | Description           | Default            | Required         |
| ---------------- | --------------------- | ------------------ | ---------------- |
| `AI_PROVIDER`    | LLM provider name     | `gemini`           | No               |
| `GEMINI_API_KEY` | Google Gemini API key | -                  | Yes (for Gemini) |
| `GEMINI_MODEL`   | Gemini model version  | `gemini-2.5-flash` | No               |

### Supported Languages

- Python (`python`)
- Java (`java`)
- C++ (`cpp`)
- JavaScript (`javascript`)

## API Endpoints

### POST /ai/explain

Generates code explanations based on provided code snippets.

**Request Body:**

```json
{
  "code": "def fibonacci(n):\n    if n <= 1:\n        return n\n    return fibonacci(n-1) + fibonacci(n-2)",
  "language": "python",
  "detail_level": "intermediate",
  "request_id": "optional-request-id"
}
```

**Response:**

```json
{
  "explanation": "This Python function implements the Fibonacci sequence using recursion...",
  "request_id": "uuid-generated-or-provided",
  "metadata": {
    "processing_time_ms": 1250,
    "detail_level": "intermediate",
    "provider": "gemini",
    "model": "gemini-2.5-flash"
  }
}
```

### GET /healthz

Basic health check endpoint.

**Response:**

```json
{
  "status": "ok",
  "service": "ai",
  "version": "1.0.0"
}
```

### GET /readyz

Readiness check endpoint.

**Response:**

```json
{
  "status": "ready",
  "service": "ai"
}
```

## Error Handling

### Error Response Format

```json
{
  "code": "error_code",
  "message": "Human-readable error message",
  "details": [
    {
      "field": "field_name",
      "reason": "validation_error_reason"
    }
  ]
}
```

### Common Error Codes

- `missing_code`: Code field is required
- `missing_language`: Language field is required
- `unsupported_language`: Language not supported
- `invalid_detail_level`: Invalid detail level specified
- `invalid_json`: Malformed JSON request
- `prompt_error`: Prompt template processing failed
- `ai_error`: LLM provider error
- `invalid_api_key`: Authentication failed
- `rate_limit_exceeded`: API rate limit reached
- `service_unavailable`: External service unavailable

## Extending the Service

### Adding New LLM Providers

1. **Create Provider Package**: `internal/llm/newprovider/`
2. **Implement Provider Interface**:

   ```go
   type NewProvider struct {
       client *SomeAPIClient
       config *Config
   }

   func (p *NewProvider) GenerateExplanation(ctx context.Context, prompt string, requestID string, detailLevel string) (*models.ExplainResponse, error) {
       // Implementation
   }

   func (p *NewProvider) GetProviderName() string {
       return "newprovider"
   }
   ```

3. **Register Provider**:
   ```go
   func init() {
       llm.RegisterProvider("newprovider", func() (llm.Provider, error) {
           return NewNewProvider()
       })
   }
   ```
4. **Update Configuration**: Add provider validation in `config/config.go`

### Adding New Prompt Templates

- TO BE UPDATED

### Adding New Supported Languages

1. **Update Request Validation**: Modify `supportedLanguages` map in `models/request.go`
2. **Update Prompt Templates**: Ensure templates handle the new language
3. **Test Integration**: Verify LLM provider supports the language
