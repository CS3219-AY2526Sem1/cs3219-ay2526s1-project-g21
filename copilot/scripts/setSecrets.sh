#!/bin/bash
declare -A PARAMS=(
    ["/collab/QUESTION_SERVIE_URL"]="localhost"
    ["/collab/REDIS_ADDR"]="localhost:6379"
    ["/collab/SANDBOX_URL"]="localhost:8090"
    
    ["/match/REDIS_ADDR"]="localhost:6379"

    ["/user/REDIS_URL"]="localhost:6379"
    ["/user/POSTGRES_HOST"]="localhost"
    ["/user/POSTGRES_USER"]="postgres"
    ["/user/POSTGRES_PASSWORD"]="postgres"
    ["/user/POSTGRES_DB"]="postgres"
    
    ["/voice/REDIS_ADDR"]="localhost:6379"
)



for key in "${!PARAMS[@]}"; do
    aws ssm put-parameter \
        --name "$key" \
        --value "${PARAMS[$key]}" \
        --type "SecureString" \
        --overwrite
done
