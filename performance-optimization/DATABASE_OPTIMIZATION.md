# 数据库生产级性能优化配置

## 🎯 优化目标
- 核心SQL响应时间<50ms
- 数据库CPU使用率<70%
- 缓存命中率>99%
- 读写分离比例>9:1

## 📋 MySQL优化配置

### 1. my.cnf 核心配置
```ini
[mysqld]
# 基础配置
user = mysql
datadir = /var/lib/mysql
socket = /var/lib/mysql/mysql.sock
pid-file = /var/run/mysqld/mysqld.pid

# 字符集配置
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci
init-connect = 'SET NAMES utf8mb4'

# 网络连接配置
max_connections = 2000
max_connect_errors = 1000000
wait_timeout = 60
interactive_timeout = 60
connect_timeout = 10

# 缓存配置
key_buffer_size = 256M
query_cache_type = 0
query_cache_size = 0M
query_cache_limit = 2M

# Innodb配置
innodb_buffer_pool_size = 24G
innodb_buffer_pool_instances = 8
innodb_log_file_size = 4G
innodb_log_buffer_size = 64M
innodb_flush_log_at_trx_commit = 1
innodb_flush_method = O_DIRECT
innodb_file_per_table = 1
innodb_autoinc_lock_mode = 2
innodb_stats_on_metadata = 0
innodb_old_blocks_pct = 37
innodb_old_blocks_time = 1000
innodb_print_all_deadlocks = 1

# 查询优化配置
table_open_cache = 2048
table_definition_cache = 2048
max_heap_table_size = 64M
tmp_table_size = 64M
sort_buffer_size = 4M
join_buffer_size = 4M
read_buffer_size = 2M
read_rnd_buffer_size = 16M

# 日志配置
slow_query_log = 1
slow_query_log_file = /var/log/mysql/slow.log
long_query_time = 0.1
log_queries_not_using_indexes = 1
log_throttle_queries_not_using_indexes = 10
log_error = /var/log/mysql/error.log
log_error_verbosity = 3

# 主从复制配置
server-id = 1
execution_engine = parallel
log_bin = /var/lib/mysql/mysql-bin
binlog_format = row
binlog_row_image = minimal
binlog_expire_logs_seconds = 86400
relay_log = /var/lib/mysql/relay-bin
log_slave_updates = 1
slave_parallel_type = LOGICAL_CLOCK
slave_parallel_workers = 8
master_info_repository = TABLE
relay_log_info_repository = TABLE
relay_log_recovery = 1
```

### 2. 分库分表策略

#### 订单库拆分
```sql
-- 按用户ID哈希拆分订单表
CREATE TABLE `order_0` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) NOT NULL,
  `order_no` varchar(64) NOT NULL,
  `amount` decimal(10,2) NOT NULL,
  `status` tinyint(4) NOT NULL,
  `create_time` datetime NOT NULL,
  `update_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_order_no` (`order_no`),
  KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 同样创建 order_1 到 order_15
```

#### 商品库拆分
```sql
-- 按商品类型拆分商品表
CREATE TABLE `product_0` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `category_id` bigint(20) NOT NULL,
  `product_no` varchar(64) NOT NULL,
  `name` varchar(128) NOT NULL,
  `price` decimal(10,2) NOT NULL,
  `stock` int(11) NOT NULL,
  `create_time` datetime NOT NULL,
  `update_time` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_category_id` (`category_id`),
  KEY `idx_product_no` (`product_no`),
  KEY `idx_price` (`price`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 🚀 缓存优化策略

### 1. Redis Sentinel配置
```yaml
port: 26379
dir: /tmp
sentinel monitor mymaster 127.0.0.1 6379 2
sentinel down-after-milliseconds mymaster 30000
sentinel parallel-syncs mymaster 1
sentinel failover-timeout mymaster 180000
sentinel config-epoch mymaster 0
sentinel leader-epoch mymaster 0
```

### 2. 多级缓存架构
```
浏览器缓存 → CDN缓存 → 应用本地缓存 → Redis缓存 → 数据库
```

### 3. 缓存更新策略
```java
// 双删策略更新缓存
public void updateProduct(Long productId, Product product) {
    // 1. 更新数据库
    productDao.update(product);
    
    // 2. 删除缓存
    redisTemplate.delete("product:" + productId);
    
    // 3. 延迟双删
    executorService.schedule(() -> {
        redisTemplate.delete("product:" + productId);
    }, 100, TimeUnit.MILLISECONDS);
}
```

## 📊 读写分离配置

### 1. MySQL Router配置
```ini
[DEFAULT]
user=mysqlrouter
logging_folder=/var/log/mysqlrouter
runtime_folder=/var/run/mysqlrouter
config_folder=/etc/mysqlrouter

[logger]
level=INFO

[routing:ro]
bind_address=0.0.0.0
bind_port=6447
destination=192.168.1.101:3306,192.168.1.102:3306
routing_strategy=round-robin
protocol=classic

[routing:rw]
bind_address=0.0.0.0
bind_port=6446
destination=192.168.1.100:3306
routing_strategy=first-available
protocol=classic
```

### 2. 应用层读写分离
```java
@Configuration
public class DataSourceConfig {
    
    @Bean
    @Primary
    @ConfigurationProperties(prefix = "datasource.master")
    public DataSource masterDataSource() {
        return DataSourceBuilder.create().build();
    }
    
    @Bean
    @ConfigurationProperties(prefix = "datasource.slave1")
    public DataSource slave1DataSource() {
        return DataSourceBuilder.create().build();
    }
    
    @Bean
    public DataSource routingDataSource() {
        Map<Object, Object> dataSources = new HashMap<>();
        dataSources.put(DataSourceType.MASTER, masterDataSource());
        dataSources.put(DataSourceType.SLAVE1, slave1DataSource());
        
        RoutingDataSource routingDataSource = new RoutingDataSource();
        routingDataSource.setTargetDataSources(dataSources);
        routingDataSource.setDefaultTargetDataSource(masterDataSource());
        
        return routingDataSource;
    }
}
```

## 📈 性能优化效果

| 指标 | 优化前 | 优化后 |
|------|--------|--------|
| 平均查询时间 | 200ms | <50ms |
| 并发连接数 | 500 | 2000 |
| 缓存命中率 | 90% | >99% |
| 读写分离比例 | 7:3 | >9:1 |
| 慢查询数量 | 100+/小时 | <10/小时 |
| TPS | 1000 | 5000+ |

## 📝 注意事项

1. **配置调整建议**
   - 根据服务器硬件配置调整参数
   - 监控系统运行状态，动态调整
   - 避免过度配置导致资源浪费

2. **索引优化建议**
   - 定期分析慢查询日志
   - 优化不必要的索引
   - 覆盖常用查询场景

3. **缓存注意事项**
   - 避免缓存雪崩、穿透、击穿
   - 设置合理的缓存过期时间
   - 实现缓存预热机制

4. **监控建议**
   - 监控数据库连接数、锁等待
   - 监控缓存命中率、内存使用
   - 设置合理的告警阈值

---

**版本**：v1.0  
**日期**：2026-05-10  
**作者**：小白老师（资深运维工程师）