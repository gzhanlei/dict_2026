package service

import (
	"errors"
	"fmt"
	"sync"
)

// ErrSearchBusy 并发闸门拒绝（全局或单IP额度已满），handler 应映射为 HTTP 429
var ErrSearchBusy = errors.New("search busy")

// 默认并发上限（config 未配置时生效）
const (
	defaultMaxConcurrentSearches = 4
	defaultMaxConcurrentPerIP    = 1
)

// SearchGuard 限制同时进行中的"未命中缓存"搜索数量，保护共享数据库
// 不被并发全表扫描拖垮。全局额度防止任意来源的并发扫描叠加，
// 单IP额度防止单一来源（爬虫）同时发起多条慢查询占满额度。
type SearchGuard struct {
	global     chan struct{}
	perIPLimit int
	mu         sync.Mutex
	perIP      map[string]int
}

// NewSearchGuard 创建并发闸门；globalLimit/perIPLimit <=0 时使用默认值
func NewSearchGuard(globalLimit, perIPLimit int) *SearchGuard {
	if globalLimit <= 0 {
		globalLimit = defaultMaxConcurrentSearches
	}
	if perIPLimit <= 0 {
		perIPLimit = defaultMaxConcurrentPerIP
	}
	return &SearchGuard{
		global:     make(chan struct{}, globalLimit),
		perIPLimit: perIPLimit,
		perIP:      make(map[string]int),
	}
}

// TryAcquire 非阻塞获取并发额度，成功后必须调用 Release；额度已满返回 ErrSearchBusy
func (g *SearchGuard) TryAcquire(clientIP string) error {
	select {
	case g.global <- struct{}{}:
	default:
		return fmt.Errorf("全局并发搜索已达上限 %d: %w", cap(g.global), ErrSearchBusy)
	}

	g.mu.Lock()
	if g.perIP[clientIP] >= g.perIPLimit {
		g.mu.Unlock()
		<-g.global
		return fmt.Errorf("IP %s 并发搜索已达上限 %d: %w", clientIP, g.perIPLimit, ErrSearchBusy)
	}
	g.perIP[clientIP]++
	g.mu.Unlock()
	return nil
}

// Release 释放全局与单IP额度；必须与成功的 TryAcquire 一一配对
func (g *SearchGuard) Release(clientIP string) {
	g.mu.Lock()
	if n := g.perIP[clientIP]; n <= 1 {
		delete(g.perIP, clientIP)
	} else {
		g.perIP[clientIP] = n - 1
	}
	g.mu.Unlock()
	<-g.global
}
