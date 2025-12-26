# 项目结构详解 v1.1

本文档详细描述了 KamaitachiGo 项目的目录结构、模块职责以及核心数据流。

---

## 完整目录树

```
KamaitachiGo/
│
├── cmd/                          # 应用程序入口
│   ├── master/                   # 主节点服务 (用于集群管理)
│   │   └── main.go
│   ├── slave/                    # 从节点服务 (核心工作节点)
│   │   └── main.go
│   └── gateway/                  # 网关服务 (请求路由与分发)
│       └── main.go
│
├── internal/                     # 内部实现 (项目核心逻辑)
│   ├── cache/                    # 缓存层实现
│   │   ├── lru/                  # LRU缓存算法
│   │   │   └── lru.go
│   │   └── snapshot/             # 缓存快照管理
│   │       └── manager.go
│   │
│   ├── model/                    # 数据模型定义
│   │   ├── api_models.go         # API请求/响应模型 (SnapshotRequest, PeriodRequest等)
│   │   ├── common.go             # 通用数据结构
│   │   └── ...
│   │
│   ├── service/                  # 业务逻辑层
│   │   ├── finance_service.go    # 核心金融数据服务, 包含二级缓存逻辑
│   │   └── ...
│   │
│   ├── repository/               # 数据访问层
│   │   ├── sqlite_repository.go  # SQLite数据库仓库实现
│   │   └── memory_repository.go  # (旧) 内存存储仓库
│   │
│   └── handler/                  # HTTP处理器 (胶水层)
│       ├── finance_handler.go    # 金融数据API处理器
│       └── ...
│
├── pkg/                          # 公共包 (可被外部项目复用)
│   ├── config/                   # 配置管理
│   │   └── config.go
│   ├── etcd/                     # etcd客户端 (服务注册与发现)
│   │   └── etcd.go
│   └── hash/                     # 一致性哈希
│       └── consistenthash.go
│
├── api/                          # API定义 (预留扩展)
│   └── proto/
│
├── conf/                         # 配置文件
│   ├── master.ini, slave1.ini, slave2.ini, gateway.ini
│
├── data/                         # 数据目录 (运行时生成)
│
├── scripts/                      # 自动化脚本
│   ├── start_cluster.ps1         # (推荐) 一键构建并启动整个集群
│   └── demo.ps1                  # 用于演示特定场景的脚本
│
├── tools/                        # 开发与测试工具
│   ├── benchmark_scenarios.go    # 场景化压测工具
│   └── generate_sample_db.go     # 示例数据库生成工具
│
├── go.mod, go.sum, Makefile, README.md, ...
```

## 各模块详细说明

### 1. `cmd/` - 应用程序入口
- **职责**: 定义三个核心服务（Master, Slave, Gateway）的启动入口。每个 `main.go` 文件负责初始化依赖、加载配置、设置路由并启动HTTP服务。

### 2. `internal/` - 内部核心逻辑
- **职责**: 实现项目所有不对外的核心业务逻辑。这是项目的心脏。
- **`handler/`**: 作为胶水层，负责解析 HTTP 请求，调用 `service` 层处理，并返回 HTTP 响应。`finance_handler.go` 是我们API的主要处理器。
- **`service/`**: 业务逻辑的核心。`finance_service.go` 实现了查询逻辑，并包含了关键的**“数据感知”二级缓存 (`StockDataMap`)** 机制。
- **`repository/`**: 数据访问层。`sqlite_repository.go` 负责与 SQLite 数据库进行交互，执行实际的 SQL 查询。
- **`cache/`**: 通用缓存组件。`lru/lru.go` 提供了线程安全的 LRU 缓存实现。
- **`model/`**: 定义项目中使用的数据结构，`api_models.go` 中定义了所有对外API的请求和响应体。

### 3. `pkg/` - 公共包
- **职责**: 提供可在项目内外复用的通用功能库。
- **`etcd/`**: 封装了与 etcd 的交互，用于服务注册和发现。Gateway 用它来感知 Slave 节点的变化。
- **`hash/`**: 实现了**一致性哈希算法**，是 Gateway 实现请求稳定路由的核心。
- **`config/`**: 提供加载 `.ini` 格式配置文件的能力。

### 4. `scripts/` & `tools/` - 脚本与工具
- **`scripts/`**: 存放自动化运维脚本。`start_cluster.ps1` 是最重要的脚本，能够一键完成环境清理、编译、启动和健康检查。
- **`tools/`**: 存放独立的辅助程序。`benchmark_scenarios.go` 是我们用来量化性能、验证优化的强大压测工具。

---

## 核心数据流 (查询流程)

```mermaid
graph TD
    A[Client] -- HTTP Request --> B(Gateway:9000);
    B -- 1. 计算'subjects'的哈希 --> C{Consistent Hash Ring};
    C -- 2. 选取目标Slave --> B;
    B -- 3. 代理请求 --> D[Slave (e.g., :8081)];
    
    subgraph Slave 内部处理流程
        D -- 4. 解析HTTP请求 --> E[finance_handler];
        E -- 5. 调用服务 --> F[finance_service];
        F -- 6. 查询L2缓存 (StockDataMap) --> G{L2 Cache (LRU)};
        G -- 7a. 缓存命中 --> F;
        F -- 8a. 直接返回数据 --> E;
        
        G -- 7b. 缓存未命中 --> H[sqlite_repository];
        H -- 8b. 查询SQLite --> I[SQLite DB];
        I -- 9. 返回数据 --> H;
        H -- 10. 返回数据 --> F;
        F -- 11. 更新L2缓存 --> G;
        F -- 12. 返回数据 --> E;
        
        E -- 13. 封装HTTP响应 --> D;
    end
    
    D -- 14. 返回响应 --> B;
    B -- 15. 返回响应 --> A;

```
**流程说明**:
1. 客户端的所有请求都发送到**Gateway**。
2. Gateway 使用**一致性哈希**算法，根据请求的`subjects`（股票代码）决定将请求转发给哪个**Slave**。
3. Slave 内部的 `finance_handler` 接收到请求，并将其传递给 `finance_service`。
4. `finance_service` 首先检查其**二级缓存 (`StockDataMap`)**。
   - 如果**命中**，则直接从内存中返回数据，路径极短，性能极高。
   - 如果**未命中**，则调用 `sqlite_repository` 从数据库查询数据，然后将查询结果**填充到二级缓存**中，再返回给客户端。这样，下一次对该股票的请求就能直接命中缓存了。

---
**最后更新**: 2025-12-26  
**版本**: v1.1

