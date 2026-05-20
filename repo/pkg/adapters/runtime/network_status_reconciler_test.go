package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestLocalNetworkStatusReconcilerPersistsObservation(t *testing.T) {
	tx := &fakeMetadataTx{}
	reconciler := NewLocalNetworkStatusReconciler(
		NewMetadataNetworkStore(fakeMetadataStore{tx: tx}),
		WithNetworkStatusReconcileClock(func() time.Time { return time.Unix(150, 0) }),
	)

	result, err := reconciler.Reconcile(context.Background(), ports.NetworkReconcileRequest{
		TenantID:     networkStoreTenantID,
		ResourceKind: "vpc",
		ResourceID:   "vpc-test",
		ApplyResult: ports.NetworkProviderApplyResult{
			Applied:      true,
			Provider:     "kubeovn",
			ResourceRefs: []string{"kubeovn/Vpc/vpc-test"},
		},
		Observation: ports.NetworkProviderStatusResult{
			TenantID:     networkStoreTenantID,
			ResourceKind: "vpc",
			ResourceID:   "vpc-test",
			Provider:     "kubeovn",
			ResourceRefs: []string{"kubeovn/Vpc/vpc-test"},
			State:        ports.NetworkResourceAvailable,
			Reason:       "ready",
			ObservedAt:   time.Unix(140, 0),
		},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.Persisted || result.State != ports.NetworkResourceAvailable {
		t.Fatalf("result = %#v, want persisted available state", result)
	}
	if got, want := tx.args[2], string(ports.NetworkResourceAvailable); got != want {
		t.Fatalf("persisted state = %v, want %s", got, want)
	}
}

func TestLocalNetworkStatusReconcilerRejectsMismatchedRefs(t *testing.T) {
	_, err := NewLocalNetworkStatusReconciler(NewMetadataNetworkStore(fakeMetadataStore{tx: &fakeMetadataTx{}})).Reconcile(context.Background(), ports.NetworkReconcileRequest{
		TenantID:     networkStoreTenantID,
		ResourceKind: "vpc",
		ResourceID:   "vpc-test",
		ApplyResult: ports.NetworkProviderApplyResult{
			Applied:      true,
			Provider:     "kubeovn",
			ResourceRefs: []string{"kubeovn/Vpc/vpc-test"},
		},
		Observation: ports.NetworkProviderStatusResult{
			TenantID:     networkStoreTenantID,
			ResourceKind: "vpc",
			ResourceID:   "vpc-test",
			Provider:     "kubeovn",
			ResourceRefs: []string{"kubeovn/Vpc/other-vpc"},
			State:        ports.NetworkResourceAvailable,
		},
	})
	if err == nil {
		t.Fatalf("Reconcile() error = nil, want mismatched refs error")
	}
}
