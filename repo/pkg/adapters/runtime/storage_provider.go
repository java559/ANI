package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

type KubernetesStorageProviderClient interface {
	ServerSideDryRun(ctx context.Context, manifests []ports.WorkloadManifest) (ports.WorkloadProviderDryRunResult, error)
	ApplyManifests(ctx context.Context, manifests []ports.WorkloadManifest) ([]string, error)
	ObserveStorageResource(ctx context.Context, request ports.StorageProviderStatusRequest) (ports.StorageProviderStatusResult, error)
}

type KubernetesStorageProviderAdapter struct {
	client       KubernetesStorageProviderClient
	applyEnabled bool
	now          func() time.Time
}

type KubernetesStorageProviderOption func(*KubernetesStorageProviderAdapter)

func WithKubernetesStorageProviderApplyEnabled(enabled bool) KubernetesStorageProviderOption {
	return func(adapter *KubernetesStorageProviderAdapter) {
		adapter.applyEnabled = enabled
	}
}

func WithKubernetesStorageProviderClock(now func() time.Time) KubernetesStorageProviderOption {
	return func(adapter *KubernetesStorageProviderAdapter) {
		if now != nil {
			adapter.now = now
		}
	}
}

func NewKubernetesStorageProviderAdapter(client KubernetesStorageProviderClient, options ...KubernetesStorageProviderOption) *KubernetesStorageProviderAdapter {
	adapter := &KubernetesStorageProviderAdapter{
		client: client,
		now:    time.Now,
	}
	for _, option := range options {
		option(adapter)
	}
	return adapter
}

func (a *KubernetesStorageProviderAdapter) DryRun(ctx context.Context, request ports.StorageProviderDryRunRequest) (ports.StorageProviderDryRunResult, error) {
	if err := validateStorageProviderDryRunRequest(request); err != nil {
		return ports.StorageProviderDryRunResult{}, err
	}
	if a.client == nil {
		return ports.StorageProviderDryRunResult{}, ports.ErrNotConfigured
	}
	result, err := a.client.ServerSideDryRun(ctx, request.Manifests)
	if err != nil {
		return ports.StorageProviderDryRunResult{}, err
	}
	return ports.StorageProviderDryRunResult{
		Accepted:      result.Accepted,
		Provider:      result.Provider,
		ManifestCount: result.ManifestCount,
		ResourceRefs:  storageResourceRefs(request.Manifests),
		Reason:        firstNetworkNonEmpty(result.Reason, "accepted by Kubernetes storage provider dry-run"),
		Warnings:      append([]string(nil), result.Warnings...),
		CheckedAt:     firstNonZeroTime(result.CheckedAt, a.now().UTC()),
	}, nil
}

func (a *KubernetesStorageProviderAdapter) Apply(ctx context.Context, request ports.StorageProviderApplyRequest) (ports.StorageProviderApplyResult, error) {
	if !a.applyEnabled {
		return ports.StorageProviderApplyResult{
			Applied:       false,
			Provider:      request.DryRunResult.Provider,
			ManifestCount: len(request.Manifests),
			Operation:     request.Operation,
			ResourceRefs:  append([]string(nil), request.DryRunResult.ResourceRefs...),
			Reason:        "storage provider apply is disabled by execution switch",
			Warnings:      append([]string(nil), request.DryRunResult.Warnings...),
			AppliedAt:     a.now().UTC(),
		}, nil
	}
	if err := validateStorageProviderApplyRequest(request); err != nil {
		return ports.StorageProviderApplyResult{}, err
	}
	if a.client == nil {
		return ports.StorageProviderApplyResult{}, ports.ErrNotConfigured
	}
	refs, err := a.client.ApplyManifests(ctx, request.Manifests)
	if err != nil {
		return ports.StorageProviderApplyResult{}, err
	}
	return ports.StorageProviderApplyResult{
		Applied:       true,
		Provider:      request.Manifests[0].Provider,
		ManifestCount: len(request.Manifests),
		Operation:     request.Operation,
		ResourceRefs:  refs,
		Reason:        "applied by Kubernetes storage provider adapter",
		Warnings:      append([]string(nil), request.DryRunResult.Warnings...),
		AppliedAt:     a.now().UTC(),
	}, nil
}

