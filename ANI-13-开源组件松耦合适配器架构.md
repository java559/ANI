# KuberCloud ANI · 开源组件松耦合适配器架构

> 版本 V1 | 广州常青云科技有限公司 | 架构强制约束文件

---

## 一、设计背景

ANI 大量复用成熟开源组件，这是产品战略的一部分。但开源组件迭代速度快，组件社区、License、部署方式、API 行为和最佳实践都可能变化。

因此，ANI 必须避免把任一开源组件固化成不可替换的产品边界。

本文件定义一个强制原则：

```text
除 Kubernetes API 和已登记的 bounded_direct 模块作为底层平台/安全/事务边界外，任何第三方开源组件都只能作为默认适配器实现，不能成为 ANI 的业务域模型、外部 API、内部服务契约或产品语义边界。
```

默认实现可以选择 MinIO、Milvus、NATS JetStream、Redis、Harbor、CloudNativePG 等组件；除明确登记的 bounded module 外，业务服务只能依赖 ANI 自己定义的能力接口。

本文中的 `port` 指“产品能力抽象/接口边界”，不是 TCP/IP 端口。`pkg/ports/` 描述 ANI 需要什么能力，`pkg/adapters/` 描述某个默认组件如何实现该能力。

---

## 二、强制原则

### 2.0 可用性与稳定性优先

开源组件松耦合不是为了追求形式上的“所有东西都包一层”，而是为了降低长期替换和升级风险。

本项目的优先级是：

```text
可用性 / 稳定性 > 性能可控性 > 扩展性 > 组件可替换性
```

因此，组件集成必须先保证核心路径可用、稳定、可观测；在这个前提下再通过 port/adapter 降低耦合。

禁止两种极端：

- 为了“松耦合”抹平核心组件高级能力，导致性能、可靠性或可运维性下降。
- 为了“性能”让组件 SDK 在业务服务中无边界扩散。

### 2.1 能力优先，而非组件优先

文档、接口、代码和 Helm values 应优先表达能力：

| 能力名 | 业务语义 | 默认适配器 |
|---|---|---|
| `MetadataStore` | 元数据、RLS、事务、一致性写入 | PostgreSQL / CloudNativePG |
| `ObjectStore` | 对象上传、下载、签名 URL、bucket bootstrap | MinIO / S3-compatible |
| `VectorStore` | collection 管理、向量 upsert/search/delete | Milvus |
| `MessageBus` | task/event 发布、消费、ack/nack、重试 | NATS JetStream |
| `CacheStore` | 限流、JWT blocklist、短 TTL 状态 | Redis |
| `ImageRegistry` | 镜像元数据、扫描状态、仓库代理 | Harbor |
| `IdentityProvider` | OIDC/SAML/LDAP 身份接入 | Dex / Keycloak |

#### 2.1.1 何时必须定义 port

满足任一条件时，必须先定义或复用 `pkg/ports` 能力接口：

- 该依赖承载 ANI 对外产品能力，例如实例、网络、存储、身份、计量、对象、向量、消息、缓存。
- Core service、Gateway handler、ANI Services 或 SDK 调用方不应知道底层组件细节。
- 存在合理的替换、多实现或云厂商差异，例如 MinIO/S3、Milvus/其他向量库、Redis/其他缓存、KubeOVN/其他 CNI 实现。
- 需要统一错误语义、审计、幂等、租户隔离、状态机或 reconcile。
- 组件 SDK 类型、连接信息、命名规则或高级参数一旦泄漏，会影响 API/Proto/DB/业务事件的长期兼容。

#### 2.1.2 何时不强制定义 port

以下情况可以不新增 port，但必须保持边界清晰：

- 标准库、日志、JSON/YAML、测试工具等通用实现细节。
- 框架 glue code，例如 HTTP router、gRPC server bootstrap，前提是不进入产品语义。
- Kubernetes/client-go/controller-runtime/CRD 等平台控制面原生 API，但只能在 `bounded_direct` 模块内使用。
- PostgreSQL/RLS 等安全与事务基线能力，若被登记为 bounded persistence module，可在限定模块内直接使用 pgx。

判断口径：

