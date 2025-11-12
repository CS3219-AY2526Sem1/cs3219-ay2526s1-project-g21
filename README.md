# CS3219 Project (PeerPrep) - AY2526S1 G21

This project is a full-stack collaborative coding platform inspired by the experience of solving problems on LeetCode, but extended with real-time teamwork and AI-powered assistance. It provides an environment where multiple users can simultaneously work on programming challenges, discuss ideas through an integrated chat system, run their code in a secure sandbox, and get generative AI support for hints, debugging, and explanations.

### Key Features

- Collaborative Code Editor: Built with React + TypeScript, enabling multiple users to edit the same file in real-time with conflict-free synchronization.

- In-session Chat: Lightweight chat panel for instant communication while coding together, supporting discussions, code snippets, and debugging notes.

- Sandboxed Execution: Backend powered by Go for securely compiling and executing user-submitted code in isolated environments.

- Gen-AI Integration: Seamlessly integrated AI assistant that can:

  - Suggest hints or alternative approaches.

  - Explain code snippets.

  - Debug common errors.

  - Provide step-by-step walkthroughs of problems.

## Quick start (local)

1. Copy the .env example file and add relevant credentials

```bash
cp .env.example .env
```

2. Run the following command to run the docker containers for each service/databasee

```bash
docker compose -f deploy/docker-compose.yaml up --build
```

Note that the following services/databases will be setup locally:

- User: http://localhost:8081
- question: http://localhost:8082
- match: http://localhost:8083
- collab: http://localhost:8084
- voice: http://localhost:8085
- sandbox: http://localhost:8090
- prometheus: http://localhost:9090
- grafana: http://localhost:3000
- MongoDB: mongodb://localhost:27017
- Redis: redis://localhost:6379
- Postgres: postgres://localhost:5432

3. (Optional but recommended) Seed local databases with sample data

```bash
docker compose -f deploy/docker-compose.yaml run --rm mongo-seed
docker compose -f deploy/docker-compose.yaml run --rm postgres-seed
```

- The Postgres seed creates two verified accounts:
  - `test_1` / `Password123!`
  - `test_2` / `Password123!`

## Observability (Prometheus + Grafana)

- All Go services expose a `GET /metrics` endpoint with request counters and latency histograms labelled by service, path, and status.
- Additional metrics capture in-flight request gauges plus request/response size histograms to spot back-pressure and payload regressions quickly.
- Prometheus is bundled in `deploy/docker-compose.yaml` and scrapes every backend automatically; reach it via `http://localhost:9090` for ad-hoc queries.
- Grafana is available at `http://localhost:3000` (default credentials `admin/admin`) with an auto-provisioned **PeerPrep Services Overview** dashboard that shows:
  - Request rate broken down by service and status.
  - 5xx error rate per service (rolling 5 minute window).
  - Live health status using Prometheus `up` metrics.
- Tweak or add dashboards by editing the files under `deploy/grafana/`; they are hot-reloaded when the container restarts.

## Structure

- services/<svc> : Go microservice with chi router and health endpoints
- deploy/docker-compose.yaml : local dev
- .github/workflows/ci.yml : minimal CI (test + build)

## Disclaimer on AI Usage

AI tools were used in this project to assist with **code autocompletion**, **refactoring**, **code review suggestions**, and **documentation improvements**. All outputs were reviewed and verified by the development team before inclusion.

### Note:

- You are required to develop individual microservices within separate folders within this repository.
- The teaching team should be given access to the repositories as we may require viewing the history of the repository in case of any disputes or disagreements.

[![Review Assignment Due Date](https://classroom.github.com/assets/deadline-readme-button-22041afd0340ce965d47ae6ef1cefeee28c7c493a6346c4f15d667ab976d596c.svg)](https://classroom.github.com/a/QUdQy4ix)
