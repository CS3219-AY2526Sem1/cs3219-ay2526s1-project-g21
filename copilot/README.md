# Overview

This directory contains manifest files for deploying the following backend services for Peerprep

- API Gateway
- Collab
- Match
- Question
- User
- Voice

These services are deployed as private backend services, using AWS Fargate, with the exception of the API Gateway, which is deployed as a front-facing, load balanced web service.

All external requests to these backend services must be made through the API Gateway, which redirects the traffic to the correct backend service.

This directory does NOT contain the deployment files for the following services

- Databases required for each backend service
- Sandbox

## Setting Up

### Prerequisites

Before deploying these services, make sure that

1. You have Amazon Copilot installed and are logged in
2. The databases required for each services have been set up
3. The sandbox service is up and running

### Deployment Steps

1. At the project root, intialize the app by running the following

`copilot app init peerprep`

2. Intialize the dev environment by running the following

`copilot env init --name dev --profile default --default-config
copilot env deploy --name dev`

3. Initialize each backend service by running the `scripts/initializeServices.sh` script.

4. Replace the `manifest.yml` files of each service with the files of each service in this repository.

5. Initialize the secrets (URL, database credentials, sandbox service location, etc) by modifying and running the `scripts/setSecrets.sh` script.

6. Deploy each backend service by running the `scripts/deployServices.sh` script.