```text
port 保护产品语义，不是为了把每个库都重新包一层。
bounded_direct 保护工程效率和平台最佳实践，但不得让底层 SDK 扩散到业务层。
```

组件名只能出现在：

- adapter 实现名。
- deploy profile 默认值。
- 运维文档中的默认部署说明。
- development record 的实现说明。

组件名不得出现在：

- 用户可见产品概念。
- OpenAPI 外部模型字段。
- Protobuf 业务语义字段。
- 服务层接口名。
- 跨服务业务事件语义。

### 2.2 Kubernetes 是唯一允许稳定绑定的底层平台 API

ANI 是 AI-Native Infrastructure，Kubernetes 是平台底层控制面。允许直接依赖：

- Kubernetes API machinery。
- CRD / controller-runtime。
- NetworkPolicy。
- StorageClass / PVC。

Kubernetes 的边界不是“所有代码都可以直接调用 K8s API”，而是：

```text
上层业务表达 ANI 产品意图；Kubernetes adapter/controller 使用原生 K8s API 实现该意图。
```

允许直接使用 Kubernetes 原生 API 的位置：

- `pkg/adapters/runtime/kubernetes_*`。
- `pkg/adapters/runtime/kubeovn_*`。
- controller/reconciler/preflight 等明确的 bounded module。
- provider 集成测试、真实集群 e2e profile。

禁止直接使用 Kubernetes 原生 API 的位置：

- Gateway handler。
- Core domain service / application service。
- ANI Services 业务服务。
- OpenAPI/Proto/SDK 暴露模型。

`ports` 不封装完整 Kubernetes SDK。Kubernetes 版本升级、feature gate、server-side apply、watch/list/informer、CRD schema 差异等应在 adapter/controller 内处理；port 只表达 ANI 稳定产品能力，例如 `WorkloadRuntime`、`WorkloadProviderDryRun`、`WorkloadProviderApply`、`WorkloadProviderStatusReader`、`NetworkProviderRenderer`。

但具体 K8s 生态组件仍必须适配化：

- KubeOVN 是默认网络实现，不是唯一网络抽象。
- HAMi 是默认异构算力虚拟化实现，不是业务层 GPU 语义。
- Volcano 是默认批调度实现，不是任务系统语义。

### 2.3 默认实现可以强，但边界必须松

ANI 可以在 v1.0.0 明确默认组合：

```text
CloudNativePG + NATS JetStream + Redis + MinIO + Milvus + Harbor
```

但必须满足：

- 业务代码通过 port/interface 访问能力。
- adapter 内部封装 SDK、连接串、topic/subject/key/collection/bucket 命名。
- 替换默认组件不需要改 OpenAPI/Proto。
- 替换默认组件不需要改业务服务的 service 层。
- 替换默认组件只影响 deploy profile、adapter 配置和少量实现包。

### 2.4 组件耦合等级

不同组件按角色、性能敏感度、替换概率和高级特性依赖程度分级治理。

| 耦合等级 | 适用场景 | 规则 |
|---|---|---|
| `port_required` | MinIO、Redis、Harbor、Dex/Keycloak 等可替换基础设施 | 业务服务必须依赖 `pkg/ports`，SDK 只允许在 adapter/bootstrap |
| `adapter_with_extensions` | Milvus、NATS JetStream 等既要抽象又要保留高级能力的组件 | 基础能力走 port，高级能力通过 ANI 自定义 extension interface 暴露，不泄漏 SDK 类型 |
| `bounded_direct` | Kubernetes、controller-runtime、CRD、GPU/CNI/推理运行时、PostgreSQL/RLS bounded persistence 等平台核心绑定 | 允许直接 SDK，但必须限定在明确 bounded module，并记录原因、性能/稳定性理由；不得扩散到 Gateway handler、Core domain service 或 ANI Services 业务服务 |
| `temporary_exception` | 迁移窗口内的历史直接依赖 | 必须写入 allowlist，标注 `migrate_by`，后续批次逐步消除 |

直接绑定不是违规，但必须是显式架构决策。默认情况仍应选择 `port_required`；选择 `bounded_direct` 时，需要说明为什么原生 API 更符合可用性、稳定性、性能或生态最佳实践。