func (a *KubernetesStorageProviderAdapter) Observe(ctx context.Context, request ports.StorageProviderStatusRequest) (ports.StorageProviderStatusResult, error) {
	if err := validateStorageProviderStatusRequest(request); err != nil {
		return ports.StorageProviderStatusResult{}, err
	}
	if a.client == nil {
		return ports.StorageProviderStatusResult{}, ports.ErrNotConfigured
	}
	result, err := a.client.ObserveStorageResource(ctx, request)
	if err != nil {
		return ports.StorageProviderStatusResult{}, err
	}
	if result.TenantID != request.TenantID || result.ResourceKind != request.ResourceKind || result.ResourceID != request.ResourceID {
		return ports.StorageProviderStatusResult{}, fmt.Errorf("%w: storage provider status identity does not match request", ports.ErrInvalid)
	}
	if result.Provider == "" {
		result.Provider = request.ApplyResult.Provider
	}
	if len(result.ResourceRefs) == 0 {
		return ports.StorageProviderStatusResult{}, fmt.Errorf("%w: storage provider status must include resource refs", ports.ErrInvalid)
	}
	if result.State == "" {
		result.State = ports.StorageResourcePending
	}
	if result.ObservedAt.IsZero() {
		result.ObservedAt = a.now().UTC()
	}
	return result, nil
}

func validateStorageProviderDryRunRequest(request ports.StorageProviderDryRunRequest) error {
	if err := requireStorageProviderIdentity(request.TenantID, request.UserID, request.ResourceKind, request.ResourceID, request.PermissionProof); err != nil {
		return err
	}
	if request.Operation != ports.StorageProviderOperationCreate {
		return fmt.Errorf("%w: storage provider dry-run currently allows create only", ports.ErrInvalid)
	}
	return validateStorageProviderManifests(request.Manifests)
}

func validateStorageProviderApplyRequest(request ports.StorageProviderApplyRequest) error {
	if err := requireStorageProviderIdentity(request.TenantID, request.UserID, request.ResourceKind, request.ResourceID, request.PermissionProof); err != nil {
		return err
	}
	if request.Operation != ports.StorageProviderOperationCreate {
		return fmt.Errorf("%w: storage provider apply currently allows create only", ports.ErrInvalid)
	}
	if !request.DryRunResult.Accepted {
		return fmt.Errorf("%w: storage provider dry-run must be accepted before apply", ports.ErrInvalid)
	}
	if request.DryRunResult.ManifestCount != 0 && request.DryRunResult.ManifestCount != len(request.Manifests) {
		return fmt.Errorf("%w: storage provider dry-run manifest count does not match apply request", ports.ErrInvalid)
	}
	return validateStorageProviderManifests(request.Manifests)
}

func validateStorageProviderStatusRequest(request ports.StorageProviderStatusRequest) error {
	if err := requireStorageProviderIdentity(request.TenantID, request.UserID, request.ResourceKind, request.ResourceID, request.PermissionProof); err != nil {
		return err
	}
	if !request.ApplyResult.Applied {
		return fmt.Errorf("%w: storage provider apply must be applied before status observation", ports.ErrInvalid)
	}
	if request.ApplyResult.Provider == "" {
		return fmt.Errorf("%w: storage provider apply result provider is required for status observation", ports.ErrInvalid)
	}
	if len(request.ApplyResult.ResourceRefs) == 0 {
		return fmt.Errorf("%w: storage provider resource refs are required for status observation", ports.ErrInvalid)
	}
	return nil
}

func requireStorageProviderIdentity(tenantID string, userID string, resourceKind string, resourceID string, permissionProof string) error {
	if tenantID == "" {
		return fmt.Errorf("%w: tenant id is required for storage provider execution", ports.ErrInvalid)
	}
	if userID == "" {
		return fmt.Errorf("%w: user id is required for storage provider execution", ports.ErrInvalid)
	}
	if resourceKind == "" {
		return fmt.Errorf("%w: resource kind is required for storage provider execution", ports.ErrInvalid)
	}
	if resourceID == "" {
		return fmt.Errorf("%w: resource id is required for storage provider execution", ports.ErrInvalid)
	}
	if permissionProof == "" {
		return fmt.Errorf("%w: permission proof is required for storage provider execution", ports.ErrInvalid)
	}
	return nil
}

func validateStorageProviderManifests(manifests []ports.WorkloadManifest) error {
	if len(manifests) == 0 {
		return fmt.Errorf("%w: at least one storage provider manifest is required", ports.ErrInvalid)
	}
	for _, manifest := range manifests {
		if manifest.Provider != "kubernetes" {
			return fmt.Errorf("%w: storage provider currently supports Kubernetes PVC manifests only", ports.ErrUnsupported)
		}
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

func storageResourceRefs(manifests []ports.WorkloadManifest) []string {
	refs := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		refs = append(refs, manifest.Provider+"/"+manifest.Kind+"/"+manifest.Name)
	}
	return refs
}

var _ ports.StorageProviderDryRun = (*KubernetesStorageProviderAdapter)(nil)
var _ ports.StorageProviderApply = (*KubernetesStorageProviderAdapter)(nil)
var _ ports.StorageProviderStatusReader = (*KubernetesStorageProviderAdapter)(nil)
var _ KubernetesStorageProviderClient = (*KubernetesRESTClient)(nil)
