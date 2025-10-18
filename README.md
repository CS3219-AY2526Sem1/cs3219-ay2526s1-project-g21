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
- User:     http://localhost:8081
- question: http://localhost:8082
- match:    http://localhost:8083
- collab:   http://localhost:8084
- MongoDB:  mongodb://localhost:27017  
- Redis:    redis://localhost:6379
- Postgres: postgres://localhost:5432

## Structure
- services/<svc> : Go microservice with chi router and health endpoints
- deploy/docker-compose.yaml : local dev
- .github/workflows/ci.yml : minimal CI (test + build)


### Note: 
- You are required to develop individual microservices within separate folders within this repository.
- The teaching team should be given access to the repositories as we may require viewing the history of the repository in case of any disputes or disagreements. 

[![Review Assignment Due Date](https://classroom.github.com/assets/deadline-readme-button-22041afd0340ce965d47ae6ef1cefeee28c7c493a6346c4f15d667ab976d596c.svg)](https://classroom.github.com/a/QUdQy4ix)
