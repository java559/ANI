package runtime

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestPrometheusInstanceObservabilityListsLogsEventsAndSecurityEvents(t *testing.T) {
	var requests []string
	service := newTestPrometheusInstanceObservability(t, func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.String())
		switch {
		case r.URL.Path == "/api/v1/namespaces/ani-tenant-tenant-a/pods/pod-a/log":
			return jsonResponse(http.StatusOK, "info booted\nwarn restarted\n"), nil
		case r.URL.Path == "/api/v1/namespaces/ani-tenant-tenant-a/events":
			return jsonResponse(http.StatusOK, `{
				"items": [
					{"metadata":{"uid":"evt-a"},"type":"Normal","reason":"Scheduled","message":"pod scheduled","count":2,"lastTimestamp":"2026-06-19T08:29:00Z"},
					{"metadata":{"uid":"evt-b"},"type":"Warning","reason":"Unhealthy","message":"readiness probe failed","count":1,"eventTime":"2026-06-19T08:30:00Z"}
				]
			}`), nil
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	logs, err := service.ListLogs(context.Background(), ports.InstanceObservationListRequest{
		TenantID:   "tenant-a",
		InstanceID: "pod-a",
		Limit:      1,
		Level:      "warn",
	})
	if err != nil {
		t.Fatalf("ListLogs() error = %v", err)
	}
	if len(logs.Items) != 1 || logs.Items[0].Level != "warn" || logs.Items[0].Message != "warn restarted" {
		t.Fatalf("logs = %+v, want one warning log from Kubernetes pod logs", logs)
	}
	if logs.DevProfile.Mode != "dev_profile" || logs.DevProfile.RealProvider {
		t.Fatalf("logs dev profile = %+v, want non-real dev_profile marker", logs.DevProfile)
	}

	events, err := service.ListEvents(context.Background(), ports.InstanceObservationListRequest{
		TenantID:   "tenant-a",
		InstanceID: "pod-a",
		Type:       "Warning",
	})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events.Items) != 1 || events.Items[0].ID != "evt-b" || events.Items[0].Reason != "Unhealthy" {
		t.Fatalf("events = %+v, want filtered Kubernetes warning event", events)
	}

	security, err := service.ListSecurityEvents(context.Background(), ports.InstanceObservationListRequest{
		TenantID:   "tenant-a",
		InstanceID: "pod-a",
		Severity:   "warning",
	})
	if err != nil {
		t.Fatalf("ListSecurityEvents() error = %v", err)
	}
	if len(security.Items) != 1 || security.Items[0].EventType != "kubernetes_warning" {
		t.Fatalf("security events = %+v, want warning event projection", security)
	}
	if len(requests) != 3 || !strings.Contains(requests[0], "tailLines=1") || !strings.Contains(requests[1], "involvedObject.name%3Dpod-a") {
		t.Fatalf("requests = %+v, want Kubernetes logs/events API calls", requests)
	}
}

