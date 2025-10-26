#!/bin/bash
set -euo pipefail

# Define services: name,type,dockerfile
services=(
  "api-gateway,Load Balanced Web Service,./services/api-gateway/Dockerfile"
  "collab,Backend Service,./services/collab/Dockerfile"
  "match,Backend Service,./services/match/Dockerfile"
  "question,Backend Service,./services/question/Dockerfile"
  "user,Backend Service,./services/user/Dockerfile"
  "voice,Backend Service,./services/voice/Dockerfile"
)

for svc in "${services[@]}"; do
  IFS=',' read -r name type dockerfile <<< "$svc"
  echo "Initializing service: $name ($type) using $dockerfile"
  
  copilot svc init --name "$name" --svc-type "$type" --dockerfile "$dockerfile" || {
    echo "Failed to initialize $name, skipping..."
    continue
  }
done

echo "All services processed."
