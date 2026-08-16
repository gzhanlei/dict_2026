package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"dict_new/internal/handler"
	"dict_new/internal/middleware"
	"dict_new/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	// 设置日志
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 初始化服务
	services, err := service.NewServices()
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}
	defer services.Close()

	// 设置路由
	router := gin.Default()

	// 设置可信代理列表（空列表表示不信任任何代理）
	if err := router.SetTrustedProxies(services.Config.Security.TrustedProxies); err != nil {
		log.Fatalf("Failed to set trusted proxies: %v", err)
	}

	// 添加安全响应头中间件
	router.Use(middleware.SecurityHeaders())

	// 添加速率限制中间件（如果配置了）
	if services.Config.Security.RateLimit > 0 {
		router.Use(middleware.RateLimit(services.Config.Security.RateLimit))
	}

	handler.SetupRoutes(router, services)

	// 启动服务器
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("Visit http://localhost:%s to use the application", port)
	// 绑定 127.0.0.1，确保外部无法直接访问（应通过 Nginx/Apache 反向代理）
	// 显式设置超时，防止慢速/挂起连接耗尽资源
	server := &http.Server{
		Addr:         "127.0.0.1:" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}