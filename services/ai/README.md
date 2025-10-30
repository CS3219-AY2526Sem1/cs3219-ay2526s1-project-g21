# AI Service

## Overview

The AI Service is a microservice that provides intelligent code related capabilities forPeerPrep. It uses LLMs to generate contextual explanations of code snippets in various programming languages and difficulty levels. (for now). More modes to be added later

## Features

- **Code Explanation**: Generate detailed explanations of code snippets with different levels of detail (beginner, intermediate, advanced)
- **Code Hints**: Provide gentle nudges and hints for coding problems with context awareness
- **Multi-Language Support**: Currently supports Python, Java, C++, and JavaScript
- **Provider Abstraction**: Pluggable LLM provider system (currently implemented with Google Gemini)
- **Template-Based Prompts**: YAML-based prompt templates with Go templating for dynamic and maintainable AI interactions
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
│   │       ├── explain.yaml        # Code explanation prompt templates
│   │       └── hint.yaml           # Code hint prompt templates
│   ├── routers/
│   │   ├── ai_routes.go            # AI service route definitions
│   │   └── health_routes.go        # Health check route definitions
│   └── utils/
│       └── response.go             # HTTP response utilities
├── go.mod                          # Go module dependencies
├── go.sum                          # Dependency checksums
└── Dockerfile                      # Container configuration
```

## Architecture

### 1. Application Bootstrap (`cmd/server/main.go`)

**Responsibilities**:

- Service initialization and dependency wiring
- Configuration loading and validation
- HTTP server lifecycle management
- Graceful shutdown coordination

**Initialization Flow**:

1. **Logger Setup**: Production-grade structured logging with Zap
2. **Configuration Loading**: Environment-based configuration with validation
3. **Prompt Manager**: Template loading and compilation from embedded files
4. **AI Provider**: Factory-based provider instantiation (currently Gemini)
5. **Handler Creation**: Dependency injection of all required components
6. **Middleware Stack**: CORS, request ID, logging, recovery, timeout middleware
7. **Server Startup**: HTTP server with configured timeouts
8. **Signal Handling**: Graceful shutdown on SIGINT/SIGTERM with 30-second timeout

**Architecture Patterns**:

- **Dependency Injection**: All dependencies passed explicitly to constructors
- **Factory Pattern**: Provider creation through registry for extensibility
- **Graceful Shutdown**: Coordinated service shutdown preserving in-flight requests
- **Configuration as Code**: Environment-driven configuration with sensible defaults

### 2. Configuration System (`internal/config/`)

**Responsibilities**:

- Environment variable loading and parsing
- Configuration validation and error reporting
- Default value management
- Provider-specific configuration delegation

**Configuration Structure**:

The configuration system follows a simple, environment-driven approach with minimal structure. The main service configuration focuses on AI provider selection, while provider-specific configuration is handled by dedicated modules.

**Design Patterns**:

- **Environment-First Configuration**: All config sourced from environment variables
- **Fail-Fast Validation**: Configuration errors detected at startup, not runtime
- **Sensible Defaults**: Optional configurations have production-ready defaults
- **Provider Delegation**: Provider-specific config handled by provider modules
- **Explicit Dependencies**: No hidden configuration dependencies between components

### 3. LLM Provider System (`internal/llm/`)

**Responsibilities**:

- Abstract interface for different LLM providers
- Provider registration and discovery
- Standardized error handling across providers
- Request/response normalization

#### Provider Interface (`provider.go`)

The provider interface defines a minimal contract for LLM integration, focusing on core functionality while maintaining extensibility. The interface supports context propagation for timeout and cancellation, and uses standardized error codes for consistent error handling across different providers.

**Key Components**:

- **Provider Interface**: Core methods for explanation generation and identification
- **ProviderError**: Structured error type with standardized codes and error wrapping
- **Error Constants**: Predefined error codes for common failure scenarios

**Design Principles**:

- **Interface Segregation**: Minimal interface focused on essential functionality
- **Standardized Errors**: Common error codes and structures across all providers
- **Context Propagation**: Full support for request timeouts and cancellation
- **Error Transparency**: Detailed error information while maintaining abstraction

#### Provider Registry (`registry.go`)

The registry implements a factory pattern for provider instantiation, enabling a plugin-like architecture where new providers can be added without modifying existing code. Providers register themselves during package initialization.

**Architecture Benefits**:

- **Plugin Architecture**: New providers added via simple registration
- **Lazy Initialization**: Providers instantiated only when needed
- **Type Safety**: Interface compliance verified at compile time
- **Decoupled Registration**: Providers self-register without central coordination

#### Gemini Implementation (`internal/llm/gemini/`)

**Responsibilities**:

- Google Gemini API integration
- Request/response marshaling
- Error translation to standard codes
- Performance metrics collection

**Implementation Structure**:

The Gemini implementation follows the standard provider pattern with three main components:

- **Client (`client.go`)**: Core API interaction with the Google GenAI SDK
- **Configuration (`config.go`)**: Environment-based API key and model configuration
- **Registration (`init.go`)**: Self-registration with the provider registry

**Key Features**:

- **API Integration**: Direct integration with Google's GenAI SDK
- **Error Mapping**: Translation of Gemini-specific errors to standard provider errors
- **Performance Tracking**: Request timing and metadata collection
- **Model Flexibility**: Configurable model selection via environment variables
- **Context Support**: Full support for request cancellation and timeouts

**Configuration Requirements**:

- `GEMINI_API_KEY`: Required API key for Google Gemini access
- `GEMINI_MODEL`: Optional model specification (defaults to `gemini-2.5-flash`)

The implementation emphasizes reliability with comprehensive error handling and provides detailed metadata for monitoring and debugging purposes.

### 4. Prompt Management System (`internal/prompts/`)

**Responsibilities**:

- Template loading and compilation at startup
- Dynamic prompt building with Go's text/template engine
- Support for multiple AI modes with flexible data injection

**Manager Structure** (`manager.go`):

The PromptManager uses a two-level mapping structure (mode -> variant -> compiled template) to organize templates efficiently. Templates are loaded from embedded YAML files and compiled with Go's text/template engine for dynamic data substitution.

**Template Structure**:

All prompt templates follow a unified YAML structure with a base prompt for consistent behavior and mode-specific variants for different use cases:

```yaml
# templates/explain.yaml
base_prompt: |
  You are a helpful programming tutor. Guidelines for explanations...

