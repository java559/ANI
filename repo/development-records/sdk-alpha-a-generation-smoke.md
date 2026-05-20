# SDK-ALPHA-A — Four-Language SDK Alpha Generation / Smoke

完成日期：2026-05-20
对应 Sprint：Sprint 3（2026-05-20 提前启动；计划窗口 2026-06-16 ~ 06-30）
验证结果：make gen-core-sdk EXIT:0，make validate-sdk-alpha EXIT:0，python scripts/validate_yaml.py api/openapi/v1.yaml api/openapi/services/v1.yaml EXIT:0，make validate-vector-alpha EXIT:0，make validate-architecture EXIT:0，make test EXIT:0，git diff --check EXIT:0

## 实现了什么

补齐 SDK Alpha 生成和 smoke 门禁：从 Core API 契约与 Services API 契约生成 Go/Python/TypeScript/Java 四语言 SDK Alpha 骨架，并保持 Core 与 Services 生成物分层隔离。Core SDK 过滤过渡期保留在 Core 契约里的 Services 业务路径，避免 SDK 层混入模型、推理、知识库业务资源。

## 关键文件改动

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `scripts/gen_sdk_alpha.py` | 新增 | 从 Core/Services API 契约生成四语言 SDK Alpha |
| `scripts/validate_sdk_alpha.py` | 新增 | 校验 SDK 元数据、Core/Services 分离和语言 smoke |
| `sdks/core/*` | 新增 | Core Go/Python/TypeScript/Java SDK Alpha 生成物 |
| `sdks/services/*` | 新增 | Services Go/Python/TypeScript/Java SDK Alpha 生成物 |
| `Makefile` | 修改 | 新增 `gen-core-sdk` 与 `validate-sdk-alpha` |

## 完工标准达成

- [x] Core SDK 从 `api/openapi/v1.yaml` 生成
- [x] Services SDK 从 `api/openapi/services/v1.yaml` 生成
- [x] Core SDK 不包含 Services 业务路径
- [x] Services SDK 不包含 Core 基础设施路径
- [x] Go SDK import/compile smoke 通过
- [x] Python SDK import smoke 通过
- [x] TypeScript SDK runtime import/syntax smoke 通过
- [x] Java SDK 在有 JDK 的环境执行 compile/run smoke；当前本机缺少 Java Runtime，验证脚本降级为 Java source smoke 并保留 compile 路径

## 备注

当前生成器是 SDK Alpha 骨架，优先保证契约来源、分层边界和 import/compile 门禁可重复。后续 SDK 加固可替换为正式 generator（如 oapi-codegen/openapi-generator/openapi-typescript-codegen），但应保留 `validate-sdk-alpha` 的边界检查。
