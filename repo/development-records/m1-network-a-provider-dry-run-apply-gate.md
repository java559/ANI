# M1-NETWORK-A — Network Provider Dry-Run And Apply Gate

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make validate-network-alpha EXIT:0，make validate-core-alpha EXIT:0，make validate-architecture EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐网络资源 provider 执行前半段：VPC/Subnet/SecurityGroup/LoadBalancer 渲染出的 KubeOVN/Kubernetes manifest 现在可以进入 server-side dry-run，并通过默认关闭的 apply gate 受控执行。该切片仍不默认开启真实 apply，防止开发 profile 误写真实集群。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `pkg/ports/network_resources.go` | 修改 | 新增 `NetworkProviderDryRun`、`NetworkProviderApply` 及 request/result 类型 |
| `pkg/adapters/runtime/kubeovn_network_provider.go` | 新增 | 新增 KubeOVN 网络 provider dry-run/apply gate adapter |
| `pkg/adapters/runtime/kubernetes_rest_client.go` | 修改 | 新增网络资源 server-side dry-run/apply 路径映射，支持 KubeOVN Vpc/Subnet、NetworkPolicy、Service |
| `pkg/bootstrap/deps.go` | 修改 | 将网络 provider dry-run/apply 暴露为 bootstrap capability |
| `scripts/validate_network_alpha_contract.py` | 修改 | 合同守卫新增网络 provider 执行边界检查 |
| `Makefile` | 修改 | `make validate-network-alpha` 纳入网络 provider adapter 和 REST client 定向测试 |

## 完工标准达成

- [x] 网络 provider 执行必须携带 tenant、user、resource、operation、permission proof 和 dry-run 证据
- [x] server-side dry-run 复用 Kubernetes `dryRun=All` 语义
- [x] apply gate 默认关闭，只有显式 enable 才调用真实 apply
- [x] KubeOVN Vpc/Subnet 支持 cluster-scoped REST path，NetworkPolicy/Service 支持 namespaced REST path
- [x] 合同守卫覆盖 port、adapter、REST path 和 bootstrap capability

## 备注

下一步仍需补状态读取、失败原因映射和 reconcile，把 provider 结果回写到网络资源持久化表。
