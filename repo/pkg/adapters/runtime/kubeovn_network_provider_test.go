package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestKubeOVNNetworkProviderAdapterUsesServerSideDryRun(t *testing.T) {
	client := &fakeKubernetesNetworkProviderClient{}
	manifests := renderedNetworkVPC(t)
	adapter := NewKubeOVNNetworkProviderAdapter(client, WithKubeOVNNetworkProviderClock(func() time.Time {
		return time.Unix(1200, 0)
	}))

	result, err := adapter.DryRun(context.Background(), ports.NetworkProviderDryRunRequest{
		TenantID:        "tenant-a",
		UserID:          "user-a",
		ResourceKind:    "vpc",
		ResourceID:      "vpc-main",
		Operation:       ports.NetworkProviderOperationCreate,
		Manifests:       manifests,
		PermissionProof: "rbac:scope:networks:create",
	})
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}
	if !result.Accepted || !strings.Contains(result.Reason, "dryRun=All") {
		t.Fatalf("result = %#v, want accepted dryRun=All", result)
	}
	if len(result.ResourceRefs) != 1 || result.ResourceRefs[0] != "kubeovn/Vpc/vpc-vpc-main" {
		t.Fatalf("ResourceRefs = %#v, want Vpc ref", result.ResourceRefs)
	}
	if client.dryRuns != 1 {
		t.Fatalf("dryRuns = %d, want 1", client.dryRuns)
	}
}

