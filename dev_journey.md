# Development Journey

## What Has Been Done

### 1. Repository Consolidation & Cleanup
- **Problem**: The project consisted of two separate services (`counting-service`, `dashboard-service`) that were seemingly treated as nested git repositories (submodules) but without proper linking.
- **Action**: 
  - Removed `.git` directories from service subfolders to treat them as regular directories in a monorepo.
  - Removed git submodule references (`git rm --cached`).
  - Created a root `.gitignore` to handle Go artifacts (`bin/`, `*.exe`) and system files.

### 2. CI/CD Unification
- **Problem**: Each service had its own GitHub Actions workflow, leading to duplicated logic and potential drift.
- **Action**:
  - Deleted individual `docker-publish.yml` workflows.
  - Created a single root workflow `.github/workflows/ci-cd.yml`.
  - Configured parallel jobs for `counting-service` and `dashboard-service`.
  - Utilized `docker/build-push-action` for efficient caching and building.

### 3. Local Development Improvements
- **Problem**: No standard way to run the stack locally.
- **Action**:
  - Created `README.md` with instructions for local Go execution and Docker commands.
  - Created `docker-compose.yml` to orchestrate both services.
  - Updated `docker-compose.yml` repeatedly to reflect architecture changes (Redis).

### 4. Redis Integration for High Availability (HA)
- **Problem**: `counting-service` used an in-memory variable for state. Restarting the service (or scaling it) meant losing the count.
- **Architecture**: Proposed Service-to-Service communication backed by Redis (Option A).
- **Implementation**:
  - **New Service**: Created `redis-counting` directory with a Dockerfile (based on `redis:alpine`).
  - **Backend Update**: Modified `counting-service/main.go` to use `go-redis/v9`. 
    - Replaced `atomic.AddUint64` with `redis.Incr`.
    - Dependencies: Since local `go` environment is missing/broken, updated `Dockerfile` to run `go mod tidy` during build. Manually updated `go.mod` to include `github.com/redis/go-redis/v9`.
  - **Orchestration**: Updated `docker-compose.yml` to include `redis-counting` service and link it to `counting-service` via `REDIS_URL`.
  - **Documentation**: Updated root `README.md` with new architecture diagram and instructions.

### 5. Robustness & Observability
- **Redis Persistence**: Disabled persistence in `docker-compose.yml` (command `redis-server --save "" --appendonly no`) to demonstrate ephemeral behavior or simply reset state easily.
- **Graceful Degradation**: 
  - Updated `counting-service` to catch Redis errors. Instead of 500ing, it returns a JSON object with `count: -1` and the error message.
  - Updated `dashboard-service` (frontend) to detect this error state and show a red error banner instead of crashing or showing nothing.
  - **Timeout Optimization**: Reduced Redis driver timeouts in `counting-service` to 1 second. This ensures the backend fails fast when Redis is down, allowing the Dashboard (which has a 2s timeout) to receive the graceful error response instead of timing out itself.
- **Redis Identification**:
  - Updated `counting-service` to fetch the Redis `run_id` via the `INFO SERVER` command.
  - Updated `dashboard-service` to pass this ID to the frontend.
  - Added a new UI card in the Dashboard to display the Redis Run ID, allowing verification of which Redis instance is being used (useful for future replication/clustering work).

### 6. Redis Sentinel & Cluster Support
- **Problem**: Need to support various Redis deployment modes (Single, Sentinel, Cluster) for different scale/availability requirements.
- **Action**:
  - Refactored `counting-service` to use an interface `CounterStore` and support multiple backends (Memory, Redis).
  - Implemented `getRedisClient` factory to handle:
    - **Single Node**: Direct connection (`REDIS_URL`).
    - **Sentinel**: HA with automatic failover (`REDIS_MODE=sentinel`, `REDIS_MASTER_NAME`, `REDIS_SENTINEL_ADDRS`).
    - **Cluster**: Sharding for scale (`REDIS_MODE=cluster`, `REDIS_CLUSTER_ADDRS`).
  - **Docker Compose**:
    - Created `docker-compose.standalone.yml` for memory-only mode.
    - Created `docker-compose.sentinel.yml` for a full local Sentinel stack (1 Master, 2 Replicas, 3 Sentinels).
    - Created `docker-compose.cluster.yml` for a full local Cluster stack (6 Nodes + Creator).

