# Dashboard-Counting

This repository contains a demonstration of a microservices architecture using Consul for service discovery. It consists of two main services: `counting-service` and `dashboard-service`.

## Services

### Counting Service
A backend service written in Go regarding counting logic.
- **Port**: 9001 (default)
- **Path**: `/counting-service`
- **Configuration**:

  | Variable | Values | Default | Description |
  |----------|--------|---------|-------------|
  | `PORT` | any port | `9001` | Service listen port |
  | `STORAGE_MODE` | `redis`, `memory`, `postgres` | `redis` | Storage backend to use |

  **Redis Options** (when `STORAGE_MODE=redis`):

  | Variable | Values | Default | Description |
  |----------|--------|---------|-------------|
  | `REDIS_URL` | `host:port` | `localhost:6379` | Redis server address (Single mode) |
  | `REDIS_MODE` | `single`, `sentinel`, `cluster` | `single` | Redis deployment topology |
  | `REDIS_MASTER_NAME` | string | `mymaster` | Sentinel master set name (Sentinel mode) |
  | `REDIS_SENTINEL_ADDRS` | `host1:port,host2:port,...` | — | Comma-separated Sentinel addresses |
  | `REDIS_CLUSTER_ADDRS` | `host1:port,host2:port,...` | — | Comma-separated Cluster seed nodes |

  **PostgreSQL Options** (when `STORAGE_MODE=postgres`):

  | Variable | Values | Default | Description |
  |----------|--------|---------|-------------|
  | `PG_URL` | connection string | — | e.g. `postgres://user:pass@host:5432/db?sslmode=disable` |
  | `PG_MODE` | `single`, `cluster` | `single` | PostgreSQL deployment topology |

### Redis Architecture Notes
When choosing between **Redis Cluster** and **Redis Sentinel**:
- **Redis Sentinel**: Best for High Availability (HA) with smaller datasets.
  - Recommended minimum: 1 Master + 1 Replica + 3 Sentinels (works with 2-3 nodes).
  - Handles failover automatically if the master dies.
- **Redis Cluster**: Best for large datasets that need sharding (Partitioning).
  - **Required minimum**: 3 Master nodes (for consensus/quorum).
  - **Recommended for HA**: 6 Nodes (3 Masters + 3 Replicas) to survive node failures without data loss.
  - A 3-node cluster (Masters only) will **stop working** if a single node fails, because data is sharded and redundancy is lost.

### PostgreSQL Architecture Notes
The counting-service also supports PostgreSQL as a backend (`STORAGE_MODE=postgres`).
- **Single Mode**: One `bitnami/postgresql` instance. Simple and reliable.
  - Use `docker-compose.postgres.yml` to test.
- **Cluster Mode** (`PG_MODE=cluster`): Uses `bitnami/postgresql` streaming replication (1 Master + 2 Slaves).
  - Slaves stream WAL from the master for real-time read replicas.
  - If the master fails, a slave can be promoted via trigger file (`/tmp/postgresql.trigger.5432`).
  - Use `docker-compose.postgres-cluster.yml` to test.

### Dashboard Service
A frontend service that displays the count from the counting service.
- **Port**: 80 (default)
- **Path**: `/dashboard-service`

### Redis Service
A Redis instance that stores the count state.
- **Port**: 6379
- **Persistence**: Disabled (count resets on restart) for demo purposes.

## Architecture

The system uses a Service-to-Service pattern where the Dashboard Service communicates with the Counting Service, which in turn persists data to Redis.

**Features:**
- **Graceful Degradation**: If Redis is unavailable, the Counting Service returns a fallback response, and the Dashboard displays a visual error banner.
- **Observability**: The Dashboard displays the hostname of the backend container and the unique Run ID of the Redis instance.

```mermaid
graph TD
    User((User))
    
    subgraph "Docker Network"
        DS[Dashboard Service]
        CS[Counting Service]
        Redis[(Redis)]
        
        DS -- HTTP GET --> CS
        CS -- INCR --> Redis
    end
    
    User -- Browser --> DS
```

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

### Standalone Mode (No Redis)
You can run the `counting-service` in standalone mode, where it uses an in-memory counter instead of Redis. This is useful for development or simple deployments where persistence is not required.

To enable standalone mode, set the `STORAGE_MODE` environment variable to `memory`.

**PowerShell:**
```powershell
$env:PORT="9001"; $env:STORAGE_MODE="memory"; go run main.go
```

**Bash:**
```bash
PORT=9001 STORAGE_MODE=memory go run main.go
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

# Run the services (and build local changes)
docker-compose up --build
```

## Testing Scenarios

You can verify the system's robustness by forcing failures:

### 1. Redis Failure (Graceful Degradation)
**Scenario**: Stop Redis but keep other services running.
```bash
docker stop demo-consul-101-redis-counting-1
```
**Expected Result**:
- Dashboard: Shows a red error banner (e.g., "Redis Error: ...").
- Count: Shows "!" or error state.
- **Critical**: The "Counting Service" hostname card **still displays a valid ID**. This proves the backend service is alive and handling the error gracefully.
- Redis Run ID: Shows "Unknown".

**Resume**:
```bash
docker start demo-consul-101-redis-counting-1
```
**Expected Result**: Application recovers automatically. Error banner disappears, and count resumes.

### 2. Counting Service Failure
**Scenario**: Stop the backend service.
```bash
docker stop demo-consul-101-counting-service-1
```
**Expected Result**:
- Dashboard: Shows "Counting Service is Unreachable" in the status badge.
- **Resilience**: The **Dashboard Hostname** card remains visible and valid, proving the frontend service is functioning.
- Count/Backend Info: Shows error or "Waiting...".

### 3. Data Persistence Test
**Scenario**: Verify if data survives a restart.
1. Increment the count a few times.
2. Restart the stack: `docker-compose restart`
3. Check if count continues or resets to 1.

**Configuration**:
- **Ephemeral (Current Default)**: Count **resets to 1**.
  configured in `docker-compose.yml`:
  ```yaml
  command: redis-server --save "" --appendonly no
  ```
- **Persistent**: Count **continues**.
  To enable, remove the `command` line in `docker-compose.yml` and restart (`docker-compose up --build --force-recreate`).

## CI/CD Pipeline

The project uses GitHub Actions for Continuous Integration and Deployment. 

- **Workflow**: `.github/workflows/ci-cd.yml`
- **Trigger**: Pushes and Pull Requests to the `main` branch.
- **Actions**:
    - Builds Docker images for both services in parallel.
    - Pushes images to Docker Hub using secrets `DOCKER_USERNAME` and `DOCKER_PASSWORD`.

To view the workflow runs, navigate to the **Actions** tab in the GitHub repository.