func TestPrometheusInstanceObservabilityGetsMetricsFromPrometheus(t *testing.T) {
	service := newTestPrometheusInstanceObservability(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/query" {
			t.Fatalf("path = %s, want Prometheus query API", r.URL.Path)
		}
		query, _ := url.QueryUnescape(r.URL.RawQuery)
		switch {
		case strings.Contains(query, "container_cpu_usage_seconds_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"23.5"]}]}}`), nil
		case strings.Contains(query, "container_memory_working_set_bytes"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"1610612736"]}]}}`), nil
		case strings.Contains(query, "container_spec_memory_limit_bytes"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"2147483648"]}]}}`), nil
		case strings.Contains(query, "container_network_receive_bytes_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"1048576"]}]}}`), nil
		case strings.Contains(query, "container_network_transmit_bytes_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"524288"]}]}}`), nil
		default:
			t.Fatalf("unexpected query = %q", query)
			return nil, nil
		}
	})

	metrics, err := service.GetMetrics(context.Background(), ports.InstanceObservationGetRequest{
		TenantID:   "tenant-a",
		InstanceID: "pod-a",
	})
	if err != nil {
		t.Fatalf("GetMetrics() error = %v", err)
	}
	if metrics.InstanceID != "pod-a" || metrics.CPUUtilizationPct == nil || *metrics.CPUUtilizationPct != 23.5 {
		t.Fatalf("metrics = %+v, want Prometheus CPU utilization", metrics)
	}
	if metrics.MemoryUsedMB == nil || *metrics.MemoryUsedMB != 1536.0 {
		t.Fatalf("memory_used_mb = %+v, want 1536 MB (1610612736 bytes)", metrics.MemoryUsedMB)
	}
	if metrics.NetworkRXBytes == nil || *metrics.NetworkRXBytes != 1048576 {
		t.Fatalf("network_rx_bytes = %+v, want 1048576", metrics.NetworkRXBytes)
	}
	if metrics.NetworkTXBytes == nil || *metrics.NetworkTXBytes != 524288 {
		t.Fatalf("network_tx_bytes = %+v, want 524288", metrics.NetworkTXBytes)
	}
	if metrics.MemoryTotalMB == nil || *metrics.MemoryTotalMB != 2048.0 {
		t.Fatalf("memory_total_mb = %+v, want 2048 MB (2147483648 bytes limit)", metrics.MemoryTotalMB)
	}
	if !metrics.Timestamp.Equal(time.Unix(1780000000, 0).UTC()) {
		t.Fatalf("timestamp = %s, want Prometheus sample timestamp", metrics.Timestamp)
	}
	if metrics.DevProfile.Provider != "prometheus-kubernetes-instance-observability" || metrics.DevProfile.RealProvider {
		t.Fatalf("metrics dev profile = %+v, want Prometheus/Kubernetes contract marker", metrics.DevProfile)
	}
}

// TestPrometheusInstanceObservabilityGetMetricsGPUContainerAggregatesDCGM 验证
// gpu_container 在 DCGM exporter 可用时填充 GPU 相关字段。
func TestPrometheusInstanceObservabilityGetMetricsGPUContainerAggregatesDCGM(t *testing.T) {
	service := newTestPrometheusInstanceObservability(t, func(r *http.Request) (*http.Response, error) {
		query, _ := url.QueryUnescape(r.URL.RawQuery)
		switch {
		case strings.Contains(query, "container_cpu_usage_seconds_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"45.0"]}]}}`), nil
		case strings.Contains(query, "container_memory_working_set_bytes"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"1073741824"]}]}}`), nil
		case strings.Contains(query, "container_network_receive_bytes_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"0"]}]}}`), nil
		case strings.Contains(query, "container_network_transmit_bytes_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"0"]}]}}`), nil
		case strings.Contains(query, "DCGM_FI_DEV_GPU_UTIL"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"82.5"]}]}}`), nil
		case strings.Contains(query, "DCGM_FI_DEV_FB_USED"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"8589934592"]}]}}`), nil
		case strings.Contains(query, "DCGM_FI_DEV_FB_TOTAL"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"17179869184"]}]}}`), nil
		default:
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[]}}`), nil
		}
	})

	metrics, err := service.GetMetrics(context.Background(), ports.InstanceObservationGetRequest{
		TenantID:   "tenant-a",
		InstanceID: "pod-a",
		Kind:       ports.WorkloadKindGPUContainer,
	})
	if err != nil {
		t.Fatalf("GetMetrics() error = %v", err)
	}
	if metrics.GPUUtilizationPct == nil || *metrics.GPUUtilizationPct != 82.5 {
		t.Fatalf("gpu_utilization_pct = %+v, want 82.5", metrics.GPUUtilizationPct)
	}
	if metrics.GPUMemoryUsedMB == nil || *metrics.GPUMemoryUsedMB != 8192.0 {
		t.Fatalf("gpu_memory_used_mb = %+v, want 8192 MB (8589934592 bytes)", metrics.GPUMemoryUsedMB)
	}
	if metrics.GPUMemoryTotalMB == nil || *metrics.GPUMemoryTotalMB != 16384.0 {
		t.Fatalf("gpu_memory_total_mb = %+v, want 16384 MB (17179869184 bytes)", metrics.GPUMemoryTotalMB)
	}
	if metrics.CPUUtilizationPct == nil {
		t.Fatalf("cpu_utilization_pct = nil, want filled from metrics.k8s.io")
	}
}

