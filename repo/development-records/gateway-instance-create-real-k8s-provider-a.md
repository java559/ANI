# GATEWAY-INSTANCE-CREATE-REAL-K8S-PROVIDER-A — 实现笔记

> Issue: #011 — Gateway 实例创建与观测链路接入 real K8s provider 路由切换
> Batch: GATEWAY-INSTANCE-CREATE-REAL-K8S-PROVIDER-A
> Product line: core
> Date: 2026-07-08
> PRD: `repo/services/tasks/modules/prd/console/compute/prd-console-instance-observability.md`
> UX: N/A（Core only）
> SPEC: `repo/services/tasks/modules/spec/console/compute/spec-console-instance-observability.md`
> Code paths: `repo/services/ani-gateway/`、`repo/pkg/bootstrap/`、`repo/pkg/adapters/runtime/`

---

## Verification Commands Run

| Command | Result |
|---------|--------|
| `go build ./pkg/adapters/runtime/ ./pkg/bootstrap/ ./services/ani-gateway/` | ✅ 通过 |
| `make validate-architecture` | ✅ 通过（component import guard passed） |
| `go test ./pkg/adapters/runtime/ ./services/ani-gateway/... ./services/ani-gateway/internal/middleware/ ./services/ani-gateway/internal/router/` | ✅ 全部通过 |
| `git diff --check` | ✅ 通过 |
| `make test`（全量） | ⚠️ 失败——预存 `pkg/bootstrap/deps_test.go:22` 使用 `t.Context()`（Go 1.24+ API，本地 Go 版本不匹配），与本批次无关 |
| Live gate（真实 K8s 可见性） | ⏳ 需在配置 `WORKLOAD_PROVIDER=kubernetes_rest` + K8s 凭证的真实集群验证，本地已验证 Pod running + 状态同步 |

---

## 1. Design Decisions

### D1. Lazy re-observe on Get/List 替代后台 reconciler

**Ambiguity:** SPEC §2.3.3 只定义了观测 handler 的调用链（`demoInstanceAPI.listXxx → observability.ListXxx → adapter`），没有定义实例创建后状态如何从 K8s 同步回 DB。SPEC §6.3 Failure Modes 也没覆盖"Pod 已 running 但 DB 仍 provisioning"的场景。

**Choice:** 在 `LocalInstanceService.Get`/`List` 中对非终态实例（`provisioning`/`pending`/`starting`）触发 lazy re-observe：从 K8s 读真实状态 → reconciler 计算状态变更 → `UpsertStatus` 回写 DB。终态实例（`running`/`stopped`/`failed`/`deleted`）不触发，避免稳态实例每次查询都打 K8s。

**Rationale:** 引入后台 reconciler controller 会显著扩大改动范围（需要 informer/watch、leader election、requeue），超出 issue-011 "打通路由链路"的 scope。Lazy re-observe 在前端轮询场景下天然收敛：用户刷新列表时触发一次 K8s 读取，状态自动同步，无需额外基础设施。

### D2. Manifest 顺序：主资源在前、Secret 在后

**Ambiguity:** SPEC 未定义 Workload Identity Secret manifest 在 `Render` 输出中的位置。

**Choice:** `dryrun_renderer.Render` 返回 `[primary, identitySecret]`（主资源在前，Secret 在后）。

**Rationale:** `KubernetesRESTClient.Observe` 硬编码读 `ResourceRefs[0]` 作为观测对象。如果 Secret 排在前面，`Observe` 会读 Secret 而非 Deployment——`phaseFromKubernetesObject` 对 Secret 无匹配分支，返回空 phase → re-observe 永远失败 → 状态永远停在 provisioning。主资源在前保证 `ResourceRefs[0]` 是 Deployment。K8s server-side apply 在调度前 PATCH 完所有 manifest，Secret 在同批次写入，pod 启动时 Secret 已存在，mount 顺序无风险。

### D3. `ConnectInstanceService` helper 放在 bootstrap 层

**Ambiguity:** SPEC §2.1 描述 Gateway→adapter local/real 切换，但没规定装配代码放在哪一层。

**Choice:** 在 `pkg/bootstrap/db.go` 新增 `ConnectInstanceService`（连 DB → `NewCapabilitiesWithConfig(pool, nil, nil, cfg)` → 返回 `caps.InstanceService` + `pool.Close`），Gateway 通过它间接使用 real K8s provider。

**Rationale:** `NewCapabilitiesWithConfig` 包含完整的 capabilities 装配（orchestrator + statusReader + reconciler + store + lifecycle + identity），直接在 Gateway 侧重装配会违反组件边界守卫（`make validate-architecture`）并复制大量逻辑。Bootstrap 层是已有 capabilities 工厂的正确归属，Gateway 只需一个轻量 helper 调用它。

---

## 2. Deviations

### V1. 新增 Secret manifest 生成与 admission/dryRun/apply 放行

