package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestLocalStorageStatusReconcilerPersistsObservation(t *testing.T) {
	tx := &fakeMetadataTx{}
	store := NewMetadataStorageStore(fakeMetadataStore{tx: tx})
	reconciler := NewLocalStorageStatusReconciler(store, WithStorageStatusReconcileClock(func() time.Time {
		return time.Unix(1800, 0)
	}))

	result, err := reconciler.Reconcile(context.Background(), ports.StorageReconcileRequest{
		TenantID:     storageStoreTenantID,
		ResourceKind: "volume",
		ResourceID:   "vol-test",
		ApplyResult: ports.StorageProviderApplyResult{
			Applied:      true,
			Provider:     "kubernetes",
			ResourceRefs: []string{"kubernetes/PersistentVolumeClaim/vol-vol-test"},
		},
		Observation: ports.StorageProviderStatusResult{
			TenantID:     storageStoreTenantID,
			ResourceKind: "volume",
			ResourceID:   "vol-test",
			Provider:     "kubernetes",
			ResourceRefs: []string{"kubernetes/PersistentVolumeClaim/vol-vol-test"},
			State:        ports.StorageResourceAvailable,
			Reason:       "observed Kubernetes PVC phase Bound",
		},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.Persisted || result.State != ports.StorageResourceAvailable {
		t.Fatalf("result = %#v, want persisted available", result)
	}
	if !strings.Contains(tx.sql, "UPDATE storage_volumes") {
		t.Fatalf("sql = %q, want storage_volumes update", tx.sql)
	}
}

func TestLocalStorageStatusReconcilerRejectsMismatchedRefs(t *testing.T) {
	reconciler := NewLocalStorageStatusReconciler(NewMetadataStorageStore(fakeMetadataStore{tx: &fakeMetadataTx{}}))

	_, err := reconciler.Reconcile(context.Background(), ports.StorageReconcileRequest{
		TenantID:     storageStoreTenantID,
		ResourceKind: "volume",
		ResourceID:   "vol-test",
		ApplyResult: ports.StorageProviderApplyResult{
			Applied:      true,
			Provider:     "kubernetes",
			ResourceRefs: []string{"kubernetes/PersistentVolumeClaim/vol-vol-test"},
		},
		Observation: ports.StorageProviderStatusResult{
			TenantID:     storageStoreTenantID,
			ResourceKind: "volume",
			ResourceID:   "vol-test",
			Provider:     "kubernetes",
			ResourceRefs: []string{"kubernetes/PersistentVolumeClaim/vol-other"},
			State:        ports.StorageResourceAvailable,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "resource refs") {
		t.Fatalf("Reconcile() error = %v, want resource refs mismatch", err)
	}
}
