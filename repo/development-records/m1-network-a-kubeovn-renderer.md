# M1-NETWORK-A — KubeOVN Provider Rendering Boundary

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make validate-network-alpha EXIT:0，make validate-core-alpha EXIT:0，make validate-architecture EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐网络资源进入真实 provider 前的渲染边界：VPC/Subnet 可转换为 KubeOVN `Vpc`/`Subnet`，SecurityGroup 可转换为 Kubernetes `NetworkPolicy`，LoadBalancer 可转换为 Kubernetes `Service`。本切片只完成 provider 资源清单渲染和 bootstrap capability，不直接执行真实集群 apply。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `pkg/ports/network_resources.go` | 修改 | 新增 `NetworkProviderRenderer` port（能力抽象） |
| `pkg/adapters/runtime/kubeovn_network_renderer.go` | 新增 | 新增 KubeOVN/Kubernetes 网络资源清单渲染器 |
| `pkg/adapters/runtime/kubeovn_network_renderer_test.go` | 新增 | 覆盖 VPC/Subnet、SecurityGroup、LoadBalancer 渲染输出 |
| `pkg/bootstrap/deps.go` | 修改 | 将 provider renderer 暴露为 bootstrap capability |
| `scripts/validate_network_alpha_contract.py` | 修改 | 网络合同守卫新增 provider renderer 和 bootstrap 接线检查 |
| `Makefile` | 修改 | `make validate-network-alpha` 纳入 KubeOVN renderer 单元测试 |

## 完工标准达成

- [x] 网络资源 provider 渲染经过 `pkg/ports` / `pkg/adapters/runtime` 边界，不直接把 KubeOVN 细节暴露给 Services
- [x] VPC/Subnet 渲染包含租户 namespace、tenant label、CIDR、gateway 和 VPC 关联
- [x] SecurityGroup 渲染为 allow-list NetworkPolicy，并忽略 deny 规则，避免伪造 Kubernetes 不支持的 deny 语义
- [x] LoadBalancer 渲染为 Kubernetes Service，并保留 scheme、vpc_id、subnet_id 和 listener 端口
- [x] 合同守卫覆盖 renderer port（能力抽象）、runtime adapter 和 bootstrap capability

## 备注

下一步仍需补真实 provider 执行路径：dry-run/apply、状态读取、失败原因映射和与持久化表的 reconcile。