### 7. PostgreSQL Support
- **Problem**: Need an alternative to Redis for teams that prefer relational databases or need ACID compliance.
- **Action**:
  - Added `PostgresStore` implementing `CounterStore` using `github.com/jackc/pgx/v5`.
  - `Incr()` uses atomic `UPDATE counters SET count = count + 1 WHERE id = 'default' RETURNING count`.
  - `GetInfo()` returns `SELECT version()` (PostgreSQL version string).
  - Added env vars: `STORAGE_MODE=postgres`, `PG_URL`, `PG_MODE` (`single` or `cluster`).
  - Created `postgres/init.sql` schema (auto-initialized `counters` table).
  - **Docker Compose**:
    - Created `docker-compose.postgres.yml` for single PostgreSQL (`bitnami/postgresql:latest`).
    - Created `docker-compose.postgres-cluster.yml` for HA cluster (`bitnami/postgresql:latest` with streaming replication — 1 Master + 2 Slaves).
  - **Verified**: Both single and cluster PostgreSQL modes tested end-to-end — count increments correctly, slave WAL streaming confirmed.

### 8. Code Review & Fixes
- **Fixes Applied**:
  - Moved `res.Body.Close()` to correct position in `dashboard-service/main.go`.
  - Aligned `go.mod` versions across services to match local toolchain (`go 1.21`).
  - Added missing `build: ./dashboard-service` to `docker-compose.yml`.

## Current State
- **Code**: Fully implemented with 4 storage backends: In-Memory, Redis (Single/Sentinel/Cluster), and PostgreSQL (Single/Cluster).
- **Git**: All changes pushed to GitHub.
- **Docker Compose files**: `docker-compose.yml` (default Redis), `docker-compose.standalone.yml` (memory), `docker-compose.sentinel.yml`, `docker-compose.cluster.yml`, `docker-compose.postgres.yml`, `docker-compose.postgres-cluster.yml`.
- **Build**: Both services compile locally with Go 1.21 and in Docker via multi-stage builds.

## Where to Continue

### 1. Deployment / Push
- **Action**: Push changes to GitHub.
- **Consideration**: The CI/CD pipeline builds Go images using Docker. All dependencies are resolved inside the container.

### 2. Future Improvements
- **Dashboard Optimization**: Dashboard could read directly from the database (CQRS pattern) to reduce load on the Counting Service.
- **Testing**: Add integration tests in the CI pipeline that spin up Redis/PostgreSQL and verify the API response.
- **Configuration**: Move hardcoded ports and URLs to a central `.env` file for `docker-compose`.

## Testing Guide

### 1. Robustness Test (Redis Down)
1. Stop Redis: `docker stop demo-consul-101-redis-counting-1`
2. **Observe**: Dashboard shows error banner, but **BackEnd Hostname** is still visible.
3. Start Redis: `docker start demo-consul-101-redis-counting-1`
4. **Observe**: System recovers automatically.

### 2. Counting Service Failure
1. Stop Counting Service: `docker stop demo-consul-101-counting-service-1`
2. **Observe**: Status changes to "Counting Service is Unreachable", but **Dashboard Hostname** remains visible.

### 3. Data Persistence
- **Ephemeral Mode** (Default): Data resets on restart. 
  - Implementation: `command: redis-server --save "" --appendonly no` in docker-compose.
- **Persistent Mode**: Data survives restart.
  - Implementation: Remove the `command` override.
- **PostgreSQL**: Data persists by default (uses a table). Reset by dropping the volume.
