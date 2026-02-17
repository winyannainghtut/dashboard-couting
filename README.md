# Dashboard-Counting

This repository demonstrates a 3-tier microservice setup:

- `dashboard-service` (UI)
- `counting-service` (API/business logic)
- `cockroachdb/cockroach` (database tier)

## Architecture

`Browser -> Dashboard Service -> Counting Service -> CockroachDB`

Dashboard displays:

- Live count
- Counting service hostname
- Dashboard service hostname
- Active CockroachDB node (for single-node setup: `Node 1`)

## Services

### Counting Service

- Default port: `9001`
- Endpoint: `GET /`
- Health: `GET /health`
- Environment variables:
  - `PORT` (default `9001`)
  - `STORAGE_MODE` (`memory` or `cockroach`)
  - `PG_URL` (required when `STORAGE_MODE=cockroach`)

Response shape:

```json
{
  "count": 1,
  "hostname": "counting-container-id",
  "db_node": "Node 1"
}
```

If DB is down/unreachable:

```json
{
  "count": -1,
  "hostname": "counting-container-id",
  "message": "DB Error: ..."
}
```

### Dashboard Service

- Default port: `80` (mapped to host `8080` in compose)
- Health: `GET /health`
- API connectivity health: `GET /health/api`
- WebSocket: `GET /ws`
- Environment variables:
  - `PORT` (default `80`)
  - `COUNTING_SERVICE_URL` (default `http://localhost:9001`)

## Docker Compose (3-Tier Single DB)

Run:

```bash
docker compose up --build
```

Access:

- Dashboard UI: `http://localhost:8080`
- Counting API: `http://localhost:9001`
- Cockroach SQL (host): `localhost:26257`
- Cockroach DB Console: `http://localhost:8081`

Notes:

- Cockroach runs single-node insecure mode for local development only.
- Internally, Cockroach SQL listens on container port `26258`, mapped to host port `26257`.

## Behavior During DB Failure

When CockroachDB is unavailable:

- `counting-service` stays up.
- API returns `count: -1` with an error `message`.
- API still returns the `hostname` of counting service.
- Dashboard continues showing counting hostname and marks DB information as unavailable.

## Local Development (Without Docker)

Prerequisites:

- Go 1.24+ (for `counting-service`)
- Go 1.21+ (for `dashboard-service`)

Run counting service in memory mode:

```bash
cd counting-service
set STORAGE_MODE=memory
set PORT=9001
go run main.go
```

Run dashboard service:

```bash
cd dashboard-service
set PORT=80
set COUNTING_SERVICE_URL=http://localhost:9001
go run main.go
```

## CI/CD

GitHub Actions workflow:

- `.github/workflows/ci-cd.yml`

It builds and publishes both service images.