// TestPrometheusInstanceObservabilityGetMetricsNonGPUContainerGPUNil 验证
// 非 gpu_container 的 GPU 字段为 nil（禁止用 0 代替缺失）。
func TestPrometheusInstanceObservabilityGetMetricsNonGPUContainerGPUNil(t *testing.T) {
	service := newTestPrometheusInstanceObservability(t, func(r *http.Request) (*http.Response, error) {
		query, _ := url.QueryUnescape(r.URL.RawQuery)
		// 任何 DCGM 查询都不应到达；若到达则返回错误以暴露越界调用。
		if strings.Contains(query, "DCGM_FI_DEV") {
			t.Fatalf("unexpected DCGM query for non-gpu_container: %q", query)
			return nil, nil
		}
		switch {
		case strings.Contains(query, "container_cpu_usage_seconds_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"10.0"]}]}}`), nil
		default:
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[]}}`), nil
		}
	})

	metrics, err := service.GetMetrics(context.Background(), ports.InstanceObservationGetRequest{
		TenantID:   "tenant-a",
		InstanceID: "pod-a",
		Kind:       ports.WorkloadKindContainer,
	})
	if err != nil {
		t.Fatalf("GetMetrics() error = %v", err)
	}
	if metrics.GPUUtilizationPct != nil {
		t.Fatalf("gpu_utilization_pct = %+v, want nil for non-gpu_container", metrics.GPUUtilizationPct)
	}
	if metrics.GPUMemoryUsedMB != nil {
		t.Fatalf("gpu_memory_used_mb = %+v, want nil for non-gpu_container", metrics.GPUMemoryUsedMB)
	}
	if metrics.GPUMemoryTotalMB != nil {
		t.Fatalf("gpu_memory_total_mb = %+v, want nil for non-gpu_container", metrics.GPUMemoryTotalMB)
	}
}

// TestPrometheusInstanceObservabilityGetMetricsSingleExporterDegradation 验证
// 单个 exporter 不可用时不阻塞其他字段采集；已采集字段正常返回，不可采集字段为 nil。
func TestPrometheusInstanceObservabilityGetMetricsSingleExporterDegradation(t *testing.T) {
	service := newTestPrometheusInstanceObservability(t, func(r *http.Request) (*http.Response, error) {
		query, _ := url.QueryUnescape(r.URL.RawQuery)
		switch {
		case strings.Contains(query, "container_cpu_usage_seconds_total"):
			// CPU exporter 可用
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"30.0"]}]}}`), nil
		case strings.Contains(query, "container_memory_working_set_bytes"):
			// 内存 exporter 不可用：返回错误状态
			return jsonResponse(http.StatusInternalServerError, `{"status":"error","error":"internal"}`), nil
		case strings.Contains(query, "container_network_receive_bytes_total"):
			// 网络 exporter 不可用：返回空结果
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[]}}`), nil
		case strings.Contains(query, "container_network_transmit_bytes_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[]}}`), nil
		default:
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[]}}`), nil
		}
	})

	metrics, err := service.GetMetrics(context.Background(), ports.InstanceObservationGetRequest{
		TenantID:   "tenant-a",
		InstanceID: "pod-a",
		Kind:       ports.WorkloadKindContainer,
	})
	if err != nil {
		t.Fatalf("GetMetrics() error = %v, err should be nil (single exporter degradation should not block)", err)
	}
	if metrics.CPUUtilizationPct == nil || *metrics.CPUUtilizationPct != 30.0 {
		t.Fatalf("cpu_utilization_pct = %+v, want 30.0 (collected)", metrics.CPUUtilizationPct)
	}
	if metrics.MemoryUsedMB != nil {
		t.Fatalf("memory_used_mb = %+v, want nil (exporter unavailable)", metrics.MemoryUsedMB)
	}
	if metrics.NetworkRXBytes != nil {
		t.Fatalf("network_rx_bytes = %+v, want nil (exporter empty result)", metrics.NetworkRXBytes)
	}
}

// TestPrometheusInstanceObservabilityGetMetricsGPUContainerDCGMUnavailable 验证
// gpu_container 当 DCGM exporter 不可用时，GPU 字段为 nil，CPU/内存等正常返回。
func TestPrometheusInstanceObservabilityGetMetricsGPUContainerDCGMUnavailable(t *testing.T) {
	service := newTestPrometheusInstanceObservability(t, func(r *http.Request) (*http.Response, error) {
		query, _ := url.QueryUnescape(r.URL.RawQuery)
		switch {
		case strings.Contains(query, "container_cpu_usage_seconds_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"55.0"]}]}}`), nil
		case strings.Contains(query, "container_memory_working_set_bytes"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"536870912"]}]}}`), nil
		case strings.Contains(query, "DCGM_FI_DEV"):
			// DCGM exporter 不可用
			return jsonResponse(http.StatusServiceUnavailable, `{"status":"error","error":"dcgm unavailable"}`), nil
		default:
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[]}}`), nil
		}
	})

	metrics, err := service.GetMetrics(context.Background(), ports.InstanceObservationGetRequest{
		TenantID:   "tenant-a",
		InstanceID: "pod-a",
		Kind:       ports.WorkloadKindGPUContainer,
	})
	if err != nil {
		t.Fatalf("GetMetrics() error = %v, want nil (DCGM degradation should not block)", err)
	}
	if metrics.CPUUtilizationPct == nil || *metrics.CPUUtilizationPct != 55.0 {
		t.Fatalf("cpu_utilization_pct = %+v, want 55.0 (collected from metrics.k8s.io)", metrics.CPUUtilizationPct)
	}
	if metrics.GPUUtilizationPct != nil {
		t.Fatalf("gpu_utilization_pct = %+v, want nil (DCGM unavailable)", metrics.GPUUtilizationPct)
	}
	if metrics.GPUMemoryUsedMB != nil {
		t.Fatalf("gpu_memory_used_mb = %+v, want nil (DCGM unavailable)", metrics.GPUMemoryUsedMB)
	}
	if metrics.GPUMemoryTotalMB != nil {
		t.Fatalf("gpu_memory_total_mb = %+v, want nil (DCGM unavailable)", metrics.GPUMemoryTotalMB)
	}
}

