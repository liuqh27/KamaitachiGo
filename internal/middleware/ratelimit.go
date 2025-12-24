package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow() bool
}

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	limiter *rate.Limiter
	mu      sync.RWMutex
}

// NewTokenBucketLimiter 创建令牌桶限流器
// qps: 每秒允许的请求数
// burst: 允许的突发请求数
func NewTokenBucketLimiter(qps int, burst int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		limiter: rate.NewLimiter(rate.Limit(qps), burst),
	}
}

// Allow 检查是否允许请求
func (l *TokenBucketLimiter) Allow() bool {
	return l.limiter.Allow()
}

// Wait 等待直到可以处理请求（暂未使用，预留接口）
func (l *TokenBucketLimiter) Wait() {
	// 预留接口，暂不实现
}

// UpdateLimit 动态更新限流配置
func (l *TokenBucketLimiter) UpdateLimit(qps int, burst int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limiter.SetLimit(rate.Limit(qps))
	l.limiter.SetBurst(burst)
}

// GlobalRateLimiterConfig 全局限流配置
type GlobalRateLimiterConfig struct {
	limiters map[string]*TokenBucketLimiter // 按路由分组的限流器
	mu       sync.RWMutex
}

var globalRateLimiter *GlobalRateLimiterConfig

// InitGlobalRateLimiter 初始化全局限流器
func InitGlobalRateLimiter() {
	globalRateLimiter = &GlobalRateLimiterConfig{
		limiters: make(map[string]*TokenBucketLimiter),
	}

	// 配置不同接口的限流
	// 场景1: 区间查询 - 300 QPS
	globalRateLimiter.AddLimiter("period", 300, 500)

	// 场景2: 最近N条 - 500 QPS
	globalRateLimiter.AddLimiter("recent", 500, 800)

	// 场景3: 多实体最新数据 - 10000 QPS（需要扩容）
	globalRateLimiter.AddLimiter("snapshot", 10000, 15000)

	// 场景4: 指定日期数据 - 300 QPS
	globalRateLimiter.AddLimiter("selection", 300, 500)

	// 默认限流 - 1000 QPS
	globalRateLimiter.AddLimiter("default", 1000, 1500)
}

// AddLimiter 添加限流器
func (g *GlobalRateLimiterConfig) AddLimiter(name string, qps int, burst int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.limiters[name] = NewTokenBucketLimiter(qps, burst)
}

// GetLimiter 获取限流器
func (g *GlobalRateLimiterConfig) GetLimiter(name string) *TokenBucketLimiter {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if limiter, ok := g.limiters[name]; ok {
		return limiter
	}
	// 返回默认限流器
	return g.limiters["default"]
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware() gin.HandlerFunc {
	// 确保全局限流器已初始化
	if globalRateLimiter == nil {
		InitGlobalRateLimiter()
	}

	return func(c *gin.Context) {
		// 根据路径确定使用哪个限流器
		limiterName := getLimiterNameByPath(c.Request.URL.Path)
		limiter := globalRateLimiter.GetLimiter(limiterName)

		// 检查是否允许请求
		if !limiter.Allow() {
			// 限流触发，返回429状态码
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "Too Many Requests - Rate limit exceeded",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// 请求通过限流
		c.Next()
	}
}

// getLimiterNameByPath 根据路径获取限流器名称
func getLimiterNameByPath(path string) string {
	switch {
	case contains(path, "/period"):
		return "period"
	case contains(path, "/snapshot"):
		return "snapshot"
	case contains(path, "/selection"):
		return "selection"
	case contains(path, "/recent"):
		return "recent"
	default:
		return "default"
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr || 
		   len(s) > len(substr) && s[:len(substr)] == substr ||
		   findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// IPRateLimiter IP级别的限流器
type IPRateLimiter struct {
	limiters map[string]*TokenBucketLimiter
	mu       sync.RWMutex
	qps      int
	burst    int
}

// NewIPRateLimiter 创建IP限流器
func NewIPRateLimiter(qps, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*TokenBucketLimiter),
		qps:      qps,
		burst:    burst,
	}
}

// GetLimiter 获取或创建IP对应的限流器
func (l *IPRateLimiter) GetLimiter(ip string) *TokenBucketLimiter {
	l.mu.RLock()
	limiter, exists := l.limiters[ip]
	l.mu.RUnlock()

	if exists {
		return limiter
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 双重检查
	if limiter, exists := l.limiters[ip]; exists {
		return limiter
	}

	limiter = NewTokenBucketLimiter(l.qps, l.burst)
	l.limiters[ip] = limiter
	return limiter
}

// IPRateLimitMiddleware IP级别限流中间件
func IPRateLimitMiddleware(qps, burst int) gin.HandlerFunc {
	limiter := NewIPRateLimiter(qps, burst)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		ipLimiter := limiter.GetLimiter(ip)

		if !ipLimiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "Too Many Requests - IP rate limit exceeded",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitStats 限流统计
type RateLimitStats struct {
	TotalRequests   int64 `json:"total_requests"`
	AllowedRequests int64 `json:"allowed_requests"`
	BlockedRequests int64 `json:"blocked_requests"`
	mu              sync.RWMutex
}

var rateLimitStats = &RateLimitStats{}

// RecordRequest 记录请求
func (s *RateLimitStats) RecordRequest(allowed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalRequests++
	if allowed {
		s.AllowedRequests++
	} else {
		s.BlockedRequests++
	}
}

// GetStats 获取统计信息
func (s *RateLimitStats) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"total_requests":   s.TotalRequests,
		"allowed_requests": s.AllowedRequests,
		"blocked_requests": s.BlockedRequests,
		"block_rate":       float64(s.BlockedRequests) / float64(s.TotalRequests) * 100,
	}
}

