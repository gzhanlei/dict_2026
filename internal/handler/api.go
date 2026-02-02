package handler

import (
	"net/http"

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

// SearchHandler 搜索处理器
func SearchHandler(services *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, SearchResponse{
				Success: false,
				Error:   "Invalid request format: " + err.Error(),
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

		// 执行搜索，传递noCache参数
		result, err := services.Search.SearchWithOptions(req.Query, req.Type, req.NoCache)
		if err != nil {
			c.JSON(http.StatusInternalServerError, SearchResponse{
				Success: false,
				Error:   "Search failed: " + err.Error(),
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
				response.Database = "error: " + err.Error()
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