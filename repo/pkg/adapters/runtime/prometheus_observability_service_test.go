package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

// fakeInstanceLookup 模拟实例记录查询，返回预设的 WorkloadInstanceRecord。
type fakeInstanceLookup struct {
	record ports.WorkloadInstanceRecord
	err    error
}

func (f *fakeInstanceLookup) Get(_ context.Context, _ ports.WorkloadInstanceGetRequest) (ports.WorkloadInstanceRecord, error) {
	return f.record, f.err
}

// newTestPrometheusObservabilityService 创建带 mock Prometheus HTTP server 的测试实例。
func newTestPrometheusObservabilityService(t *testing.T, handler http.HandlerFunc, lookup InstanceLookup) *PrometheusObservabilityService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	service, err := NewPrometheusObservabilityService(PrometheusObservabilityServiceConfig{
		PrometheusURL:  server.URL,
		InstanceLookup: lookup,
		Now:            func() time.Time { return time.Unix(1780000000, 0).UTC() },
	})
	if err != nil {
		t.Fatalf("NewPrometheusObservabilityService error = %v", err)
	}
	return service
}

func TestPrometheusObservabilityServiceRewritePromQLLabels(t *testing.T) {
	lookup := &fakeInstanceLookup{record: ports.WorkloadInstanceRecord{
		TenantID:   "00000000-0000-0000-0000-000000000001",
		InstanceID: "inst_1",
		Name:       "test-kjs-container-6",
	}}
	service, err := NewPrometheusObservabilityService(PrometheusObservabilityServiceConfig{
		PrometheusURL:  "http://prometheus:9090",
		InstanceLookup: lookup,
	})
	if err != nil {
		t.Fatalf("NewPrometheusObservabilityService error = %v", err)
	}

	// 前端把 instance_id 同时注入 namespace 和 pod 占位符
	input := `100 * (container_memory_working_set_bytes{namespace="inst_1",pod="inst_1"} / container_spec_memory_limit_bytes{namespace="inst_1",pod="inst_1"})`
	rewritten, err := service.rewritePromQLLabels(context.Background(), "00000000-0000-0000-0000-000000000001", input)
	if err != nil {
		t.Fatalf("rewritePromQLLabels error = %v", err)
	}

	// namespace 应被重写为真实 namespace（精确匹配），pod 应被重写为正则
	if !strings.Contains(rewritten, `namespace="ani-tenant-00000000-0000-0000-0000-000000000001"`) {
		t.Fatalf("namespace not rewritten: %s", rewritten)
	}
	if !strings.Contains(rewritten, `pod=~"^test-kjs-container-6(-.*)?$"`) {
		t.Fatalf("pod not rewritten to regex: %s", rewritten)
	}
	// 原始精确匹配不应残留
	if strings.Contains(rewritten, `namespace="inst_1"`) {
		t.Fatalf("original namespace label still present: %s", rewritten)
	}
	if strings.Contains(rewritten, `pod="inst_1"`) {
		t.Fatalf("original pod label still present: %s", rewritten)
	}
}

func TestPrometheusObservabilityServiceQueryForwardsToPrometheus(t *testing.T) {
	lookup := &fakeInstanceLookup{record: ports.WorkloadInstanceRecord{
		TenantID:   "00000000-0000-0000-0000-000000000001",
		InstanceID: "inst_1",
		Name:       "test-kjs-container-6",
	}}
	var capturedQuery string
	service := newTestPrometheusObservabilityService(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []map[string]any{
					{
						"value": []any{float64(1780000000), "42.5"},
					},
				},
			},
		})
	}, lookup)

	result, err := service.Query(context.Background(), ports.ObservabilityQueryRequest{
		TenantID: "00000000-0000-0000-0000-000000000001",
		Query:    `container_memory_working_set_bytes{namespace="inst_1",pod="inst_1"}`,
	})
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	// 验证转发到 Prometheus 的查询已重写 label
	if !strings.Contains(capturedQuery, `namespace="ani-tenant-00000000-0000-0000-0000-000000000001"`) {
		t.Fatalf("forwarded query namespace not rewritten: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, `pod=~"^test-kjs-container-6(-.*)?$"`) {
		t.Fatalf("forwarded query pod not rewritten: %s", capturedQuery)
	}
	// 验证返回结果
	if len(result.Results) != 1 || result.Results[0].Value != 42.5 {
		t.Fatalf("result = %+v, want single sample value 42.5", result.Results)
	}
	if result.DevProfile.Provider != "prometheus-observability-service" || !result.DevProfile.RealProvider {
		t.Fatalf("dev_profile = %+v, want real prometheus provider", result.DevProfile)
	}
}

