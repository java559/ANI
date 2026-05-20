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

type LocalVectorStoreService struct {
	mu      sync.RWMutex
	now     func() time.Time
	backend ports.VectorStore
	stores  map[string]ports.VectorStoreRecord
}

type VectorStoreServiceOption func(*LocalVectorStoreService)

func WithVectorStoreServiceClock(now func() time.Time) VectorStoreServiceOption {
	return func(service *LocalVectorStoreService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithVectorStoreBackend(backend ports.VectorStore) VectorStoreServiceOption {
	return func(service *LocalVectorStoreService) {
		service.backend = backend
	}
}

func NewLocalVectorStoreService(options ...VectorStoreServiceOption) *LocalVectorStoreService {
	service := &LocalVectorStoreService{
		now:    func() time.Time { return time.Now().UTC() },
		stores: map[string]ports.VectorStoreRecord{},
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *LocalVectorStoreService) CreateVectorStore(ctx context.Context, request ports.VectorStoreCreateRequest) (ports.VectorStoreRecord, error) {
	if err := requireVectorStoreTenantAndName(request.TenantID, request.Name); err != nil {
		return ports.VectorStoreRecord{}, err
	}
	if request.Dimension <= 0 {
		return ports.VectorStoreRecord{}, fmt.Errorf("%w: vector store dimension must be greater than zero", ports.ErrInvalid)
	}
	metric := strings.ToLower(firstNetworkNonEmpty(request.Metric, "cosine"))
	if metric != "cosine" && metric != "l2" && metric != "ip" {
		return ports.VectorStoreRecord{}, fmt.Errorf("%w: unsupported vector store metric %q", ports.ErrUnsupported, request.Metric)
	}

	now := s.now().UTC()
	record := ports.VectorStoreRecord{
		TenantID:  request.TenantID,
		StoreID:   "vst_" + uuid.NewString(),
		Name:      strings.TrimSpace(request.Name),
		Dimension: request.Dimension,
		Metric:    metric,
		State:     ports.VectorStoreReady,
		Reason:    "created by local vector store profile",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if s.backend != nil {
		if err := s.backend.EnsureCollection(ctx, vectorCollectionRef(record), record.Dimension); err != nil {
			return ports.VectorStoreRecord{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.stores[record.StoreID] = record
	return record, nil
}

func (s *LocalVectorStoreService) ListVectorStores(_ context.Context, request ports.VectorStoreResourceListRequest) ([]ports.VectorStoreRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]ports.VectorStoreRecord, 0, len(s.stores))
	for _, record := range s.stores {
		if record.TenantID == request.TenantID && record.State != ports.VectorStoreDeleted {
			items = append(items, record)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (s *LocalVectorStoreService) GetVectorStore(_ context.Context, request ports.VectorStoreResourceGetRequest) (ports.VectorStoreRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.stores[request.ResourceID]
	if !ok || record.TenantID != request.TenantID || record.State == ports.VectorStoreDeleted {
		return ports.VectorStoreRecord{}, ports.ErrNotFound
	}
	return record, nil
}

func (s *LocalVectorStoreService) DeleteVectorStore(_ context.Context, request ports.VectorStoreResourceGetRequest) (ports.VectorStoreRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.stores[request.ResourceID]
	if !ok || record.TenantID != request.TenantID || record.State == ports.VectorStoreDeleted {
		return ports.VectorStoreRecord{}, ports.ErrNotFound
	}
	record.State = ports.VectorStoreDeleted
	record.Reason = "deleted by local vector store profile"
	record.UpdatedAt = s.now().UTC()
	s.stores[record.StoreID] = record
	return record, nil
}

func (s *LocalVectorStoreService) SearchVectorStore(ctx context.Context, request ports.VectorStoreResourceSearchRequest) ([]ports.VectorSearchResult, error) {
	record, err := s.GetVectorStore(ctx, ports.VectorStoreResourceGetRequest{TenantID: request.TenantID, ResourceID: request.ResourceID})
	if err != nil {
		return nil, err
	}
	if len(request.Vector) != record.Dimension {
		return nil, fmt.Errorf("%w: vector dimension does not match vector store dimension", ports.ErrInvalid)
	}
	topK := request.TopK
	if topK <= 0 {
		topK = 10
	}
	if topK > 100 {
		return nil, fmt.Errorf("%w: vector search top_k must not exceed 100", ports.ErrInvalid)
	}
	if s.backend == nil {
		return []ports.VectorSearchResult{}, nil
	}
	return s.backend.Search(ctx, ports.VectorSearchQuery{
		Collection: vectorCollectionRef(record),
		Vector:     append([]float32(nil), request.Vector...),
		TopK:       topK,
		Filter:     cloneStringMap(request.Filter),
	})
}

func requireVectorStoreTenantAndName(tenantID string, name string) error {
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("%w: tenant_id is required", ports.ErrInvalid)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name is required", ports.ErrInvalid)
	}
	return nil
}

func vectorCollectionRef(record ports.VectorStoreRecord) ports.VectorCollectionRef {
	return ports.VectorCollectionRef{
		TenantID: record.TenantID,
		KBID:     record.StoreID,
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

var _ ports.VectorStoreService = (*LocalVectorStoreService)(nil)
