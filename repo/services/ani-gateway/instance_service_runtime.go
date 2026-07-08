package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kubercloud/ani/pkg/bootstrap"
	"github.com/kubercloud/ani/pkg/ports"
)

// gatewayInstanceServiceRuntimeConfig configures the Gateway-side instance
// service runtime. It mirrors the env names used by bootstrap.Config (see
// server.go withEnvironmentOverrides) so the gateway and core services share
// a single provider-switching contract.
type gatewayInstanceServiceRuntimeConfig struct {
	WorkloadProvider             string
	WorkloadProviderApplyEnabled bool
	DatabaseURL                  string
	KubernetesAPIHost            string
	KubernetesServiceHost        string
	KubernetesServicePort        string
	KubernetesBearerToken        string
	KubernetesServiceAccountTokenFile string
	KubernetesServiceAccountCAFile    string
	KubernetesProviderFieldManager    string
}

func gatewayInstanceServiceRuntimeConfigFromEnv() gatewayInstanceServiceRuntimeConfig {
	return gatewayInstanceServiceRuntimeConfig{
		WorkloadProvider:                   os.Getenv("WORKLOAD_PROVIDER"),
		WorkloadProviderApplyEnabled:       strings.EqualFold(strings.TrimSpace(os.Getenv("WORKLOAD_PROVIDER_APPLY_ENABLED")), "true"),
		DatabaseURL:                        os.Getenv("DATABASE_URL"),
		KubernetesAPIHost:                  os.Getenv("KUBERNETES_API_HOST"),
		KubernetesServiceHost:              os.Getenv("KUBERNETES_SERVICE_HOST"),
		KubernetesServicePort:              os.Getenv("KUBERNETES_SERVICE_PORT"),
		KubernetesBearerToken:              os.Getenv("KUBERNETES_BEARER_TOKEN"),
		KubernetesServiceAccountTokenFile:  os.Getenv("KUBERNETES_SERVICE_ACCOUNT_TOKEN_FILE"),
		KubernetesServiceAccountCAFile:     os.Getenv("KUBERNETES_SERVICE_ACCOUNT_CA_FILE"),
		KubernetesProviderFieldManager:    os.Getenv("KUBERNETES_PROVIDER_FIELD_MANAGER"),
	}
}

// newGatewayInstanceService assembles the real-K8s provider instance service
// runtime. It is nil-safe: a nil service (plus noop close) tells the router to
// keep its existing local in-memory loop (CORE-DEV-PROFILE-A boundary stays
// intact when env is unset).
func newGatewayInstanceService(ctx context.Context, cfg gatewayInstanceServiceRuntimeConfig) (ports.WorkloadInstanceService, func(), error) {
	noopClose := func() {}
	switch provider := strings.TrimSpace(cfg.WorkloadProvider); provider {
	case "", "local", "not_configured":
		return nil, noopClose, nil
	case "kubernetes_rest":
		if strings.TrimSpace(cfg.DatabaseURL) == "" {
			return nil, noopClose, fmt.Errorf("%w: DATABASE_URL is required for kubernetes_rest instance service", ports.ErrNotConfigured)
		}
		return bootstrap.ConnectInstanceService(ctx, cfg.DatabaseURL, bootstrap.Config{
			WorkloadProvider:                   cfg.WorkloadProvider,
			WorkloadProviderApplyEnabled:       cfg.WorkloadProviderApplyEnabled,
			KubernetesAPIHost:                  cfg.KubernetesAPIHost,
			KubernetesServiceHost:              cfg.KubernetesServiceHost,
			KubernetesServicePort:              cfg.KubernetesServicePort,
			KubernetesBearerToken:              cfg.KubernetesBearerToken,
			KubernetesServiceAccountTokenFile:  cfg.KubernetesServiceAccountTokenFile,
			KubernetesServiceAccountCAFile:     cfg.KubernetesServiceAccountCAFile,
			KubernetesProviderFieldManager:     cfg.KubernetesProviderFieldManager,
		})
	default:
		return nil, noopClose, fmt.Errorf("%w: unsupported WORKLOAD_PROVIDER %q", ports.ErrUnsupported, provider)
	}
}