func TestPrometheusObservabilityServiceQueryDegradesOnLookupFailure(t *testing.T) {
	lookup := &fakeInstanceLookup{err: fmt.Errorf("%w: instance not found", ports.ErrNotFound)}
	service := newTestPrometheusObservabilityService(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("Prometheus should not be called on lookup failure")
	}, lookup)

	result, err := service.Query(context.Background(), ports.ObservabilityQueryRequest{
		TenantID: "tenant-a",
		Query:    `container_memory_working_set_bytes{namespace="inst_1",pod="inst_1"}`,
	})
	if err != nil {
		t.Fatalf("Query error = %v, err should degrade to empty result not error", err)
	}
	if len(result.Results) != 0 {
		t.Fatalf("result should be empty on lookup failure, got %+v", result.Results)
	}
}

func TestPrometheusObservabilityServiceQueryNoLabelsPassesThrough(t *testing.T) {
	lookup := &fakeInstanceLookup{}
	var capturedQuery string
	service := newTestPrometheusObservabilityService(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result":     []map[string]any{},
			},
		})
	}, lookup)

	// PromQL 不含 namespace/pod label，应原样转发不查实例记录
	_, err := service.Query(context.Background(), ports.ObservabilityQueryRequest{
		TenantID: "tenant-a",
		Query:    "up",
	})
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	if capturedQuery != "up" {
		t.Fatalf("forwarded query = %q, want %q", capturedQuery, "up")
	}
}

func TestPrometheusObservabilityServiceQueryInfValueDegradesGracefully(t *testing.T) {
	lookup := &fakeInstanceLookup{record: ports.WorkloadInstanceRecord{
		TenantID:   "00000000-0000-0000-0000-000000000001",
		InstanceID: "inst_1",
		Name:       "test-kjs-container-6",
	}}
	service := newTestPrometheusObservabilityService(t, func(w http.ResponseWriter, _ *http.Request) {
		// 模拟内存利用率 used/limit 当 limit=0 时 Prometheus 返回 +Inf
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []map[string]any{
					{"value": []any{float64(1780000000), "+Inf"}},
				},
			},
		})
	}, lookup)

	result, err := service.Query(context.Background(), ports.ObservabilityQueryRequest{
		TenantID: "00000000-0000-0000-0000-000000000001",
		Query:    `100 * (container_memory_working_set_bytes{namespace="inst_1",pod="inst_1"} / container_spec_memory_limit_bytes{namespace="inst_1",pod="inst_1"})`,
	})
	if err != nil {
		t.Fatalf("Query error = %v, should degrade to empty result not error", err)
	}
	// +Inf 被过滤后结果应为空，而非触发 JSON 序列化 panic
	if len(result.Results) != 0 {
		t.Fatalf("results should be empty for +Inf value, got %+v", result.Results)
	}
}

func TestPrometheusObservabilityServiceAlertRuleDelegatesToLocal(t *testing.T) {
	lookup := &fakeInstanceLookup{}
	service := newTestPrometheusObservabilityService(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("Prometheus should not be called for alert rules")
	}, lookup)

	rule, err := service.CreateAlertRule(context.Background(), ports.ObservabilityAlertRuleCreateRequest{
		TenantID:       "tenant-a",
		IdempotencyKey: "obs-alert-create",
		Name:           "High GPU",
		PromQL:         "avg(DCGM_FI_DEV_GPU_UTIL) > 80",
		Severity:       ports.ObservabilityAlertSeverityWarning,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("CreateAlertRule error = %v", err)
	}
	if rule.RuleID == "" || rule.State != "active" {
		t.Fatalf("rule = %+v, want active rule", rule)
	}
}

func TestNewPrometheusObservabilityServiceRequiresConfig(t *testing.T) {
	if _, err := NewPrometheusObservabilityService(PrometheusObservabilityServiceConfig{
		PrometheusURL:  "",
		InstanceLookup: &fakeInstanceLookup{},
	}); err == nil {
		t.Fatal("expected error for empty prometheus_url")
	}
	if _, err := NewPrometheusObservabilityService(PrometheusObservabilityServiceConfig{
		PrometheusURL:  "http://prometheus:9090",
		InstanceLookup: nil,
	}); err == nil {
		t.Fatal("expected error for nil instance_lookup")
	}
}

