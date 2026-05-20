# M1-NETWORK-A — Network Provider Status Reader

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make validate-network-alpha EXIT:0，make validate-core-alpha EXIT:0，make validate-architecture EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐网络资源 provider 状态读取边界：VPC/Subnet/SecurityGroup/LoadBalancer 的真实 provider 资源现在可以通过 adapter 读取状态，并归一化为 ANI 网络资源的 `available/failed/deleting` 等状态与失败原因。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `pkg/ports/network_resources.go` | 修改 | 新增 `NetworkProviderStatusReader`、状态读取 request/result |
| `pkg/adapters/runtime/kubeovn_network_provider.go` | 修改 | KubeOVN 网络 provider adapter 新增 `Observe` 状态读取 |
| `pkg/adapters/runtime/kubernetes_rest_client.go` | 修改 | 新增 `ObserveNetworkResource`，通过 Kubernetes REST GET 映射网络资源状态 |
| `pkg/bootstrap/deps.go` | 修改 | 将 `NetworkStatus` 暴露为 bootstrap capability |
| `scripts/validate_network_alpha_contract.py` | 修改 | 合同守卫新增网络状态读取边界检查 |
| `Makefile` | 修改 | `make validate-network-alpha` 纳入网络状态读取定向测试 |

## 完工标准达成

- [x] 网络状态读取必须携带 tenant、user、resource、permission proof 和 apply 结果证据
- [x] Kubernetes/KubeOVN 原生状态只在 adapter 边界读取，不进入 Gateway handler 或 Services
- [x] provider 状态归一化为 ANI 网络资源状态和 reason
- [x] `NetworkStatus` capability 已在 bootstrap 中显式暴露
- [x] 合同守卫覆盖 port、adapter、REST client 和 bootstrap capability

## 备注

后续切片 `m1-network-a-status-reconcile.md` 已补齐状态回写闭环，把 `NetworkProviderStatusResult` 写回网络资源持久化表。
