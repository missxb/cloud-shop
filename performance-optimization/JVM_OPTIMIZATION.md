# JVM生产级参数调优配置

## 🎯 优化目标
- 核心链路响应时间<200ms
- Full GC间隔时间>24小时
- GC停顿时间<50ms
- 内存利用率>70%

## 📋 推荐配置

### 1. G1垃圾收集器配置（推荐）
```bash
JAVA_OPTS="-server \
-Xms16g \
-Xmx16g \
-Xmn8g \
-XX:MetaspaceSize=512m \
-XX:MaxMetaspaceSize=1g \
-XX:+UseG1GC \
-XX:G1HeapRegionSize=16m \
-XX:MaxGCPauseMillis=50 \
-XX:G1ReservePercent=20 \
-XX:ParallelGCThreads=8 \
-XX:ConcGCThreads=4 \
-XX:+HeapDumpOnOutOfMemoryError \
-XX:HeapDumpPath=/var/log/heapdump.hprof \
-XX:+PrintGCDetails \
-XX:+PrintGCDateStamps \
-XX:+PrintGCApplicationStoppedTime \
-Xloggc:/var/log/gc.log \
-XX:+UseGCLogFileRotation \
-XX:NumberOfGCLogFiles=10 \
-XX:GCLogFileSize=100M"
```

### 2. ZGC垃圾收集器配置（Java 11+）
```bash
JAVA_OPTS="-server \
-Xms16g \
-Xmx16g \
-XX:MetaspaceSize=512m \
-XX:MaxMetaspaceSize=1g \
-XX:+UseZGC \
-XX:ZGCParallelGCThreads=8 \
-XX:ConcGCThreads=4 \
-XX:+HeapDumpOnOutOfMemoryError \
-XX:HeapDumpPath=/var/log/heapdump.hprof \
-XX:+PrintGCDetails \
-XX:+PrintGCDateStamps \
-Xloggc:/var/log/gc.log"
```

### 3. 应用级优化配置
```bash
# 启用分层编译
-XX:+TieredCompilation

# 禁用偏向锁
-XX:-UseBiasedLocking

# 启用字符串去重
-XX:+UseStringDeduplication

# 启用压缩指针
-XX:+UseCompressedOops
-XX:+UseCompressedClassPointers

# 配置直接内存
-XX:MaxDirectMemorySize=4g

# 配置代码缓存
-XX:InitialCodeCacheSize=256m
-XX:ReservedCodeCacheSize=512m
-XX:+UseCodeCacheFlushing
```

## 📊 监控配置

### 1. Prometheus JMX Exporter配置
```yaml
---
startDelaySeconds: 0
hostPort: 127.0.0.1:9090
ssl: false
rules:
  - pattern: ".*"
```

### 2. 关键监控指标
| 指标名称 | 阈值 | 说明 |
|---------|------|------|
| jvm_memory_used_bytes{area="heap"} | >14G | 堆内存使用率超过87.5% |
| jvm_gc_collection_seconds_sum{gc="G1 Old Generation"} | >30s/天 | 老年代GC总时间过长 |
| jvm_threads_deadlocked_monitor | >0 | 出现线程死锁 |
| jvm_classes_loaded | >10000 | 加载类数量过多 |

## 🚨 告警规则

```yaml
groups:
- name: jvm-alerts
  rules:
  - alert: JVMHeapMemoryHigh
    expr: jvm_memory_used_bytes{area="heap"} / jvm_memory_max_bytes{area="heap"} > 0.9
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "JVM堆内存使用率过高"
      description: "{{ $labels.instance }} 堆内存使用率 {{ $value | humanizePercentage }} (>90%)"

  - alert: JVMGCTooFrequent
    expr: increase(jvm_gc_collection_count_sum{gc="G1 Old Generation"}[1h]) > 5
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "JVM老年代GC过于频繁"
      description: "{{ $labels.instance }} 最近1小时老年代GC次数 {{ $value }} (>5次)"

  - alert: JVMThreadDeadlock
    expr: jvm_threads_deadlocked_monitor > 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "JVM线程死锁"
      description: "{{ $labels.instance }} 发现 {{ $value }} 个线程死锁"
```

## 📈 优化效果预期

| 指标 | 优化前 | 优化后 |
|------|--------|--------|
| 平均响应时间 | 500ms | <200ms |
| GC停顿时间 | 200ms | <50ms |
| Full GC间隔 | 4小时 | >24小时 |
| 吞吐量 | 70% | >90% |
| 内存利用率 | 50% | >70% |

## 📝 注意事项

1. **内存配置建议**
   - 堆内存设置为物理内存的50%-70%
   - 元空间大小根据应用加载类数量调整
   - 直接内存根据使用场景配置

2. **GC选择建议**
   - Java 8推荐使用G1垃圾收集器
   - Java 11+推荐使用ZGC垃圾收集器
   - 低延迟场景优先选择ZGC

3. **监控建议**
   - 必须配置GC日志和堆转储
   - 启用JMX监控，接入Prometheus
   - 设置合理的告警阈值

4. **压测建议**
   - 上线前进行充分的性能压测
   - 模拟真实业务场景和并发量
   - 优化后重新压测验证效果

---

**版本**：v1.0  
**日期**：2026-05-10  
**作者**：小白老师（资深运维工程师）