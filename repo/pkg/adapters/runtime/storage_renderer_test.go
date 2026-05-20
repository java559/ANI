package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestKubernetesStorageRendererRendersVolumePVC(t *testing.T) {
	renderer := NewKubernetesStorageRenderer()

	manifests, err := renderer.RenderVolume(context.Background(), ports.StorageVolumeRecord{
		TenantID:     "tenant-a",
		VolumeID:     "vol_data",
		Name:         "data",
		SizeGiB:      100,
		StorageClass: "fast",
		State:        ports.StorageResourceAvailable,
	})
	if err != nil {
		t.Fatalf("RenderVolume() error = %v", err)
	}
	content := manifests[0].Content
	for _, want := range []string{`"kind": "PersistentVolumeClaim"`, `"storage": "100Gi"`, `"storageClassName": "fast"`, "vol-vol-data", "ani-tenant-tenant-a", "ReadWriteOnce"} {
		if !strings.Contains(content, want) {
			t.Fatalf("rendered volume PVC missing %q:\n%s", want, content)
		}
	}
	dryRun, err := NewLocalProviderDryRun().DryRun(context.Background(), manifests, ports.WorkloadAdmissionResult{Allowed: true})
	if err != nil {
		t.Fatalf("DryRun(rendered PVC) error = %v", err)
	}
	if !dryRun.Accepted {
		t.Fatalf("DryRun(rendered PVC) accepted = false, reason = %s", dryRun.Reason)
	}
}

func TestKubernetesStorageRendererRendersFilesystemPVC(t *testing.T) {
	renderer := NewKubernetesStorageRenderer()

	manifests, err := renderer.RenderFilesystem(context.Background(), ports.StorageFilesystemRecord{
		TenantID:     "tenant-a",
		FilesystemID: "fs_shared",
		Name:         "shared",
		Protocol:     "cephfs",
		SizeGiB:      500,
		Endpoint:     "local://shared",
		State:        ports.StorageResourceAvailable,
	})
	if err != nil {
		t.Fatalf("RenderFilesystem() error = %v", err)
	}
	content := manifests[0].Content
	for _, want := range []string{`"kind": "PersistentVolumeClaim"`, `"storage": "500Gi"`, `"storageClassName": "cephfs"`, "ReadWriteMany", "filesystem-protocol", "local://shared"} {
		if !strings.Contains(content, want) {
			t.Fatalf("rendered filesystem PVC missing %q:\n%s", want, content)
		}
	}
}

func TestKubernetesStorageRendererRendersObjectMetadataIntent(t *testing.T) {
	renderer := NewKubernetesStorageRenderer()

	manifests, err := renderer.RenderObject(context.Background(), ports.StorageObjectRecord{
		TenantID:    "tenant-a",
		ObjectID:    "obj_model",
		Bucket:      "models",
		Key:         "llm/model.bin",
		SizeBytes:   1024,
		ContentType: "application/octet-stream",
		State:       ports.StorageResourceAvailable,
	})
	if err != nil {
		t.Fatalf("RenderObject() error = %v", err)
	}
	if manifests[0].Provider != "objectstore" || manifests[0].Kind != "ObjectMetadata" {
		t.Fatalf("manifest = %#v, want objectstore ObjectMetadata", manifests[0])
	}
	content := manifests[0].Content
	for _, want := range []string{`"kind": "ObjectMetadata"`, `"bucket": "models"`, `"key": "llm/model.bin"`, `"sizeBytes": 1024`, "obj-obj-model"} {
		if !strings.Contains(content, want) {
			t.Fatalf("rendered object metadata missing %q:\n%s", want, content)
		}
	}
}