**Spec:** SPEC §2.2.2 列出的 Core 端待补组件只有 console session handler、port 扩展、local adapter、多 exporter 聚合 adapter。未提及 Workload Identity Secret manifest 生成。

**Implementation:** 发现 real K8s 模式下 `KubernetesProviderAdapter` 的 `Render` 只生成 Deployment manifest，但 Deployment env 通过 `secretKeyRef` 引用 `ani-wi-...` Secret——该 Secret 从未被创建到 K8s，导致 Pod 启动时找不到 Secret 而 CrashLoopBackOff。新增 `renderWorkloadIdentitySecret` 在 `Render` 中生成 Secret manifest，并让 admission（`admission.go`）、dryRun（`provider_dryrun.go`）、apply（`provider_apply.go` + `kubernetes_provider_adapter.go`）放行 `Secret` kind（加入 allowedKinds、跳过 mixed-provider 检查）。

**Why:** 这是 real K8s 模式下实例创建能成功的必要条件——没有 Secret，Pod 无法挂载 Workload Identity token。SPEC 未覆盖是因为 SPEC 聚焦观测层，实例创建的 manifest 细节不在其范围。

### V2. auth.go 注入 `types.TenantContext` 到 Go context.Context

**Spec:** SPEC §7.1 定义 RBAC scope 由 Core middleware 校验，但未提及 Go `context.Context` 层的 tenant 注入。

**Implementation:** 发现 auth 中间件 dev/bearer/apikey 三个分支只往 Hertz `RequestContext` 设了 `tenant_id`（字符串），没往 Go `context.Context` 注入 `types.TenantContext`。切到 `kubernetes_rest` 走 DB 后，`MetadataInstanceStore.List` → `WithTenantTx` → `SetDBTenant(ctx, tx)` → `types.FromContext(ctx)` 找不到 `TenantContext` → panic（`tenant context missing`）→ 500 + 空 body。新增 `withTenantContext`（dev 宽松回退）+ `withTenantContextStrict`（认证分支 fail-closed）注入 `types.TenantContext`。

**Why:** 这是 real K8s 模式下 DB RLS（Row-Level Security）能工作的必要条件——`SetDBTenant` 依赖 `types.FromContext` 拿到 `TenantContext.TenantID` 设 `SET LOCAL`。local 内存模式不触发因为内存 store 不走 `WithTenantTx`。SPEC 未覆盖是因为 SPEC 假设观测层走 local adapter，不涉及 DB tenant tx。

### V3. `demoManifests` 对 Secret 脱敏

**Spec:** SPEC §4.0 Frozen Schemas 列出各 endpoint 的响应 schema，`POST /api/v1/instances` 不在观测 7 endpoint 冻结表内。

**Implementation:** 发现新增的 Workload Identity Secret manifest 的 `Content` 包含明文 token（`stringData.token = KeyValue`），`demoManifests` 原样拷贝进 HTTP 创建响应，导致 token 泄露。新增对 `Kind=="Secret"` 的 manifest 脱敏（`Content=""`）。

**Why:** 安全边界——HTTP 响应是不可信边界，明文 token 不应经 API 响应返回。这是 review-it 发现的 CRITICAL 问题（不是 SPEC 要求，是安全要求）。

### V4. 测试文件修复（预存问题）

**Spec:** N/A

**Implementation:** `demo_instances_test.go:581` 存在预存语法损坏（`TestDemoInstanceServiceRealShellExecutesCommand` 函数声明丢失，孤立 `t.Skipf` 导致整个 router 测试包编译失败）。`go build` 通过是因为不编译 `_test.go`。重构该函数 + 加 `os/exec` import。

**Why:** review-it 发现 CRITICAL——测试包编译失败导致所有 router 测试无法运行，必须修复才能验证改动。

---

## 3. Tradeoffs

### T1. Lazy re-observe vs 后台 controller

| 方案 | 优点 | 缺点 |
|------|------|------|
| **Lazy re-observe（选定）** | 改动小、零新基础设施、前端轮询天然收敛、终态不打 K8s | 非终态实例每次 Get/List 打一次 K8s（可接受——非终态是短暂的）；无实例时不会主动同步 |
| 后台 reconciler controller | 主动收敛、不依赖前端访问 | 需 informer/watch、leader election、requeue 机制，改动范围远超 issue-011 scope |

**选定理由:** issue-011 scope 是"打通路由链路"，不是"实现状态同步基础设施"。Lazy re-observe 在用户可见路径上收敛，满足 AC 第 6 条"实例观测前置耦合自动解决"。

### T2. Secret manifest 排序（主资源在前 vs Secret 在前）

