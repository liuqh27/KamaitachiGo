package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// CircuitState 熔断器状态
type CircuitState int

const (
	StateClosed   CircuitState = iota // 关闭状态（正常）
	StateOpen                          // 打开状态（熔断）
	StateHalfOpen                      // 半开状态（探测）
)

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	MaxRequests       uint32        // 半开状态允许的最大请求数
	Interval          time.Duration // 统计时间窗口
	Timeout           time.Duration // 熔断超时时间
	FailureThreshold  float64       // 失败率阈值
	MinRequestCount   uint32        // 最小请求数（低于此数不熔断）
	SuccessThreshold  uint32        // 半开状态连续成功次数阈值
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxRequests:      10,              // 半开状态允许10个请求
		Interval:         10 * time.Second, // 10秒统计窗口
		Timeout:          30 * time.Second, // 30秒后尝试恢复
		FailureThreshold: 0.5,              // 50%失败率触发熔断
		MinRequestCount:  10,               // 至少10个请求才判断
		SuccessThreshold: 5,                // 连续5次成功则恢复
	}
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	name   string
	config *CircuitBreakerConfig
	state  CircuitState
	counts *Counts
	mu     sync.RWMutex

	// 状态转换时间
	stateChangedAt time.Time
}

// Counts 统计计数
type Counts struct {
	Requests       uint32    // 总请求数
	Successes      uint32    // 成功数
	Failures       uint32    // 失败数
	ConsecutiveSuccesses uint32    // 连续成功数
	ConsecutiveFails     uint32    // 连续失败数
	LastResetTime  time.Time // 上次重置时间
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(name string, config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		name:           name,
		config:         config,
		state:          StateClosed,
		counts:         &Counts{LastResetTime: time.Now()},
		stateChangedAt: time.Now(),
	}
}

// Call 执行请求
func (cb *CircuitBreaker) Call(fn func() error) error {
	// 检查是否允许请求
	if !cb.allowRequest() {
		return ErrCircuitBreakerOpen
	}

	// 执行请求
	err := fn()

	// 记录结果
	cb.recordResult(err == nil)

	return err
}

// allowRequest 检查是否允许请求
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否超时，是否应该转为半开状态
		return cb.shouldAttemptReset()
	case StateHalfOpen:
		// 半开状态限制请求数
		cb.mu.RLock()
		defer cb.mu.RUnlock()
		return cb.counts.Requests < cb.config.MaxRequests
	}

	return false
}

// recordResult 记录请求结果
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 检查是否需要重置统计
	if time.Since(cb.counts.LastResetTime) > cb.config.Interval {
		cb.resetCounts()
	}

	cb.counts.Requests++

	if success {
		cb.counts.Successes++
		cb.counts.ConsecutiveSuccesses++
		cb.counts.ConsecutiveFails = 0

		// 半开状态下，连续成功达到阈值则恢复
		if cb.state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= cb.config.SuccessThreshold {
			cb.setState(StateClosed)
			logrus.Infof("[CircuitBreaker] %s recovered to CLOSED state", cb.name)
		}
	} else {
		cb.counts.Failures++
		cb.counts.ConsecutiveFails++
		cb.counts.ConsecutiveSuccesses = 0

		// 检查是否应该触发熔断
		if cb.shouldTrip() {
			cb.setState(StateOpen)
			logrus.Warnf("[CircuitBreaker] %s tripped to OPEN state", cb.name)
		}
	}
}

// shouldTrip 判断是否应该触发熔断
func (cb *CircuitBreaker) shouldTrip() bool {
	// 请求数不足，不触发
	if cb.counts.Requests < cb.config.MinRequestCount {
		return false
	}

	// 计算失败率
	failureRate := float64(cb.counts.Failures) / float64(cb.counts.Requests)
	return failureRate >= cb.config.FailureThreshold
}

// shouldAttemptReset 判断是否应该尝试恢复
func (cb *CircuitBreaker) shouldAttemptReset() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 检查是否超过熔断超时时间
	if time.Since(cb.stateChangedAt) > cb.config.Timeout {
		cb.setState(StateHalfOpen)
		logrus.Infof("[CircuitBreaker] %s changed to HALF_OPEN state", cb.name)
		return true
	}

	return false
}