func TestKubeOVNNetworkProviderAdapterApplyFailsClosed(t *testing.T) {
	client := &fakeKubernetesNetworkProviderClient{}
	manifests := renderedNetworkVPC(t)
	result, err := NewKubeOVNNetworkProviderAdapter(client).Apply(context.Background(), ports.NetworkProviderApplyRequest{
		TenantID:        "tenant-a",
		UserID:          "user-a",
		ResourceKind:    "vpc",
		ResourceID:      "vpc-main",
		Operation:       ports.NetworkProviderOperationCreate,
		Manifests:       manifests,
		PermissionProof: "rbac:scope:networks:create",
		DryRunResult: ports.NetworkProviderDryRunResult{
			Accepted:      true,
			Provider:      "kubeovn",
			ManifestCount: len(manifests),
		},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.Applied {
		t.Fatalf("Applied = true, want false")
	}
	if client.applies != 0 {
		t.Fatalf("applies = %d, want 0", client.applies)
	}
}

func TestKubeOVNNetworkProviderAdapterAppliesWhenEnabled(t *testing.T) {
	client := &fakeKubernetesNetworkProviderClient{}
	manifests := renderedNetworkLoadBalancer(t)
	result, err := NewKubeOVNNetworkProviderAdapter(
		client,
		WithKubeOVNNetworkProviderApplyEnabled(true),
		WithKubeOVNNetworkProviderClock(func() time.Time { return time.Unix(1300, 0) }),
	).Apply(context.Background(), ports.NetworkProviderApplyRequest{
		TenantID:        "tenant-a",
		UserID:          "user-a",
		ResourceKind:    "load-balancer",
		ResourceID:      "lb-web",
		Operation:       ports.NetworkProviderOperationCreate,
		Manifests:       manifests,
		PermissionProof: "rbac:scope:networks:create",
		DryRunResult: ports.NetworkProviderDryRunResult{
			Accepted:      true,
			Provider:      "kubernetes",
			ManifestCount: len(manifests),
			ResourceRefs:  []string{"kubernetes/Service/lb-lb-web"},
		},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !result.Applied {
		t.Fatalf("Applied = false, reason = %s", result.Reason)
	}
	if len(result.ResourceRefs) != 1 || result.ResourceRefs[0] != "kubernetes/Service/lb-lb-web" {
		t.Fatalf("ResourceRefs = %#v, want Service ref", result.ResourceRefs)
	}
	if client.applies != 1 {
		t.Fatalf("applies = %d, want 1", client.applies)
	}
}

func TestKubeOVNNetworkProviderAdapterObservesNetworkStatus(t *testing.T) {
	client := &fakeKubernetesNetworkProviderClient{
		status: ports.NetworkProviderStatusResult{
			TenantID:     "tenant-a",
			ResourceKind: "vpc",
			ResourceID:   "vpc-main",
			Provider:     "kubeovn",
			ResourceRefs: []string{"kubeovn/Vpc/vpc-vpc-main"},
			State:        ports.NetworkResourceAvailable,
			Reason:       "observed by Kubernetes network provider",
		},
	}
	result, err := NewKubeOVNNetworkProviderAdapter(
		client,
		WithKubeOVNNetworkProviderClock(func() time.Time { return time.Unix(1400, 0) }),
	).Observe(context.Background(), ports.NetworkProviderStatusRequest{
		TenantID:        "tenant-a",
		UserID:          "user-a",
		ResourceKind:    "vpc",
		ResourceID:      "vpc-main",
		PermissionProof: "rbac:scope:networks:read",
		ApplyResult: ports.NetworkProviderApplyResult{
			Applied:      true,
			Provider:     "kubeovn",
			ResourceRefs: []string{"kubeovn/Vpc/vpc-vpc-main"},
		},
	})
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	if result.State != ports.NetworkResourceAvailable {
		t.Fatalf("State = %q, want available", result.State)
	}
	if result.ObservedAt != time.Unix(1400, 0).UTC() {
		t.Fatalf("ObservedAt = %v, want fixed clock", result.ObservedAt)
	}
	if client.observes != 1 {
		t.Fatalf("observes = %d, want 1", client.observes)
	}
}

func renderedNetworkVPC(t *testing.T) []ports.WorkloadManifest {
	t.Helper()
	manifests, err := NewKubeOVNNetworkRenderer().RenderVPC(context.Background(), ports.NetworkVPCRecord{
		TenantID: "tenant-a",
		VPCID:    "vpc-main",
		Name:     "main",
		CIDR:     "10.40.0.0/16",
		State:    ports.NetworkResourceAvailable,
	})
	if err != nil {
		t.Fatalf("RenderVPC() error = %v", err)
	}
	return manifests
}

func renderedNetworkLoadBalancer(t *testing.T) []ports.WorkloadManifest {
	t.Helper()
	manifests, err := NewKubeOVNNetworkRenderer().RenderLoadBalancer(context.Background(), ports.NetworkLoadBalancerRecord{
		TenantID:       "tenant-a",
		LoadBalancerID: "lb-web",
		Name:           "web",
		VPCID:          "vpc-main",
		Scheme:         "public",
		State:          ports.NetworkResourceAvailable,
		Listeners: []ports.NetworkLoadBalancerListener{
			{Protocol: "http", Port: 80, TargetPort: 8080},
		},
	})
	if err != nil {
		t.Fatalf("RenderLoadBalancer() error = %v", err)
	}
	return manifests
}

type fakeKubernetesNetworkProviderClient struct {
	dryRuns  int
	applies  int
	observes int
	status   ports.NetworkProviderStatusResult
}

func (c *fakeKubernetesNetworkProviderClient) ServerSideDryRun(_ context.Context, manifests []ports.WorkloadManifest) (ports.WorkloadProviderDryRunResult, error) {
	c.dryRuns++
	return ports.WorkloadProviderDryRunResult{
		Accepted:      true,
		Provider:      manifests[0].Provider,
		ManifestCount: len(manifests),
		Reason:        "accepted by Kubernetes server-side dry-run dryRun=All",
	}, nil
}

func (c *fakeKubernetesNetworkProviderClient) ApplyManifests(_ context.Context, manifests []ports.WorkloadManifest) ([]string, error) {
	c.applies++
	return networkResourceRefs(manifests), nil
}

func (c *fakeKubernetesNetworkProviderClient) ObserveNetworkResource(_ context.Context, request ports.NetworkProviderStatusRequest) (ports.NetworkProviderStatusResult, error) {
	c.observes++
	if c.status.ResourceID != "" {
		return c.status, nil
	}
	return ports.NetworkProviderStatusResult{
		TenantID:     request.TenantID,
		ResourceKind: request.ResourceKind,
		ResourceID:   request.ResourceID,
		Provider:     request.ApplyResult.Provider,
		ResourceRefs: append([]string(nil), request.ApplyResult.ResourceRefs...),
		State:        ports.NetworkResourceAvailable,
	}, nil
}

var _ KubernetesNetworkProviderClient = (*fakeKubernetesNetworkProviderClient)(nil)
