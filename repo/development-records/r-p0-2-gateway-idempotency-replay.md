# R-P0-2 Gateway Idempotency Replay

> 记录类型：Sprint 14 execution batch / Core resilience P0
> 完成日期：2026-06-23
> 范围：ANI Core / ani-gateway middleware idempotent response replay

## 目标

把 HTTP mutating 请求的幂等响应重放收敛到 gateway middleware。对同一 `(tenant_id, method, path, idempotency_key)` 的重复请求，首次完成后回放首次响应；首次仍在处理中时，重复请求返回 `409 IDEMPOTENCY_IN_PROGRESS`。这样重复 HTTP 请求不会再次进入 handler 触发副作用。

## 实现

- 新增 `services/ani-gateway/internal/middleware/idempotency.go`：
  - `Idempotency(store GatewayStore)` 仅作用于 `POST` / `PUT` / `PATCH`。
  - 优先读取 `Idempotency-Key` header；没有 header 时读取 JSON body 的 `idempotency_key`。
  - 使用 R-P0-0 的 `GatewayStore.SetNX` 写入 `processing` 哨兵，TTL 为 24h。
  - 首次 handler 完成后缓存 `{status_code, content_type, body}`。
  - 重复完成请求直接回放缓存响应，并加 `Idempotent-Replay: true`。
  - 重复处理中请求返回 `409 IDEMPOTENCY_IN_PROGRESS`。
- 修改 `services/ani-gateway/internal/middleware/chain.go`：
  - 执行顺序更新为 `RequestID -> Auth -> RBAC -> RateLimit -> Idempotency -> Audit -> Route`。
- 新增 `services/ani-gateway/internal/middleware/idempotency_test.go`：
  - `TestIdempotentReplayReturnsSameResponse` 覆盖同 key 重放与 handler 只执行一次。
  - `TestConcurrentIdempotentInProgressReturns409` 覆盖 processing 哨兵并发语义。
- 新增 `make validate-gateway-idempotency`：
  - 固定 R-P0-2 本地逻辑 gate。

## 边界

- 本批不改 Core OpenAPI，不改 Services OpenAPI，不新增 Services 逻辑。
- 本批没有删除 observability/vector/registry 等服务层已有局部幂等结构；它们保留为局部防线。HTTP 重复请求由 gateway replay 在 handler 前拦截。
- 本批为 local/logic verified；未执行真实 Redis 多副本、进程重启、故障注入或 live gate，不声明 production ready。

## TDD 证据

红灯：

```bash
GOCACHE=/private/tmp/ani-go-build GOMODCACHE=/Users/zhangfan/ANI/repo/.cache/gomod go test ./services/ani-gateway/internal/middleware -run Idempotent -v
```

结果：失败，`Idempotency` 未定义。

绿灯：

```bash
GOCACHE=/private/tmp/ani-go-build GOMODCACHE=/Users/zhangfan/ANI/repo/.cache/gomod go test ./services/ani-gateway/internal/middleware -run Idempotent -v
make validate-gateway-idempotency
```

结果：以上命令通过。

## 收口验证

```bash
GOCACHE=/private/tmp/ani-go-build GOMODCACHE=/Users/zhangfan/ANI/repo/.cache/gomod go test ./services/ani-gateway/... -run 'TestIdempotent|TestRateLimit|TestGatewayStore|TestAuthPublicPaths|TestAuthProtectedPaths|TestInferPermission' -v
make validate-gateway-idempotency
make validate-architecture
make validate-doc-entrypoints
make test
git diff --check
```

结果：以上命令通过。

## 后续

- R-P0-3 可继续建立 `pkg/adapters/resilience` 超时骨架。
- R-P1-5 前仍需修正 Kubernetes REST client 错误分类；本批不触碰 adapter 重试/断路器范围。
