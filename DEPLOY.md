# 云原生电商系统部署文档

## 📋 前置要求

- Kubernetes 集群（1.20+ 版本）
- kubectl 配置正确
- Helm 3 安装完成
- 镜像仓库（Harbor）可用
- Ingress Controller 已安装

## 🚀 部署步骤

### 第一步：创建命名空间和RBAC

```bash
kubectl apply -f security/rbac.yaml
```

### 第二步：部署基础组件

```bash
# 部署MySQL
kubectl apply -f k8s/mysql-deployment.yaml

# 部署Redis
kubectl apply -f k8s/redis-deployment.yaml

# 部署RabbitMQ
kubectl apply -f k8s/rabbitmq-deployment.yaml
```

### 第三步：部署微服务

```bash
# 部署各个服务
kubectl apply -f k8s/user-deployment.yaml
kubectl apply -f k8s/product-deployment.yaml
kubectl apply -f k8s/order-deployment.yaml
kubectl apply -f k8s/payment-deployment.yaml
kubectl apply -f k8s/inventory-deployment.yaml
```

### 第四步：配置自动扩缩容

```bash
kubectl apply -f k8s/hpa.yaml
```

### 第五步：部署Ingress

```bash
kubectl apply -f k8s/ingress.yaml
```

### 第六步：配置网络策略（安全加固）

```bash
kubectl apply -f security/network-policy.yaml
```

### 第七步：配置监控告警

1. 将 `monitoring/alerts.yml` 导入 Prometheus
2. 导入Grafana仪表盘（文件 `monitoring/grafana-dashboard.json`）

## 🧪 本地开发（Docker Compose）

```bash
# 启动所有服务
docker-compose up -d

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f [service-name]

# 停止服务
docker-compose down
```

## 📊 测试API

### 用户注册

```bash
curl -X POST http://ecommerce.example.com/api/v1/user/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "test",
    "password": "123456",
    "email": "test@example.com"
  }'
```

### 用户登录

```bash
curl -X POST http://ecommerce.example.com/api/v1/user/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "test",
    "password": "123456"
  }'
```

### 获取商品列表

```bash
curl http://ecommerce.example.com/api/v1/product/list
```

### 创建订单

```bash
curl -X POST http://ecommerce.example.com/api/v1/order/create \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": 1,
    "quantity": 2
  }'
```

## 🔄 CI/CD流程

1. 开发提交代码到GitLab
2. GitLab Webhook触发Jenkins构建
3. Jenkins检出代码、代码扫描、构建镜像、推送Harbor
4. Jenkins更新K8s Deployment镜像
5. Kubernetes滚动更新，完成部署
6. 健康检查，发送通知

或者使用ArgoCD GitOps方式：

1. 代码变更提交到Git
2. ArgoCD检测到Git变化
3. ArgoCD自动同步到K8s集群
4. 完成部署

## 📈 扩缩容

HPA已配置，CPU超过70%会自动扩容，最小2个副本，最大10个副本。

手动扩缩容：

```bash
kubectl scale deployment user --replicas=5 -n ecommerce
```

## 🛡️ 安全加固

- 网络策略：最小权限原则，只允许必要的访问
- RBAC：使用独立ServiceAccount，不使用默认admin权限
- 镜像安全：Trivy扫描漏洞
- 健康检查：liveness/readiness探针自动恢复
- 资源限制：防止资源耗尽

## 📝 项目目录

```
cloud-native-ecommerce/
├── README.md                # 项目介绍
├── ARCHITECTURE.md          # 架构设计
├── DEPLOY.md                # 部署文档
├── docker-compose.yml       # 本地开发配置
├── services/                # 微服务代码
│   ├── user/                # 用户服务
│   ├── product/             # 商品服务
│   ├── order/               # 订单服务
│   ├── payment/             # 支付服务
│   └── inventory/           # 库存服务
├── k8s/                     # Kubernetes配置
├── cicd/                    # CI/CD配置
├── monitoring/              # 监控配置
└── security/                # 安全配置
```
