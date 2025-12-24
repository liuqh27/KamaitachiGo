package main

import (
	"KamaitachiGo/internal/cache/lru"
	"KamaitachiGo/internal/cache/snapshot"
	"KamaitachiGo/internal/handler"
	"KamaitachiGo/internal/repository"
	"KamaitachiGo/internal/service"
	"KamaitachiGo/pkg/config"
	"KamaitachiGo/pkg/etcd"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	dbPath = flag.String("db", "./data/master.db", "Database file path")
)

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig("conf/master.ini")
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.Infof("Starting Kamaitachi Master Server on port %s", cfg.Server.Port)

	// 创建LRU缓存
	cache := lru.NewCache(cfg.Cache.MaxBytes, nil)
	logrus.Infof("LRU cache initialized with max bytes: %d", cfg.Cache.MaxBytes)

	// 创建快照管理器
	snapshotMgr := snapshot.NewManager(cache, cfg.Cache.SnapshotPath)

	// 尝试加载历史快照
	count, err := snapshotMgr.Load()
	if err != nil {
		logrus.Warnf("Failed to load snapshot: %v (This is normal for first run)", err)
	} else {
		logrus.Infof("Loaded %d entries from snapshot", count)
	}

	// 启动自动快照
	snapshotInterval := time.Duration(cfg.Cache.SnapshotInterval) * time.Minute
	snapshotMgr.AutoSnapshot(snapshotInterval)
	logrus.Infof("Auto snapshot enabled with interval: %v", snapshotInterval)

	// 创建SQLite仓库
	sqliteRepo, err := repository.NewSQLiteRepository(*dbPath)
	if err != nil {
		logrus.Fatalf("Failed to initialize SQLite repository: %v", err)
	}
	defer sqliteRepo.Close()
	logrus.Infof("SQLite repository initialized (DB: %s)", *dbPath)

	// 创建服务
	financeService := service.NewFinanceService(sqliteRepo, cfg.Cache.MaxBytes)

	// 预热缓存
	go func() {
		time.Sleep(2 * time.Second) // 等待服务启动
		if err := financeService.WarmupCache(); err != nil {
			logrus.Warnf("Cache warmup failed: %v", err)
		}
	}()

	// 兼容旧的服务（如果需要）
	memRepo := repository.NewMemoryRepository(cache)
	dataService := service.NewDataInfoService(memRepo)
	selectionService := service.NewSelectionService(memRepo)

	// 创建处理器
	financeHandler := handler.NewFinanceHandler(financeService)
	dataHandler := handler.NewDataHandler(dataService)
	selectionHandler := handler.NewSelectionHandler(selectionService)

	// 设置路由（使用新的finance API）
	router := setupRouter(financeHandler, dataHandler, selectionHandler)

	// 如果配置了etcd，注册服务
	if cfg.Etcd.Endpoints != "" {
		etcdClient, err := etcd.NewClient([]string{cfg.Etcd.Endpoints})
		if err != nil {
			logrus.Errorf("Failed to connect to etcd: %v", err)
		} else {
			// 注册服务到etcd
			serviceName := cfg.Server.ServiceName
			serviceAddr := cfg.Server.ServiceAddr
			err = etcdClient.Register(serviceName, serviceAddr, cfg.Etcd.TTL)
			if err != nil {
				logrus.Errorf("Failed to register service: %v", err)
			} else {
				logrus.Infof("Service registered to etcd: %s -> %s", serviceName, serviceAddr)
			}
			defer etcdClient.Close()
		}
	}

	// 启动HTTP服务器
	go func() {
		addr := ":" + cfg.Server.Port
		logrus.Infof("HTTP server listening on %s", addr)
		if err := router.Run(addr); err != nil {
			logrus.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")

	// 保存快照
	snapshotMgr.Stop()

	logrus.Info("Server stopped")
}

func setupRouter(financeHandler *handler.FinanceHandler, dataHandler *handler.DataHandler, selectionHandler *handler.SelectionHandler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	// 使用gin.New()而非Default()，关闭Logger提升性能
	r := gin.New()
	// 只保留Recovery中间件（错误恢复）
	r.Use(gin.Recovery())

	// 赛事方Finance API
	apiGroup := r.Group("/kamaitachi/api/data/v1")
	{
		apiGroup.POST("/snapshot", financeHandler.Snapshot)
		apiGroup.POST("/snapshot/", financeHandler.Snapshot)
		apiGroup.POST("/period", financeHandler.Period)
		apiGroup.POST("/period/", financeHandler.Period)
		apiGroup.GET("/stats", financeHandler.Stats)
	}

	// 数据管理接口
	dataGroup := r.Group("/data/v1")
	{
		dataGroup.POST("/search", dataHandler.Search)
		dataGroup.POST("/save", dataHandler.Save)
		dataGroup.GET("/get/:id", dataHandler.Get)
		dataGroup.DELETE("/delete/:id", dataHandler.Delete)
	}

	// 选股接口
	selectionGroup := r.Group("/kamaitachi/api/selection/v1")
	{
		selectionGroup.POST("/snapshot", selectionHandler.SelectionSnapshot)
		selectionGroup.POST("/snapshot/", selectionHandler.SelectionSnapshot)
		selectionGroup.POST("/period", selectionHandler.SelectionPeriod)
		selectionGroup.POST("/period/", selectionHandler.SelectionPeriod)
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
