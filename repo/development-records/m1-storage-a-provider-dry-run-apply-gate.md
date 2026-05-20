# M1-STORAGE-A — Storage Provider Dry-Run / Apply Gate

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make validate-storage-alpha EXIT:0，make validate-network-alpha EXIT:0，make validate-core-alpha EXIT:0，make validate-architecture EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐存储 provider 执行门控：volumes 与 filesystems 渲染出的 Kubernetes PVC manifest 可进入 server-side dry-run，并在 apply 开关开启后执行 apply；默认 apply 仍关闭。objects 继续停留在 objectstore metadata intent，不经 Kubernetes provider 执行，后续通过 `ObjectStore` port 或专用 adapter 接入。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `pkg/ports/storage_resources.go` | 修改 | 新增 `StorageProviderDryRun` / `StorageProviderApply` port 和请求/结果类型 |
| `pkg/adapters/runtime/storage_provider.go` | 新增 | 新增 Kubernetes storage provider adapter、server-side dry-run 和默认关闭的 apply gate |
| `pkg/adapters/runtime/storage_provider_test.go` | 新增 | 覆盖 PVC dry-run、apply disabled、apply enabled 和 objectstore manifest 拒绝 |
| `pkg/bootstrap/deps.go` | 修改 | 将 `StorageDryRun` / `StorageApply` 暴露为 bootstrap capability |
| `scripts/validate_storage_alpha_contract.py` | 修改 | 合同守卫覆盖 storage provider execution boundary |
| `Makefile` | 修改 | `make validate-storage-alpha` 纳入 storage provider 单元测试 |

## 完工标准达成

- [x] PVC manifest 可进入 Kubernetes server-side dry-run 边界
- [x] apply 默认关闭，开启后必须携带已接受的 dry-run 结果
- [x] provider execution 必须校验 tenant/user/resource/permission proof
- [x] objectstore metadata intent 不被 Kubernetes provider 接管
- [x] `StorageDryRun` / `StorageApply` capability 已在 bootstrap 中显式暴露

## 备注

后续切片 `m1-storage-a-status-reconcile.md` 已补齐 provider 状态读取和 metadata 回写；object 状态仍应经 `ObjectStore` port 或专用 adapter，不直接在业务层依赖对象存储 SDK。
