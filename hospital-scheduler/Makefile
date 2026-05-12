.PHONY: build run dev test clean tidy

APP    = hospital-scheduler
BINARY = ./bin/$(APP)

## build: compile production binary (requires gcc for CGO/sqlite3)
build:
	@mkdir -p bin
	CGO_ENABLED=1 go build -ldflags="-s -w" -o $(BINARY) ./cmd/server

## run: build and run with default settings
run: build
	@mkdir -p data
	DB_PATH=./data/hospital.db PORT=8080 $(BINARY)

## dev: run without pre-building (fast reload during development)
dev:
	@mkdir -p data
	CGO_ENABLED=1 go run ./cmd/server

## tidy: sync go.mod / go.sum
tidy:
	go mod tidy

## test: run integration test suite
test:
	CGO_ENABLED=1 go test ./...

## clean: remove build artifacts and local database
clean:
	rm -rf bin/ data/

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/## //'
