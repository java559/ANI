# M1-STORAGE-A — Storage Provider Renderer

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make validate-storage-alpha EXIT:0，make validate-network-alpha EXIT:0，make validate-core-alpha EXIT:0，make validate-architecture EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐存储 provider 渲染边界：volumes 与 filesystems 可渲染为 Kubernetes `PersistentVolumeClaim`，objects 渲染为 `objectstore/ObjectMetadata` 元数据意图，避免把对象内容读写伪装成 Kubernetes 资源。PVC 渲染结果已能被本地 provider dry-run 识别。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `pkg/ports/storage_resources.go` | 修改 | 新增 `StorageProviderRenderer` port |
| `pkg/adapters/runtime/storage_renderer.go` | 新增 | 新增 Kubernetes/objectstore 存储 provider renderer |
| `pkg/adapters/runtime/provider_dryrun.go` | 修改 | 允许 Kubernetes `PersistentVolumeClaim` 进入 dry-run 校验 |
| `pkg/adapters/runtime/kubernetes_rest_client.go` | 修改 | 新增 PVC REST resource mapping |
| `pkg/bootstrap/deps.go` | 修改 | 将 `StorageRenderer` 暴露为 bootstrap capability |
| `scripts/validate_storage_alpha_contract.py` | 修改 | 合同守卫覆盖 renderer 和 bootstrap capability |
| `Makefile` | 修改 | `make validate-storage-alpha` 纳入 renderer 单元测试 |

## 完工标准达成

- [x] volume 渲染为 namespaced PVC，默认 ReadWriteOnce
- [x] filesystem 渲染为 namespaced PVC，默认 ReadWriteMany
- [x] object 渲染为 objectstore metadata intent，不直接依赖 MinIO SDK
- [x] PVC provider manifest 可进入本地 dry-run 校验
- [x] `StorageRenderer` capability 已在 bootstrap 中显式暴露

## 备注

后续已补 `StorageProviderDryRun` / `StorageProviderApply` 执行门控；ObjectMetadata 仍保持 objectstore metadata intent，避免业务层直接依赖底层对象存储 SDK。
