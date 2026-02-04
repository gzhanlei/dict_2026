package main

import (
	"log"
	"os"

	"dict_new/internal/handler"
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
	handler.SetupRoutes(router, services)

	// 启动服务器
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("Visit http://localhost:%s to use the application", port)
	if err := router.Run("127.0.0.1:" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}