| 方案 | 优点 | 缺点 |
|------|------|------|
| Secret 在前 | 语义上"依赖先创建" | `Observe` 读 `ResourceRefs[0]`=Secret → phase 计算失败 |
| **主资源在前（选定）** | `Observe` 读 `ResourceRefs[0]`=Deployment → phase 正确 | 语义上"依赖后创建"——但 K8s server-side apply 同批次 PATCH，无实际顺序问题 |

**选定理由:** 正确性优先——`Observe` 硬编码 `ResourceRefs[0]`，主资源在前是唯一能正确观测的顺序。

### T3. 认证分支 fail-closed vs fail-open

| 方案 | 优点 | 缺点 |
|------|------|------|
| fail-open（原实现：回退 dev 默认） | dev 模式韧性高 | 已认证分支若 auth 返回非 UUID → 静默换租户 → 跨租户风险 |
| **fail-closed（选定）** | 认证分支安全——非法 UUID 返回 401 | dev 分支需单独保留 fail-open |

**选定理由:** 安全优先——认证分支（bearer/apikey）已通过 auth 服务校验，tenantID 应为合法 UUID；若不是，说明 auth 服务异常，应拒绝而非静默换租户。dev 分支保留 fail-open 保持开发韧性。

---

## 4. Open Questions

### Q1. 真实 K8s 可见性需 live gate 验证

AC 第 5 条要求 `WORKLOAD_PROVIDER=kubernetes_rest` 时创建的实例在真实 K8s 集群可见。本地已验证 Pod running + 状态同步（provisioning→running），但正式 live gate 需在配置完整 K8s 凭证 + DB + Prometheus 的真实集群执行 `POST /api/v1/instances` 后检查 Pod/Workload 对象。参考 Sprint 13 instance-observability-live-gate 模式。

### Q2. DB migration 完整性

本地验证发现 DB 需预先执行 `deploy/migrations/20260502_002_operations_idempotency.sql`（建 `workload_instance_operations` 表）+ `deploy/real-k8s-lab/instance-create-bootstrap-tables.sql`（建 `instance_plan_audits` + `workload_instance_operation_steps`）+ `deploy/real-k8s-lab/auth-dex-production-db-init.sql`（建 `tenants` 表 + 默认租户）。这些 migration 是否应纳入 deployment 自动化流程？还是仍由运维手动执行？

### Q3. 后台 reconciler 何时引入

Lazy re-observe 依赖前端访问触发同步。若用户不刷新列表，非终态实例状态不会主动收敛。后续是否需要后台 controller 主动同步？这取决于产品对状态收敛延迟的容忍度。

---

## Files Changed (本批次新增/修改)

| File | Status | Summary |
|------|--------|---------|
| `services/ani-gateway/instance_service_runtime.go` | NEW | env 切换 config + `newGatewayInstanceService`（local→nil, kubernetes_rest→ConnectInstanceService, unsupported→error） |
| `services/ani-gateway/instance_service_runtime_test.go` | NEW | 7 个单元测试覆盖 env 切换/注入/nil 回退 |
| `pkg/bootstrap/db.go` | MODIFIED | 新增 `ConnectInstanceService` helper |
| `pkg/adapters/runtime/instance_service.go` | MODIFIED | 新增 `statusReader`/`reconciler` 字段 + lazy re-observe on Get/List |
| `pkg/adapters/runtime/dryrun_renderer.go` | MODIFIED | 新增 `renderWorkloadIdentitySecret` + manifest 顺序（主资源在前） |
| `pkg/adapters/runtime/admission.go` | MODIFIED | 放行 `Secret` kind + 豁免 workload label 检查 |
| `pkg/adapters/runtime/provider_dryrun.go` | MODIFIED | 放行 `Secret` kind + 跳过 mixed-provider 检查 |
| `pkg/adapters/runtime/provider_apply.go` | MODIFIED | 跳过 Secret 的 mixed-provider 检查 |
| `pkg/adapters/runtime/kubernetes_provider_adapter.go` | MODIFIED | 跳过 Secret 的 mixed-provider 检查 |
| `pkg/bootstrap/deps.go` | MODIFIED | 注入 `WithInstanceStatusReader`/`WithInstanceStatusReconciler` |
| `services/ani-gateway/internal/middleware/auth.go` | MODIFIED | 注入 `types.TenantContext` + `withTenantContextStrict`（认证分支 fail-closed） |
| `services/ani-gateway/internal/router/demo_instances.go` | MODIFIED | `demoManifests` 对 Secret 脱敏 |
| `services/ani-gateway/internal/router/demo_instances_test.go` | MODIFIED | 修复预存语法损坏 + manifest 数量断言更新 |
| `services/ani-gateway/main.go` | MODIFIED | 调用 `newGatewayInstanceService` + `defer closeInstanceService()`（预存 wiring） |
| `services/ani-gateway/internal/router/router.go` | MODIFIED | `RegisterOptions.InstanceService` 字段（预存） |
| `development-records/README.md` | MODIFIED | 批次索引条目（预存） |