---

## 三、架构分层

```
业务服务层
  model-service / kb-service / task-service / auth-service / gateway
        │
        ▼
ANI 能力接口层（ports）
  MetadataStore / ObjectStore / VectorStore / MessageBus / CacheStore / ImageRegistry / IdentityProvider
  GPUInventory / WorkloadRuntime / WorkloadProviderApply / NetworkProviderRenderer
        │
        ▼
适配器层（adapters）
  postgres / s3 / minio / milvus / nats / redis / harbor / dex / gpu / runtime
        │
        ▼
开源组件或外部服务
```

### 3.1 推荐代码结构

```text
repo/pkg/
├── ports/
│   ├── metadata.go
│   ├── object_store.go
│   ├── vector_store.go
│   ├── message_bus.go
│   ├── cache_store.go
│   ├── image_registry.go
│   ├── identity_provider.go
│   ├── workload_runtime.go
│   └── network_resources.go
├── adapters/
│   ├── postgres/
│   ├── s3/
│   ├── minio/
│   ├── milvus/
│   ├── nats/
│   ├── redis/
│   ├── harbor/
│   ├── dex/
│   └── runtime/
└── bootstrap/
    └── capability wiring
```

### 3.2 命名规则

接口命名必须使用能力名：

```go
type ObjectStore interface {}
type VectorStore interface {}
type MessageBus interface {}
```

实现命名可以使用组件名：

```go
type MinIOObjectStore struct {}
type MilvusVectorStore struct {}
type NATSMessageBus struct {}
```

业务服务禁止出现：

```go
minio.Client
milvus.Client
nats.Conn
redis.Client
harbor.Client
```

Kubernetes/provider-specific 类型同样不得出现在 Gateway handler、Core domain service 或 ANI Services 业务服务中：

```go
kubernetes.Clientset
client.Client
kubevirt.Client
unstructured.Unstructured
```

例外：

- adapter 包，尤其是 Kubernetes/KubeVirt/KubeOVN provider adapter。
- controller/reconciler/preflight 等已登记的 bounded module。
- bootstrap wiring 包。
- 专门的集成测试和真实集群 e2e profile。

---

## 四、能力接口边界

### 4.1 ObjectStore

业务语义：

- 创建或检查 bucket。
- 生成上传 URL。
- 生成下载 URL。
- 获取对象元数据。
- 流式读写对象。
- 删除对象。

禁止泄漏：

- MinIO bucket policy 细节。
- MinIO ETag 语义。
- MinIO presigned API 形态。

建议领域类型：

```text
ObjectRef {
  bucket_class: model | dataset | kb_doc | branding
  tenant_id
  object_key
  version
}
```

### 4.2 VectorStore

业务语义：

- 知识库 collection 初始化。
- 向量 upsert。
- 相似度搜索。
- 删除知识库向量。
- 获取 collection 健康状态。

禁止泄漏：

- Milvus collection 命名规则。
- Milvus index 参数。
- Milvus SDK 类型。

建议领域类型：

```text
VectorCollectionRef {
  tenant_id
  kb_id
}
```

### 4.3 MessageBus

业务语义：

- 发布任务。
- 订阅任务。
- ack/nack。
- 重试与死信。
- 发布领域事件。

禁止泄漏：

- NATS subject 作为业务服务唯一语义。
- JetStream stream/consumer 命名。
- NATS ack 细节进入 service 层。

允许：

- 在 `pkg/nats/messages.go` 过渡期保留默认 NATS payload，但后续应迁移到 provider-neutral `pkg/events`。

### 4.4 CacheStore

业务语义：

- 短 TTL 键值。
- 原子计数。
- 分布式限流基础操作。
- JWT/API token blocklist。

禁止泄漏：

- Redis key 格式散落在服务代码里。
- Redis 命令直接出现在业务服务层。

### 4.5 ImageRegistry

业务语义：

- 镜像仓库项目元数据。
- 镜像 tag 列表。
- 漏洞扫描摘要。
- 镜像代理 URL。

禁止泄漏：

- Harbor API 路径。
- Harbor 项目模型直接成为 ANI API 模型。

### 4.6 MetadataStore

