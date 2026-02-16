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

## Current State
- **Code**: Fully implemented locally.
- **Git**: Changes for Redis integration are **staged locally but NOT pushed**.
- **Build**: The `counting-service` build relies on `go mod tidy` running inside the container because local `go get` failed.

## Where to Continue

### 1. Verification
- **Action**: Run `docker-compose up --build` locally.
- **Expected Outcome**: 
  - All 3 containers (dashboard, counting, redis) start.
  - Dashboard shows the count.
  - Restarting `counting-service` preserves the count (Redis persistence).

### 2. Deployment / Push
- **Action**: Once verified, push the changes to GitHub.
- **Consideration**: The CI/CD pipeline currently builds the Go images. The new `Dockerfile` logic (`go mod tidy`) should work in CI as well since it runs in a standard Docker build environment.

### 3. Future Improvements
- **Dashboard Optimization**: Currently, the Dashboard calls `counting-service` to get the count. For scale, the Dashboard could read directly from Redis (CQRS pattern) to reduce load on the Counting Service.
- **Testing**: Add integration tests in the CI pipeline that spin up Redis and verify the API response.
- **Configuration**: Move hardcoded ports and URLs to a central `.env` file for `docker-compose`.
