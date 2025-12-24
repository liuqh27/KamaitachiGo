package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"KamaitachiGo/internal/handler"
	"KamaitachiGo/internal/middleware"
	"KamaitachiGo/internal/repository"
	"KamaitachiGo/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	dbPath    = flag.String("db", "./data/finance_test.db", "SQLite database path")
	port      = flag.Int("port", 8080, "Server port")
	cacheSize = flag.Int64("cache", 2*1024*1024*1024, "LRU cache size in bytes")
	debug     = flag.Bool("debug", false, "Enable debug logging")
)

func main() {
	flag.Parse()

	// 设置日志级别
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logrus.Info("===========================================")
	logrus.Info("  Kamaitachi Finance Data Service")
	logrus.Info("===========================================")
	logrus.Infof("Database: %s", *dbPath)
	logrus.Infof("Port: %d", *port)
	logrus.Infof("Cache Size: %.2f GB", float64(*cacheSize)/(1024*1024*1024))

	// 检查数据库文件是否存在
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		log.Fatalf("Database file not found: %s", *dbPath)
	}

	// 初始化Repository
	repo, err := repository.NewSQLiteRepository(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.Close()
	logrus.Info("Repository initialized")

	// 初始化Service
	financeService := service.NewFinanceService(repo, *cacheSize)
	logrus.Info("Service initialized")

	// 预热缓存
	go func() {
		time.Sleep(2 * time.Second) // 等待服务启动
		if err := financeService.WarmupCache(); err != nil {
			logrus.Warnf("Cache warmup failed: %v", err)
		}
	}()

	// 初始化Handler
	financeHandler := handler.NewFinanceHandler(financeService)
	logrus.Info("Handler initialized")

	// 初始化路由
	router := setupRouter(financeHandler)
	logrus.Info("Router initialized")

	// 启动服务器
	addr := fmt.Sprintf(":%d", *port)
	logrus.Infof("Starting server on %s", addr)
	logrus.Info("===========================================")

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func setupRouter(financeHandler *handler.FinanceHandler) *gin.Engine {
	// 设置Gin模式
	if !*debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 初始化限流器和熔断器
	middleware.InitGlobalRateLimiter()
	middleware.InitGlobalCircuitBreaker()
	logrus.Info("Middleware initialized (rate limiter & circuit breaker)")

	// 应用全局中间件
	r.Use(middleware.RateLimitMiddleware())
	r.Use(middleware.CircuitBreakerMiddleware())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// 赛事方API接口
	apiGroup := r.Group("/kamaitachi/api/data/v1")
	{
		// 快照查询
		apiGroup.POST("/snapshot", financeHandler.Snapshot)
		apiGroup.POST("/snapshot/", financeHandler.Snapshot)

		// 区间查询
		apiGroup.POST("/period", financeHandler.Period)
		apiGroup.POST("/period/", financeHandler.Period)

		// 统计信息
		apiGroup.GET("/stats", financeHandler.Stats)
	}

	// 监控接口
	monitorGroup := r.Group("/monitor")
	{
		// 熔断器状态
		monitorGroup.GET("/circuitbreaker", func(c *gin.Context) {
			stats := middleware.GetAllCircuitBreakerStats()
			c.JSON(200, gin.H{
				"code":    200,
				"message": "success",
				"data":    stats,
			})
		})

		// 系统状态
		monitorGroup.GET("/status", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"circuit_breaker": middleware.GetAllCircuitBreakerStats(),
				},
			})
		})
	}

	return r
}
