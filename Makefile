.PHONY: wire
# 生成wire代码
wire:
	cd ./cmd/server && wire

.PHONY: run
# 运行
run:
	go run ./cmd/server

.PHONY: lint
# 代码检查
lint:
	mise x golangci-lint@2.11.4 -- golangci-lint fmt
	mise x golangci-lint@2.11.4 -- golangci-lint run -v --timeout=10m  --allow-parallel-runners

.PHONY: build
# 编译
build:
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -trimpath -ldflags "-s -w" -o ./bin/server ./cmd/server
