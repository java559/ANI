package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

type KubernetesNetworkProviderClient interface {
	ServerSideDryRun(ctx context.Context, manifests []ports.WorkloadManifest) (ports.WorkloadProviderDryRunResult, error)
	ApplyManifests(ctx context.Context, manifests []ports.WorkloadManifest) ([]string, error)
	ObserveNetworkResource(ctx context.Context, request ports.NetworkProviderStatusRequest) (ports.NetworkProviderStatusResult, error)
}

type KubeOVNNetworkProviderAdapter struct {
	client       KubernetesNetworkProviderClient
	applyEnabled bool
	now          func() time.Time
}

type KubeOVNNetworkProviderOption func(*KubeOVNNetworkProviderAdapter)

func WithKubeOVNNetworkProviderApplyEnabled(enabled bool) KubeOVNNetworkProviderOption {
	return func(adapter *KubeOVNNetworkProviderAdapter) {
		adapter.applyEnabled = enabled
	}
}

func WithKubeOVNNetworkProviderClock(now func() time.Time) KubeOVNNetworkProviderOption {
	return func(adapter *KubeOVNNetworkProviderAdapter) {
		if now != nil {
			adapter.now = now
		}
	}
}

func NewKubeOVNNetworkProviderAdapter(client KubernetesNetworkProviderClient, options ...KubeOVNNetworkProviderOption) *KubeOVNNetworkProviderAdapter {
	adapter := &KubeOVNNetworkProviderAdapter{
		client: client,
		now:    time.Now,
	}
	for _, option := range options {
		option(adapter)
	}
	return adapter
}

func (a *KubeOVNNetworkProviderAdapter) DryRun(ctx context.Context, request ports.NetworkProviderDryRunRequest) (ports.NetworkProviderDryRunResult, error) {
	if err := validateNetworkProviderDryRunRequest(request); err != nil {
		return ports.NetworkProviderDryRunResult{}, err
	}
	if a.client == nil {
		return ports.NetworkProviderDryRunResult{}, ports.ErrNotConfigured
	}
	result, err := a.client.ServerSideDryRun(ctx, request.Manifests)
	if err != nil {
		return ports.NetworkProviderDryRunResult{}, err
	}
	return ports.NetworkProviderDryRunResult{
		Accepted:      result.Accepted,
		Provider:      result.Provider,
		ManifestCount: result.ManifestCount,
		ResourceRefs:  networkResourceRefs(request.Manifests),
		Reason:        firstNetworkNonEmpty(result.Reason, "accepted by KubeOVN network provider dry-run"),
		Warnings:      append([]string(nil), result.Warnings...),
		CheckedAt:     firstNonZeroTime(result.CheckedAt, a.now().UTC()),
	}, nil
}

func (a *KubeOVNNetworkProviderAdapter) Apply(ctx context.Context, request ports.NetworkProviderApplyRequest) (ports.NetworkProviderApplyResult, error) {
	if !a.applyEnabled {
		return ports.NetworkProviderApplyResult{
			Applied:       false,
			Provider:      request.DryRunResult.Provider,
			ManifestCount: len(request.Manifests),
			Operation:     request.Operation,
			ResourceRefs:  append([]string(nil), request.DryRunResult.ResourceRefs...),
			Reason:        "network provider apply is disabled by execution switch",
			Warnings:      append([]string(nil), request.DryRunResult.Warnings...),
			AppliedAt:     a.now().UTC(),
		}, nil
	}
	if err := validateNetworkProviderApplyRequest(request); err != nil {
		return ports.NetworkProviderApplyResult{}, err
	}
	if a.client == nil {
		return ports.NetworkProviderApplyResult{}, ports.ErrNotConfigured
	}
	refs, err := a.client.ApplyManifests(ctx, request.Manifests)
	if err != nil {
		return ports.NetworkProviderApplyResult{}, err
	}
	return ports.NetworkProviderApplyResult{
		Applied:       true,
		Provider:      request.Manifests[0].Provider,
		ManifestCount: len(request.Manifests),
		Operation:     request.Operation,
		ResourceRefs:  refs,
		Reason:        "applied by KubeOVN network provider adapter",
		Warnings:      append([]string(nil), request.DryRunResult.Warnings...),
		AppliedAt:     a.now().UTC(),
	}, nil
}