prompts:
  beginner: |
    Explain this {{.Language}} code in simple terms...
    Code: {{.Code}}
  intermediate: |
    Explain this {{.Language}} code with technical details...
  advanced: |
    Provide deep technical analysis...

# templates/hint.yaml
base_prompt: |
  You are a helpful programming tutor. Guidelines for hints...

prompts:
  default: |
    {{if .PreviousHints}}Previous hints: {{range .PreviousHints}}{{.}}{{end}}{{end}}
    Provide a hint for {{.Language}} code: {{.Code}}
    {{if .ProblemDescription}}Context: {{.ProblemDescription}}{{end}}
```

**Dynamic Data Support**:

The prompt system supports flexible data injection using Go templates. The `BuildPrompt(mode, variant, data)` method accepts any data structure, enabling:

- **Code Context**: Language, code snippets, problem descriptions
- **Session History**: Previous hints, conversation context
- **User Preferences**: Detail levels, specific formatting needs
- **Problem Metadata**: Difficulty, tags, expected solutions

**Architecture Benefits**:

- **Compile-Time Embedding**: Templates embedded in binary for distribution
- **Template Compilation**: Go's text/template provides safe, powerful templating
- **Mode Extensibility**: Easy addition of new AI modes (test, refactor, summary)
- **Context Awareness**: Templates can access rich contextual data
- **Type Safety**: Template compilation catches syntax errors at startup

### 5. Request/Response Pipeline

#### Request Models (`internal/models/request.go`)

**Responsibilities**:

- Request structure definition and JSON binding
- Input validation with detailed error reporting
- Default value assignment for optional fields
- Support for multiple AI modes

**Request Structure**:

Request models implement a common validation interface that enables generic middleware processing. Each request type encapsulates its validation logic and error reporting.

**Validation Features**:

- **Required Field Validation**: Ensures essential fields are present and non-empty
- **Language Support**: Validates against supported programming languages (Python, Java, C++, JavaScript)
- **Detail Level Handling**: Validates explanation detail levels with intelligent defaults
- **Error Standardization**: Consistent error codes and messages across all validation failures
- **Extensible Design**: Easy addition of new request types and validation rules

**Current Request Types**:

- **ExplainRequest**: Code explanation with language and detail level specification
- **Future**: HintRequest, TestRequest, RefactorRequest for additional AI modes

#### Response Models (`internal/models/response.go`)

**Responsibilities**:

- Response structure definition and JSON serialization
- Metadata collection for monitoring and debugging
- Error response standardization
- Support for future response types

**Response Structure**:

Response models provide a consistent API interface with rich metadata for client applications and monitoring systems.

**Key Components**:

- **ExplainResponse**: Contains generated explanation with comprehensive metadata
- **ExplanationMetadata**: Processing time, provider info, and request context
- **ErrorResponse**: Standardized error format with codes and detailed messages
- **Future Types**: HintResponse, TestResponse for additional AI modes

**Metadata Features**:

- **Performance Tracking**: Processing time measurement for SLA monitoring
- **Provider Information**: Which LLM provider and model generated the response
- **Request Context**: Detail level and other request-specific information
- **Debugging Support**: Request IDs for distributed tracing and log correlation

#### Validation Middleware (`internal/middleware/validation.go`)

**Responsibilities**:

- Generic request validation using Go generics
- JSON deserialization with error handling
- Request context propagation to handlers
- Standardized error response formatting

**Validation Architecture**:

The validation middleware uses Go's generic type system to provide type-safe, reusable validation across all request types. It combines JSON parsing, model validation, and context propagation in a single, composable middleware.

**Key Features**:

- **Generic Design**: Single middleware handles any request type implementing the Validator interface
- **Type Safety**: Compile-time verification of request types and validation methods
- **Reflection-Based**: Automatic request instantiation using reflection for maximum flexibility
- **Context Integration**: Validated requests stored in request context for handler access
- **Error Consistency**: Uniform error response format across all validation failures

**Benefits**:

- **Code Reuse**: One middleware implementation serves all request types
- **Type Safety**: Generics eliminate runtime type assertion errors
- **Extensibility**: New request types automatically supported via interface implementation
- **Performance**: Efficient request processing with minimal overhead

### 6. HTTP Handlers (`internal/handlers/`)

#### AI Handler (`ai_handler.go`)

**Responsibilities**:

- HTTP request processing for AI operations
- Prompt building and AI provider coordination
- Error handling and response formatting
- Request logging and performance tracking

**Handler Architecture**:

The AI handler orchestrates the complete request processing pipeline, from validated request extraction through prompt building to AI provider interaction. It maintains comprehensive logging for monitoring and debugging.

**Request Processing Flow**:

1. **Request Extraction**: Retrieves validated request from middleware context
2. **Request ID Generation**: Ensures every request has a unique identifier for tracing
3. **Prompt Building**: Uses template manager with `BuildPrompt(mode, variant, data)` for dynamic prompts
4. **AI Provider Call**: Delegates to configured LLM provider with context propagation
5. **Response Processing**: Formats successful responses with metadata
6. **Error Handling**: Translates errors to standardized API responses
7. **Performance Logging**: Records processing metrics for monitoring

**Key Features**:

- **Dependency Injection**: Clean separation of concerns through injected dependencies
- **Context Awareness**: Full request context propagation for timeouts and cancellation
- **Error Translation**: Provider errors mapped to appropriate HTTP status codes
- **Structured Logging**: Comprehensive request tracking with correlation IDs

#### Health Handler (`health_handler.go`)

**Responsibilities**:

- Service health and readiness reporting
- Kubernetes-compatible health check endpoints
- Basic service information exposure

**Health Check Design**:

The health handler provides simple, lightweight endpoints for service monitoring and orchestration systems. It follows Kubernetes health check conventions with separate liveness and readiness endpoints.

**Endpoints**:

- **Healthz**: Basic liveness check indicating the service is running
- **Readyz**: Readiness check indicating the service is ready to handle requests

**Integration Points**:

- **Load Balancers**: Health checks for traffic routing decisions
- **Kubernetes**: Liveness and readiness probe configuration
- **Monitoring**: Basic service availability tracking

### 7. Routing System (`internal/routers/`)

**Responsibilities**:

- HTTP route definition and organization
- Middleware application and request flow
- Endpoint grouping and versioning preparation
- Clean separation between functional areas

#### AI Routes (`ai_routes.go`)

Defines all AI-related endpoints with appropriate middleware stack. Uses generic validation middleware for type-safe request processing.

**Current Endpoints**:

- `POST /ai/explain`: Code explanation with validation middleware
- **Future**: `/ai/hint`, `/ai/test`, `/ai/refactor`, `/ai/summary` endpoints prepared

#### Health Routes (`health_routes.go`)

Provides standard health check endpoints following cloud-native conventions.

**Endpoints**:

- `GET /healthz`: Liveness probe for orchestration systems
- `GET /readyz`: Readiness probe for load balancer configuration

### 8. Utility Layer (`internal/utils/`)

#### Response Utilities (`response.go`)

**Responsibilities**:

- HTTP response formatting and serialization
- Consistent JSON response structure
- Content-type and status code management

**Utility Functions**:

The utility layer provides a simple, focused JSON response helper that ensures consistent response formatting across all handlers. It handles proper content-type headers and JSON serialization with error handling.

## Error Handling Architecture

### Error Type Hierarchy

**Three-Tier Error System**:

1. **Provider Errors**: LLM-specific errors with standardized codes and error wrapping
2. **Application Errors**: Structured business logic errors with detailed validation information
3. **HTTP Errors**: Client-facing errors with appropriate status codes and user-friendly messages

### Error Propagation Flow

**Error Translation Chain**:

```
External API Error → Provider Error → HTTP Error Response
Validation Failure → Application Error → HTTP Error Response
System Exception → Generic Error → HTTP Error Response
```

**Error Processing Stages**:

1. **Error Detection**: Identify error source and type at the boundary
2. **Error Translation**: Convert external errors to internal error types
3. **Error Enhancement**: Add context, request IDs, and debugging information
4. **Error Formatting**: Transform to client-appropriate HTTP responses
5. **Error Logging**: Record detailed error information for monitoring

### Error Handling Patterns

**Consistent Error Processing**:

- **Provider Integration**: External API errors mapped to standardized provider error codes
- **Handler Processing**: Errors translated to appropriate HTTP status codes with structured responses
- **Middleware Validation**: Request validation errors formatted consistently across endpoints
- **Context Preservation**: Request IDs and correlation data maintained through error chain

## Dependency Management

### Dependency Injection Architecture

**Constructor-Based Injection**:

The service uses explicit constructor-based dependency injection throughout, promoting testability and loose coupling. All dependencies are injected at component creation time, eliminating hidden dependencies and making the dependency graph explicit.

**Dependency Strategy**:

- **Interface Dependencies**: Core business logic depends on interfaces (Provider, Validator) enabling easy testing and provider swapping
- **Concrete Dependencies**: Infrastructure concerns (logging, configuration) use concrete types for simplicity
- **Factory Pattern**: Complex dependencies (AI providers) created through factory functions with proper error handling
- **Lifecycle Management**: Dependencies have clear initialization and cleanup semantics

### Interface-Driven Design

**Abstraction Layers**:

- **Provider Interface**: Abstracts LLM provider implementation details from business logic
- **Validator Interface**: Enables generic validation middleware across all request types
- **Handler Interfaces**: Clean separation between HTTP concerns and business logic

**Benefits**:

- **Testability**: Easy mocking and unit testing with interface dependencies
- **Extensibility**: New providers and validators added without changing existing code
- **Maintainability**: Clear dependency boundaries and explicit component contracts

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

**Extension Process**:

1. **Create Provider Package**: Add new provider in `internal/llm/newprovider/` directory
2. **Implement Provider Interface**: Create client implementing `GenerateExplanation()` and `GetProviderName()` methods
3. **Provider Registration**: Self-register using factory function in `init()` method
4. **Configuration Integration**: Add provider-specific configuration handling
5. **Error Mapping**: Map provider-specific errors to standard error codes
6. **Testing**: Ensure provider meets interface contract and error handling requirements

**Architecture Benefits**: The plugin-style architecture allows new providers without modifying existing code.

### Adding New AI Modes

**Template-Based Extension**:

1. **Create YAML Template**: Add new template file in `internal/prompts/templates/`
2. **Define Mode Structure**: Use `base_prompt` + `prompts` structure for consistency
3. **Request/Response Models**: Create mode-specific request and response structures
4. **Handler Implementation**: Add handler method using existing prompt and provider infrastructure
5. **Route Registration**: Add new endpoint to routing configuration

**Current Template Structure**: All modes follow unified YAML format with base prompt and variant-specific prompts.

### Adding New Supported Languages

**Extension Steps**:

1. **Request Validation**: Update supported languages map in request validation
2. **Template Compatibility**: Verify prompt templates handle new language syntax
3. **Provider Testing**: Confirm LLM provider supports the programming language
4. **Integration Testing**: Test end-to-end functionality with real code samples

**Current Support**: Python, Java, C++, JavaScript with validation ensuring only supported languages are processed.
