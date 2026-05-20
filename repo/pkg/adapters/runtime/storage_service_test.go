package runtime

import (
	"context"
	"testing"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestLocalStorageServiceVolumeDevProfile(t *testing.T) {
	service := NewLocalStorageService()
	volume, err := service.CreateVolume(context.Background(), ports.StorageVolumeCreateRequest{
		TenantID:     "tenant-a",
		Name:         "data-a",
		SizeGiB:      100,
		StorageClass: "fast",
	})
	if err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}
	if volume.VolumeID == "" || volume.State != ports.StorageResourceAvailable || volume.StorageClass != "fast" {
		t.Fatalf("volume = %#v, want available fast volume", volume)
	}
	if _, err := service.GetVolume(context.Background(), ports.StorageResourceGetRequest{TenantID: "tenant-b", ResourceID: volume.VolumeID}); err == nil {
		t.Fatalf("GetVolume from another tenant succeeded, want isolation error")
	}
	deleted, err := service.DeleteVolume(context.Background(), ports.StorageResourceGetRequest{TenantID: "tenant-a", ResourceID: volume.VolumeID})
	if err != nil {
		t.Fatalf("DeleteVolume() error = %v", err)
	}
	if deleted.State != ports.StorageResourceDeleted {
		t.Fatalf("deleted state = %q, want deleted", deleted.State)
	}
}

func TestLocalStorageServiceFilesystemAndObjectDevProfile(t *testing.T) {
	service := NewLocalStorageService()
	filesystem, err := service.CreateFilesystem(context.Background(), ports.StorageFilesystemCreateRequest{
		TenantID: "tenant-a",
		Name:     "shared",
		Protocol: "cephfs",
		SizeGiB:  500,
	})
	if err != nil {
		t.Fatalf("CreateFilesystem() error = %v", err)
	}
	if filesystem.FilesystemID == "" || filesystem.Protocol != "cephfs" || filesystem.Endpoint == "" {
		t.Fatalf("filesystem = %#v, want cephfs endpoint", filesystem)
	}
	object, err := service.CreateObject(context.Background(), ports.StorageObjectCreateRequest{
		TenantID:    "tenant-a",
		Bucket:      "models",
		Key:         "llm/model.bin",
		SizeBytes:   1024,
		ContentType: "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("CreateObject() error = %v", err)
	}
	if object.ObjectID == "" || object.State != ports.StorageResourceAvailable || object.Bucket != "models" {
		t.Fatalf("object = %#v, want available object metadata", object)
	}
	objects, err := service.ListObjects(context.Background(), ports.StorageResourceListRequest{TenantID: "tenant-a"})
	if err != nil {
		t.Fatalf("ListObjects() error = %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("objects = %d, want 1", len(objects))
	}
}
