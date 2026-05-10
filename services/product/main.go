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

// 全局Prometheus指标
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"},
	)

	inFlightRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_in_flight_requests",
			Help: "Number of in-flight HTTP requests",
		},
	)
)

// Product 模型
type Product struct {
	ID          int64      `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Price       float64    `json:"price"`
	Stock       int        `json:"stock"`
	Status      int        `json:"status"` // 1-上架 0-下架
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// prometheusMiddleware Prometheus中间件
func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		start := time.Now()
		
		// 增加in-flight请求数
		inFlightRequests.Inc()
		defer inFlightRequests.Dec()
		
		// 处理请求
		c.Next()
		
		// 记录指标
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, c.Request.URL.Path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, c.Request.URL.Path).Observe(duration)
	}
}

// initPrometheus 初始化Prometheus指标
func initPrometheus() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(inFlightRequests)
}

// CreateProductRequest 创建商品请求
type CreateProductRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required"`
	Stock       int     `json:"stock"`
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
	db.AutoMigrate(&Product{})
}

func main() {
	initDB()
	initPrometheus()

	r := gin.Default()

	// Prometheus中间件
	r.Use(prometheusMiddleware())

	// 健康检查
	r.GET("/health", healthCheck)
	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 商品相关路由
	productGroup := r.Group("/api/v1/product")
	{
		productGroup.GET("/list", listProduct)
		productGroup.GET("/:id", getProduct)
		productGroup.POST("", createProduct)
	}

	// 启动服务
	log.Println("商品服务启动，端口:8081")
	server := &http.Server{
		Addr:    ":8081",
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

// listProduct 获取商品列表
func listProduct(c *gin.Context) {
	var products []Product
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	offset := (page - 1) * pageSize

	var total int64
	db.Where("status = ?", 1).Model(&Product{}).Count(&total)
	db.Where("status = ?", 1).Offset(offset).Limit(pageSize).Find(&products)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"list":      products,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// getProduct 获取商品详情
func getProduct(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	var product Product
	result := db.First(&product, id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "商品不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": product,
	})
}

// createProduct 创建商品
func createProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	product := Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Status:      1,
	}

	result := db.Create(&product)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "创建失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "创建成功",
		"data": product,
	})
}

// healthCheck 健康检查
func healthCheck(c *gin.Context) {
	// 检查数据库连接
	dbStatus := "ok"
	if err := db.Exec("SELECT 1").Error; err != nil {
		dbStatus = "error: " + err.Error()
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "up",
		"services": {
			"database": dbStatus,
		},
		"timestamp": time.Now().Unix(),
	})
}