// TestPrometheusInstanceObservabilityGetMetricsDeploymentPodNameRegex 验证
// 当实例由 Deployment 创建（pod 名带 ReplicaSet/Job hash 后缀）时，GetMetrics
// 用正则 pod=~"^instance(-.*)?$" 匹配真实 pod，而非精确匹配实例名。
// 复现 issue-011：实例名 test-kjs-container-6，真实 pod test-kjs-container-6-5599cd774d-p428s，
// 精确 pod="test-kjs-container-6" 查不到数据导致所有指标为 null。
func TestPrometheusInstanceObservabilityGetMetricsDeploymentPodNameRegex(t *testing.T) {
	service := newTestPrometheusInstanceObservability(t, func(r *http.Request) (*http.Response, error) {
		query, _ := url.QueryUnescape(r.URL.RawQuery)
		// 验证所有 container 指标查询都使用正则 pod=~ 而非精确 pod=
		if strings.Contains(query, "container_cpu_usage_seconds_total") {
			if !strings.Contains(query, `pod=~"^test-kjs-container-6(-.*)?$"`) {
				t.Fatalf("cpu query should use pod regex matcher, got: %s", query)
			}
			if !strings.Contains(query, `container!="",container!="POD"`) {
				t.Fatalf("cpu query should filter pause container, got: %s", query)
			}
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"42.0"]}]}}`), nil
		}
		switch {
		case strings.Contains(query, "container_memory_working_set_bytes"):
			if !strings.Contains(query, `pod=~"^test-kjs-container-6(-.*)?$"`) {
				t.Fatalf("memory query should use pod regex matcher, got: %s", query)
			}
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"536870912"]}]}}`), nil
		case strings.Contains(query, "container_network_receive_bytes_total"):
			if !strings.Contains(query, `pod=~"^test-kjs-container-6(-.*)?$"`) {
				t.Fatalf("network rx query should use pod regex matcher, got: %s", query)
			}
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"2048"]}]}}`), nil
		case strings.Contains(query, "container_network_transmit_bytes_total"):
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[{"value":[1780000000,"1024"]}]}}`), nil
		default:
			return jsonResponse(http.StatusOK, `{"status":"success","data":{"resultType":"vector","result":[]}}`), nil
		}
	})

	metrics, err := service.GetMetrics(context.Background(), ports.InstanceObservationGetRequest{
		TenantID:   "00000000-0000-0000-0000-000000000001",
		InstanceID: "test-kjs-container-6",
		Kind:       ports.WorkloadKindContainer,
	})
	if err != nil {
		t.Fatalf("GetMetrics() error = %v", err)
	}
	if metrics.CPUUtilizationPct == nil || *metrics.CPUUtilizationPct != 42.0 {
		t.Fatalf("cpu_utilization_pct = %+v, want 42.0 (matched via pod name regex)", metrics.CPUUtilizationPct)
	}
	if metrics.MemoryUsedMB == nil || *metrics.MemoryUsedMB != 512.0 {
		t.Fatalf("memory_used_mb = %+v, want 512 MB", metrics.MemoryUsedMB)
	}
	if metrics.NetworkRXBytes == nil || *metrics.NetworkRXBytes != 2048 {
		t.Fatalf("network_rx_bytes = %+v, want 2048", metrics.NetworkRXBytes)
	}
}

