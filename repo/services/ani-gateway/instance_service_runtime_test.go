package main

import (
	"context"
	"testing"
)

func TestGatewayInstanceServiceDefaultsToLocalNil(t *testing.T) {
	service, closeService, err := newGatewayInstanceService(context.Background(), gatewayInstanceServiceRuntimeConfig{})
	if err != nil {
		t.Fatalf("newGatewayInstanceService() error = %v", err)
	}
	if service != nil {
		t.Fatalf("service=%T, want nil so router keeps local in-memory loop", service)
	}
	closeService()
}

func TestGatewayInstanceServiceLocalProviderReturnsNil(t *testing.T) {
	for _, provider := range []string{"local", "not_configured", ""} {
		cfg := gatewayInstanceServiceRuntimeConfig{WorkloadProvider: provider}
		service, closeService, err := newGatewayInstanceService(context.Background(), cfg)
		if err != nil {
			t.Fatalf("provider %q: error = %v", provider, err)
		}
		if service != nil {
			t.Fatalf("provider %q: service=%T, want nil", provider, service)
		}
		closeService()
	}
}

func TestGatewayInstanceServiceRejectsUnsupportedProvider(t *testing.T) {
	cfg := gatewayInstanceServiceRuntimeConfig{WorkloadProvider: "invalid_provider"}
	_, _, err := newGatewayInstanceService(context.Background(), cfg)
	if err == nil {
		t.Fatal("newGatewayInstanceService() error = nil, want unsupported provider error")
	}
}

func TestGatewayInstanceServiceKubernetesRestRequiresDatabaseURL(t *testing.T) {
	cfg := gatewayInstanceServiceRuntimeConfig{WorkloadProvider: "kubernetes_rest"}
	_, _, err := newGatewayInstanceService(context.Background(), cfg)
	if err == nil {
		t.Fatal("newGatewayInstanceService() error = nil, want DATABASE_URL required error")
	}
}

func TestGatewayInstanceServiceConfigFromEnv(t *testing.T) {
	t.Setenv("WORKLOAD_PROVIDER", "kubernetes_rest")
	t.Setenv("WORKLOAD_PROVIDER_APPLY_ENABLED", "true")
	t.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db")
	t.Setenv("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc")
	t.Setenv("KUBERNETES_SERVICE_PORT", "443")
	t.Setenv("KUBERNETES_PROVIDER_FIELD_MANAGER", "ani-gateway")

	cfg := gatewayInstanceServiceRuntimeConfigFromEnv()
	if cfg.WorkloadProvider != "kubernetes_rest" {
		t.Fatalf("WorkloadProvider=%q, want kubernetes_rest", cfg.WorkloadProvider)
	}
	if !cfg.WorkloadProviderApplyEnabled {
		t.Fatal("WorkloadProviderApplyEnabled=false, want true")
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("DatabaseURL empty, want loaded from env")
	}
	if cfg.KubernetesServiceHost == "" || cfg.KubernetesServicePort == "" {
		t.Fatal("kubernetes service host/port not loaded from env")
	}
	if cfg.KubernetesProviderFieldManager == "" {
		t.Fatal("kubernetes field manager not loaded from env")
	}
}

func TestGatewayInstanceServiceConfigFromEnvApplyDisabledByDefault(t *testing.T) {
	t.Setenv("WORKLOAD_PROVIDER_APPLY_ENABLED", "")
	cfg := gatewayInstanceServiceRuntimeConfigFromEnv()
	if cfg.WorkloadProviderApplyEnabled {
		t.Fatal("WorkloadProviderApplyEnabled=true, want false when env unset")
	}
}

func TestGatewayInstanceServiceCloseOnNilIsNoop(t *testing.T) {
	_, closeService, _ := newGatewayInstanceService(context.Background(), gatewayInstanceServiceRuntimeConfig{})
	closeService()
}
