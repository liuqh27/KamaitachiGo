# 项目结构详解

## 完整目录树

```
KamaitachiGo/
│
├── cmd/                          # 应用程序入口
│   ├── master/                   # 主节点服务（用于数据管理）
│   │   └── main.go              # 主节点启动文件
│   ├── slave/                    # 从节点服务（用于数据查询）
│   │   └── main.go              # 从节点启动文件
│   └── gateway/                  # 网关服务（用于请求路由）
│       └── main.go              # 网关启动文件
│
├── internal/                     # 内部实现（不对外暴露）
│   ├── cache/                   # 缓存层实现
│   │   ├── lru/                 # LRU缓存算法
│   │   │   └── lru.go          # LRU缓存核心实现
│   │   └── snapshot/            # 快照管理
│   │       └── manager.go       # 快照管理器
│   │
│   ├── model/                   # 数据模型定义
│   │   ├── common.go           # 公共模型（SecuritySubject, TableInfo等）
│   │   ├── datainfo.go         # 数据信息模型
│   │   ├── snapshot.go         # 快照请求/响应模型
│   │   └── period.go           # 区间请求/响应模型
│   │
│   ├── service/                 # 业务逻辑层
│   │   ├── datainfo_service.go # 数据信息服务
│   │   └── query_service.go    # 查询服务
│   │
│   ├── repository/              # 数据访问层
│   │   └── memory_repository.go # 内存存储仓库
│   │
│   └── handler/                 # HTTP处理器
│       ├── data_handler.go     # 数据管理接口
│       ├── query_handler.go    # 查询接口
│       └── router.go           # 路由配置
│
├── pkg/                          # 公共包（可对外暴露）
│   ├── config/                  # 配置管理
│   │   └── config.go           # 配置加载器
│   │
│   ├── etcd/                    # etcd客户端
│   │   └── etcd.go             # 服务注册与发现
│   │
│   ├── hash/                    # 一致性哈希
│   │   └── consistenthash.go   # 一致性哈希实现
│   │
│   └── common/                  # 通用工具
│       ├── byteview.go         # 字节视图
│       └── response.go         # HTTP响应结构
│
├── api/                          # API定义
│   └── proto/                   # protobuf定义（预留扩展）
│
├── conf/                         # 配置文件
│   ├── master.ini              # 主节点配置
│   ├── slave.ini               # 从节点配置
│   └── gateway.ini             # 网关配置
│
├── data/                         # 数据目录（运行时生成）
│   ├── master_snapshot.dat     # 主节点快照
│   └── slave_snapshot.dat      # 从节点快照
│
├── go.mod                        # Go模块定义
├── go.sum                        # 依赖版本锁定
├── .gitignore                    # Git忽略文件
├── Makefile                      # 构建脚本
├── README.md                     # 项目说明
├── EXAMPLE.md                    # 使用示例
├── doc.md                        # 技术文档
└── PROJECT_STRUCTURE.md          # 本文件
```

## 各模块详细说明

### 1. cmd/ - 应用程序入口

**职责**：定义不同类型服务的启动入口

#### cmd/master/main.go
- 启动主节点服务
- 初始化缓存和快照
- 注册到 etcd
- 提供数据管理接口

#### cmd/slave/main.go
- 启动从节点服务
- 初始化缓存和快照
- 注册到 etcd
- 提供数据查询接口

#### cmd/gateway/main.go
- 启动网关服务
- 连接 etcd 发现节点
- 实现请求路由和负载均衡
- 代理请求到后端节点

### 2. internal/ - 内部实现

**职责**：核心业务逻辑和数据处理

#### internal/cache/ - 缓存层

**lru/lru.go**
- LRU缓存算法实现
- 支持容量限制
- 支持数据过期
- 线程安全

**snapshot/manager.go**
- 快照保存和加载
- 定期自动保存
- JSON格式存储
- 原子性写入

#### internal/model/ - 数据模型

**datainfo.go**
- DataInfo: 核心数据结构
- DataInfoOption: 查询选项
- DataSet: 数据集合

**snapshot.go**
- SnapshotRequest: 快照请求
- SnapshotData: 快照响应

**period.go**
- PeriodRequest: 区间请求
- PeriodData: 区间响应

**common.go**
- SecuritySubject: 证券实体
- SubjectsGroup: 实体分组
- TableInfo: 表信息
- KamaitachiError: 错误类型

#### internal/service/ - 服务层

**datainfo_service.go**
- 数据保存、查询、删除
- 表信息管理
- 业务逻辑处理

