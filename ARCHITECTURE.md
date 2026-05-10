# 云原生电商系统架构设计

## 系统架构

### 整体架构图

```
                    ┌─────────────┐
                    │   用户端    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   CDN/WAF   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
         ┌────▼────┐  ┌──▼───┐  ┌────▼────┐
         │ 网关层  │  │ 监控 │  │ 链路追踪│
         └────┬────┘  └───┬───┘  └────┬────┘
              │           │           │
              └───────────┼───────────┘
                          │
           ┌──────────────┼──────────────┐
           │              │              │
      ┌────▼────┐   ┌────▼────┐   ┌────▼────┐
      │ 用户服务│   │商品服务 │   │订单服务 │
      └────┬────┘   └────┬────┘   └────┬────┘
           │             │             │
      ┌────▼────┐   ┌────▼────┐   ┌────▼────┐
      │支付服务│   │库存服务 │   │消息队列│
      └─────────┘   └─────────┘   └─────────┘
           │             │             │
      ┌────▼─────────────▼─────────────▼──┐
      │         数据存储层                │
      │  MySQL   Redis   RabbitMQ         │
      └───────────────────────────────────┘
```

## 服务架构

### 服务列表

| 服务名称 | 端口 | 功能描述 | 技术栈 |
|---------|------|---------|--------|
| user | 8080 | 用户注册、登录、信息管理 | Go/Gin |
| product | 8081 | 商品管理、搜索、推荐 | Go/Gin |
| order | 8082 | 订单创建、查询、取消 | Go/Gin |
| payment | 8083 | 支付接口、回调处理 | Go/Gin |
| inventory | 8084 | 库存管理、扣减、回滚 | Go/Gin |

## 数据库设计

### 数据库表结构

#### 用户表 (user)
```sql
CREATE TABLE `user` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(50) NOT NULL COMMENT '用户名',
  `password` VARCHAR(255) NOT NULL COMMENT '密码',
  `email` VARCHAR(100) NOT NULL COMMENT '邮箱',
  `phone` VARCHAR(20) COMMENT '手机号',
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_username` (`username`),
  UNIQUE KEY `idx_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
```

#### 商品表 (product)
```sql
CREATE TABLE `product` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(200) NOT NULL COMMENT '商品名称',
  `description` TEXT COMMENT '商品描述',
  `price` DECIMAL(10,2) NOT NULL COMMENT '价格',
  `stock` INT DEFAULT 0 COMMENT '库存',
  `status` TINYINT DEFAULT 1 COMMENT '状态 1-上架 0-下架',
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品表';
```

#### 订单表 (order)
```sql
CREATE TABLE `order` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_no` VARCHAR(64) NOT NULL COMMENT '订单号',
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `product_id` BIGINT NOT NULL COMMENT '商品ID',
  `quantity` INT NOT NULL COMMENT '数量',
  `amount` DECIMAL(10,2) NOT NULL COMMENT '金额',
  `status` TINYINT DEFAULT 0 COMMENT '状态 0-待支付 1-已支付 2-已取消',
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_order_no` (`order_no`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';
```

## API设计

### RESTful API

#### 用户服务
- POST /api/v1/user/register - 用户注册
- POST /api/v1/user/login - 用户登录
- GET /api/v1/user/info - 获取用户信息

#### 商品服务
- GET /api/v1/product/list - 商品列表
- GET /api/v1/product/:id - 商品详情
- POST /api/v1/product - 创建商品

#### 订单服务
- POST /api/v1/order/create - 创建订单
- GET /api/v1/order/:id - 订单详情
- GET /api/v1/order/list - 订单列表

#### 支付服务
- POST /api/v1/payment/pay - 支付
- POST /api/v1/payment/callback - 支付回调

#### 库存服务
- POST /api/v1/inventory/decrease - 扣减库存
- POST /api/v1/inventory/rollback - 回滚库存
