# M1-NETWORK-A — Network Status Reconcile

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make validate-network-alpha EXIT:0，make validate-core-alpha EXIT:0，make validate-architecture EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐网络资源状态回写闭环：provider 状态读取结果现在会经过 network reconcile 校验，再写回 VPC/Subnet/SecurityGroup/LoadBalancer 的持久化状态与失败原因。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `pkg/ports/network_resources.go` | 修改 | 新增 `NetworkStatusReconciler`、`NetworkReconcileRequest/Result` 和状态更新 request |
| `pkg/adapters/runtime/network_status_reconciler.go` | 新增 | 新增本地网络状态 reconcile adapter |
| `pkg/adapters/runtime/network_store.go` | 修改 | 新增 `UpdateResourceState`，按资源类型更新 state/reason/updated_at |
| `pkg/bootstrap/deps.go` | 修改 | 将 `NetworkReconcile` 暴露为 bootstrap capability |
| `scripts/validate_network_alpha_contract.py` | 修改 | 合同守卫新增网络 reconcile 边界检查 |
| `Makefile` | 修改 | `make validate-network-alpha` 纳入网络 reconcile 定向测试 |

## 完工标准达成

- [x] reconcile 必须校验 apply 已执行、provider/resource refs 与 observation 对齐
- [x] 状态写回只通过 `NetworkResourceStore` port，不在 Gateway 或 Services 中直接操作 provider/DB 细节
- [x] metadata adapter 支持按网络资源类型更新 `state/reason/updated_at`
- [x] `NetworkReconcile` capability 已在 bootstrap 中显式暴露
- [x] 合同守卫覆盖 port、store、reconciler 和 bootstrap capability

## 备注

`M1-NETWORK-A` 的 Core API dev profile、持久化、provider 渲染、dry-run/apply、状态读取和状态回写主链路已闭环。下一步可进入该批次收尾审查，或按 Sprint 3 优先级切换到 `M1-STORAGE-A`。
