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
  - `DB_REQUEST_TIMEOUT_MS` (optional DB request timeout in milliseconds, default `1000`)
  - `DNS_SERVER` (optional custom DNS server, e.g. `127.0.0.1:8600`)
  - `CONSUL_DNS_ADDR` (optional alias of `DNS_SERVER`, useful for Consul DNS)
  - `DNS_NETWORK` (optional DNS protocol: `udp` or `tcp`, default `udp`)
  - `DNS_TIMEOUT_MS` (optional DNS dial timeout in milliseconds, default `1500`)

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
  - `DNS_SERVER` (optional custom DNS server, e.g. `127.0.0.1:8600`)
  - `CONSUL_DNS_ADDR` (optional alias of `DNS_SERVER`, useful for Consul DNS)
  - `DNS_NETWORK` (optional DNS protocol: `udp` or `tcp`, default `udp`)
  - `DNS_TIMEOUT_MS` (optional DNS dial timeout in milliseconds, default `1500`)

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

## Docker Compose (CockroachDB Cluster)

A separate compose file is available for a 3-node CockroachDB cluster:

```bash
docker compose -f docker-compose.cockroach-cluster.yml up --build
```

Access:

- Dashboard UI: `http://localhost:8080`
- Counting API: `http://localhost:9001`
- Cockroach Node 1 SQL/UI: `localhost:26257` / `http://localhost:8081`
- Cockroach Node 2 SQL/UI: `localhost:26258` / `http://localhost:8082`
- Cockroach Node 3 SQL/UI: `localhost:26259` / `http://localhost:8083`

In this cluster compose file, all Cockroach nodes share the Docker DNS alias `roachdb`, and `counting-service` connects via `PG_URL=...@roachdb:26257...` for smoother node failover.

This file also sets an explicit Compose project name (`dashboard-couting-cluster`) to avoid collisions with the default `docker-compose.yml` stack.

Important: with 3 nodes, Cockroach tolerates 1 node failure for writes (quorum). If 2 nodes are down, write operations will fail.

## Consul DNS Example

If you run Consul and want DNS-based service discovery, you can point both services at Consul DNS:

```bash
# counting-service
set CONSUL_DNS_ADDR=127.0.0.1:8600

# dashboard-service
set CONSUL_DNS_ADDR=127.0.0.1:8600
set COUNTING_SERVICE_URL=http://counting-service.service.consul:9001
```

`DNS_SERVER` and `CONSUL_DNS_ADDR` are interchangeable. If both are set, `DNS_SERVER` is used.

## Behavior During DB Failure

When CockroachDB is unavailable:

- `counting-service` stays up.
- API returns `count: -1` with an error `message`.
- API still returns the `hostname` of counting service.
- Dashboard continues showing counting hostname and marks DB information as unavailable.

## Verify Data In CockroachDB

After the stack is running, you can verify writes directly from CockroachDB.

### Cluster Compose

Open SQL shell on node 1:

```bash
docker compose -f docker-compose.cockroach-cluster.yml exec roach1 /cockroach/cockroach sql --insecure --host=roach1:26257
```

Run:

```sql
USE defaultdb;
SHOW TABLES;
SELECT id, count FROM counts;
```

Quick end-to-end check:

```bash
curl http://localhost:9001/
curl http://localhost:9001/
docker compose -f docker-compose.cockroach-cluster.yml exec roach1 /cockroach/cockroach sql --insecure --host=roach1:26257 -e "SELECT id, count FROM counts;"
```

### Single-Node Compose

Open SQL shell:

```bash
docker compose exec roach1 /cockroach/cockroach sql --insecure --host=roach1:26258
```

Run:

```sql
USE defaultdb;
SHOW TABLES;
SELECT id, count FROM counts;
```

If `counts` does not exist yet, call `http://localhost:9001/` once first. The table is created lazily by `counting-service`.

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
