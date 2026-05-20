# M1-STORAGE-A — Core API Dev Profile

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make validate-storage-alpha EXIT:0，make validate-network-alpha EXIT:0，make validate-core-alpha EXIT:0，make validate-architecture EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐存储资源首个 Core API 切片：volumes、filesystems、objects 已具备 Alpha API 契约、Gateway dev/local profile、租户隔离语义和合同守卫。该切片只提供 contract-compatible 本地 profile，不引入真实 Ceph/MinIO provider 执行。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `api/openapi/v1.yaml` | 修改 | 新增 `/volumes`、`/filesystems`、`/objects` path/schema/RBAC scope |
| `pkg/ports/storage_resources.go` | 新增 | 新增 `StorageService` 和存储资源 record/request 类型 |
| `pkg/adapters/runtime/storage_service.go` | 新增 | 新增本地存储 dev profile |
| `services/ani-gateway/internal/router/storage_resources.go` | 新增 | 新增 Gateway 存储资源路由和响应映射 |
| `pkg/bootstrap/deps.go` | 修改 | 将 `StorageResources` 暴露为 bootstrap capability |
| `scripts/validate_storage_alpha_contract.py` | 新增 | 新增存储 Alpha 合同守卫 |
| `Makefile` | 修改 | 新增 `make validate-storage-alpha` |

## 完工标准达成

- [x] volumes/filesystems/objects Core API 契约存在于 `api/openapi/v1.yaml`
- [x] Gateway dev/local profile 支持 create/list/get/delete
- [x] 存储状态包含 owner tenant、state、reason、created_at、updated_at
- [x] dev/local profile 状态机使用 `pending/available/failed/deleting/deleted`
- [x] 合同守卫覆盖 path/schema/RBAC scope、Gateway 注册、port、adapter 和 bootstrap capability

## 备注

下一步补 `M1-STORAGE-A` 持久化边界：metadata store、RLS 迁移和状态回写基础，为后续真实 provider 接入留出明确边界。