func (a *KubeOVNNetworkProviderAdapter) Observe(ctx context.Context, request ports.NetworkProviderStatusRequest) (ports.NetworkProviderStatusResult, error) {
	if err := validateNetworkProviderStatusRequest(request); err != nil {
		return ports.NetworkProviderStatusResult{}, err
	}
	if a.client == nil {
		return ports.NetworkProviderStatusResult{}, ports.ErrNotConfigured
	}
	result, err := a.client.ObserveNetworkResource(ctx, request)
	if err != nil {
		return ports.NetworkProviderStatusResult{}, err
	}
	if result.TenantID != request.TenantID || result.ResourceKind != request.ResourceKind || result.ResourceID != request.ResourceID {
		return ports.NetworkProviderStatusResult{}, fmt.Errorf("%w: network provider status identity does not match request", ports.ErrInvalid)
	}
	if result.Provider == "" {
		result.Provider = request.ApplyResult.Provider
	}
	if len(result.ResourceRefs) == 0 {
		return ports.NetworkProviderStatusResult{}, fmt.Errorf("%w: network provider status must include resource refs", ports.ErrInvalid)
	}
	if result.State == "" {
		result.State = ports.NetworkResourcePending
	}
	if result.ObservedAt.IsZero() {
		result.ObservedAt = a.now().UTC()
	}
	return result, nil
}

func validateNetworkProviderDryRunRequest(request ports.NetworkProviderDryRunRequest) error {
	if err := requireNetworkProviderIdentity(request.TenantID, request.UserID, request.ResourceKind, request.ResourceID, request.PermissionProof); err != nil {
		return err
	}
	if request.Operation != ports.NetworkProviderOperationCreate {
		return fmt.Errorf("%w: network provider dry-run currently allows create only", ports.ErrInvalid)
	}
	return validateNetworkProviderManifests(request.Manifests)
}

func validateNetworkProviderApplyRequest(request ports.NetworkProviderApplyRequest) error {
	if err := requireNetworkProviderIdentity(request.TenantID, request.UserID, request.ResourceKind, request.ResourceID, request.PermissionProof); err != nil {
		return err
	}
	if request.Operation != ports.NetworkProviderOperationCreate {
		return fmt.Errorf("%w: network provider apply currently allows create only", ports.ErrInvalid)
	}
	if !request.DryRunResult.Accepted {
		return fmt.Errorf("%w: network provider dry-run must be accepted before apply", ports.ErrInvalid)
	}
	if request.DryRunResult.ManifestCount != 0 && request.DryRunResult.ManifestCount != len(request.Manifests) {
		return fmt.Errorf("%w: network provider dry-run manifest count does not match apply request", ports.ErrInvalid)
	}
	return validateNetworkProviderManifests(request.Manifests)
}

func validateNetworkProviderStatusRequest(request ports.NetworkProviderStatusRequest) error {
	if err := requireNetworkProviderIdentity(request.TenantID, request.UserID, request.ResourceKind, request.ResourceID, request.PermissionProof); err != nil {
		return err
	}
	if !request.ApplyResult.Applied {
		return fmt.Errorf("%w: network provider apply must be applied before status observation", ports.ErrInvalid)
	}
	if request.ApplyResult.Provider == "" {
		return fmt.Errorf("%w: network provider apply result provider is required for status observation", ports.ErrInvalid)
	}
	if len(request.ApplyResult.ResourceRefs) == 0 {
		return fmt.Errorf("%w: network provider resource refs are required for status observation", ports.ErrInvalid)
	}
	return nil
}

func requireNetworkProviderIdentity(tenantID string, userID string, resourceKind string, resourceID string, permissionProof string) error {
	if tenantID == "" {
		return fmt.Errorf("%w: tenant id is required for network provider execution", ports.ErrInvalid)
	}
	if userID == "" {
		return fmt.Errorf("%w: user id is required for network provider execution", ports.ErrInvalid)
	}
	if resourceKind == "" {
		return fmt.Errorf("%w: resource kind is required for network provider execution", ports.ErrInvalid)
	}
	if resourceID == "" {
		return fmt.Errorf("%w: resource id is required for network provider execution", ports.ErrInvalid)
	}
	if permissionProof == "" {
		return fmt.Errorf("%w: permission proof is required for network provider execution", ports.ErrInvalid)
	}
	return nil
}

func validateNetworkProviderManifests(manifests []ports.WorkloadManifest) error {
	if len(manifests) == 0 {
		return fmt.Errorf("%w: at least one network provider manifest is required", ports.ErrInvalid)
	}
	for _, manifest := range manifests {
		doc, err := parseManifestDocument(manifest.Content)
		if err != nil {
			return err
		}
		if err := validateProviderDryRunDocument(manifest.Provider, doc); err != nil {
			return err
		}
	}
	return nil
}

func networkResourceRefs(manifests []ports.WorkloadManifest) []string {
	refs := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		refs = append(refs, manifest.Provider+"/"+manifest.Kind+"/"+manifest.Name)
	}
	return refs
}

var _ ports.NetworkProviderDryRun = (*KubeOVNNetworkProviderAdapter)(nil)
var _ ports.NetworkProviderApply = (*KubeOVNNetworkProviderAdapter)(nil)
var _ ports.NetworkProviderStatusReader = (*KubeOVNNetworkProviderAdapter)(nil)
var _ KubernetesNetworkProviderClient = (*KubernetesRESTClient)(nil)