// TestPrometheusObservabilityServiceQueryRange 验证 range query 正确解析 Prometheus matrix 响应，
// 返回多个时间采样点用于绘制时序曲线。
func TestPrometheusObservabilityServiceQueryRange(t *testing.T) {
	lookup := &fakeInstanceLookup{record: ports.WorkloadInstanceRecord{
		TenantID:   "00000000-0000-0000-0000-000000000001",
		InstanceID: "inst_1",
		Name:       "test-container-6",
	}}
	service := newTestPrometheusObservabilityService(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v1/query_range") {
			t.Fatalf("expected query_range endpoint, got %s", r.URL.Path)
		}
		// 返回含两条 series 的 matrix，每条 series 含 3 个采样点
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "matrix",
				"result": []map[string]any{
					{
						"metric": map[string]string{"container": "test-container-6", "pod": "test-container-6-78d-p428s"},
						"values": [][]any{
							{float64(1780000000), "6.95"},
							{float64(1780000015), "7.10"},
							{float64(1780000030), "6.80"},
						},
					},
				},
			},
		})
	}, lookup)

	result, err := service.QueryRange(context.Background(), ports.ObservabilityRangeQueryRequest{
		TenantID: "00000000-0000-0000-0000-000000000001",
		Query:    `100 * (container_memory_working_set_bytes{namespace="inst_1",pod="inst_1"} / container_spec_memory_limit_bytes{namespace="inst_1",pod="inst_1"})`,
		Start:    time.Unix(1780000000, 0).UTC(),
		End:      time.Unix(1780000030, 0).UTC(),
		Step:     15 * time.Second,
	})
	if err != nil {
		t.Fatalf("QueryRange error = %v", err)
	}
	if result.ResultType != ports.ObservabilityResultMatrix {
		t.Fatalf("result_type = %s, want matrix", result.ResultType)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results len = %d, want 1 series", len(result.Results))
	}
	if len(result.Results[0].Values) != 3 {
		t.Fatalf("values len = %d, want 3 points", len(result.Results[0].Values))
	}
	if result.Results[0].Values[0].Value != 6.95 {
		t.Fatalf("first value = %v, want 6.95", result.Results[0].Values[0].Value)
	}
	if !result.DevProfile.RealProvider {
		t.Fatalf("dev_profile.real_provider = false, want true for successful range query")
	}
}

// TestPrometheusObservabilityServiceQueryRangeFiltersInf 验证 range query 过滤 NaN/Inf 采样点。
func TestPrometheusObservabilityServiceQueryRangeFiltersInf(t *testing.T) {
	lookup := &fakeInstanceLookup{record: ports.WorkloadInstanceRecord{
		TenantID:   "00000000-0000-0000-0000-000000000001",
		InstanceID: "inst_1",
		Name:       "test-container-6",
	}}
	service := newTestPrometheusObservabilityService(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "matrix",
				"result": []map[string]any{
					{
						"metric": map[string]string{"container": "test-container-6"},
						"values": [][]any{
							{float64(1780000000), "+Inf"},  // 过滤
							{float64(1780000015), "6.95"},   // 保留
							{float64(1780000030), "NaN"},    // 过滤
						},
					},
				},
			},
		})
	}, lookup)

	result, err := service.QueryRange(context.Background(), ports.ObservabilityRangeQueryRequest{
		TenantID: "00000000-0000-0000-0000-000000000001",
		Query:    `container_memory_working_set_bytes{namespace="inst_1",pod="inst_1"}`,
		Start:    time.Unix(1780000000, 0).UTC(),
		End:      time.Unix(1780000030, 0).UTC(),
		Step:     15 * time.Second,
	})
	if err != nil {
		t.Fatalf("QueryRange error = %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results len = %d, want 1 series", len(result.Results))
	}
	if len(result.Results[0].Values) != 1 {
		t.Fatalf("values len = %d, want 1 (Inf/NaN filtered)", len(result.Results[0].Values))
	}
	if result.Results[0].Values[0].Value != 6.95 {
		t.Fatalf("value = %v, want 6.95", result.Results[0].Values[0].Value)
	}
}
