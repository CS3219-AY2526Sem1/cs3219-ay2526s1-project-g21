[![Review Assignment Due Date](https://classroom.github.com/assets/deadline-readme-button-22041afd0340ce965d47ae6ef1cefeee28c7c493a6346c4f15d667ab976d596c.svg)](https://classroom.github.com/a/QUdQy4ix)
# CS3219 Project (PeerPrep) - AY2526S1
## Group: G21

## Quick start (local)

```bash
docker compose -f deploy/docker-compose.yaml up --build
```

Services:
- user:     http://localhost:8081
- question: http://localhost:8082
- match:    http://localhost:8083
- collab:   http://localhost:8084

MongoDB: mongodb://localhost:27017  
Redis:   redis://localhost:6379

## Structure
- services/<svc> : Go microservice with chi router and health endpoints
- deploy/docker-compose.yaml : local dev
- .github/workflows/ci.yml : minimal CI (test + build)


### Note: 
- You are required to develop individual microservices within separate folders within this repository.
- The teaching team should be given access to the repositories as we may require viewing the history of the repository in case of any disputes or disagreements. 
