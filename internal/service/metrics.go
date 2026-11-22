package service

import "sync/atomic"

// SearchMetrics 搜索接口的累计计数器（原子操作，进程内），经 /api/health 暴露，
// 用于确认限流/缓存/闸门的实际效果（如缓存命中率、闸门拒绝量）
type SearchMetrics struct {
	Requests    atomic.Int64 // /api/search 总请求数
	CacheHits   atomic.Int64 // 缓存命中数
	DBSearches  atomic.Int64 // 未命中缓存、进入数据库的搜索数
	BusyRejects atomic.Int64 // 并发闸门拒绝数（429）
	DBErrors    atomic.Int64 // 数据库查询失败数
	SQLQueries  atomic.Int64 // 发往数据库的子查询条数
	Active      atomic.Int64 // 当前进行中的数据库搜索数（瞬时值）
}

// SearchMetricsSnapshot SearchMetrics 的只读快照（用于JSON输出）
type SearchMetricsSnapshot struct {
	Requests       int64 `json:"requests"`
	CacheHits      int64 `json:"cache_hits"`
	DBSearches     int64 `json:"db_searches"`
	BusyRejects    int64 `json:"busy_rejects"`
	DBErrors       int64 `json:"db_errors"`
	SQLQueries     int64 `json:"sql_queries"`
	ActiveSearches int64 `json:"active_searches"`
}

// Snapshot 返回当前计数快照
func (m *SearchMetrics) Snapshot() SearchMetricsSnapshot {
	return SearchMetricsSnapshot{
		Requests:       m.Requests.Load(),
		CacheHits:      m.CacheHits.Load(),
		DBSearches:     m.DBSearches.Load(),
		BusyRejects:    m.BusyRejects.Load(),
		DBErrors:       m.DBErrors.Load(),
		SQLQueries:     m.SQLQueries.Load(),
		ActiveSearches: m.Active.Load(),
	}
}
