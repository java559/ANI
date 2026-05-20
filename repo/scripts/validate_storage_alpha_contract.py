#!/usr/bin/env python3
"""Validate Sprint 3 M1-STORAGE-A Core API contract."""

from pathlib import Path
import re
import sys
import yaml


EXPECTED_PATHS = {
    "/volumes": {
        "get": ("listStorageVolumes", "scope:volumes:read", {"200", "401", "403"}),
        "post": ("createStorageVolume", "scope:volumes:create", {"201", "400", "401", "403"}),
    },
    "/volumes/{volume_id}": {
        "get": ("getStorageVolume", "scope:volumes:read", {"200", "401", "403", "404"}),
        "delete": ("deleteStorageVolume", "scope:volumes:delete", {"200", "401", "403", "404"}),
    },
    "/filesystems": {
        "get": ("listStorageFilesystems", "scope:filesystems:read", {"200", "401", "403"}),
        "post": ("createStorageFilesystem", "scope:filesystems:create", {"201", "400", "401", "403"}),
    },
    "/filesystems/{filesystem_id}": {
        "get": ("getStorageFilesystem", "scope:filesystems:read", {"200", "401", "403", "404"}),
        "delete": ("deleteStorageFilesystem", "scope:filesystems:delete", {"200", "401", "403", "404"}),
    },
    "/objects": {
        "get": ("listStorageObjects", "scope:objects:read", {"200", "401", "403"}),
        "post": ("createStorageObject", "scope:objects:create", {"201", "400", "401", "403"}),
    },
    "/objects/{object_id}": {
        "get": ("getStorageObject", "scope:objects:read", {"200", "401", "403", "404"}),
        "delete": ("deleteStorageObject", "scope:objects:delete", {"200", "401", "403", "404"}),
    },
}

EXPECTED_SCHEMAS = {
    "StorageResourceState",
    "StorageVolume",
    "StorageFilesystem",
    "StorageObject",
    "StorageVolumeListResponse",
    "StorageFilesystemListResponse",
    "StorageObjectListResponse",
    "CreateStorageVolumeRequest",
    "CreateStorageFilesystemRequest",
    "CreateStorageObjectRequest",
}

EXPECTED_FIELDS = {
    "StorageVolume": {"id", "tenant_id", "name", "size_gib", "storage_class", "state", "reason", "created_at", "updated_at"},
    "StorageFilesystem": {"id", "tenant_id", "name", "protocol", "size_gib", "endpoint", "state", "reason", "created_at", "updated_at"},
    "StorageObject": {"id", "tenant_id", "bucket", "key", "size_bytes", "content_type", "state", "reason", "created_at", "updated_at"},
}

EXPECTED_ROUTES = {
    'v1.GET("/volumes"',
    'v1.POST("/volumes"',
    'v1.GET("/volumes/:volume_id"',
    'v1.DELETE("/volumes/:volume_id"',
    'v1.GET("/filesystems"',
    'v1.POST("/filesystems"',
    'v1.GET("/objects"',
    'v1.POST("/objects"',
}


def load_yaml(path: Path) -> dict:
    with path.open(encoding="utf-8") as handle:
        return yaml.safe_load(handle)


def fail(errors: list[str]) -> None:
    if errors:
        for error in errors:
            print(f"storage alpha contract error: {error}", file=sys.stderr)
        raise SystemExit(1)


def validate_openapi(root: Path, errors: list[str]) -> None:
    core = load_yaml(root / "api/openapi/v1.yaml")
    services = load_yaml(root / "api/openapi/services/v1.yaml")
    paths = core.get("paths", {})
    schemas = core.get("components", {}).get("schemas", {})
    for path, methods in EXPECTED_PATHS.items():
        if path not in paths:
            errors.append(f"api/openapi/v1.yaml missing path {path}")
            continue
        for method, (operation_id, scope, expected_responses) in methods.items():
            operation = paths[path].get(method)
            if not operation:
                errors.append(f"api/openapi/v1.yaml missing {method.upper()} {path}")
                continue
            if operation.get("operationId") != operation_id:
                errors.append(f"{method.upper()} {path} operationId must be {operation_id}")
            if operation.get("x-ani-rbac-scope") != scope:
                errors.append(f"{method.upper()} {path} RBAC scope must be {scope}")
            missing = expected_responses - set(operation.get("responses", {}).keys())
            if missing:
                errors.append(f"{method.upper()} {path} missing responses: {sorted(missing)}")

    for schema in EXPECTED_SCHEMAS:
        if schema not in schemas:
            errors.append(f"api/openapi/v1.yaml missing schema {schema}")
    for schema, fields in EXPECTED_FIELDS.items():
        properties = schemas.get(schema, {}).get("properties", {})
        missing = fields - set(properties.keys())
        if missing:
            errors.append(f"schema {schema} missing fields: {sorted(missing)}")
    expected_states = {"pending", "available", "failed", "deleting", "deleted"}
    if set(schemas.get("StorageResourceState", {}).get("enum", [])) != expected_states:
        errors.append(f"StorageResourceState enum must be {sorted(expected_states)}")

    service_paths = services.get("paths", {})
    leaked = [path for path in service_paths if path.startswith(("/volumes", "/filesystems", "/objects"))]
    if leaked:
        errors.append(f"Services API must not contain Core storage paths: {leaked}")


