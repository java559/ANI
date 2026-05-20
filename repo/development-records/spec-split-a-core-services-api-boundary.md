# SPEC-SPLIT-A — Core/Services API Boundary

完成日期：2026-05-20
对应 Sprint：Sprint 4（2026-05-20 提前启动；计划窗口 2026-07-01 ~ 2026-07-15）
验证结果：`make validate-spec-split`、`make validate-sdk-alpha`、`python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml`、`python scripts/validate_sprint3_closure.py`、`make test`、`make validate-architecture`、`git diff --check` 通过

## 实现了什么

完成 Core/Services API 分层收口的首个切片：`/models`、`/inference-services`、`/knowledge-bases` 迁移到 Services API，Core API 不再承载这些业务路径。Gateway 过渡 stub 改挂 `/api/v1/svc/*`，Core SDK 和 Services SDK 由各自 API 契约自然生成，不再靠 Core SDK 排除列表维持分层。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `api/openapi/v1.yaml` | 修改 | 移除 Services 业务路径和过渡 tags |
| `api/openapi/services/v1.yaml` | 修改 | 新增 Services API base path、业务 path/schema/responses |
| `services/ani-gateway/internal/router/router.go` | 修改 | Services stub 改挂 `/api/v1/svc` |
| `services/ani-gateway/internal/middleware/rbac.go` | 修改 | RBAC resource 推导跳过 `/svc` 前缀 |
| `scripts/gen_sdk_alpha.py` | 修改 | Core SDK 不再使用 Services path 排除列表 |
| `scripts/validate_sdk_alpha.py` | 修改 | SDK metadata 校验按真实 API 契约生成 |
| `scripts/validate_spec_split_contract.py` | 新增 | 校验 Core/Services API、Gateway 和 SDK metadata 分层 |
| `Makefile` | 修改 | 新增 `make validate-spec-split` |

## 完工标准达成

- [x] Core API 不包含 `/models`、`/inference-services`、`/knowledge-bases`。
- [x] Services API 维护 models、inference-services、knowledge-bases 路径。
- [x] Gateway Services 过渡 stub 挂载到 `/api/v1/svc/*`。
- [x] Core SDK metadata 不包含 Services 业务路径；Services SDK metadata 包含迁移后的业务路径。
- [x] `make validate-spec-split` 通过。
- [x] `make validate-sdk-alpha`、`make test`、`make validate-architecture`、`git diff --check` 通过。

## 备注

本批次只做分层边界与契约收口，不实现 Services 业务逻辑。Services 业务 mock、模型仓库、推理服务和知识库实现仍由 ANI Services 团队负责。
