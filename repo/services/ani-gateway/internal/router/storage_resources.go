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

type storageAPI struct {
	service ports.StorageService
}

type storageCreateVolumeRequest struct {
	Name         string `json:"name"`
	SizeGiB      int64  `json:"size_gib"`
	StorageClass string `json:"storage_class"`
}

type storageCreateFilesystemRequest struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	SizeGiB  int64  `json:"size_gib"`
}

type storageCreateObjectRequest struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

type storageVolumeResponse struct {
	ID           string                 `json:"id"`
	TenantID     string                 `json:"tenant_id"`
	Name         string                 `json:"name"`
	SizeGiB      int64                  `json:"size_gib"`
	StorageClass string                 `json:"storage_class"`
	State        string                 `json:"state"`
	Reason       string                 `json:"reason,omitempty"`
	DevProfile   coreDevProfileResponse `json:"dev_profile"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

type storageFilesystemResponse struct {
	ID         string                 `json:"id"`
	TenantID   string                 `json:"tenant_id"`
	Name       string                 `json:"name"`
	Protocol   string                 `json:"protocol"`
	SizeGiB    int64                  `json:"size_gib"`
	Endpoint   string                 `json:"endpoint,omitempty"`
	State      string                 `json:"state"`
	Reason     string                 `json:"reason,omitempty"`
	DevProfile coreDevProfileResponse `json:"dev_profile"`
	CreatedAt  string                 `json:"created_at"`
	UpdatedAt  string                 `json:"updated_at"`
}

type storageObjectResponse struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	Bucket      string                 `json:"bucket"`
	Key         string                 `json:"key"`
	SizeBytes   int64                  `json:"size_bytes"`
	ContentType string                 `json:"content_type"`
	State       string                 `json:"state"`
	Reason      string                 `json:"reason,omitempty"`
	DevProfile  coreDevProfileResponse `json:"dev_profile"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

func newStorageAPI() *storageAPI {
	return &storageAPI{service: runtimeadapter.NewLocalStorageService()}
}

func registerStorageResources(v1 *route.RouterGroup) {
	api := newStorageAPI()
	v1.GET("/volumes", api.listVolumes)
	v1.POST("/volumes", api.createVolume)
	v1.GET("/volumes/:volume_id", api.getVolume)
	v1.DELETE("/volumes/:volume_id", api.deleteVolume)

	v1.GET("/filesystems", api.listFilesystems)
	v1.POST("/filesystems", api.createFilesystem)
	v1.GET("/filesystems/:filesystem_id", api.getFilesystem)
	v1.DELETE("/filesystems/:filesystem_id", api.deleteFilesystem)

	v1.GET("/objects", api.listObjects)
	v1.POST("/objects", api.createObject)
	v1.GET("/objects/:object_id", api.getObject)
	v1.DELETE("/objects/:object_id", api.deleteObject)
}

func (api *storageAPI) createVolume(ctx context.Context, c *app.RequestContext) {
	var req storageCreateVolumeRequest
	if err := c.BindJSON(&req); err != nil {
		writeDemoError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid volume request")
		return
	}
	record, err := api.service.CreateVolume(ctx, ports.StorageVolumeCreateRequest{
		TenantID:     demoTenantID(c),
		Name:         req.Name,
		SizeGiB:      req.SizeGiB,
		StorageClass: req.StorageClass,
	})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, storageVolumeFromRecord(record))
}

func (api *storageAPI) listVolumes(ctx context.Context, c *app.RequestContext) {
	records, err := api.service.ListVolumes(ctx, ports.StorageResourceListRequest{TenantID: demoTenantID(c)})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	items := make([]storageVolumeResponse, 0, len(records))
	for _, record := range records {
		items = append(items, storageVolumeFromRecord(record))
	}
	c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (api *storageAPI) getVolume(ctx context.Context, c *app.RequestContext) {
	record, err := api.service.GetVolume(ctx, ports.StorageResourceGetRequest{TenantID: demoTenantID(c), ResourceID: c.Param("volume_id")})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, storageVolumeFromRecord(record))
}

func (api *storageAPI) deleteVolume(ctx context.Context, c *app.RequestContext) {
	record, err := api.service.DeleteVolume(ctx, ports.StorageResourceGetRequest{TenantID: demoTenantID(c), ResourceID: c.Param("volume_id")})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, storageVolumeFromRecord(record))
}

func (api *storageAPI) createFilesystem(ctx context.Context, c *app.RequestContext) {
	var req storageCreateFilesystemRequest
	if err := c.BindJSON(&req); err != nil {
		writeDemoError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid filesystem request")
		return
	}
	record, err := api.service.CreateFilesystem(ctx, ports.StorageFilesystemCreateRequest{
		TenantID: demoTenantID(c),
		Name:     req.Name,
		Protocol: req.Protocol,
		SizeGiB:  req.SizeGiB,
	})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, storageFilesystemFromRecord(record))
}

