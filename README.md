# KamaitachiGo

KamaitachiGo 是一个用 Go 实现的分布式时序数据缓存参考实现。

**声明**：本仓库为公开示例版本，包含脱敏/示例数据与演示工具。

---

## 主要特性

- 纯 Go 实现（无 CGO），便于构建与跨平台部署
- Master + 多 Slave 分布式架构，单节点使用独立 SQLite 数据库以降低文件锁竞争
- 内存 LRU 缓存与查询优化，提升吞吐与降低延迟
- 支持限流与熔断示例实现
- 自研压测工具（位于 `tools/`）支持多场景压测

---

## 运行模式（两种）

1) 直接模式（不使用 Gateway）

- 描述：客户端或压测工具直接请求后端节点地址（例如 `http://localhost:8081`）。
- 适用场景：关注单节点性能、调试或对比节点差异。
- 示例（手动启动节点并压测）：

```powershell
.\bin\master -db data/master.db
.\bin\slave -config conf/slave.ini -db data/slave1.db
.\bin\slave -config conf/slave2.ini -db data/slave2.db
.\bin\benchmark_scenarios -scenario 1 -requests 10000 -concurrent 50 -nodesAddr "http://localhost:8081,http://localhost:8082"
```

2) Gateway 模式（通过统一网关代理）

- 描述：对外通过 Gateway（如 `http://localhost:9000/data/...`）统一入口，Gateway 负责路由、负载均衡、限流与熔断。
- 注意：Gateway 代理时会对路径做处理（例如去掉 `/data` 前缀），具体实现请查看 `cmd/gateway`。
- 示例（启动 Gateway 并通过 Gateway 压测）：

```powershell
.\bin\gateway -config conf/gateway.ini
.\bin\benchmark_scenarios -scenario 1 -requests 10000 -concurrent 50 -nodesAddr "http://localhost:9000/data"
```

选择建议：直接模式用于节点级别诊断与性能基线；Gateway 模式用于对外压力评估与验证统一限流/路由策略。

---

## 启动脚本
## 启动脚本

- 推荐脚本：`start_cluster.ps1`（或 `start_3nodes.ps1`）可一键启动 Master、两个 Slave 与 Gateway（若存在），并做基本健康检查。  
- 脚本路径：[KamaitachiGo/scripts/start_cluster.ps1](KamaitachiGo/scripts/start_cluster.ps1)

示例：

```powershell
.\scripts\start_cluster.ps1
```

---

## 快速开始

前提：已安装 Go 1.21+

1. 构建（可选）：

```bash
go build -o bin/master cmd/master/main.go
go build -o bin/slave cmd/slave/main.go
go build -o bin/gateway cmd/gateway/main.go
go build -o bin/benchmark_scenarios tools/benchmark_scenarios.go
```

或直接运行源代码：

```bash
go run cmd/master/main.go
go run cmd/slave/main.go -config conf/slave.ini
go run cmd/gateway/main.go
```

2. 生成示例数据（轻量用于演示）：

```bash
go run tools/generate_sample_db.go
# 结果：data/sample.db
```

3. 验证健康接口：

```powershell
Invoke-RestMethod -Uri "http://localhost:9000/health"
```

---

## 配置与数据

- `conf/` 目录应仅保留示例配置，发布时建议使用 `conf/*.ini.example` 并把敏感字段标注为 `REPLACE_ME`。
- 仓库包含生成与脱敏工具（`tools/`），生产测试请使用受控私有数据或自行准备数据集。

---

## 文档

- 测试与部署说明请参见： [KamaitachiGo/测试流程手册.md](KamaitachiGo/测试流程手册.md)
- 其它技术文档位于 `docs/archive/`

---


## 文档目录（docs/archive）

- [docs/archive/doc.md](docs/archive/doc.md)
- [docs/archive/PROJECT_STRUCTURE.md](docs/archive/PROJECT_STRUCTURE.md)
- [docs/archive/分布式部署指南.md](docs/archive/%E5%88%86%E5%B8%83%E5%BC%8F%E9%83%A8%E7%BD%B2%E6%8C%87%E5%8D%97.md)
- [docs/archive/测试流程手册.md](docs/archive/%E6%B5%8B%E8%AF%95%E6%B5%81%E7%A8%8B%E6%89%8B%E5%86%8C.md)
- [docs/archive/环境要求与依赖.md](docs/archive/%E7%8E%AF%E5%A2%83%E8%A6%81%E6%B1%82%E4%B8%8E%E4%BE%9D%E8%B5%96.md)
- [docs/archive/缓存机制与测试说明.md](docs/archive/%E7%BC%93%E5%AD%98%E6%9C%BA%E5%88%B6%E4%B8%8E%E6%B5%8B%E8%AF%95%E8%AF%B4%E6%98%8E.md)

如果你希望我把某些已删除的文档恢复到 `docs/`，请告诉我要恢复的文件名或把它们放回 `docs/`，我会更新目录并在 README 中链接。

---

## 构建与测试

```bash
go build ./...
go test ./...
go fmt ./...
go vet ./...
```

