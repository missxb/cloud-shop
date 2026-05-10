package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Order 模型
type Order struct {
	ID         int64       `json:"id" gorm:"primaryKey"`
	OrderNo    string      `json:"order_no"`
	UserID     int64       `json:"user_id"`
	ProductID  int64       `json:"product_id"`
	Quantity   int         `json:"quantity"`
	Amount     float64     `json:"amount"`
	Status     int         `json:"status"` // 0-待支付 1-已支付 2-已取消
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	UserID    int64 `json:"user_id" binding:"required"`
	ProductID int64 `json:"product_id" binding:"required"`
	Quantity  int   `json:"quantity" binding:"required"`
}

var db *gorm.DB

// Config 配置结构体
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	LogLevel   string
}

// loadConfig 加载配置
func loadConfig() *Config {
	return &Config{
		DBHost:     getEnv("DB_HOST", "mysql"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBName:     getEnv("DB_NAME", "ecommerce"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),
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

func initDB() {
	var err error
	config := loadConfig()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.DBUser,
		config.DBPassword,
		config.DBHost,
		config.DBPort,
		config.DBName,
	)

	// 根据日志级别配置GORM日志
	var logLevel logger.LogLevel
	switch config.LogLevel {
	case "debug":
		logLevel = logger.Info
	case "info":
		logLevel = logger.Warn
	case "error":
		logLevel = logger.Error
	default:
		logLevel = logger.Silent
	}

	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	// 自动迁移
	db.AutoMigrate(&Order{})
}

func generateOrderNo() string {
	return "ORD" + time.Now().Format("20060102150405") + strconv.FormatInt(time.Now().UnixNano()%1000, 10)
}

func main() {
	initDB()

	r := gin.Default()

	// 健康检查
	r.GET("/health", healthCheck)

	// 订单相关路由
	orderGroup := r.Group("/api/v1/order")
	{
		orderGroup.POST("/create", createOrder)
		orderGroup.GET("/:id", getOrder)
		orderGroup.GET("/list", listOrder)
	}

	// 启动服务
	log.Println("订单服务启动，端口:8082")
	server := &http.Server{
		Addr:    ":8082",
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

// createOrder 创建订单
func createOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	// 生成订单号
	orderNo := generateOrderNo()

	// TODO: 调用商品服务获取商品信息和价格
	// TODO: 调用库存服务扣减库存
	// 这里简化处理

	order := Order{
		OrderNo:   orderNo,
		UserID:    req.UserID,
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
		// Amount:   product.Price * float64(req.Quantity),
		Status: 0, // 待支付
	}

	result := db.Create(&order)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "创建订单失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "创建订单成功",
		"data": order,
	})
}

// getOrder 获取订单详情
func getOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	var order Order
	result := db.First(&order, id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "订单不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": order,
	})
}

// listOrder 获取订单列表
func listOrder(c *gin.Context) {
	userIDStr := c.Query("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	offset := (page - 1) * pageSize

	var orders []Order
	var total int64

	db.Where("user_id = ?", userID).Model(&Order{}).Count(&total)
	db.Where("user_id = ?", userID).Offset(offset).Limit(pageSize).Find(&orders)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"list":      orders,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// healthCheck 健康检查
func healthCheck(c *gin.Context) {
	// 检查数据库连接
	dbStatus := "ok"
	if err := db.Exec("SELECT 1").Error; err != nil {
		dbStatus = "error: " + err.Error()
	}

	// 检查RabbitMQ连接
	mqStatus := "ok"
	// TODO: 添加RabbitMQ健康检查

	c.JSON(http.StatusOK, gin.H{
		"status": "up",
		"services": {
			"database": dbStatus,
			"rabbitmq": mqStatus,
		},
		"timestamp": time.Now().Unix(),
	})
}