**query_service.go**
- 快照查询
- 区间查询
- 实体解析和数据聚合

#### internal/repository/ - 数据访问层

**memory_repository.go**
- 内存数据存储
- 缓存操作封装
- 数据序列化/反序列化

#### internal/handler/ - HTTP处理器

**data_handler.go**
- POST /data/v1/save - 保存数据
- GET /data/v1/get/:id - 获取数据
- POST /data/v1/search - 搜索数据
- DELETE /data/v1/delete/:id - 删除数据

**query_handler.go**
- POST /data/v1/snapshot - 快照查询
- POST /data/v1/period - 区间查询

**router.go**
- 路由配置
- 中间件注册
- 健康检查接口

### 3. pkg/ - 公共包

**职责**：可复用的工具和组件

#### pkg/config/ - 配置管理

**config.go**
- 配置文件加载（INI格式）
- 配置结构定义
- 数据库DSN生成

#### pkg/etcd/ - etcd客户端

**etcd.go**
- 服务注册与心跳
- 服务发现
- 键值操作（Get/Put/Delete）
- 监听（Watch）

#### pkg/hash/ - 一致性哈希

**consistenthash.go**
- 一致性哈希环
- 虚拟节点
- 节点增删
- 负载均衡

#### pkg/common/ - 通用工具

**byteview.go**
- 只读字节视图
- 防止数据被意外修改

**response.go**
- 统一HTTP响应格式
- 成功/失败响应构建

### 4. api/ - API定义

**职责**：对外接口定义（预留扩展）

可以添加：
- protobuf 定义
- GraphQL schema
- OpenAPI 规范

### 5. conf/ - 配置文件

**职责**：不同环境的配置

#### master.ini
- 主节点配置
- 端口：8080
- 支持数据写入和读取

#### slave.ini
- 从节点配置
- 端口：8081+
- 只支持数据读取

#### gateway.ini
- 网关配置
- 端口：9000
- 请求路由和负载均衡

## 数据流向

### 写入流程

```
客户端
  ↓
HTTP Request (POST /data/v1/save)
  ↓
data_handler.Save()
  ↓
datainfo_service.Save()
  ↓
memory_repository.Save()
  ↓
lru.Cache.Add()
  ↓
内存缓存
  ↓
(定期) snapshot_manager.Save()
  ↓
磁盘快照文件
```

### 查询流程

```
客户端
  ↓
HTTP Request (POST /data/v1/snapshot)
  ↓
query_handler.Snapshot()
  ↓
query_service.Snapshot()
  ↓
memory_repository.Get()
  ↓
lru.Cache.Get()
  ↓
返回数据
```

### 网关路由流程

```
客户端
  ↓
HTTP Request -> Gateway (端口9000)
  ↓
解析请求参数
  ↓
一致性哈希计算
  ↓
选择目标节点
  ↓
代理请求到节点
  ↓
返回结果给客户端
```

## 扩展点

### 1. 数据库持久化

在 `internal/repository/` 下添加：
```
mysql_repository.go
postgresql_repository.go
```

### 2. gRPC 支持

在 `api/proto/` 下添加：
```
kamaitachi.proto
```

在 `internal/handler/` 下添加：
```
grpc_handler.go
```

### 3. 监控指标

在 `pkg/` 下添加：
```
pkg/metrics/
  └── prometheus.go
```

### 4. 认证授权

在 `pkg/` 下添加：
```
pkg/auth/
  ├── jwt.go
  └── middleware.go
```

## 依赖关系

```
cmd/*
  └─> internal/handler
        └─> internal/service
              └─> internal/repository
                    └─> internal/cache
                          └─> internal/model

cmd/*
  └─> pkg/config
  └─> pkg/etcd
  └─> pkg/hash
```

## 接口设计原则

1. **分层清晰**：handler -> service -> repository -> cache
2. **依赖倒置**：上层依赖接口，不依赖具体实现
3. **单一职责**：每个模块职责明确
4. **可扩展性**：预留扩展接口

## 命名规范

1. **包名**：小写，简短，有意义
2. **文件名**：小写+下划线，如 `data_handler.go`
3. **类型名**：大驼峰，如 `DataInfo`
4. **方法名**：大驼峰（公开），小驼峰（私有）
5. **接口名**：-er 结尾，如 `DataInfoService`


## 总结

本项目采用清晰的分层架构，模块职责明确，易于理解和扩展。通过合理的目录组织和接口设计，可以方便地添加新功能或替换底层实现。