func (api *storageAPI) listFilesystems(ctx context.Context, c *app.RequestContext) {
	records, err := api.service.ListFilesystems(ctx, ports.StorageResourceListRequest{TenantID: demoTenantID(c)})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	items := make([]storageFilesystemResponse, 0, len(records))
	for _, record := range records {
		items = append(items, storageFilesystemFromRecord(record))
	}
	c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (api *storageAPI) getFilesystem(ctx context.Context, c *app.RequestContext) {
	record, err := api.service.GetFilesystem(ctx, ports.StorageResourceGetRequest{TenantID: demoTenantID(c), ResourceID: c.Param("filesystem_id")})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, storageFilesystemFromRecord(record))
}

func (api *storageAPI) deleteFilesystem(ctx context.Context, c *app.RequestContext) {
	record, err := api.service.DeleteFilesystem(ctx, ports.StorageResourceGetRequest{TenantID: demoTenantID(c), ResourceID: c.Param("filesystem_id")})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, storageFilesystemFromRecord(record))
}

func (api *storageAPI) createObject(ctx context.Context, c *app.RequestContext) {
	var req storageCreateObjectRequest
	if err := c.BindJSON(&req); err != nil {
		writeDemoError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid object request")
		return
	}
	record, err := api.service.CreateObject(ctx, ports.StorageObjectCreateRequest{
		TenantID:    demoTenantID(c),
		Bucket:      req.Bucket,
		Key:         req.Key,
		SizeBytes:   req.SizeBytes,
		ContentType: req.ContentType,
	})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, storageObjectFromRecord(record))
}

func (api *storageAPI) listObjects(ctx context.Context, c *app.RequestContext) {
	records, err := api.service.ListObjects(ctx, ports.StorageResourceListRequest{TenantID: demoTenantID(c)})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	items := make([]storageObjectResponse, 0, len(records))
	for _, record := range records {
		items = append(items, storageObjectFromRecord(record))
	}
	c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (api *storageAPI) getObject(ctx context.Context, c *app.RequestContext) {
	record, err := api.service.GetObject(ctx, ports.StorageResourceGetRequest{TenantID: demoTenantID(c), ResourceID: c.Param("object_id")})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, storageObjectFromRecord(record))
}

func (api *storageAPI) deleteObject(ctx context.Context, c *app.RequestContext) {
	record, err := api.service.DeleteObject(ctx, ports.StorageResourceGetRequest{TenantID: demoTenantID(c), ResourceID: c.Param("object_id")})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, storageObjectFromRecord(record))
}

func storageVolumeFromRecord(record ports.StorageVolumeRecord) storageVolumeResponse {
	return storageVolumeResponse{
		ID:           record.VolumeID,
		TenantID:     record.TenantID,
		Name:         record.Name,
		SizeGiB:      record.SizeGiB,
		StorageClass: record.StorageClass,
		State:        string(record.State),
		Reason:       record.Reason,
		DevProfile:   localCoreDevProfile("local-storage-service", "Core dev/local profile; provider execution is gated separately"),
		CreatedAt:    networkTime(record.CreatedAt),
		UpdatedAt:    networkTime(record.UpdatedAt),
	}
}

func storageFilesystemFromRecord(record ports.StorageFilesystemRecord) storageFilesystemResponse {
	return storageFilesystemResponse{
		ID:         record.FilesystemID,
		TenantID:   record.TenantID,
		Name:       record.Name,
		Protocol:   record.Protocol,
		SizeGiB:    record.SizeGiB,
		Endpoint:   record.Endpoint,
		State:      string(record.State),
		Reason:     record.Reason,
		DevProfile: localCoreDevProfile("local-storage-service", "Core dev/local profile; provider execution is gated separately"),
		CreatedAt:  networkTime(record.CreatedAt),
		UpdatedAt:  networkTime(record.UpdatedAt),
	}
}

func storageObjectFromRecord(record ports.StorageObjectRecord) storageObjectResponse {
	return storageObjectResponse{
		ID:          record.ObjectID,
		TenantID:    record.TenantID,
		Bucket:      record.Bucket,
		Key:         record.Key,
		SizeBytes:   record.SizeBytes,
		ContentType: record.ContentType,
		State:       string(record.State),
		Reason:      record.Reason,
		DevProfile:  localCoreDevProfile("local-storage-service", "Core dev/local profile; provider execution is gated separately"),
		CreatedAt:   networkTime(record.CreatedAt),
		UpdatedAt:   networkTime(record.UpdatedAt),
	}
}

func writeStorageError(c *app.RequestContext, err error) {
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
