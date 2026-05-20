package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

type MetadataNetworkStore struct {
	store ports.MetadataStore
	now   func() time.Time
}

type NetworkStoreOption func(*MetadataNetworkStore)

func WithNetworkStoreClock(now func() time.Time) NetworkStoreOption {
	return func(store *MetadataNetworkStore) {
		if now != nil {
			store.now = now
		}
	}
}

func NewMetadataNetworkStore(store ports.MetadataStore, options ...NetworkStoreOption) *MetadataNetworkStore {
	networkStore := &MetadataNetworkStore{
		store: store,
		now:   time.Now,
	}
	for _, option := range options {
		option(networkStore)
	}
	return networkStore
}

func (s *MetadataNetworkStore) UpsertVPC(ctx context.Context, record ports.NetworkVPCRecord) error {
	if s.store == nil {
		return ports.ErrNotConfigured
	}
	if err := requireNetworkRecord(record.TenantID, record.VPCID, record.Name, record.State); err != nil {
		return err
	}
	createdAt, updatedAt := networkRecordTimes(s.now, record.CreatedAt, record.UpdatedAt)
	return s.store.WithTenantTx(ctx, func(ctx context.Context, tx ports.MetadataTx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO network_vpcs (tenant_id, vpc_id, name, cidr, state, reason, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, NULLIF($6, ''), $7, $8)
			ON CONFLICT (tenant_id, vpc_id) DO UPDATE SET
				name = EXCLUDED.name,
				cidr = EXCLUDED.cidr,
				state = EXCLUDED.state,
				reason = EXCLUDED.reason,
				updated_at = EXCLUDED.updated_at
		`, record.TenantID, record.VPCID, record.Name, record.CIDR, string(record.State), record.Reason, createdAt, updatedAt)
		if err != nil {
			return fmt.Errorf("upsert network vpc: %w", err)
		}
		return nil
	})
}

func (s *MetadataNetworkStore) UpsertSubnet(ctx context.Context, record ports.NetworkSubnetRecord) error {
	if s.store == nil {
		return ports.ErrNotConfigured
	}
	if err := requireNetworkRecord(record.TenantID, record.SubnetID, record.Name, record.State); err != nil {
		return err
	}
	if strings.TrimSpace(record.VPCID) == "" {
		return fmt.Errorf("%w: vpc_id is required", ports.ErrInvalid)
	}
	createdAt, updatedAt := networkRecordTimes(s.now, record.CreatedAt, record.UpdatedAt)
	return s.store.WithTenantTx(ctx, func(ctx context.Context, tx ports.MetadataTx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO network_subnets (tenant_id, subnet_id, vpc_id, name, cidr, gateway, state, reason, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, NULLIF($6, ''), $7, NULLIF($8, ''), $9, $10)
			ON CONFLICT (tenant_id, subnet_id) DO UPDATE SET
				vpc_id = EXCLUDED.vpc_id,
				name = EXCLUDED.name,
				cidr = EXCLUDED.cidr,
				gateway = EXCLUDED.gateway,
				state = EXCLUDED.state,
				reason = EXCLUDED.reason,
				updated_at = EXCLUDED.updated_at
		`, record.TenantID, record.SubnetID, record.VPCID, record.Name, record.CIDR, record.Gateway, string(record.State), record.Reason, createdAt, updatedAt)
		if err != nil {
			return fmt.Errorf("upsert network subnet: %w", err)
		}
		return nil
	})
}

