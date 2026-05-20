package router

import (
	"context"
	"testing"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestVectorStoreAPIDevProfileCreateSearchAndDelete(t *testing.T) {
	api := newVectorStoreAPI()
	store, err := api.service.CreateVectorStore(context.Background(), ports.VectorStoreCreateRequest{
		TenantID:  "tenant-a",
		Name:      "kb-main",
		Dimension: 3,
		Metric:    "cosine",
	})
	if err != nil {
		t.Fatalf("CreateVectorStore error = %v", err)
	}
	if got := vectorStoreFromRecord(store); got.ID == "" || got.State != "ready" || got.Dimension != 3 {
		t.Fatalf("vector store response = %+v, want ready vector store", got)
	} else {
		requireLocalCoreDevProfile(t, got.DevProfile, "local-vector-store-service")
	}
	results, err := api.service.SearchVectorStore(context.Background(), ports.VectorStoreResourceSearchRequest{
		TenantID:   "tenant-a",
		ResourceID: store.StoreID,
		Vector:     []float32{0.1, 0.2, 0.3},
		TopK:       5,
	})
	if err != nil {
		t.Fatalf("SearchVectorStore error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %d, want empty dev profile search result", len(results))
	}
	deleted, err := api.service.DeleteVectorStore(context.Background(), ports.VectorStoreResourceGetRequest{
		TenantID:   "tenant-a",
		ResourceID: store.StoreID,
	})
	if err != nil {
		t.Fatalf("DeleteVectorStore error = %v", err)
	}
	if deleted.State != ports.VectorStoreDeleted {
		t.Fatalf("deleted state = %q, want deleted", deleted.State)
	}
}

func TestVectorStoreAPIServiceKeepsTenantIsolation(t *testing.T) {
	api := newVectorStoreAPI()
	store, err := api.service.CreateVectorStore(context.Background(), ports.VectorStoreCreateRequest{
		TenantID:  "tenant-a",
		Name:      "tenant-a-store",
		Dimension: 3,
	})
	if err != nil {
		t.Fatalf("CreateVectorStore error = %v", err)
	}
	if _, err := api.service.GetVectorStore(context.Background(), ports.VectorStoreResourceGetRequest{
		TenantID:   "tenant-b",
		ResourceID: store.StoreID,
	}); err == nil {
		t.Fatalf("GetVectorStore from another tenant succeeded, want isolation error")
	}
}
