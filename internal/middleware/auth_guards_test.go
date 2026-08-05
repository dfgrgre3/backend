package middleware

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestAdminAPIPermission_UploadRoutesRequireResourcesPermissions(t *testing.T) {
    assert.Equal(t, "resources:view", adminAPIPermission("/api/admin/upload", "GET"))
    assert.Equal(t, "resources:manage", adminAPIPermission("/api/admin/upload", "POST"))
    assert.Equal(t, "resources:manage", adminAPIPermission("/api/admin/upload/presign", "POST"))
    assert.Equal(t, "resources:manage", adminAPIPermission("/api/admin/upload/chunked", "POST"))
    assert.Equal(t, "resources:manage", adminAPIPermission("/api/admin/upload/chunked", "PUT"))
    assert.Equal(t, "resources:manage", adminAPIPermission("/api/admin/upload/chunked", "PATCH"))
}
