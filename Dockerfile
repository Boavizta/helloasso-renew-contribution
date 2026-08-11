FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o /bin/helloasso-renew-contribution ./

FROM debian:trixie-slim

RUN apt-get update \
    && apt-get install -y ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/helloasso-renew-contribution /usr/local/bin/helloasso-renew-contribution

ENTRYPOINT ["/usr/local/bin/helloasso-renew-contribution"]
