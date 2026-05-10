# Kubernetes生产级性能优化配置

## 🎯 优化目标
- 集群资源利用率>70%
- 应用启动时间缩短50%
- 调度延迟<100ms
- 服务可用性99.99%

## 📋 Kubelet配置优化

### 1. kubelet.conf 核心配置
```ini
KUBELET_EXTRA_ARGS="--cgroups-per-qos=true \
--enforce-node-allocatable=pods,kube-reserved,system-reserved \
--kube-reserved=cpu=500m,memory=1Gi,ephemeral-storage=10Gi \
--system-reserved=cpu=500m,memory=1Gi,ephemeral-storage=10Gi \
--eviction-hard=memory.available<500Mi,nodefs.available<10%,imagefs.available<15% \
--eviction-soft=memory.available<1Gi,nodefs.available<15%,imagefs.available<20% \
--eviction-soft-grace-period=memory.available=2m,nodefs.available=2m,imagefs.available=2m \
--eviction-max-pod-grace-period=30 \
--eviction-minimum-reclaim=memory.available=0Mi,nodefs.available=5%,imagefs.available=5% \
--image-gc-high-threshold=85 \
--image-gc-low-threshold=70 \
--max-pods=200 \
--pod-pids-limit=1000 \
--serialize-image-pulls=false \
--feature-gates=KubeletConfigFile=true \
--rotate-certificates=true \
--authorization-mode=Webhook \
--client-ca-file=/etc/kubernetes/pki/ca.crt \
--read-only-port=0 \
--protect-kernel-defaults=true \
--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256"
```

### 2. 容器运行时配置（containerd）
```toml
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = "registry.aliyuncs.com/google_containers/pause:3.9"
  max_container_log_line_size = 16384
  [plugins."io.containerd.grpc.v1.cri"].containerd
    default_runtime_name = "runc"
    discard_unpacked_layers = true
    [plugins."io.containerd.grpc.v1.cri"].containerd.runtimes.runc
      runtime_type = "io.containerd.runc.v2"
      [plugins."io.containerd.grpc.v1.cri"].containerd.runtimes.runc.options
        SystemdCgroup = true
        NoNewPrivileges = true
  [plugins."io.containerd.grpc.v1.cri"].cni
    bin_dir = "/opt/cni/bin"
    conf_dir = "/etc/cni/net.d"
  [plugins."io.containerd.grpc.v1.cri"].registry
    [plugins."io.containerd.grpc.v1.cri"].registry.mirrors
      [plugins."io.containerd.grpc.v1.cri"].registry.mirrors."docker.io"
        endpoint = ["https://registry-1.docker.io"]
      [plugins."io.containerd.grpc.v1.cri"].registry.mirrors."k8s.gcr.io"
        endpoint = ["https://registry.aliyuncs.com/google_containers"]
  [plugins."io.containerd.grpc.v1.cri"].image_decryption
    key_model = "node"
  [plugins."io.containerd.grpc.v1.cri"].x509_key_pair_streaming
    tls_cert_file = "/etc/kubernetes/pki/kubelet-client-current.pem"
    tls_key_file = "/etc/kubernetes/pki/kubelet-client-current.pem"
```

## 🚀 调度器优化配置

### 1. kube-scheduler配置
```yaml
apiVersion: kubescheduler.config.k8s.io/v1beta3
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: true
  resourceName: kube-scheduler
  resourceNamespace: kube-system
clientConnection:
  kubeconfig: "/etc/kubernetes/scheduler.conf"
  qps: 100
  burst: 200
enableContentionProfiling: true
enableProfiling: true
hardPodAffinitySymmetricWeight: 100
percentageOfNodesToScore: 50
podInitialBackoffSeconds: 1
podMaxBackoffSeconds: 10
schedulerName: default-scheduler
profiles:
- schedulerName: default-scheduler
  plugins:
    score:
      disabled:
      - name: NodeResourcesLeastAllocated
      enabled:
      - name: NodeResourcesMostAllocated
        weight: 1
      - name: NodeResourcesBalancedAllocation
        weight: 1
      - name: NodeAffinity
        weight: 2
      - name: InterPodAffinity
        weight: 3
      - name: NodePreferAvoidPods
        weight: 100
      - name: NodeTaints
        weight: 3
      - name: ImageLocality
        weight: 1
```

### 2. 拓扑感知调度
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: topology-scheduler-config
  namespace: kube-system
