package router

import (
	"context"
	"testing"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestStorageAPIDevProfileVolumeFilesystemAndObject(t *testing.T) {
	api := newStorageAPI()
	volume, err := api.service.CreateVolume(context.Background(), ports.StorageVolumeCreateRequest{
		TenantID:     "tenant-a",
		Name:         "data-a",
		SizeGiB:      100,
		StorageClass: "fast",
	})
	if err != nil {
		t.Fatalf("CreateVolume error = %v", err)
	}
	if got := storageVolumeFromRecord(volume); got.ID == "" || got.State != "available" || got.TenantID != "tenant-a" {
		t.Fatalf("volume response = %+v, want available tenant-a volume", got)
	} else {
		requireLocalCoreDevProfile(t, got.DevProfile, "local-storage-service")
	}
	filesystem, err := api.service.CreateFilesystem(context.Background(), ports.StorageFilesystemCreateRequest{
		TenantID: "tenant-a",
		Name:     "shared",
		Protocol: "nfs",
		SizeGiB:  500,
	})
	if err != nil {
		t.Fatalf("CreateFilesystem error = %v", err)
	}
	if got := storageFilesystemFromRecord(filesystem); got.ID == "" || got.Protocol != "nfs" || got.Endpoint == "" {
		t.Fatalf("filesystem response = %+v, want nfs endpoint", got)
	} else {
		requireLocalCoreDevProfile(t, got.DevProfile, "local-storage-service")
	}
	object, err := api.service.CreateObject(context.Background(), ports.StorageObjectCreateRequest{
		TenantID:    "tenant-a",
		Bucket:      "models",
		Key:         "llm/model.bin",
		SizeBytes:   1024,
		ContentType: "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("CreateObject error = %v", err)
	}
	if got := storageObjectFromRecord(object); got.ID == "" || got.Bucket != "models" || got.State != "available" {
		t.Fatalf("object response = %+v, want object metadata", got)
	} else {
		requireLocalCoreDevProfile(t, got.DevProfile, "local-storage-service")
	}
}

func TestStorageAPIServiceKeepsTenantIsolation(t *testing.T) {
	api := newStorageAPI()
	volume, err := api.service.CreateVolume(context.Background(), ports.StorageVolumeCreateRequest{
		TenantID: "tenant-a",
		Name:     "tenant-a-volume",
		SizeGiB:  10,
	})
	if err != nil {
		t.Fatalf("CreateVolume error = %v", err)
	}
	if _, err := api.service.GetVolume(context.Background(), ports.StorageResourceGetRequest{
		TenantID:   "tenant-b",
		ResourceID: volume.VolumeID,
	}); err == nil {
		t.Fatalf("GetVolume from another tenant succeeded, want isolation error")
	}
}
