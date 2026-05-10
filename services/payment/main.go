package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/gin-gonic/gin"
)

// PayRequest 支付请求
type PayRequest struct {
	OrderID int64   `json:"order_id" binding:"required"`
	Amount  float64 `json:"amount" binding:"required"`
}

// Config 配置结构体
type Config struct {
	Port      string
	LogLevel  string
}

// loadConfig 加载配置
func loadConfig() *Config {
	return &Config{
		Port:     getEnv("PORT", "8083"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

// getEnv 获取环境变量，没有则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func main() {
	config := loadConfig()
	
	r := gin.Default()

	// 健康检查
	r.GET("/health", healthCheck)

	// 支付相关路由
	paymentGroup := r.Group("/api/v1/payment")
	{
		paymentGroup.POST("/pay", pay)
		paymentGroup.POST("/callback", callback)
	}

	// 启动服务
	log.Printf("支付服务启动，端口:%s\n", config.Port)
	server := &http.Server{
		Addr:    ":" + config.Port,
		Handler: r,
	}

	// 优雅关闭
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("服务正在关闭...")

	// 5秒超时关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("服务强制关闭: %v", err)
	}

	log.Println("服务已关闭")
}

// pay 发起支付
func pay(c *gin.Context) {
	var req PayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	// 这里简化处理，实际应该调用第三方支付接口
	// 比如微信支付、支付宝支付等

	orderIDStr := strconv.FormatInt(req.OrderID, 10)
	log.Printf("发起支付，订单ID: %s, 金额: %.2f\n", orderIDStr, req.Amount)

	// 返回支付链接或二维码信息
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "支付发起成功",
		"data": gin.H{
			"order_id": req.OrderID,
			"status":   "pending",
			"qr_code":  "data:image/png;base64,...", // 实际返回支付二维码
		},
	})
}

// callback 支付回调
func callback(c *gin.Context) {
	// 第三方支付平台回调
	// 验证签名，更新订单状态

	var callbackData map[string]interface{}
	if err := c.ShouldBindJSON(&callbackData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	log.Printf("收到支付回调: %v\n", callbackData)

	// 处理回调逻辑
	// 1. 验证签名
	// 2. 更新订单状态
	// 3. 通知库存服务扣减库存

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "回调处理成功",
	})
}

// healthCheck 健康检查
func healthCheck(c *gin.Context) {
	// 检查外部支付服务连接
	paymentStatus := "ok"
	// TODO: 添加支付服务健康检查

	c.JSON(http.StatusOK, gin.H{
		"status": "up",
		"services": {
			"payment": paymentStatus,
		},
		"timestamp": time.Now().Unix(),
	})
}
