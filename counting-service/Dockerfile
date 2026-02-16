# Build Stage
FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o counting-service main.go

# Run Stage
FROM debian:bookworm-slim

WORKDIR /app

# Install requested tools for debugging
RUN apt-get update && apt-get install -y \
    curl \
    net-tools \
    iproute2 \
    iputils-ping \
    dnsutils \
    tcpdump \
    netcat-openbsd \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/counting-service .

# Default port
ENV PORT=9001
EXPOSE 9001

CMD ["./counting-service"]