// TestPromQLPodMatcher 验证 pod 名正则匹配器构造正确，兼容直接 Pod 与控制器生成的 pod。
func TestPromQLPodMatcher(t *testing.T) {
	cases := []struct {
		name string
		pod  string
		want string
	}{
		{"直接 Pod 无后缀", "pod-a", "^pod-a(-.*)?$"},
		{"Deployment pod 带 hash", "test-kjs-container-6", "^test-kjs-container-6(-.*)?$"},
		{"含点号转义", "app.v1", "^app\\.v1(-.*)?$"},
		{"空字符串", "", "^(-.*)?$"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := promQLPodMatcher(tc.pod)
			if got != tc.want {
				t.Fatalf("promQLPodMatcher(%q) = %q, want %q", tc.pod, got, tc.want)
			}
		})
	}
}

func TestPrometheusInstanceObservabilityCreatesIdempotentShortLivedExecSession(t *testing.T) {
	now := time.Date(2026, 6, 19, 8, 30, 0, 0, time.UTC)
	service := newTestPrometheusInstanceObservabilityWithClock(t, nil, func() time.Time { return now })
	request := ports.InstanceExecSessionCreateRequest{
		TenantID:       "tenant-a",
		InstanceID:     "pod-a",
		IdempotencyKey: "exec-once",
		Command:        []string{"/bin/sh"},
		TTY:            true,
		Rows:           24,
		Cols:           80,
	}

	first, err := service.CreateExecSession(context.Background(), request)
	if err != nil {
		t.Fatalf("CreateExecSession() first error = %v", err)
	}
	second, err := service.CreateExecSession(context.Background(), request)
	if err != nil {
		t.Fatalf("CreateExecSession() replay error = %v", err)
	}
	if first.ID == "" || second.ID != first.ID || second.WSURL != first.WSURL {
		t.Fatalf("replay = %+v, want same session as %+v", second, first)
	}
	if first.Token != "" {
		t.Fatalf("token = %q, want no long-lived credential", first.Token)
	}
	if !strings.HasPrefix(first.WSURL, "wss://gateway.example.test/api/v1/instances/pod-a/exec/") {
		t.Fatalf("ws_url = %q, want gateway exec URL", first.WSURL)
	}
	if !first.ExpiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("expires_at = %s, want 15 minute TTL", first.ExpiresAt)
	}
}

func newTestPrometheusInstanceObservability(t *testing.T, roundTrip roundTripFunc) *PrometheusInstanceObservability {
	t.Helper()
	return newTestPrometheusInstanceObservabilityWithClock(t, roundTrip, func() time.Time {
		return time.Date(2026, 6, 19, 8, 30, 0, 0, time.UTC)
	})
}

func newTestPrometheusInstanceObservabilityWithClock(t *testing.T, roundTrip roundTripFunc, now func() time.Time) *PrometheusInstanceObservability {
	t.Helper()
	var transport http.RoundTripper = http.DefaultTransport
	if roundTrip != nil {
		transport = roundTrip
	}
	service, err := NewPrometheusInstanceObservability(PrometheusInstanceObservabilityConfig{
		PrometheusURL:         "https://prometheus.example.test",
		KubernetesAPIHost:     "https://kubernetes.example.test",
		KubernetesBearerToken: "token",
		ExecBaseURL:           "wss://gateway.example.test/api/v1",
		HTTPClient:            &http.Client{Transport: transport},
		Now:                   now,
	})
	if err != nil {
		t.Fatalf("NewPrometheusInstanceObservability() error = %v", err)
	}
	return service
}
