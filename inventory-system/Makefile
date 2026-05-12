.PHONY: build run test clean

build:
	CGO_ENABLED=1 go build -o inventory_server .

run: build
	./inventory_server

test:
	CGO_ENABLED=1 go run ./cmd/test/main.go

clean:
	rm -f inventory_server inventory.db

# 仅在有 proxy 访问时使用
deps:
	go mod tidy
