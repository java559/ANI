# M1-STORAGE-A — Storage Provider Status / Reconcile

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make validate-storage-alpha EXIT:0，make validate-network-alpha EXIT:0，make validate-core-alpha EXIT:0，make validate-architecture EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐存储 provider 状态读取和回写闭环：Kubernetes PVC 状态可被读取并映射到 ANI storage state/reason，随后通过 `StorageStatusReconciler` 回写 metadata。对象存储状态仍保留在 `ObjectStore` port 后续接入，不在业务层直接访问底层对象存储 SDK。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `pkg/ports/storage_resources.go` | 修改 | 新增 `StorageProviderStatusReader` / `StorageStatusReconciler` port 和状态请求/结果类型 |
| `pkg/adapters/runtime/storage_provider.go` | 修改 | storage provider adapter 新增 status observation |
| `pkg/adapters/runtime/kubernetes_rest_client.go` | 修改 | 新增 PVC 状态读取和 storage state/reason 映射 |
| `pkg/adapters/runtime/storage_status_reconciler.go` | 新增 | 新增 storage metadata state/reason 回写闭环 |
| `pkg/adapters/runtime/storage_store.go` | 修改 | 新增 `UpdateResourceState` |
| `pkg/bootstrap/deps.go` | 修改 | 将 `StorageStatus` / `StorageReconcile` 暴露为 bootstrap capability |
| `scripts/validate_storage_alpha_contract.py` | 修改 | 合同守卫覆盖 status/reconcile boundary |
| `Makefile` | 修改 | `make validate-storage-alpha` 纳入 status/reconcile 单元测试 |

## 完工标准达成

- [x] PVC provider observation 可读取 Kubernetes resource status
- [x] PVC `Bound/Pending/Lost` 状态映射到 ANI storage state
- [x] metadata store 支持 storage resource state/reason 回写
- [x] reconcile 校验 provider、resource refs 和 observation identity
- [x] `StorageStatus` / `StorageReconcile` capability 已在 bootstrap 中显式暴露

## 备注

`M1-STORAGE-A` 主链路已覆盖 Core API dev profile、持久化、provider 渲染、dry-run/apply gate、status observation 和 metadata reconcile。后续对象内容读写、对象状态观察仍应经 `ObjectStore` port 或专用 adapter 演进。