data:
  policy.cfg: |
    {
      "kind": "Policy",
      "apiVersion": "v1",
      "predicates": [
        {
          "name": "LeastRequestedPriority"
        },
        {
          "name": "BalancedResourceAllocation"
        },
        {
          "name": "NodeAffinityPriority"
        },
        {
          "name": "TaintTolerationPriority"
        }
      ],
      "priorities": [
        {
          "name": "ServiceSpreadingPriority",
          "weight": 1
        },
        {
          "name": "EqualPriority",
          "weight": 1
        }
      ],
      "hardPodAffinitySymmetricWeight": 10
    }
```

## 📦 资源QoS配置

### 1. Guaranteed等级配置
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx-guaranteed
spec:
  containers:
  - name: nginx
    image: nginx
    resources:
      requests:
        memory: "1Gi"
        cpu: "1000m"
      limits:
        memory: "1Gi"
        cpu: "1000m"
    ports:
    - containerPort: 80
```

### 2. Burstable等级配置
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx-burstable
spec:
  containers:
  - name: nginx
    image: nginx
    resources:
      requests:
        memory: "512Mi"
        cpu: "500m"
      limits:
        memory: "2Gi"
        cpu: "2000m"
    ports:
    - containerPort: 80
```

## 📊 网络性能优化

### 1. Calico网络优化
```yaml
apiVersion: projectcalico.org/v3
kind: FelixConfiguration
metadata:
  name: default
  namespace: kube-system
spec:
  bpfEnabled: true
  bpfLogLevel: None
  ipipEnabled: false
  vxlanEnabled: true
  vxlanPort: 4789
  mtu: 1450
  prometheusMetricsEnabled: true
  flowLogsEnabled: true
  flowLogFileEnabled: false
  logSeverityScreen: Info
  reportingInterval: 30s
  interfacePrefix: "eth,ens"
```

### 2. CoreDNS性能优化
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        errors
        health {
           lameduck 5s
        }
        ready
        kubernetes cluster.local in-addr.arpa ip6.arpa {
           pods verified
           fallthrough in-addr.arpa ip6.arpa
           ttl 30
        }
        prometheus :9153
        forward . /etc/resolv.conf {
           max_concurrent 1000
        }
        cache 30
        loop
        reload
        loadbalance
    }
```

## 📈 监控与告警

### 1. Prometheus Node Exporter配置
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: node-exporter
  namespace: monitoring
  labels:
    app: node-exporter
spec:
  selector:
    matchLabels:
      app: node-exporter
  template:
    metadata:
      labels:
        app: node-exporter
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9100"
    spec:
      hostNetwork: true
      hostPID: true
      containers:
      - name: node-exporter
        image: quay.io/prometheus/node-exporter:v1.8.2
        args:
        - --path.rootfs=/host
        - --path.procfs=/host/proc
        - --path.sysfs=/host/sys
        - --path.udev.data=/host/run/udev/data
        - --web.listen-address=0.0.0.0:9100
        - --collector.cpu
        - --collector.diskstats
        - --collector.filesystem
        - --collector.loadavg
        - --collector.meminfo
        - --collector.netdev
        - --collector.netstat
        - --collector.stat
        - --collector.time
        - --collector.vmstat
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi
        volumeMounts:
        - name: rootfs
          mountPath: /host
          readOnly: true
      volumes:
      - name: rootfs
        hostPath:
          path: /
```

## 📊 优化效果预期

| 指标 | 优化前 | 优化后 |
|------|--------|--------|
| 资源利用率 | 40% | >70% |
| 调度延迟 | 500ms | <100ms |
| 启动时间 | 120s | <60s |
| 服务可用性 | 99.5% | 99.99% |
| 网络吞吐量 | 1Gbps | 10Gbps |
| 集群密度 | 100 pod/node | 200 pod/node |

## 📝 注意事项

1. **节点资源预留**
   - 根据节点规格合理预留系统资源
   - 避免资源过度分配导致节点不稳定
   - 配置合理的驱逐阈值

2. **QoS策略**
   - 核心业务使用Guaranteed等级
   - 非核心业务使用Burstable等级
   - 临时任务使用BestEffort等级

3. **网络优化**
   - 选择合适的CNI插件
   - 配置合理的MTU值
   - 启用BPF加速

4. **监控建议**
   - 监控节点资源使用情况
   - 监控调度成功率和延迟
   - 设置合理的告警阈值

---

**版本**：v1.0  
**日期**：2026-05-10  
**作者**：小白老师（资深运维工程师）