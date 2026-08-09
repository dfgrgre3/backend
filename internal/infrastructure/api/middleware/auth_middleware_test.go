package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetUserIDFromContext_UsesPreferredAndLegacyKeys(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("user_id", "user-123")
	assert.Equal(t, "user-123", getUserIDFromContext(ctx))

	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("userId", "user-456")
	assert.Equal(t, "user-456", getUserIDFromContext(ctx))
}

func TestGetRoleFromContext_UsesPreferredAndLegacyKeys(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("user_role", "ADMIN")
	assert.Equal(t, "ADMIN", getRoleFromContext(ctx))

	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("role", "MODERATOR")
	assert.Equal(t, "MODERATOR", getRoleFromContext(ctx))
}
