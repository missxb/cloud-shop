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

// User 模型
type User struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Username  string    `json:"username"`
	Password  string    `json:"-"` // 不返回密码
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
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
	db.AutoMigrate(&User{})
}

func main() {
	initDB()

	r := gin.Default()

	// 健康检查
	r.GET("/health", healthCheck)

	// 用户相关路由
	userGroup := r.Group("/api/v1/user")
	{
		userGroup.POST("/register", register)
		userGroup.POST("/login", login)
		userGroup.GET("/info", getUserInfo)
	}

	// 启动服务
	log.Println("用户服务启动，端口:8080")
	server := &http.Server{
		Addr:    ":8080",
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

// register 用户注册
func register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	// 检查用户是否已存在
	var existingUser User
	result := db.Where("username = ?", req.Username).First(&existingUser)
	if result.RowsAffected > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "用户已存在",
		})
		return
	}

	// 创建用户
	user := User{
		Username: req.Username,
		Password: req.Password, // 实际项目应该加密
		Email:    req.Email,
	}

	result = db.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "注册失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "注册成功",
		"data": user,
	})
}

// login 用户登录
func login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	// 查询用户
	var user User
	result := db.Where("username = ? AND password = ?", req.Username, req.Password).First(&user)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "用户名或密码错误",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "登录成功",
		"data": user,
	})
}

// getUserInfo 获取用户信息
func getUserInfo(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	var user User
	result := db.First(&user, id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "用户不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": user,
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
