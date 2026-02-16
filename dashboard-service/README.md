# Dashboard Service

A web application that displays a numeric dashboard. It retrieves a count from a backend counting service and displays a live update.

For use in learning to use Consul for service discovery and segmentation (connection via secure proxies).

Defaults to running on port 80. Set `PORT` as ENV var to specify another port.

Defaults to looking for the `counting-service` running at `localhost:9001`. Can be set with the `COUNTING_SERVICE_URL` ENV var.

### Run precompiled binary

To run with the defaults (port 80, looking for the backend counting service at `localhost:9001`):

    dashboard-service

To run on a specific port or looking for the `counting-service` elsewhere:

    PORT=9002 COUNTING_SERVICE_URL=counting.service.consul dashboard-service

### Build

Build for Linux and Darwin:

    ./bin/build

Output can be found in `dist`.

### Run from source

    go get
    PORT=9002 go run main.go

### View

    http://localhost:9002

### Dependencies

This application assumes that a counting service is running on `localhost:9001`.

## Docker Support

### Prerequisites
- Docker installed (Desktop or Engine)

### Building the Image
```bash
docker build -t dashboard-service .
```

### Running the Container
Run the container mapping port 8080 (host) to 80 (container):
```bash
docker run -d -p 8080:80 --name dashboard -e COUNTING_SERVICE_URL="http://<counting-service-host>:<port>" dashboard-service
```
*Note: Replace `<counting-service-host>:<port>` with the actual address of the counting service. If running locally with another container, use the Docker network alias or host IP.*

### Debugging
The container image is based on Ubuntu and includes helpful network debugging tools:
- `curl`
- `netstat`
- `nslookup`
- `tcpdump`
- `bash`

To access the shell for debugging:
```bash
docker exec -it dashboard bash
```

### CI/CD
A GitHub Actions workflow is included in `.github/workflows/docker-publish.yml` to automatically build and push the image to Docker Hub on commits to `main` or `master`.
Requires `DOCKER_USERNAME` and `DOCKER_PASSWORD` repository secrets.
