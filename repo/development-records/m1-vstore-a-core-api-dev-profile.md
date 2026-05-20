# M1-VSTORE-A — Vector Store Core API Dev Profile

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make validate-vector-alpha EXIT:0，make validate-storage-alpha EXIT:0，make validate-network-alpha EXIT:0，make validate-core-alpha EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make validate-architecture EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐 vector-stores Core API dev profile：向量存储作为 Core 基础设施资源进入 API 契约，Gateway 支持 create/list/get/delete/search，搜索在本地 dev profile 中返回空结果但保持请求校验和响应结构。Milvus 访问仍被限制在 `VectorStore` port 后的 adapter 边界内，业务层和 Gateway 不直接依赖 Milvus SDK。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `api/openapi/v1.yaml` | 修改 | 新增 `/vector-stores` 与 `/vector-stores/{id}/search` Core API 契约 |
| `pkg/ports/vector_store.go` | 修改 | 新增 `VectorStoreService`、资源记录和 create/list/get/search 请求类型 |
| `pkg/adapters/runtime/vector_store_service.go` | 新增 | 新增本地 vector store dev profile |
| `services/ani-gateway/internal/router/vector_store_resources.go` | 新增 | 新增 Gateway vector-stores 路由和响应转换 |
| `pkg/bootstrap/deps.go` | 修改 | 将 `VectorStoreResources` 暴露为 bootstrap capability |
| `scripts/validate_vector_alpha_contract.py` | 新增 | 新增 vector-stores API 合同守卫 |
| `Makefile` | 修改 | 新增 `make validate-vector-alpha` |

## 完工标准达成

- [x] `POST /api/v1/vector-stores` 可创建 dev/local profile 资源
- [x] `GET /api/v1/vector-stores/{id}` 可返回租户内资源
- [x] `DELETE /api/v1/vector-stores/{id}` 返回 deleted 状态
- [x] `POST /api/v1/vector-stores/{id}/search` 返回 contract-compatible 结果
- [x] Milvus 访问仍经 `VectorStore` port，不在 Gateway 或业务层直接 import Milvus SDK

## 备注

当前切片先完成 Core API 与 dev profile 解锁。后续如接真实 Milvus，可在 `pkg/adapters/vectorstore` 下实现 `VectorStore` adapter，并由 `VectorStoreResources` 注入 backend，不改变 Gateway/API 契约。
