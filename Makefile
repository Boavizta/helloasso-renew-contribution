.PHONY: test test-cover build build-all build-darwin-arm64 build-linux-amd64 build-linux-arm64

test:
	go test -race ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

build:
	go build -o bin/helloasso-renew-contribution

build-all: build-darwin-arm64 build-linux-amd64 build-linux-arm64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o bin/helloasso-renew-contribution-darwin-arm64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o bin/helloasso-renew-contribution-linux-amd64

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o bin/helloasso-renew-contribution-linux-arm64
