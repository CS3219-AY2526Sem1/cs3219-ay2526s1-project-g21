# Question Service

A lightweight HTTP service that manages coding questions for PeerPrep.

## Overview
- **Language/Framework**: Go, `chi` router, `zap` logging
- **Default Port**: 8080 (override with env `PORT`)
- **Health**: `/healthz`, `/readyz`

## API Endpoints
Base URL: `http://localhost:8080`

- GET `/healthz` — Liveness probe, returns `ok`
- GET `/readyz` — Readiness probe, returns `ready`
- GET `/questions` — List all questions
- POST `/questions` — Create a question
- GET `/questions/{id}` — Get a question by ID
- PUT `/questions/{id}` — Update a question by ID
- DELETE `/questions/{id}` — Delete a question by ID
- GET `/questions/random` — Get a random eligible question

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

## Architecture (high level)
- **Entry point**: `cmd/server/main.go` sets up `chi` router, middleware, and graceful shutdown.
- **Routing**: `internal/routers` registers routes for health and questions.
- **Handlers**: `internal/handlers` perform request validation, map to repository calls, and return JSON using `utils`.
- **Repository layer**: `internal/repositories` abstracts data access. Currently stubbed; to be replaced with a real datastore.
- **Models**: `internal/models` define API/data shapes (e.g., `Question`, `TestCase`).
- **Middleware**: Request ID, real IP, structured logging, panic recovery, and 60s request timeout.
- **Config**: `PORT` environment variable controls listen address (defaults to 8080).
- **Observability**: `zap` for structured logs; `/healthz` and `/readyz` for probes.

Data flow: HTTP request → router → handler → repository → response JSON.

Run directly:
```bash
cd services/question
go run ./cmd/server
```

Testing examples (From bash terminal):
```bash
# To list all
Invoke-RestMethod -Uri "http://localhost:8082/questions"  

# To create a question
Invoke-RestMethod -Uri "http://localhost:8082/questions" `
  -Method POST -Headers @{ "Content-Type" = "application/json" } `
  -Body '{"id":"t1","title":"Seed – Arrays Intro","difficulty":"Easy","tags":["Array"],"status":"active"}'

# To list a specific question
Invoke-RestMethod -Uri "http://localhost:8082/questions/t1"  

# To update a question
Invoke-RestMethod -Uri "http://localhost:8082/questions/t1" `
  -Method PUT -Headers @{ "Content-Type" = "application/json" } `
  -Body '{"id":"t1","title":"Seed – Arrays Intro","difficulty":"Hard","tags":["Array"],"status":"active"}'  

# To Delete a question
Invoke-RestMethod -Uri "http://localhost:8082/questions/t1" `
  -Method DELETE
```

Alternatively, see repo `deploy/docker-compose.yaml` for multi-service setup.

## Notes
- Requests/Responses are JSON unless noted.
- Logging uses production `zap` configuration.
- The Database is currently initialized with 3 example questions, found in deploy/seeds/questions.seed.js