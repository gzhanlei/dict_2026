package handler

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"dict_new/internal/service"
	"github.com/gin-gonic/gin"
)

// SearchRequest 搜索请求结构
type SearchRequest struct {
	Query    string `json:"query" binding:"required"`
	Type     string `json:"type" binding:"required"`     // names, places, both
	Language string `json:"language,omitempty"`          // zh, ja, en (可选，自动检测)
	NoCache  bool   `json:"noCache,omitempty"`           // 是否禁用缓存
}

// SearchResponse 搜索响应结构
type SearchResponse struct {
	Success bool                   `json:"success"`
	Data    *service.SearchResponse `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// SaveQueryLog 保存查询记录到数据库
func SaveQueryLog(services *service.Services, query, queryType, ip string) {
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

	sql := "INSERT INTO queries (query_word, query_time, ip, queryType) VALUES (?, ?, ?, ?)"
	_, err := services.DB.DB.Exec(sql, query, time.Now(), ip, dbQueryType)
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

		// 保存查询记录（异步，不阻塞主流程）
		go SaveQueryLog(services, req.Query, req.Type, GetClientIP(c, services.Config.Security.TrustedProxies))

		// 执行搜索，传递noCache参数
		result, err := services.Search.SearchWithOptions(req.Query, req.Type, req.NoCache)
		if err != nil {
			c.JSON(http.StatusInternalServerError, SearchResponse{
				Success: false,
				Error:   "Search failed",
			})
			return
		}

		c.JSON(http.StatusOK, SearchResponse{
			Success: true,
			Data:    result,
		})
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string `json:"status"`
	Database  string `json:"database"`
	Cache     string `json:"cache"`
	Timestamp int64  `json:"timestamp"`
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

// SetupRoutes 设置路由
func SetupRoutes(router *gin.Engine, services *service.Services) *gin.Engine {
	// API路由组
	api := router.Group("/api")
	{
		api.POST("/search", SearchHandler(services))
		api.GET("/health", HealthHandler(services))
	}

	// 静态文件服务
	router.Static("/static", "./static")
	
	// URL路由（必须在根路径之前）
	router.GET("/:type/:query", URLSearchHandler(services))
	
	// 根路径
	router.StaticFile("/", "./static/index.html")

	return router
}