// setState 设置状态
func (cb *CircuitBreaker) setState(state CircuitState) {
	cb.state = state
	cb.stateChangedAt = time.Now()
	cb.resetCounts()
}

// resetCounts 重置计数
func (cb *CircuitBreaker) resetCounts() {
	cb.counts = &Counts{LastResetTime: time.Now()}
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 获取统计信息
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stateStr := "CLOSED"
	switch cb.state {
	case StateOpen:
		stateStr = "OPEN"
	case StateHalfOpen:
		stateStr = "HALF_OPEN"
	}

	failureRate := 0.0
	if cb.counts.Requests > 0 {
		failureRate = float64(cb.counts.Failures) / float64(cb.counts.Requests) * 100
	}

	return map[string]interface{}{
		"name":              cb.name,
		"state":             stateStr,
		"requests":          cb.counts.Requests,
		"successes":         cb.counts.Successes,
		"failures":          cb.counts.Failures,
		"failure_rate":      failureRate,
		"consecutive_successes": cb.counts.ConsecutiveSuccesses,
		"consecutive_fails": cb.counts.ConsecutiveFails,
		"state_changed_at":  cb.stateChangedAt.Format("2006-01-02 15:04:05"),
	}
}

// 错误定义
var ErrCircuitBreakerOpen = &CircuitBreakerError{Message: "circuit breaker is open"}

type CircuitBreakerError struct {
	Message string
}

func (e *CircuitBreakerError) Error() string {
	return e.Message
}

// GlobalCircuitBreakerManager 全局熔断器管理器
type GlobalCircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

var globalCircuitBreakerManager *GlobalCircuitBreakerManager

// InitGlobalCircuitBreaker 初始化全局熔断器
func InitGlobalCircuitBreaker() {
	globalCircuitBreakerManager = &GlobalCircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}

	// 为不同的服务创建熔断器
	config := DefaultCircuitBreakerConfig()

	globalCircuitBreakerManager.AddBreaker("period", config)
	globalCircuitBreakerManager.AddBreaker("snapshot", config)
	globalCircuitBreakerManager.AddBreaker("selection", config)
	globalCircuitBreakerManager.AddBreaker("default", config)
}

// AddBreaker 添加熔断器
func (m *GlobalCircuitBreakerManager) AddBreaker(name string, config *CircuitBreakerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.breakers[name] = NewCircuitBreaker(name, config)
}

// GetBreaker 获取熔断器
func (m *GlobalCircuitBreakerManager) GetBreaker(name string) *CircuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if breaker, ok := m.breakers[name]; ok {
		return breaker
	}
	return m.breakers["default"]
}

// CircuitBreakerMiddleware 熔断器中间件
func CircuitBreakerMiddleware() gin.HandlerFunc {
	// 确保全局熔断器已初始化
	if globalCircuitBreakerManager == nil {
		InitGlobalCircuitBreaker()
	}

	return func(c *gin.Context) {
		// 根据路径获取熔断器
		breakerName := getLimiterNameByPath(c.Request.URL.Path)
		breaker := globalCircuitBreakerManager.GetBreaker(breakerName)

		// 检查熔断器状态
		if !breaker.allowRequest() {
			logrus.Warnf("[CircuitBreaker] Request blocked by circuit breaker: %s", breakerName)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    503,
				"message": "Service Unavailable - Circuit breaker is open",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// 执行请求
		c.Next()

		// 记录结果（根据HTTP状态码判断成功失败）
		success := c.Writer.Status() < 500
		breaker.recordResult(success)
	}
}

// GetAllStats 获取所有熔断器统计
func GetAllCircuitBreakerStats() map[string]interface{} {
	if globalCircuitBreakerManager == nil {
		return nil
	}

	globalCircuitBreakerManager.mu.RLock()
	defer globalCircuitBreakerManager.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, breaker := range globalCircuitBreakerManager.breakers {
		stats[name] = breaker.GetStats()
	}

	return stats
}

