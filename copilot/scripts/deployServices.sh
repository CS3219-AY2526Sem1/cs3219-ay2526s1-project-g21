#!/bin/bash
set -euo pipefail

# List of services in the order you want to deploy
services=(
  "collab"
  "match"
  "user"
  "question"
  "voice"
  "api-gateway"
)

# Target environment
env="dev"

for svc in "${services[@]}"; do
  echo "Deploying service: $svc to environment: $env"
  
  copilot svc deploy --name "$svc" --env "$env" || {
    echo "Failed to deploy $svc, continuing to next service..."
    continue
  }
done

echo "All services deployment attempted."
