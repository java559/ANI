# Gateway 实例创建与观测链路接入 real K8s provider 路由切换

## Document Links
- PRD: repo/services/tasks/modules/prd/console/compute/prd-console-instance-observability.md
- UX: repo/services/tasks/modules/ux/console/compute/ux-console-instance-observability.md
- SPEC: repo/services/tasks/modules/spec/console/compute/spec-console-instance-observability.md

## Description

当前实例创建 API 在 Gateway 走 local 内存实现，创建成功但 K8s 里看不到。真实 K8s adapter 代码已有（`KubernetesProviderAdapter` / `KubernetesRESTClient` / `KubernetesInstanceOps`），路由层注入分支已预留（`demo_instances.go` 第 380-391 行 + `router.RegisterOptions.InstanceService` 字段），但 `main.go` 侧的装配函数（`newGatewayInstanceService` / `gatewayInstanceServiceRuntimeConfigFromEnv`）和 `bootstrap.ConnectInstanceService` helper 实际缺失，导致实例创建和实例观测都无法在真实 K8s 环境验证。本 Issue 补齐这条接入链路。

注：`development-records/README.md` 第 33 行已记录批次 `GATEWAY-INSTANCE-CREATE-REAL-K8S-PROVIDER-A` 为"已完成"，但工作区代码未落地，本 Issue 为真正实现该批次内容。

## Scope
- Product line: core
- Code paths allowed: repo/services/ani-gateway/、repo/pkg/bootstrap/、repo/pkg/adapters/runtime/

## Acceptance Criteria
- [ ] 新增 `repo/services/ani-gateway/instance_service_runtime.go`，实现 `gatewayInstanceServiceRuntimeConfig` 结构体与 `gatewayInstanceServiceRuntimeConfigFromEnv()`（读取 `WORKLOAD_PROVIDER`、`WORKLOAD_PROVIDER_APPLY_ENABLED` 及 K8s 连接参数，对齐 `server.go` 第 165-173 行 env 命名）
- [ ] 实现 `newGatewayInstanceService(ctx, cfg) (ports.WorkloadInstanceService, func(), error)`：
  - Provider 为 `""` / `"local"` / `"not_configured"` → 返回 `(nil, noopClose, nil)`，router 回退 local 内存闭环
  - Provider 为 `"kubernetes_rest"` → 通过 `bootstrap.ConnectInstanceService` 组装 real K8s provider 链路的 InstanceService，返回 `(service, closeFn, nil)`
  - 其他值 → 返回 `(nil, noopClose, ErrUnsupported)`
- [ ] 新增 `bootstrap.ConnectInstanceService` helper（连 DB → `NewCapabilitiesWithConfig(pool, nil, nil, cfg)` → 返回 `caps.InstanceService` + close），让 Gateway 间接使用 real K8s provider 而不违反组件边界守卫
- [ ] `main.go` 第 85-91 行现有调用链可编译通过；`defer closeInstanceService()` 在 nil 时为 noop
- [ ] `WORKLOAD_PROVIDER=kubernetes_rest` 启动时，`POST /api/v1/instances` 创建的实例在真实 K8s 集群可见（Pod / Workload 对象真实存在），不再仅停留在内存 store
- [ ] 实例观测前置耦合自动解决：`instanceForObservation` → `api.service.Get` 从真实 DB / K8s 读取实例记录
- [ ] 实例观测链路同步可切：`INSTANCE_OBSERVABILITY_PROVIDER=prometheus_kubernetes` 时 logs/events/metrics/exec/console 走 `PrometheusInstanceObservability`（`instance_observability_runtime.go` 已落地，本 Issue 仅验证集成可通，不修改该文件）
- [ ] 默认配置（env 未设置）保持 local 内存闭环，dev_profile 响应不变，CORE-DEV-PROFILE-A 边界契约不变
- [ ] operations 仍用 `LocalOperationStore`（`demo_instances.go` 第 381 行现有逻辑不动）
- [ ] 新增单元测试覆盖：env 切换（local / kubernetes_rest / unsupported）、注入逻辑、nil 回退路径
- [ ] `make test` 通过；`make validate-architecture` 通过；`go build ./services/ani-gateway/` 通过
- [ ] 不修改 OpenAPI `v1.yaml`（consume only）
- [ ] 真实 K8s 可见性需 live gate 验证（参考 Sprint 13 instance-observability-live-gate 模式）

## Dependencies
#1（Core console handler 补全）、#2（Core metrics 多 exporter 聚合）

## Type
core

## Priority
high

## Labels
core

## Batch
GATEWAY-INSTANCE-CREATE-REAL-K8S-PROVIDER-A

## SPEC Reference
- §2.1 System Context（Gateway → adapter 层 local / real 切换）
- §2.2.2 Core 端组件（多 exporter 聚合 adapter 待补，本 Issue 不含聚合实现，仅打通路由）
- §2.3.3 Core handler 调用链（observability.ListXxx ← Local / Prometheus adapter）
- §4.0 Frozen Facts（7 个 endpoint handler 已声明）
- §6.3 Failure Modes（Core adapter exporter 不可用降级）

## UX Reference
N/A（Core only）
