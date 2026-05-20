package runtime

import (
	"context"
	"testing"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestLocalVectorStoreServiceDevProfile(t *testing.T) {
	service := NewLocalVectorStoreService()

	store, err := service.CreateVectorStore(context.Background(), ports.VectorStoreCreateRequest{
		TenantID:  "tenant-a",
		Name:      "kb-main",
		Dimension: 3,
		Metric:    "cosine",
	})
	if err != nil {
		t.Fatalf("CreateVectorStore() error = %v", err)
	}
	if store.StoreID == "" || store.State != ports.VectorStoreReady || store.Metric != "cosine" {
		t.Fatalf("store = %#v, want ready cosine store", store)
	}
	if _, err := service.GetVectorStore(context.Background(), ports.VectorStoreResourceGetRequest{TenantID: "tenant-b", ResourceID: store.StoreID}); err == nil {
		t.Fatalf("GetVectorStore from another tenant succeeded, want isolation error")
	}
	results, err := service.SearchVectorStore(context.Background(), ports.VectorStoreResourceSearchRequest{
		TenantID:   "tenant-a",
		ResourceID: store.StoreID,
		Vector:     []float32{0.1, 0.2, 0.3},
		TopK:       5,
	})
	if err != nil {
		t.Fatalf("SearchVectorStore() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %d, want empty local dev profile result", len(results))
	}
	deleted, err := service.DeleteVectorStore(context.Background(), ports.VectorStoreResourceGetRequest{TenantID: "tenant-a", ResourceID: store.StoreID})
	if err != nil {
		t.Fatalf("DeleteVectorStore() error = %v", err)
	}
	if deleted.State != ports.VectorStoreDeleted {
		t.Fatalf("deleted state = %q, want deleted", deleted.State)
	}
}

func TestLocalVectorStoreServiceSearchValidatesDimension(t *testing.T) {
	service := NewLocalVectorStoreService()
	store, err := service.CreateVectorStore(context.Background(), ports.VectorStoreCreateRequest{
		TenantID:  "tenant-a",
		Name:      "kb-main",
		Dimension: 3,
	})
	if err != nil {
		t.Fatalf("CreateVectorStore() error = %v", err)
	}

	_, err = service.SearchVectorStore(context.Background(), ports.VectorStoreResourceSearchRequest{
		TenantID:   "tenant-a",
		ResourceID: store.StoreID,
		Vector:     []float32{0.1, 0.2},
	})
	if err == nil {
		t.Fatalf("SearchVectorStore() error = nil, want dimension mismatch")
	}
}
