# Makefile for KamaitachiGo

.PHONY: all build clean run-master run-slave run-gateway test fmt vet

# 默认目标
all: build

# 编译所有程序
build:
	@echo "Building all services..."
	@go build -o bin/master.exe cmd/master/main.go
	@go build -o bin/slave.exe cmd/slave/main.go
	@go build -o bin/gateway.exe cmd/gateway/main.go
	@echo "Build completed!"

# 清理编译产物
clean:
	@echo "Cleaning..."
	@if exist bin rmdir /s /q bin
	@if exist data rmdir /s /q data
	@echo "Clean completed!"

# 运行主节点
run-master:
	@echo "Starting master node..."
	@go run cmd/master/main.go

# 运行从节点
run-slave:
	@echo "Starting slave node..."
	@go run cmd/slave/main.go

# 运行网关
run-gateway:
	@echo "Starting gateway..."
	@go run cmd/gateway/main.go

# 运行测试
test:
	@echo "Running tests..."
	@go test -v ./...

# 格式化代码
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# 代码检查
vet:
	@echo "Vetting code..."
	@go vet ./...

# 下载依赖
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@echo "Dependencies downloaded!"

# 初始化数据目录
init:
	@if not exist data mkdir data
	@echo "Data directory initialized!"

