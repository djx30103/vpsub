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
	golangci-lint run -v -c ./.golangci.yml --timeout=10m

.PHONY: build
# 编译
build:
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -trimpath -ldflags "-s -w" -o ./bin/server ./cmd/server
