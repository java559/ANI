package router

import (
	"context"
	"errors"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	runtimeadapter "github.com/kubercloud/ani/pkg/adapters/runtime"
	"github.com/kubercloud/ani/pkg/ports"
)

type vectorStoreAPI struct {
	service ports.VectorStoreService
}

type createVectorStoreRequest struct {
	Name      string `json:"name"`
	Dimension int    `json:"dimension"`
	Metric    string `json:"metric"`
}

type searchVectorStoreRequest struct {
	Vector []float32         `json:"vector"`
	TopK   int               `json:"top_k"`
	Filter map[string]string `json:"filter"`
}

type vectorStoreResponse struct {
	ID         string                 `json:"id"`
	TenantID   string                 `json:"tenant_id"`
	Name       string                 `json:"name"`
	Dimension  int                    `json:"dimension"`
	Metric     string                 `json:"metric"`
	State      string                 `json:"state"`
	Reason     string                 `json:"reason,omitempty"`
	DevProfile coreDevProfileResponse `json:"dev_profile"`
	CreatedAt  string                 `json:"created_at"`
	UpdatedAt  string                 `json:"updated_at"`
}

type vectorSearchHitResponse struct {
	ID       string            `json:"id"`
	Score    float32           `json:"score"`
	Metadata map[string]string `json:"metadata"`
}

func newVectorStoreAPI() *vectorStoreAPI {
	return &vectorStoreAPI{service: runtimeadapter.NewLocalVectorStoreService()}
}

func registerVectorStoreResources(v1 *route.RouterGroup) {
	api := newVectorStoreAPI()
	v1.GET("/vector-stores", api.listVectorStores)
	v1.POST("/vector-stores", api.createVectorStore)
	v1.GET("/vector-stores/:vector_store_id", api.getVectorStore)
	v1.DELETE("/vector-stores/:vector_store_id", api.deleteVectorStore)
	v1.POST("/vector-stores/:vector_store_id/search", api.searchVectorStore)
}

func (api *vectorStoreAPI) createVectorStore(ctx context.Context, c *app.RequestContext) {
	var req createVectorStoreRequest
	if err := c.BindJSON(&req); err != nil {
		writeDemoError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid vector store request")
		return
	}
	record, err := api.service.CreateVectorStore(ctx, ports.VectorStoreCreateRequest{
		TenantID:  demoTenantID(c),
		Name:      req.Name,
		Dimension: req.Dimension,
		Metric:    req.Metric,
	})
	if err != nil {
		writeVectorStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, vectorStoreFromRecord(record))
}

func (api *vectorStoreAPI) listVectorStores(ctx context.Context, c *app.RequestContext) {
	records, err := api.service.ListVectorStores(ctx, ports.VectorStoreResourceListRequest{TenantID: demoTenantID(c)})
	if err != nil {
		writeVectorStoreError(c, err)
		return
	}
	items := make([]vectorStoreResponse, 0, len(records))
	for _, record := range records {
		items = append(items, vectorStoreFromRecord(record))
	}
	c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (api *vectorStoreAPI) getVectorStore(ctx context.Context, c *app.RequestContext) {
	record, err := api.service.GetVectorStore(ctx, ports.VectorStoreResourceGetRequest{TenantID: demoTenantID(c), ResourceID: c.Param("vector_store_id")})
	if err != nil {
		writeVectorStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, vectorStoreFromRecord(record))
}

func (api *vectorStoreAPI) deleteVectorStore(ctx context.Context, c *app.RequestContext) {
	record, err := api.service.DeleteVectorStore(ctx, ports.VectorStoreResourceGetRequest{TenantID: demoTenantID(c), ResourceID: c.Param("vector_store_id")})
	if err != nil {
		writeVectorStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, vectorStoreFromRecord(record))
}

func (api *vectorStoreAPI) searchVectorStore(ctx context.Context, c *app.RequestContext) {
	var req searchVectorStoreRequest
	if err := c.BindJSON(&req); err != nil {
		writeDemoError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid vector search request")
		return
	}
	results, err := api.service.SearchVectorStore(ctx, ports.VectorStoreResourceSearchRequest{
		TenantID:   demoTenantID(c),
		ResourceID: c.Param("vector_store_id"),
		Vector:     req.Vector,
		TopK:       req.TopK,
		Filter:     req.Filter,
	})
	if err != nil {
		writeVectorStoreError(c, err)
		return
	}
	items := make([]vectorSearchHitResponse, 0, len(results))
	for _, result := range results {
		items = append(items, vectorSearchHitResponse{
			ID:       result.ID,
			Score:    result.Score,
			Metadata: result.Metadata,
		})
	}
	c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func vectorStoreFromRecord(record ports.VectorStoreRecord) vectorStoreResponse {
	return vectorStoreResponse{
		ID:         record.StoreID,
		TenantID:   record.TenantID,
		Name:       record.Name,
		Dimension:  record.Dimension,
		Metric:     record.Metric,
		State:      string(record.State),
		Reason:     record.Reason,
		DevProfile: localCoreDevProfile("local-vector-store-service", "Core dev/local profile; provider execution is gated separately"),
		CreatedAt:  networkTime(record.CreatedAt),
		UpdatedAt:  networkTime(record.UpdatedAt),
	}
}

func writeVectorStoreError(c *app.RequestContext, err error) {
	switch {
	case errors.Is(err, ports.ErrNotFound):
		writeDemoError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, ports.ErrUnsupported):
		writeDemoError(c, http.StatusBadRequest, "UNSUPPORTED", err.Error())
	case errors.Is(err, ports.ErrInvalid):
		writeDemoError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
	default:
		writeDemoError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
	}
}
