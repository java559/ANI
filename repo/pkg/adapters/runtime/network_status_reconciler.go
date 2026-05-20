package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

type LocalNetworkStatusReconciler struct {
	store ports.NetworkResourceStore
	now   func() time.Time
}

type NetworkStatusReconcilerOption func(*LocalNetworkStatusReconciler)

func WithNetworkStatusReconcileClock(now func() time.Time) NetworkStatusReconcilerOption {
	return func(reconciler *LocalNetworkStatusReconciler) {
		if now != nil {
			reconciler.now = now
		}
	}
}

func NewLocalNetworkStatusReconciler(store ports.NetworkResourceStore, options ...NetworkStatusReconcilerOption) *LocalNetworkStatusReconciler {
	reconciler := &LocalNetworkStatusReconciler{
		store: store,
		now:   time.Now,
	}
	for _, option := range options {
		option(reconciler)
	}
	return reconciler
}

func (r *LocalNetworkStatusReconciler) Reconcile(ctx context.Context, request ports.NetworkReconcileRequest) (ports.NetworkReconcileResult, error) {
	if err := validateNetworkReconcileRequest(request); err != nil {
		return ports.NetworkReconcileResult{}, err
	}
	if r.store == nil {
		return ports.NetworkReconcileResult{}, ports.ErrNotConfigured
	}
	reconciledAt := firstNonZeroTime(request.Observation.ObservedAt, r.now().UTC())
	if err := r.store.UpdateResourceState(ctx, ports.NetworkResourceStateUpdateRequest{
		TenantID:     request.TenantID,
		ResourceKind: request.ResourceKind,
		ResourceID:   request.ResourceID,
		State:        request.Observation.State,
		Reason:       request.Observation.Reason,
		UpdatedAt:    reconciledAt,
	}); err != nil {
		return ports.NetworkReconcileResult{}, err
	}
	return ports.NetworkReconcileResult{
		TenantID:     request.TenantID,
		ResourceKind: request.ResourceKind,
		ResourceID:   request.ResourceID,
		State:        request.Observation.State,
		Reason:       request.Observation.Reason,
		Persisted:    true,
		ReconciledAt: reconciledAt,
	}, nil
}

func validateNetworkReconcileRequest(request ports.NetworkReconcileRequest) error {
	if strings.TrimSpace(request.TenantID) == "" {
		return fmt.Errorf("%w: tenant id is required for network reconcile", ports.ErrInvalid)
	}
	if strings.TrimSpace(request.ResourceKind) == "" {
		return fmt.Errorf("%w: resource kind is required for network reconcile", ports.ErrInvalid)
	}
	if strings.TrimSpace(request.ResourceID) == "" {
		return fmt.Errorf("%w: resource id is required for network reconcile", ports.ErrInvalid)
	}
	if !request.ApplyResult.Applied {
		return fmt.Errorf("%w: network provider apply must be applied before reconcile", ports.ErrInvalid)
	}
	if request.Observation.TenantID != request.TenantID ||
		request.Observation.ResourceKind != request.ResourceKind ||
		request.Observation.ResourceID != request.ResourceID {
		return fmt.Errorf("%w: network observation identity does not match reconcile request", ports.ErrInvalid)
	}
	if request.Observation.Provider == "" {
		return fmt.Errorf("%w: network observation provider is required", ports.ErrInvalid)
	}
	if request.ApplyResult.Provider != "" && request.ApplyResult.Provider != request.Observation.Provider {
		return fmt.Errorf("%w: network observation provider does not match apply result", ports.ErrInvalid)
	}
	if len(request.ApplyResult.ResourceRefs) == 0 {
		return fmt.Errorf("%w: network apply resource refs are required before reconcile", ports.ErrInvalid)
	}
	if len(request.Observation.ResourceRefs) == 0 {
		return fmt.Errorf("%w: network observation resource refs are required", ports.ErrInvalid)
	}
	if !resourceRefsOverlap(request.ApplyResult.ResourceRefs, request.Observation.ResourceRefs) {
		return fmt.Errorf("%w: network observation resource refs do not match apply result", ports.ErrInvalid)
	}
	if request.Observation.State == "" {
		return fmt.Errorf("%w: network observation state is required", ports.ErrInvalid)
	}
	return nil
}

var _ ports.NetworkStatusReconciler = (*LocalNetworkStatusReconciler)(nil)
