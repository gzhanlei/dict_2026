package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders 添加安全响应头中间件
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 防止点击劫持
		c.Header("X-Frame-Options", "DENY")
		// 防止MIME类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")
		// XSS保护
		c.Header("X-XSS-Protection", "1; mode=block")
		// 强制HTTPS (生产环境应启用，开发环境可注释)
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		// 内容安全策略
		// challenges.cloudflare.com：Turnstile widget 的脚本/iframe/回调请求来源
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://challenges.cloudflare.com; "+
				"style-src 'self' 'unsafe-inline'; "+
				"frame-src https://challenges.cloudflare.com; "+
				"connect-src 'self' https://challenges.cloudflare.com")
		// referrer 策略
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}

// RateLimiter 速率限制器
type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
	stopChan chan struct{}
}

// NewRateLimiter 创建新的速率限制器
// limit: 在window时间内允许的最大请求数
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		stopChan: make(chan struct{}),
	}
	// 启动后台清理协程
	go rl.cleanupRoutine()
	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// 获取该IP的历史请求
	times := rl.requests[clientIP]

	// 过滤掉过期的请求
	var validTimes []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}

	// 检查是否超过限制
	if len(validTimes) >= rl.limit {
		rl.requests[clientIP] = validTimes
		return false
	}

	// 添加当前请求时间
	validTimes = append(validTimes, now)
	rl.requests[clientIP] = validTimes

	return true
}

// cleanupRoutine 定期清理过期的请求记录，防止内存泄漏
func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopChan:
			return
		}
	}
}

// cleanup 清理过期的请求记录
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	for ip, times := range rl.requests {
		var validTimes []time.Time
		for _, t := range times {
			if t.After(cutoff) {
				validTimes = append(validTimes, t)
			}
		}
		if len(validTimes) == 0 {
			delete(rl.requests, ip) // 删除无有效记录的IP
		} else {
			rl.requests[ip] = validTimes
		}
	}
}

// Stop 停止速率限制器
func (rl *RateLimiter) Stop() {
	close(rl.stopChan)
}

// RateLimit 速率限制中间件
// requestsPerMinute: 每分钟允许的请求数
func RateLimit(requestsPerMinute int) gin.HandlerFunc {
	limiter := NewRateLimiter(requestsPerMinute, time.Minute)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if !limiter.Allow(clientIP) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "Too many requests",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
