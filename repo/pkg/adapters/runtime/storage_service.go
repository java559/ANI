package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kubercloud/ani/pkg/ports"
)

type LocalStorageService struct {
	mu          sync.RWMutex
	now         func() time.Time
	store       ports.StorageResourceStore
	volumes     map[string]ports.StorageVolumeRecord
	filesystems map[string]ports.StorageFilesystemRecord
	objects     map[string]ports.StorageObjectRecord
}

type StorageServiceOption func(*LocalStorageService)

func WithStorageServiceClock(now func() time.Time) StorageServiceOption {
	return func(service *LocalStorageService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithStorageResourceStore(store ports.StorageResourceStore) StorageServiceOption {
	return func(service *LocalStorageService) {
		service.store = store
	}
}

func NewLocalStorageService(options ...StorageServiceOption) *LocalStorageService {
	service := &LocalStorageService{
		now:         func() time.Time { return time.Now().UTC() },
		volumes:     map[string]ports.StorageVolumeRecord{},
		filesystems: map[string]ports.StorageFilesystemRecord{},
		objects:     map[string]ports.StorageObjectRecord{},
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *LocalStorageService) CreateVolume(ctx context.Context, request ports.StorageVolumeCreateRequest) (ports.StorageVolumeRecord, error) {
	if err := requireStorageTenantAndName(request.TenantID, request.Name); err != nil {
		return ports.StorageVolumeRecord{}, err
	}
	if request.SizeGiB <= 0 {
		return ports.StorageVolumeRecord{}, fmt.Errorf("%w: volume size_gib must be greater than zero", ports.ErrInvalid)
	}
	now := s.now().UTC()
	record := ports.StorageVolumeRecord{
		TenantID:     request.TenantID,
		VolumeID:     "vol_" + uuid.NewString(),
		Name:         strings.TrimSpace(request.Name),
		SizeGiB:      request.SizeGiB,
		StorageClass: firstNetworkNonEmpty(request.StorageClass, "standard"),
		State:        ports.StorageResourceAvailable,
		Reason:       "created by local storage profile",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.volumes[record.VolumeID] = record
	if err := s.upsertVolume(ctx, record); err != nil {
		return ports.StorageVolumeRecord{}, err
	}
	return record, nil
}

func (s *LocalStorageService) ListVolumes(_ context.Context, request ports.StorageResourceListRequest) ([]ports.StorageVolumeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]ports.StorageVolumeRecord, 0, len(s.volumes))
	for _, record := range s.volumes {
		if record.TenantID == request.TenantID && record.State != ports.StorageResourceDeleted {
			items = append(items, record)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (s *LocalStorageService) GetVolume(_ context.Context, request ports.StorageResourceGetRequest) (ports.StorageVolumeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.volumes[request.ResourceID]
	if !ok || record.TenantID != request.TenantID || record.State == ports.StorageResourceDeleted {
		return ports.StorageVolumeRecord{}, ports.ErrNotFound
	}
	return record, nil
}

func (s *LocalStorageService) DeleteVolume(ctx context.Context, request ports.StorageResourceGetRequest) (ports.StorageVolumeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.volumes[request.ResourceID]
	if !ok || record.TenantID != request.TenantID || record.State == ports.StorageResourceDeleted {
		return ports.StorageVolumeRecord{}, ports.ErrNotFound
	}
	record.State = ports.StorageResourceDeleted
	record.Reason = "deleted by local storage profile"
	record.UpdatedAt = s.now().UTC()
	s.volumes[record.VolumeID] = record
	if err := s.upsertVolume(ctx, record); err != nil {
		return ports.StorageVolumeRecord{}, err
	}
	return record, nil
}

func (s *LocalStorageService) CreateFilesystem(ctx context.Context, request ports.StorageFilesystemCreateRequest) (ports.StorageFilesystemRecord, error) {
	if err := requireStorageTenantAndName(request.TenantID, request.Name); err != nil {
		return ports.StorageFilesystemRecord{}, err
	}
	if request.SizeGiB <= 0 {
		return ports.StorageFilesystemRecord{}, fmt.Errorf("%w: filesystem size_gib must be greater than zero", ports.ErrInvalid)
	}
	protocol := strings.ToLower(strings.TrimSpace(request.Protocol))
	if protocol == "" {
		protocol = "nfs"
	}
	if protocol != "nfs" && protocol != "cephfs" {
		return ports.StorageFilesystemRecord{}, fmt.Errorf("%w: unsupported filesystem protocol %q", ports.ErrUnsupported, request.Protocol)
	}
	now := s.now().UTC()
	record := ports.StorageFilesystemRecord{
		TenantID:     request.TenantID,
		FilesystemID: "fs_" + uuid.NewString(),
		Name:         strings.TrimSpace(request.Name),
		Protocol:     protocol,
		SizeGiB:      request.SizeGiB,
		Endpoint:     "local://" + strings.TrimSpace(request.Name),
		State:        ports.StorageResourceAvailable,
		Reason:       "created by local storage profile",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filesystems[record.FilesystemID] = record
	if err := s.upsertFilesystem(ctx, record); err != nil {
		return ports.StorageFilesystemRecord{}, err
	}
	return record, nil
}

func (s *LocalStorageService) ListFilesystems(_ context.Context, request ports.StorageResourceListRequest) ([]ports.StorageFilesystemRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]ports.StorageFilesystemRecord, 0, len(s.filesystems))
	for _, record := range s.filesystems {
		if record.TenantID == request.TenantID && record.State != ports.StorageResourceDeleted {
			items = append(items, record)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (s *LocalStorageService) GetFilesystem(_ context.Context, request ports.StorageResourceGetRequest) (ports.StorageFilesystemRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.filesystems[request.ResourceID]
	if !ok || record.TenantID != request.TenantID || record.State == ports.StorageResourceDeleted {
		return ports.StorageFilesystemRecord{}, ports.ErrNotFound
	}
	return record, nil
}

func (s *LocalStorageService) DeleteFilesystem(ctx context.Context, request ports.StorageResourceGetRequest) (ports.StorageFilesystemRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.filesystems[request.ResourceID]
	if !ok || record.TenantID != request.TenantID || record.State == ports.StorageResourceDeleted {
		return ports.StorageFilesystemRecord{}, ports.ErrNotFound
	}
	record.State = ports.StorageResourceDeleted
	record.Reason = "deleted by local storage profile"
	record.UpdatedAt = s.now().UTC()
	s.filesystems[record.FilesystemID] = record
	if err := s.upsertFilesystem(ctx, record); err != nil {
		return ports.StorageFilesystemRecord{}, err
	}
	return record, nil
}

func (s *LocalStorageService) CreateObject(ctx context.Context, request ports.StorageObjectCreateRequest) (ports.StorageObjectRecord, error) {
	if strings.TrimSpace(request.TenantID) == "" {
		return ports.StorageObjectRecord{}, fmt.Errorf("%w: tenant_id is required", ports.ErrInvalid)
	}
	if strings.TrimSpace(request.Bucket) == "" || strings.TrimSpace(request.Key) == "" {
		return ports.StorageObjectRecord{}, fmt.Errorf("%w: bucket and key are required", ports.ErrInvalid)
	}
	if request.SizeBytes < 0 {
		return ports.StorageObjectRecord{}, fmt.Errorf("%w: object size_bytes must not be negative", ports.ErrInvalid)
	}
	now := s.now().UTC()
	record := ports.StorageObjectRecord{
		TenantID:    request.TenantID,
		ObjectID:    "obj_" + uuid.NewString(),
		Bucket:      strings.TrimSpace(request.Bucket),
		Key:         strings.TrimSpace(request.Key),
		SizeBytes:   request.SizeBytes,
		ContentType: firstNetworkNonEmpty(request.ContentType, "application/octet-stream"),
		State:       ports.StorageResourceAvailable,
		Reason:      "created by local storage profile",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[record.ObjectID] = record
	if err := s.upsertObject(ctx, record); err != nil {
		return ports.StorageObjectRecord{}, err
	}
	return record, nil
}

func (s *LocalStorageService) ListObjects(_ context.Context, request ports.StorageResourceListRequest) ([]ports.StorageObjectRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]ports.StorageObjectRecord, 0, len(s.objects))
	for _, record := range s.objects {
		if record.TenantID == request.TenantID && record.State != ports.StorageResourceDeleted {
			items = append(items, record)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (s *LocalStorageService) GetObject(_ context.Context, request ports.StorageResourceGetRequest) (ports.StorageObjectRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.objects[request.ResourceID]
	if !ok || record.TenantID != request.TenantID || record.State == ports.StorageResourceDeleted {
		return ports.StorageObjectRecord{}, ports.ErrNotFound
	}
	return record, nil
}

func (s *LocalStorageService) DeleteObject(ctx context.Context, request ports.StorageResourceGetRequest) (ports.StorageObjectRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.objects[request.ResourceID]
	if !ok || record.TenantID != request.TenantID || record.State == ports.StorageResourceDeleted {
		return ports.StorageObjectRecord{}, ports.ErrNotFound
	}
	record.State = ports.StorageResourceDeleted
	record.Reason = "deleted by local storage profile"
	record.UpdatedAt = s.now().UTC()
	s.objects[record.ObjectID] = record
	if err := s.upsertObject(ctx, record); err != nil {
		return ports.StorageObjectRecord{}, err
	}
	return record, nil
}

func (s *LocalStorageService) upsertVolume(ctx context.Context, record ports.StorageVolumeRecord) error {
	if s.store == nil {
		return nil
	}
	return s.store.UpsertVolume(ctx, record)
}

func (s *LocalStorageService) upsertFilesystem(ctx context.Context, record ports.StorageFilesystemRecord) error {
	if s.store == nil {
		return nil
	}
	return s.store.UpsertFilesystem(ctx, record)
}

func (s *LocalStorageService) upsertObject(ctx context.Context, record ports.StorageObjectRecord) error {
	if s.store == nil {
		return nil
	}
	return s.store.UpsertObject(ctx, record)
}

func requireStorageTenantAndName(tenantID string, name string) error {
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("%w: tenant_id is required", ports.ErrInvalid)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name is required", ports.ErrInvalid)
	}
	return nil
}

var _ ports.StorageService = (*LocalStorageService)(nil)
