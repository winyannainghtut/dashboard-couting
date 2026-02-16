FROM golang:1.22 AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download || true

COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o dashboard-service .

FROM ubuntu:latest

RUN apt-get update && apt-get install -y \
    curl \
    net-tools \
    dnsutils \
    tcpdump \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/dashboard-service .

ENV PORT=80
ENV COUNTING_SERVICE_URL=http://localhost:9001

EXPOSE 80

CMD ["./dashboard-service"]
