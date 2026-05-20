# CORE-DEV-PROFILE-A — Core Dev/Local Profile Boundary Contract

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 ~ 2026-06-30）
验证结果：`make validate-core-dev-profile`、`make validate-sdk-alpha`、`make validate-architecture`、`make test`、`git diff --check` 通过

## 实现了什么

完成 Core dev/local profile 一致性收口：Core P0 API 的本地成功响应统一暴露 `dev_profile`，明确这是本地联调路径而不是真实 provider 执行成功。同步补上合同守卫，明确 `CORE-DEV-PROFILE-A` 不做 ANI Services 业务 mock，防止它被误解为 Services 业务 mock 或 Demo 假数据建设。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `api/openapi/v1.yaml` | 修改 | 新增 `CoreDevProfileInfo`，并挂接到 instances、network、storage、vector-store 响应 schema |
| `services/ani-gateway/internal/router/core_dev_profile.go` | 新增 | 统一定义 Core dev/local profile 响应标记 |
| `services/ani-gateway/internal/router/*_resources.go` | 修改 | Core P0 本地响应统一返回 `dev_profile` |
| `services/ani-gateway/internal/router/*_test.go` | 修改/新增 | 增加 `dev_profile` 单元断言 |
| `scripts/validate_core_dev_profile_contract.py` | 新增 | 校验文档命名、API 契约、Core/Services 边界、Gateway stub 和测试覆盖 |
| `Makefile` | 修改 | 新增 `make validate-core-dev-profile` |
| `CURRENT-SPRINT.md`、`ANI-06-开发计划.md`、`ANI-DOCS-INDEX.md`、`CLAUDE.md` | 修改 | 同步当前阶段和批次状态 |

## 完工标准达成

- [x] Core dev/local profile 不做 Services 业务 mock，边界已写入约束文档。
- [x] Core P0 本地成功响应可被测试识别为 local profile，不能伪装成 real provider。
- [x] Services API 不包含 Core P0 路径。
- [x] Gateway Core P0 路径不走无 owner/date 的 `NOT_IMPLEMENTED` stub。
- [x] `make validate-core-dev-profile` 通过。
- [x] `make validate-sdk-alpha`、`make validate-architecture`、`make test`、`git diff --check` 通过。

## 备注

`dev_profile` 是兼容性新增响应字段，用于联调、测试和 SDK 调用方识别执行剖面。真实 provider 路径后续可以使用同一 schema 返回 `mode=real`、`real_provider=true`，但不能复用 local profile 的成功语义冒充真实资源落地。
