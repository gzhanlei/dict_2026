package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"dict_new/internal/service"
	"github.com/gin-gonic/gin"
)

// SearchRequest 搜索请求结构
type SearchRequest struct {
	Query           string `json:"query" binding:"required"`
	Type            string `json:"type" binding:"required"`     // names, places, both
	Language        string `json:"language,omitempty"`          // zh, ja, en (可选，自动检测)
	NoCache         bool   `json:"noCache,omitempty"`           // 是否禁用缓存
	TurnstileToken  string `json:"cf-turnstile-response"`       // Cloudflare Turnstile token（启用人机校验时必填）
}

// SearchResponse 搜索响应结构
type SearchResponse struct {
	Success bool                   `json:"success"`
	Data    *service.SearchResponse `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// SaveQueryLog 保存查询记录到数据库
func SaveQueryLog(services *service.Services, query, queryType, ip string) {
	// 异步执行时不受 Gin Recovery 保护，防止 panic 导致进程崩溃
	defer func() {
		_ = recover()
	}()

	// 将内部类型转换为数据库存储格式
	var dbQueryType string
	switch queryType {
	case "names":
		dbQueryType = "person"
	case "places":
		dbQueryType = "place"
	default:
		dbQueryType = "both" // 未知类型默认为both
	}

	// 数据库写入加超时，避免连接被长时间占用导致连接池耗尽
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sql := "INSERT INTO queries (query_word, query_time, ip, queryType) VALUES (?, ?, ?, ?)"
	_, err := services.DB.DB.ExecContext(ctx, sql, query, time.Now(), ip, dbQueryType)
	if err != nil {
		// 记录失败不影响主流程，仅记录错误
		// 实际项目中可以使用日志库
		_ = err
	}
}

// GetClientIP 获取客户端IP地址
// 仅在配置了可信代理时信任 X-Forwarded-For 头
func GetClientIP(c *gin.Context, trustedProxies []string) string {
	clientIP := c.ClientIP()

	// 如果没有配置可信代理，直接返回 Gin 解析的 IP（不信任代理头）
	if len(trustedProxies) == 0 {
		return clientIP
	}

	// 检查请求是否来自可信代理
	isTrusted := false
	for _, proxy := range trustedProxies {
		if proxy == "0.0.0.0/0" || clientIP == proxy {
			isTrusted = true
			break
		}
	}

	// 如果请求来自可信代理，尝试从 X-Forwarded-For 获取真实 IP
	if isTrusted {
		// X-Forwarded-For 格式: client, proxy1, proxy2
		// 取第一个有效 IP
		forwardedFor := c.GetHeader("X-Forwarded-For")
		if forwardedFor != "" {
			// 按逗号分割，取第一个（最原始的客户端IP）
			parts := strings.Split(forwardedFor, ",")
			if len(parts) > 0 {
				realIP := strings.TrimSpace(parts[0])
				if realIP != "" {
					return realIP
				}
			}
		}

		// 尝试 X-Real-IP
		realIP := c.GetHeader("X-Real-IP")
		if realIP != "" {
			return realIP
		}
	}

	return clientIP
}

// SearchHandler 搜索处理器
func SearchHandler(services *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, SearchResponse{
				Success: false,
				Error:   "Invalid request format",
			})
			return
		}

		// 验证搜索类型
		if req.Type != "names" && req.Type != "places" && req.Type != "both" {
			c.JSON(http.StatusBadRequest, SearchResponse{
				Success: false,
				Error:   "Invalid search type. Must be 'names', 'places', or 'both'",
			})
			return
		}

		// 验证查询词（至少2个非空格字符）
		if utf8.RuneCountInString(strings.TrimSpace(req.Query)) < 2 {
			c.JSON(http.StatusBadRequest, SearchResponse{
				Success: false,
				Error:   "Query must be at least 2 characters",
			})
			return
		}

		// 限制查询词最大长度，防止超长输入导致分词爆炸和资源耗尽
		if utf8.RuneCountInString(req.Query) > 100 {
			c.JSON(http.StatusBadRequest, SearchResponse{
				Success: false,
				Error:   "Query is too long. Maximum 100 characters",
			})
			return
		}

		// 保存查询记录（异步，不阻塞主流程）
		clientIP := GetClientIP(c, services.Config.Security.TrustedProxies)

		// Turnstile 人机校验网关：启用时在查库与查询日志之前先校验，
		// 爬虫/无token请求在此被拦截，不消耗数据库资源
		if services.Turnstile != nil && services.Turnstile.Enforced() {
			if err := services.Turnstile.Verify(c.Request.Context(), req.TurnstileToken, clientIP); err != nil {
				log.Printf("turnstile ip=%s type=%s q=%q reject=%v", clientIP, req.Type, req.Query, err)
				c.JSON(http.StatusForbidden, SearchResponse{
					Success: false,
					Error:   "人机验证失败，请刷新页面后重试",
				})
				return
			}
		}

		go SaveQueryLog(services, req.Query, req.Type, clientIP)

		// 执行搜索，传递noCache参数；记录耗时与来源，便于排查爬虫/慢查询
		start := time.Now()
		result, err := services.Search.SearchWithOptions(c.Request.Context(), req.Query, req.Type, req.NoCache, clientIP)
		duration := time.Since(start)

		if err != nil {
			// 并发闸门拒绝：返回429，让爬虫退避而非堆积
			if errors.Is(err, service.ErrSearchBusy) {
				log.Printf("search ip=%s type=%s q=%q dur=%dms busy_reject", clientIP, req.Type, req.Query, duration.Milliseconds())
				c.JSON(http.StatusTooManyRequests, SearchResponse{
					Success: false,
					Error:   "Too many concurrent searches, please try again later",
				})
				return
			}
			log.Printf("search ip=%s type=%s q=%q dur=%dms error=%v", clientIP, req.Type, req.Query, duration.Milliseconds(), err)
			c.JSON(http.StatusInternalServerError, SearchResponse{
				Success: false,
				Error:   "Search failed",
			})
			return
		}

		log.Printf("search ip=%s type=%s lang=%s q=%q dur=%dms cache=%v parts=%d names=%d places=%d",
			clientIP, req.Type, result.Language, req.Query, duration.Milliseconds(),
			result.Cache, len(result.Tabs), len(result.Names), len(result.Places))

		c.JSON(http.StatusOK, SearchResponse{
			Success: true,
			Data:    result,
		})
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string                           `json:"status"`
	Database  string                           `json:"database"`
	Cache     string                           `json:"cache"`
	Timestamp int64                            `json:"timestamp"`
	Metrics   *service.SearchMetricsSnapshot   `json:"metrics,omitempty"`
}

// HealthHandler 健康检查处理器
func HealthHandler(services *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		response := HealthResponse{
			Status:    "ok",
			Timestamp: 0, // 这里可以添加实际的时间戳
		}

		// 检查数据库连接
		if services.DB != nil {
			if err := services.DB.DB.Ping(); err != nil {
				response.Database = "error"
				response.Status = "degraded"
			} else {
				response.Database = "ok"
			}
		} else {
			response.Database = "not initialized"
			response.Status = "degraded"
		}

		// 检查缓存服务
		if services.Cache != nil {
			response.Cache = "ok"
		} else {
			response.Cache = "not initialized"
			response.Status = "degraded"
		}

		// 附加搜索计数器（累计值，用于观测限流/缓存/闸门效果）
		if services.Search != nil {
			snapshot := services.Search.Metrics()
			response.Metrics = &snapshot
		}

		statusCode := http.StatusOK
		if response.Status != "ok" {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, response)
	}
}

// URLSearchHandler 处理 /:type/:query 路由
func URLSearchHandler(services *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		searchType := c.Param("type")
		_ = c.Param("query") // 路由需要，具体值由前端从 URL 读取
		// 验证搜索类型
		if searchType != "names" && searchType != "places" && searchType != "both" && 
		   searchType != "person" && searchType != "place" {
			c.HTML(http.StatusBadRequest, "index.html", nil)
			return
		}
		
		// 标准化类型名称
		if searchType == "person" {
			searchType = "names"
		} else if searchType == "place" {
			searchType = "places"
		}
		_ = c.Query("cache") // cache=no 由前端从 URL 读取
		// 返回HTML页面，前端JavaScript会处理搜索
		c.File("./static/index.html")
	}
}

// PublicConfigResponse /api/config 响应（仅公开值，不含任何 secret）
type PublicConfigResponse struct {
	Success   bool                          `json:"success"`
	Turnstile *service.PublicConfig         `json:"turnstile"`
}

// ConfigHandler 前端启动时获取公开运行配置（Turnstile site key 等）
func ConfigHandler(services *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp := PublicConfigResponse{Success: true}
		if services.Turnstile != nil {
			cfg := services.Turnstile.GetPublicConfig()
			resp.Turnstile = &cfg
		}
		c.JSON(http.StatusOK, resp)
	}
}

// SetupRoutes 设置路由
func SetupRoutes(router *gin.Engine, services *service.Services) *gin.Engine {
	// API路由组
	api := router.Group("/api")
	{
		api.POST("/search", SearchHandler(services))
		api.GET("/health", HealthHandler(services))
		api.GET("/config", ConfigHandler(services))
	}

	// 静态文件服务
	router.Static("/static", "./static")
	
	// URL路由（必须在根路径之前）
	router.GET("/:type/:query", URLSearchHandler(services))
	
	// 根路径
	router.StaticFile("/", "./static/index.html")

	return router
}