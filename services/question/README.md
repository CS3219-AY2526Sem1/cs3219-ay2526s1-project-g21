# Question Service

A lightweight HTTP service that manages coding questions for PeerPrep.

## Overview
- **Language/Framework**: Go, `chi` router, `zap` logging
- **Default Port**: 8080 (override with env `PORT`)
- **Docker Port**: 8082 (as configured in docker-compose.yaml)
- **Database**: MongoDB
- **CORS**: Configured for frontend at localhost:5173

## API Endpoints
Base URL: `http://localhost:8082` (Docker Compose setup)

### Health Endpoints
- GET `/health` — Health check endpoint

### Question Endpoints
- GET `/questions` — List all questions
- POST `/questions` — Create a question
- GET `/questions/{id}` — Get a question by ID
- PUT `/questions/{id}` — Update a question by ID
- DELETE `/questions/{id}` — Delete a question by ID
- GET `/questions/random` — Get a random question with optional filtering

#### Random Question Filtering
The `/questions/random` endpoint supports query parameters:
- `difficulty` - Filter by difficulty (Easy, Medium, Hard)
- `topic` - Filter by topic tags (comma-separated)

Examples:
- `GET /questions/random?difficulty=Medium`
- `GET /questions/random?topic=array,sorting`
- `GET /questions/random?difficulty=Hard&topic=dynamic-programming`

### Question schema (simplified)
```json
{
  "id": "string",
  "title": "string",
  "difficulty": "Easy|Medium|Hard",
  "topic_tags": ["arrays", "dp"],
  "prompt_markdown": "string",
  "constraints": "string",
  "test_cases": [{ "input": "string", "output": "string", "description": "string" }],
  "image_urls": ["https://..."],
  "status": "active|deprecated",
  "author": "string",
  "created_at": "RFC3339",
  "updated_at": "RFC3339",
  "deprecated_at": "RFC3339|null",
  "deprecated_reason": "string"
}
```

## Features
- **Question Lifecycle Management** - Active/deprecated status support
- **Advanced Filtering** - Random question selection by difficulty and topics
- **MongoDB Integration** - Persistent data storage with BSON mapping
- **CORS Support** - Cross-origin requests enabled for frontend
- **Input Validation** - Request payload validation and error handling
- **Graceful Shutdown** - Proper cleanup on service termination

## Architecture (high level)
- **Entry point**: `cmd/server/main.go` sets up `chi` router, middleware, CORS, and graceful shutdown.
- **Routing**: `internal/routers` registers routes for health and questions.
- **Handlers**: `internal/handlers` perform request validation, filtering, and map to repository calls.
- **Repository layer**: `internal/repositories` handles MongoDB operations with proper error handling.
- **Models**: `internal/models` define API/data shapes with both JSON and BSON tags.
- **Middleware**: CORS, Request ID, real IP, structured logging, panic recovery, and 60s request timeout.
- **Config**: `PORT` environment variable controls listen address (defaults to 8080).
- **Observability**: `zap` for structured logs and `/health` endpoint for monitoring.

Data flow: HTTP request → router → handler → repository → response JSON.

Run directly:
```bash
cd services/question
go run ./cmd/server
```

## Error Responses
All error responses follow this format:
```json
{
  "code": "error_code", 
  "message": "Human readable error message"
}
```

Common error codes:
- `invalid_request` - Invalid request payload
- `invalid_difficulty` - Invalid difficulty parameter (must be Easy, Medium, or Hard)  
- `question_not_found` - Question with specified ID not found
- `no_eligible_question` - No questions match the random query criteria
- `internal_error` - Server-side error

## Testing Examples
```bash
# List all questions
curl http://localhost:8082/questions

# Create a question  
curl -X POST http://localhost:8082/questions \
  -H "Content-Type: application/json" \
  -d '{"title":"Two Sum","difficulty":"Easy","topic_tags":["arrays","hash-table"],"prompt_markdown":"Given an array..."}'

# Get specific question
curl http://localhost:8082/questions/{id}

# Get random question with filters
curl "http://localhost:8082/questions/random?difficulty=Medium&topic=arrays"

# Update a question
curl -X PUT http://localhost:8082/questions/{id} \
  -H "Content-Type: application/json" \
  -d '{"title":"Updated Title","difficulty":"Hard"}'

# Delete a question
curl -X DELETE http://localhost:8082/questions/{id}
```

To refresh seeded data 
```
cd deploy
docker compose up mongo-seed
```

Alternatively, see repo `deploy/docker-compose.yaml` for multi-service setup.

## Notes
- Requests/Responses are JSON unless noted.
- Logging uses production `zap` configuration.
- The Database is currently initialized with 3 example questions, found in deploy/seeds/questions.seed.js