def validate_gateway(root: Path, errors: list[str]) -> None:
    routes_go = (root / "services/ani-gateway/internal/router/storage_resources.go").read_text(encoding="utf-8")
    router_go = (root / "services/ani-gateway/internal/router/router.go").read_text(encoding="utf-8")
    ports_go = (root / "pkg/ports/storage_resources.go").read_text(encoding="utf-8")
    adapter_go = (root / "pkg/adapters/runtime/storage_service.go").read_text(encoding="utf-8")
    store_go = (root / "pkg/adapters/runtime/storage_store.go").read_text(encoding="utf-8")
    renderer_go = (root / "pkg/adapters/runtime/storage_renderer.go").read_text(encoding="utf-8")
    provider_go = (root / "pkg/adapters/runtime/storage_provider.go").read_text(encoding="utf-8")
    reconciler_go = (root / "pkg/adapters/runtime/storage_status_reconciler.go").read_text(encoding="utf-8")
    bootstrap_go = (root / "pkg/bootstrap/deps.go").read_text(encoding="utf-8")
    for route in EXPECTED_ROUTES:
        if route not in routes_go:
            errors.append(f"storage_resources.go missing route token {route}")
    if "registerStorageResources(v1)" not in router_go:
        errors.append("router.go must register storage resources")
    for token in ("StorageService interface", "StorageResourceStore interface", "StorageProviderRenderer interface", "StorageProviderDryRun interface", "StorageProviderApply interface", "StorageProviderStatusReader interface", "StorageStatusReconciler interface", "StorageResourceState", "StorageVolumeRecord", "StorageFilesystemRecord", "StorageObjectRecord"):
        if token not in ports_go:
            errors.append(f"pkg/ports/storage_resources.go missing token {token}")
    for token in ("NewLocalStorageService", "WithStorageResourceStore", "CreateVolume", "CreateFilesystem", "CreateObject", "DeleteVolume"):
        if token not in adapter_go:
            errors.append(f"storage_service.go missing token {token}")
    for token in ("MetadataStorageStore", "UpsertVolume", "UpsertFilesystem", "UpsertObject", "UpdateResourceState"):
        if token not in store_go:
            errors.append(f"storage_store.go missing token {token}")
    for token in ("KubernetesStorageRenderer", "RenderVolume", "RenderFilesystem", "RenderObject", "PersistentVolumeClaim", "ObjectMetadata"):
        if token not in renderer_go:
            errors.append(f"storage_renderer.go missing token {token}")
    for token in ("KubernetesStorageProviderAdapter", "DryRun", "Apply", "Observe", "WithKubernetesStorageProviderApplyEnabled", "Kubernetes PVC manifests only"):
        if token not in provider_go:
            errors.append(f"storage_provider.go missing token {token}")
    for token in ("LocalStorageStatusReconciler", "Reconcile", "StorageResourceStateUpdateRequest", "resource refs"):
        if token not in reconciler_go:
            errors.append(f"storage_status_reconciler.go missing token {token}")
    for pattern, label in (
        (r"StorageStore\s+ports\.StorageResourceStore", "StorageStore ports.StorageResourceStore"),
        (r"StorageRenderer\s+ports\.StorageProviderRenderer", "StorageRenderer ports.StorageProviderRenderer"),
        (r"StorageDryRun\s+ports\.StorageProviderDryRun", "StorageDryRun ports.StorageProviderDryRun"),
        (r"StorageApply\s+ports\.StorageProviderApply", "StorageApply ports.StorageProviderApply"),
        (r"StorageStatus\s+ports\.StorageProviderStatusReader", "StorageStatus ports.StorageProviderStatusReader"),
        (r"StorageReconcile\s+ports\.StorageStatusReconciler", "StorageReconcile ports.StorageStatusReconciler"),
        (r"StorageResources\s+ports\.StorageService", "StorageResources ports.StorageService"),
        (r"NewMetadataStorageStore", "NewMetadataStorageStore"),
        (r"NewKubernetesStorageRenderer", "NewKubernetesStorageRenderer"),
        (r"NewKubernetesStorageProviderAdapter", "NewKubernetesStorageProviderAdapter"),
        (r"NewLocalStorageStatusReconciler", "NewLocalStorageStatusReconciler"),
        (r"NewLocalStorageService", "NewLocalStorageService"),
    ):
        if not re.search(pattern, bootstrap_go):
            errors.append(f"pkg/bootstrap/deps.go missing token {label}")


def validate_persistence(root: Path, errors: list[str]) -> None:
    migration = root / "deploy/migrations/20260520_006_storage_resources.sql"
    if not migration.exists():
        errors.append("missing storage persistence migration 20260520_006_storage_resources.sql")
        return
    sql = migration.read_text(encoding="utf-8")
    for table in (
        "storage_volumes",
        "storage_filesystems",
        "storage_objects",
    ):
        if f"CREATE TABLE IF NOT EXISTS {table}" not in sql:
            errors.append(f"storage migration missing table {table}")
        if f"ALTER TABLE {table} ENABLE ROW LEVEL SECURITY" not in sql:
            errors.append(f"storage migration missing RLS enable for {table}")
        if f"DROP POLICY IF EXISTS tenant_isolation ON {table}" not in sql:
            errors.append(f"storage migration missing tenant policy reset for {table}")
    for token in ("current_setting('app.current_tenant_id'", "CHECK (state IN", "PRIMARY KEY (tenant_id", "GRANT SELECT, INSERT, UPDATE, DELETE ON"):
        if token not in sql:
            errors.append(f"storage migration missing token {token}")


def main() -> None:
    root = Path(__file__).resolve().parents[1]
    errors: list[str] = []
    validate_openapi(root, errors)
    validate_gateway(root, errors)
    validate_persistence(root, errors)
    fail(errors)
    print("storage alpha contract valid")


if __name__ == "__main__":
    main()
