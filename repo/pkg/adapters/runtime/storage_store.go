package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubercloud/ani/pkg/ports"
)

type MetadataStorageStore struct {
	store ports.MetadataStore
	now   func() time.Time
}

type StorageStoreOption func(*MetadataStorageStore)

func WithStorageStoreClock(now func() time.Time) StorageStoreOption {
	return func(store *MetadataStorageStore) {
		if now != nil {
			store.now = now
		}
	}
}

func NewMetadataStorageStore(store ports.MetadataStore, options ...StorageStoreOption) *MetadataStorageStore {
	storageStore := &MetadataStorageStore{
		store: store,
		now:   time.Now,
	}
	for _, option := range options {
		option(storageStore)
	}
	return storageStore
}

func (s *MetadataStorageStore) UpsertVolume(ctx context.Context, record ports.StorageVolumeRecord) error {
	if s.store == nil {
		return ports.ErrNotConfigured
	}
	if err := requireStorageRecord(record.TenantID, record.VolumeID, record.Name, record.State); err != nil {
		return err
	}
	if record.SizeGiB <= 0 {
		return fmt.Errorf("%w: volume size_gib must be greater than zero", ports.ErrInvalid)
	}
	createdAt, updatedAt := networkRecordTimes(s.now, record.CreatedAt, record.UpdatedAt)
	return s.store.WithTenantTx(ctx, func(ctx context.Context, tx ports.MetadataTx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO storage_volumes (tenant_id, volume_id, name, size_gib, storage_class, state, reason, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9)
			ON CONFLICT (tenant_id, volume_id) DO UPDATE SET
				name = EXCLUDED.name,
				size_gib = EXCLUDED.size_gib,
				storage_class = EXCLUDED.storage_class,
				state = EXCLUDED.state,
				reason = EXCLUDED.reason,
				updated_at = EXCLUDED.updated_at
		`, record.TenantID, record.VolumeID, record.Name, record.SizeGiB, record.StorageClass, string(record.State), record.Reason, createdAt, updatedAt)
		if err != nil {
			return fmt.Errorf("upsert storage volume: %w", err)
		}
		return nil
	})
}

func (s *MetadataStorageStore) UpsertFilesystem(ctx context.Context, record ports.StorageFilesystemRecord) error {
	if s.store == nil {
		return ports.ErrNotConfigured
	}
	if err := requireStorageRecord(record.TenantID, record.FilesystemID, record.Name, record.State); err != nil {
		return err
	}
	if record.SizeGiB <= 0 {
		return fmt.Errorf("%w: filesystem size_gib must be greater than zero", ports.ErrInvalid)
	}
	createdAt, updatedAt := networkRecordTimes(s.now, record.CreatedAt, record.UpdatedAt)
	return s.store.WithTenantTx(ctx, func(ctx context.Context, tx ports.MetadataTx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO storage_filesystems (tenant_id, filesystem_id, name, protocol, size_gib, endpoint, state, reason, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, NULLIF($6, ''), $7, NULLIF($8, ''), $9, $10)
			ON CONFLICT (tenant_id, filesystem_id) DO UPDATE SET
				name = EXCLUDED.name,
				protocol = EXCLUDED.protocol,
				size_gib = EXCLUDED.size_gib,
				endpoint = EXCLUDED.endpoint,
				state = EXCLUDED.state,
				reason = EXCLUDED.reason,
				updated_at = EXCLUDED.updated_at
		`, record.TenantID, record.FilesystemID, record.Name, record.Protocol, record.SizeGiB, record.Endpoint, string(record.State), record.Reason, createdAt, updatedAt)
		if err != nil {
			return fmt.Errorf("upsert storage filesystem: %w", err)
		}
		return nil
	})
}

func (s *MetadataStorageStore) UpsertObject(ctx context.Context, record ports.StorageObjectRecord) error {
	if s.store == nil {
		return ports.ErrNotConfigured
	}
	if strings.TrimSpace(record.TenantID) == "" {
		return fmt.Errorf("%w: tenant_id is required", ports.ErrInvalid)
	}
	if strings.TrimSpace(record.ObjectID) == "" {
		return fmt.Errorf("%w: object id is required", ports.ErrInvalid)
	}
	if strings.TrimSpace(record.Bucket) == "" || strings.TrimSpace(record.Key) == "" {
		return fmt.Errorf("%w: bucket and key are required", ports.ErrInvalid)
	}
	if record.State == "" {
		return fmt.Errorf("%w: state is required", ports.ErrInvalid)
	}
	if record.SizeBytes < 0 {
		return fmt.Errorf("%w: object size_bytes must not be negative", ports.ErrInvalid)
	}
	createdAt, updatedAt := networkRecordTimes(s.now, record.CreatedAt, record.UpdatedAt)
	return s.store.WithTenantTx(ctx, func(ctx context.Context, tx ports.MetadataTx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO storage_objects (tenant_id, object_id, bucket, object_key, size_bytes, content_type, state, reason, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10)
			ON CONFLICT (tenant_id, object_id) DO UPDATE SET
				bucket = EXCLUDED.bucket,
				object_key = EXCLUDED.object_key,
				size_bytes = EXCLUDED.size_bytes,
				content_type = EXCLUDED.content_type,
				state = EXCLUDED.state,
				reason = EXCLUDED.reason,
				updated_at = EXCLUDED.updated_at
		`, record.TenantID, record.ObjectID, record.Bucket, record.Key, record.SizeBytes, record.ContentType, string(record.State), record.Reason, createdAt, updatedAt)
		if err != nil {
			return fmt.Errorf("upsert storage object: %w", err)
		}
		return nil
	})
}

func (s *MetadataStorageStore) UpdateResourceState(ctx context.Context, request ports.StorageResourceStateUpdateRequest) error {
	if s.store == nil {
		return ports.ErrNotConfigured
	}
	if err := requireStorageStateUpdate(request); err != nil {
		return err
	}
	table, idColumn, err := storageResourceStateTable(request.ResourceKind)
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
			return fmt.Errorf("update storage resource state: %w", err)
		}
		if tag.RowsAffected == 0 {
			return ports.ErrNotFound
		}
		return nil
	})
}

func requireStorageRecord(tenantID string, resourceID string, name string, state ports.StorageResourceState) error {
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

func requireStorageStateUpdate(request ports.StorageResourceStateUpdateRequest) error {
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

func storageResourceStateTable(resourceKind string) (string, string, error) {
	switch strings.TrimSpace(resourceKind) {
	case "volume":
		return "storage_volumes", "volume_id", nil
	case "filesystem":
		return "storage_filesystems", "filesystem_id", nil
	case "object":
		return "storage_objects", "object_id", nil
	default:
		return "", "", fmt.Errorf("%w: unsupported storage resource kind %q", ports.ErrUnsupported, resourceKind)
	}
}

var _ ports.StorageResourceStore = (*MetadataStorageStore)(nil)