MetadataStore 是特殊能力。ANI 当前强依赖 PostgreSQL RLS 实现多租户隔离，因此 v1.0.0 前允许将 PostgreSQL 作为安全基线默认实现。

但仍必须遵守：

- Repository interface 在业务服务内定义。
- SQL 实现隔离在 repo/adapters 层。
- service 层不拼 SQL。
- RLS tenant context 设置必须集中封装。

### 4.5 GPU 与运行时实例边界

异构 GPU 和实例运行时是 M3 模型部署之前的强制底座。

必须遵守：

- GPU 发现、分类和调度决策通过 `GPUInventory` port 表达。
- NVIDIA GPU Operator、GPU Feature Discovery、DCGM、HAMi、Ascend device plugin、Hygon DCU device plugin 等实现细节只能出现在 adapter、deploy profile、preflight 或 bounded runtime module。
- VM、普通容器、GPU 容器、推理、Notebook、Agent Sandbox、Batch Job 统一通过 `WorkloadRuntime` port 表达。
- 模型部署 Operator 可以在 bounded module 内直接使用 Kubernetes/controller-runtime，但业务服务不得直接拼装 Pod、Deployment、Job、RuntimeClass 或 KubeVirt VM。
- 传统 VM 是平台实例类型之一，不得被模型推理 Pod 隐式替代。
- 所有实例必须显式声明网络平面和存储附件。VM 与 Pod 的业务互通优先使用 `tenant_vpc`；平台服务互联、存储访问、管理入口不得被强行塞入租户 VPC，必须分别使用 `foundation_mesh`、`storage`、`management` 或 `public_ingress` 平面。
- `PlanningRuntime` 是进入真实 provider adapter 前的强制规划层，用于提前校验实例 spec、网络平面、存储附件、GPUInventory 依赖和生命周期状态机；真实 Kubernetes/KubeVirt adapter 必须复用该规划结果或提供等价校验。
- Kubernetes/KubeVirt provider renderer 可使用 provider 资源语义，但输出必须停留在 `WorkloadRenderer` 能力边界内，不得绕过 `WorkloadRuntime` 直接从业务服务创建资源。
- Provider renderer 只能输出 dry-run/review manifest；真实 apply/create 必须在后续受控 adapter 中接入 server-side dry-run、审计和权限检查后才允许。
- 所有 renderer 输出必须先通过 `WorkloadAdmission`，拒绝缺少 dry-run/租户/实例/网络平面声明的 manifest，并拒绝 hostNetwork、privileged 等高风险运行时配置。
- `WorkloadPlanAuditStore` 是实例执行链路的强制审计边界。计划、renderer 输出和 admission 结果必须先落入租户 RLS 审计表，后续 provider server-side dry-run/apply 才允许继续。
- `WorkloadProviderDryRun` 是真实 provider 执行前的最后验证边界。Kubernetes/KubeVirt adapter 必须使用 server-side dry-run，客户云/VM provider 必须提供等价验证 API；本地 dry-run 只能作为离线校验与测试替身。
- `WorkloadProviderApply` 是真实 provider apply/create 的受控执行边界。默认执行开关必须关闭；真实 adapter 必须校验用户、租户、权限证明、审计 ID、admission、provider dry-run、操作白名单和租户作用域，业务服务不得直接 apply Kubernetes/KubeVirt/客户云资源。
- `WorkloadStatusReconciler` 是 provider 状态回写与生命周期收敛边界。Kubernetes/KubeVirt/客户云状态 reader 只能出现在 adapter 内，并必须输出标准 provider observation，携带 tenant、instance、audit 和 resource refs 关联证据。
- `WorkloadProviderStatusReader` 是 provider 状态读取边界；`WorkloadInstanceOrchestrator` 是业务服务创建实例的统一入口，必须按 plan/render/admission/audit/dry-run/apply/status/reconcile 顺序编排，业务服务不得手动串联底层 provider 步骤。
- `WorkloadInstanceStore` 是实例持久化与查询恢复边界。实例查询、列表和进程重启后的状态恢复不得依赖 `PlanningRuntime` 内存状态，必须使用租户 RLS 持久化记录及 audit/resource refs 关联。
- `KubernetesProviderAdapter` 是 Kubernetes/KubeVirt provider adapter 边界。真实 client-go/controller-runtime/KubeVirt client 只能出现在 adapter-owned package 内，必须保留 `dryRun=All`、apply 默认关闭、audit、permission proof 和 resource-ref 关联。
- `WorkloadInstanceService` 是 VM、普通容器、GPU 容器的业务 API 边界。gRPC/HTTP API 可以包装该服务，但不得暴露 provider manifest、SDK 对象或 provider-specific status。
- `WorkloadInstanceOps` 是实例可视化运维操作边界。logs、events、metrics、terminal、exec、KubeVirt console/VNC 等 provider-specific 能力只能出现在 adapter 内，业务服务必须通过 `WorkloadInstanceService.Ops` 或上层 API 调用。
- M1 端到端集成剖面必须在离线 profile 中覆盖 VM、普通容器、GPU 容器的 create、lifecycle、query 和 ops 合同链路；进入真实集群 profile 时，只允许替换 provider adapter/client，不得绕过上述 port、audit、dry-run、admission 和 apply gate。
- `M1-INSTANCE-N` 是真实 Kubernetes/KubeVirt client 接入前的强制执行剖面。后续真实 client-go、dynamic client、controller-runtime 或 KubeVirt client 实现只能替换 `KubernetesProviderClient`，不得改变 `WorkloadInstanceOrchestrator`、`WorkloadPlanAuditStore`、`WorkloadAdmission`、`WorkloadProviderDryRun`、`WorkloadProviderApply`、`WorkloadProviderStatusReader`、`WorkloadStatusReconciler` 和 `WorkloadInstanceStore` 合同链路。
- `M1-INSTANCE-O` 提供第一版 adapter-owned `KubernetesRESTClient`。该实现只使用标准库 HTTP 调用 Kubernetes API，覆盖 `dryRun=All`、server-side apply、Deployment/Job/KubeVirt VM observe；业务服务仍不得导入 client-go、KubeVirt client 或 provider-specific 对象。
- `M1-INSTANCE-P` 将 provider 选择接入 bootstrap/config。默认 provider 必须保持 `local`；真实 Kubernetes REST provider 必须显式设置 `WORKLOAD_PROVIDER=kubernetes_rest`、`KUBERNETES_API_HOST`，并且 `WORKLOAD_PROVIDER_APPLY_ENABLED` 默认关闭。
- `M1-INSTANCE-Q` 将已存在实例的生命周期 provider 执行隔离到 `WorkloadInstanceLifecycleExecutor`。真实执行必须显式设置 `WORKLOAD_LIFECYCLE_PROVIDER=kubernetes_rest` 和 `WORKLOAD_LIFECYCLE_APPLY_ENABLED=true`，业务服务不得直接调用 Kubernetes scale/delete、KubeVirt start/stop 等 API。
- `M1-INSTANCE-R` 将可视化运维 provider 执行隔离到 `WorkloadInstanceOps` adapter。真实执行必须显式设置 `WORKLOAD_OPS_PROVIDER=kubernetes_rest` 和 `WORKLOAD_OPS_ENABLED=true`，业务服务不得直接调用 Kubernetes logs/events/metrics/exec API。
- `M1-E2E-B` 是 M1 真实 provider 路径的统一回归剖面。它必须覆盖 `KubernetesRESTClient`、`KubernetesProviderAdapter`、`KubernetesLifecycleExecutor`、`KubernetesInstanceOps` 和 `WorkloadInstanceService` 的 create/observe/lifecycle/ops 链路。

