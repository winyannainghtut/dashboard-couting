# Dashboard-Counting

This repository contains a demonstration of a microservices architecture using Consul for service discovery. It consists of two main services: `counting-service` and `dashboard-service`.

## Services

### Counting Service
A backend service written in Go regarding counting logic.
- **Port**: 9001 (default)
- **Path**: `/counting-service`

### Dashboard Service
A frontend service that displays the count from the counting service.
- **Port**: 80 (default)
- **Path**: `/dashboard-service`

## Local Development

### Prerequisites
- Go 1.22+
- Docker

### Running Locally
To run the services locally:

1. **Counting Service**:
    ```bash
    cd counting-service
    go run main.go
    ```

2. **Dashboard Service**:
    ```bash
    cd dashboard-service
    go run main.go
    ```

## Docker

You can build and run the services using Docker. Each service has its own `Dockerfile`.

### Building Images

```bash
# Counting Service
docker build -t counting-service ./counting-service

# Dashboard Service
docker build -t dashboard-service ./dashboard-service
```

### Running Containers

You can run the containers individually using `docker run`:

```bash
docker run -p 9001:9001 winyannainghtut/counting-service:latest
docker run -p 8080:80 winyannainghtut/dashboard-service:latest
```

### Docker Compose

Alternatively, you can run both services together using Docker Compose.

**Note:** You must pull the Docker images locally before running the services.

```bash
# Pull the images
docker pull winyannainghtut/counting-service:latest
docker pull winyannainghtut/dashboard-service:latest

# Run the services
docker-compose up
```

## CI/CD Pipeline

The project uses GitHub Actions for Continuous Integration and Deployment. 

- **Workflow**: `.github/workflows/ci-cd.yml`
- **Trigger**: Pushes and Pull Requests to the `main` branch.
- **Actions**:
    - Builds Docker images for both services in parallel.
    - Pushes images to Docker Hub using secrets `DOCKER_USERNAME` and `DOCKER_PASSWORD`.

To view the workflow runs, navigate to the **Actions** tab in the GitHub repository.
