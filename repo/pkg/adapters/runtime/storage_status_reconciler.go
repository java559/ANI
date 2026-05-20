package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

type LocalStorageStatusReconciler struct {
	store ports.StorageResourceStore
	now   func() time.Time
}

type StorageStatusReconcilerOption func(*LocalStorageStatusReconciler)

func WithStorageStatusReconcileClock(now func() time.Time) StorageStatusReconcilerOption {
	return func(reconciler *LocalStorageStatusReconciler) {
		if now != nil {
			reconciler.now = now
		}
	}
}

func NewLocalStorageStatusReconciler(store ports.StorageResourceStore, options ...StorageStatusReconcilerOption) *LocalStorageStatusReconciler {
	reconciler := &LocalStorageStatusReconciler{
		store: store,
		now:   time.Now,
	}
	for _, option := range options {
		option(reconciler)
	}
	return reconciler
}

func (r *LocalStorageStatusReconciler) Reconcile(ctx context.Context, request ports.StorageReconcileRequest) (ports.StorageReconcileResult, error) {
	if err := validateStorageReconcileRequest(request); err != nil {
		return ports.StorageReconcileResult{}, err
	}
	if r.store == nil {
		return ports.StorageReconcileResult{}, ports.ErrNotConfigured
	}
	reconciledAt := firstNonZeroTime(request.Observation.ObservedAt, r.now().UTC())
	if err := r.store.UpdateResourceState(ctx, ports.StorageResourceStateUpdateRequest{
		TenantID:     request.TenantID,
		ResourceKind: request.ResourceKind,
		ResourceID:   request.ResourceID,
		State:        request.Observation.State,
		Reason:       request.Observation.Reason,
		UpdatedAt:    reconciledAt,
	}); err != nil {
		return ports.StorageReconcileResult{}, err
	}
	return ports.StorageReconcileResult{
		TenantID:     request.TenantID,
		ResourceKind: request.ResourceKind,
		ResourceID:   request.ResourceID,
		State:        request.Observation.State,
		Reason:       request.Observation.Reason,
		Persisted:    true,
		ReconciledAt: reconciledAt,
	}, nil
}

func validateStorageReconcileRequest(request ports.StorageReconcileRequest) error {
	if strings.TrimSpace(request.TenantID) == "" {
		return fmt.Errorf("%w: tenant id is required for storage reconcile", ports.ErrInvalid)
	}
	if strings.TrimSpace(request.ResourceKind) == "" {
		return fmt.Errorf("%w: resource kind is required for storage reconcile", ports.ErrInvalid)
	}
	if strings.TrimSpace(request.ResourceID) == "" {
		return fmt.Errorf("%w: resource id is required for storage reconcile", ports.ErrInvalid)
	}
	if !request.ApplyResult.Applied {
		return fmt.Errorf("%w: storage provider apply must be applied before reconcile", ports.ErrInvalid)
	}
	if request.Observation.TenantID != request.TenantID ||
		request.Observation.ResourceKind != request.ResourceKind ||
		request.Observation.ResourceID != request.ResourceID {
		return fmt.Errorf("%w: storage observation identity does not match reconcile request", ports.ErrInvalid)
	}
	if request.Observation.Provider == "" {
		return fmt.Errorf("%w: storage observation provider is required", ports.ErrInvalid)
	}
	if request.ApplyResult.Provider != "" && request.ApplyResult.Provider != request.Observation.Provider {
		return fmt.Errorf("%w: storage observation provider does not match apply result", ports.ErrInvalid)
	}
	if len(request.ApplyResult.ResourceRefs) == 0 {
		return fmt.Errorf("%w: storage apply resource refs are required before reconcile", ports.ErrInvalid)
	}
	if len(request.Observation.ResourceRefs) == 0 {
		return fmt.Errorf("%w: storage observation resource refs are required", ports.ErrInvalid)
	}
	if !resourceRefsOverlap(request.ApplyResult.ResourceRefs, request.Observation.ResourceRefs) {
		return fmt.Errorf("%w: storage observation resource refs do not match apply result", ports.ErrInvalid)
	}
	if request.Observation.State == "" {
		return fmt.Errorf("%w: storage observation state is required", ports.ErrInvalid)
	}
	return nil
}

var _ ports.StorageStatusReconciler = (*LocalStorageStatusReconciler)(nil)