---

## 五、配置与部署规则

Helm values 和 installer 配置应从组件中心转为能力中心。

推荐结构：

```yaml
capabilities:
  objectStore:
    provider: minio
    mode: external
  vectorStore:
    provider: milvus
    mode: managed
  messageBus:
    provider: nats-jetstream
    mode: managed
  cacheStore:
    provider: redis
    mode: managed
  metadataStore:
    provider: postgresql
    mode: managed
  imageRegistry:
    provider: harbor
    mode: external
```

组件级配置只能挂在 provider 配置下：

```yaml
providers:
  minio: {}
  milvus: {}
  natsJetstream: {}
  redis: {}
  postgresql: {}
  harbor: {}
```

---

## 六、审查清单

每个新功能 PR 必须检查：

- 是否在业务服务中直接 import 第三方组件 SDK。
- 是否在 OpenAPI/Proto 中暴露组件名。
- 是否在数据库字段中把组件概念作为不可替换语义。
- 是否在 Helm values 顶层以组件而非能力表达依赖。
- 是否绕过 `WorkloadRuntime` / `GPUInventory` / Instance Fabric 直接创建 VM、Pod、Deployment、Job 或 GPU 调度约束。
- 是否为实例补齐网络平面、存储附件和生命周期策略。
- 是否为默认组件提供 adapter 边界。
- 是否记录替换该组件时影响哪些文件。