func (s *MetadataNetworkStore) UpsertSecurityGroup(ctx context.Context, record ports.NetworkSecurityGroupRecord) error {
	if s.store == nil {
		return ports.ErrNotConfigured
	}
	if err := requireNetworkRecord(record.TenantID, record.SecurityGroupID, record.Name, record.State); err != nil {
		return err
	}
	rules, err := json.Marshal(record.Rules)
	if err != nil {
		return fmt.Errorf("marshal security group rules: %w", err)
	}
	createdAt, updatedAt := networkRecordTimes(s.now, record.CreatedAt, record.UpdatedAt)
	return s.store.WithTenantTx(ctx, func(ctx context.Context, tx ports.MetadataTx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO network_security_groups (tenant_id, security_group_id, name, description, rules, state, reason, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, NULLIF($4, ''), $5::jsonb, $6, NULLIF($7, ''), $8, $9)
			ON CONFLICT (tenant_id, security_group_id) DO UPDATE SET
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				rules = EXCLUDED.rules,
				state = EXCLUDED.state,
				reason = EXCLUDED.reason,
				updated_at = EXCLUDED.updated_at
		`, record.TenantID, record.SecurityGroupID, record.Name, record.Description, string(rules), string(record.State), record.Reason, createdAt, updatedAt)
		if err != nil {
			return fmt.Errorf("upsert network security group: %w", err)
		}
		return nil
	})
}

func (s *MetadataNetworkStore) UpsertLoadBalancer(ctx context.Context, record ports.NetworkLoadBalancerRecord) error {
	if s.store == nil {
		return ports.ErrNotConfigured
	}
	if err := requireNetworkRecord(record.TenantID, record.LoadBalancerID, record.Name, record.State); err != nil {
		return err
	}
	if strings.TrimSpace(record.VPCID) == "" {
		return fmt.Errorf("%w: vpc_id is required", ports.ErrInvalid)
	}
	listeners, err := json.Marshal(record.Listeners)
	if err != nil {
		return fmt.Errorf("marshal load balancer listeners: %w", err)
	}
	createdAt, updatedAt := networkRecordTimes(s.now, record.CreatedAt, record.UpdatedAt)
	return s.store.WithTenantTx(ctx, func(ctx context.Context, tx ports.MetadataTx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO network_load_balancers (tenant_id, load_balancer_id, name, vpc_id, subnet_id, scheme, vip, listeners, state, reason, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, NULLIF($5, ''), $6, NULLIF($7, ''), $8::jsonb, $9, NULLIF($10, ''), $11, $12)
			ON CONFLICT (tenant_id, load_balancer_id) DO UPDATE SET
				name = EXCLUDED.name,
				vpc_id = EXCLUDED.vpc_id,
				subnet_id = EXCLUDED.subnet_id,
				scheme = EXCLUDED.scheme,
				vip = EXCLUDED.vip,
				listeners = EXCLUDED.listeners,
				state = EXCLUDED.state,
				reason = EXCLUDED.reason,
				updated_at = EXCLUDED.updated_at
		`, record.TenantID, record.LoadBalancerID, record.Name, record.VPCID, record.SubnetID, record.Scheme, record.VIP, string(listeners), string(record.State), record.Reason, createdAt, updatedAt)
		if err != nil {
			return fmt.Errorf("upsert network load balancer: %w", err)
		}
		return nil
	})
}

func (s *MetadataNetworkStore) UpdateResourceState(ctx context.Context, request ports.NetworkResourceStateUpdateRequest) error {
	if s.store == nil {
		return ports.ErrNotConfigured
	}
	if err := requireNetworkStateUpdate(request); err != nil {
		return err
	}
	table, idColumn, err := networkResourceStateTable(request.ResourceKind)
	if err != nil {
		return err
	}
	updatedAt := firstNonZeroTime(request.UpdatedAt, s.now().UTC())
	return s.store.WithTenantTx(ctx, func(ctx context.Context, tx ports.MetadataTx) error {
		tag, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s
			SET state = $3,
				reason = NULLIF($4, ''),
				updated_at = $5
			WHERE tenant_id = $1::uuid AND %s = $2
		`, table, idColumn), request.TenantID, request.ResourceID, string(request.State), request.Reason, updatedAt)
		if err != nil {
			return fmt.Errorf("update network resource state: %w", err)
		}
		if tag.RowsAffected == 0 {
			return ports.ErrNotFound
		}
		return nil
	})
}

func requireNetworkRecord(tenantID string, resourceID string, name string, state ports.NetworkResourceState) error {
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("%w: tenant_id is required", ports.ErrInvalid)
	}
	if strings.TrimSpace(resourceID) == "" {
		return fmt.Errorf("%w: resource id is required", ports.ErrInvalid)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name is required", ports.ErrInvalid)
	}
	if state == "" {
		return fmt.Errorf("%w: state is required", ports.ErrInvalid)
	}
	return nil
}

func requireNetworkStateUpdate(request ports.NetworkResourceStateUpdateRequest) error {
	if strings.TrimSpace(request.TenantID) == "" {
		return fmt.Errorf("%w: tenant_id is required", ports.ErrInvalid)
	}
	if strings.TrimSpace(request.ResourceKind) == "" {
		return fmt.Errorf("%w: resource kind is required", ports.ErrInvalid)
	}
	if strings.TrimSpace(request.ResourceID) == "" {
		return fmt.Errorf("%w: resource id is required", ports.ErrInvalid)
	}
	if request.State == "" {
		return fmt.Errorf("%w: state is required", ports.ErrInvalid)
	}
	return nil
}

func networkResourceStateTable(resourceKind string) (string, string, error) {
	switch strings.TrimSpace(resourceKind) {
	case "vpc":
		return "network_vpcs", "vpc_id", nil
	case "subnet":
		return "network_subnets", "subnet_id", nil
	case "security-group":
		return "network_security_groups", "security_group_id", nil
	case "load-balancer":
		return "network_load_balancers", "load_balancer_id", nil
	default:
		return "", "", fmt.Errorf("%w: unsupported network resource kind %q", ports.ErrUnsupported, resourceKind)
	}
}

func networkRecordTimes(now func() time.Time, createdAt time.Time, updatedAt time.Time) (time.Time, time.Time) {
	current := time.Now().UTC()
	if now != nil {
		current = now().UTC()
	}
	createdAt = firstNonZeroTime(createdAt, current)
	updatedAt = firstNonZeroTime(updatedAt, createdAt, current)
	return createdAt, updatedAt
}

var _ ports.NetworkResourceStore = (*MetadataNetworkStore)(nil)
