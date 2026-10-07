VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo 0.1.0)
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN     := bin

.PHONY: all build run demo test race vet fmt smoke test-ui dist docker clean help

all: build

help: ## 显示可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n",$$1,$$2}'

build: ## 编译 imserver 和 imcli 到 ./bin
	@mkdir -p $(BIN)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)/imserver ./cmd/imserver
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)/imcli ./cmd/imcli
	@echo "已生成 $(BIN)/imserver  $(BIN)/imcli"

run: build ## 启动服务（数据在 ./data）
	$(BIN)/imserver

demo: build ## 启动服务并创建演示账号 alice/bob/carol（密码 123456）
	$(BIN)/imserver -seed

test: ## 运行全部 Go 测试
	go test ./...

race: ## 带竞态检测运行全部测试（推荐）
	go test -race -count=1 ./...

vet: ## 静态检查
	go vet ./...

fmt: ## 格式化
	gofmt -w cmd internal web

smoke: build ## 命令行冒烟测试（起服务 → 注册 → 发消息 → 离线拉取）
	./scripts/smoke.sh

test-ui: build ## Web 客户端 UI 测试（需要 Node.js）
	./scripts/test-ui.sh

dist: ## 交叉编译 5 个平台到 ./dist
	VERSION=$(VERSION) ./scripts/build-all.sh

docker: ## 构建 Docker 镜像 im-server:latest
	docker build -f deploy/Dockerfile -t im-server:latest --build-arg VERSION=$(VERSION) .

clean: ## 清理编译产物（不删除 ./data）
	rm -rf $(BIN) dist