出现以下情况必须阻止合并：

- 业务 service 层直接调用 MinIO/Milvus/NATS/Redis/Harbor SDK。
- 外部 API 字段命名为 `minio_*`、`milvus_*`、`nats_*`、`harbor_*`。
- 新增组件配置没有 `provider`、`mode`、`external/managed` 边界。
- 为了方便而引入组件专用全局单例。

工程护栏：

- `repo/scripts/validate_component_imports.py` 检查组件 SDK 直接导入。
- `repo/architecture/component-import-allowlist.yaml` 记录允许保留的直接依赖及其 `coupling_level`。
- `make validate-architecture` 独立运行架构护栏。
- `make test` 默认先执行架构护栏。
- `temporary_exception` 和 `port_required` 例外必须写明迁移批次，不能作为长期豁免。
- `bounded_direct` 必须限定模块边界，并写明为什么直接绑定更符合可用性、稳定性或性能目标。
- Kubernetes/client-go/controller-runtime/KubeVirt/KubeOVN 直接依赖若新增到非 adapter/controller/preflight/e2e 路径，必须先更新 allowlist；业务层不得以“熟悉原生 API”为理由绕过 ANI port 和 service 边界。

---

## 七、迁移计划

### ARCH-ADAPTER-A / M1-ARCH-A

本设计批次完成：

- 明确开源组件松耦合强制原则。
- 定义能力接口与默认 adapter 边界。
- 更新开发规范和架构文档。

### ARCH-ADAPTER-B

本实现批次完成：

- 新增 `pkg/ports` 接口骨架。
- 新增 `pkg/adapters` 默认适配器包目录。
- 新增 `bootstrap.Capabilities` 与 `Deps.Ports`，允许新代码优先依赖 capability deps。
- 不改业务行为。

### ARCH-ADAPTER-C

第一批迁移已完成：

- auth-service JWT blocklist 从 Redis SDK 迁移到 `CacheStore`。
- task-service outbox publisher 从 NATS JetStream SDK publish 迁移到 `MessageBus`。
- 对应直接 SDK import 已从 allowlist 移除。

建议后续继续：

- `ARCH-ADAPTER-C-2` 已将 pgx/metadata 直接依赖按 `bounded_direct` 分类：PostgreSQL/RLS repository、outbox scanner、JWT blocklist persistence 允许直接使用 pgx，但必须限定在 persistence/bounded module 内。
- 普通业务服务新增 pgx 依赖仍需经过 allowlist 审查；能通过 `CacheStore`、`MessageBus`、`ObjectStore`、`VectorStore` 表达的能力不得退回 SDK 直连。
- 将未来 model/kb 的 MinIO/Milvus 调用限制在 `ObjectStore` / `VectorStore` adapter 内。

---

## 八、版本影响

`ARCH-ADAPTER-A / M1-ARCH-A` 本身不改变运行时代码，属于架构治理文档。

`ARCH-ADAPTER-B` 已新增 `pkg/ports`、`pkg/adapters` 与 bootstrap capability wiring，应按 `ANI-12` 判定为向后兼容的 `MINOR`。

后续一旦调整 Helm values 顶层结构，应按 `ANI-12` 判定：

- 修改 Helm values 结构且不兼容旧配置：`MAJOR` 风险。
- 内部重构不改变 API/Proto/Helm/DB：`PATCH` 或 `no-release-impact`。
