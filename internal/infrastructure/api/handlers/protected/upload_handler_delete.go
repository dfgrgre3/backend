package protected

import (
	"net/http"
	"strings"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/storage"

	"github.com/gin-gonic/gin"
)

// ─── Delete Uploaded File ────────────────────────────────────────────────────────

// selfServiceDeletePrefixes are storage folders any authenticated user may
// delete from without holding an elevated permission: their own in-flight
// chunked-upload scratch space. Every other folder (avatars/, covers/,
// books/, lessons/, exams/, attachments/, announcements/, blog/, uploads/)
// is shared/managed content whose keys carry no per-user ownership segment
// (see buildFilename in upload_handler_helpers.go — keys are just
// "folder/<uuid>.<ext>"), so there is no way to verify the caller actually
// owns a given key. Deleting there requires resources:manage.
var selfServiceDeletePrefixes = []string{"temp/"}

// DeleteUpload deletes a previously uploaded file from storage.
//
// SECURITY: file keys carry no ownership information, so any authenticated
// user who could name/guess/observe a file key (e.g. another user's public
// avatar URL) used to be able to delete it — an IDOR. Non-privileged callers
// are now restricted to their own scratch space (temp/); deleting shared or
// managed content requires the resources:manage permission already used to
// gate the equivalent admin upload endpoints.
func DeleteUpload(c *gin.Context) {
	var req struct {
		FileKey string `json:"fileKey" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "fileKey is required")
		return
	}

	// Security: only allow deletion of files in expected folders
	allowedPrefixes := []string{"avatars/", "covers/", "books/", "lessons/", "exams/", "attachments/", "announcements/", "blog/", "uploads/", "temp/"}
	allowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(req.FileKey, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		api_response.Error(c, http.StatusForbidden, "File key is not within an allowed storage path")
		return
	}

	isSelfService := false
	for _, prefix := range selfServiceDeletePrefixes {
		if strings.HasPrefix(req.FileKey, prefix) {
			isSelfService = true
			break
		}
	}
	if !isSelfService {
		actorPermsRaw, _ := c.Get("permissions")
		actorPerms, _ := actorPermsRaw.([]string)
		authorized := false
		for _, grant := range actorPerms {
			if grant == models.PermAdminBypass || models.PermissionGrantMatches(grant, models.PermResourcesManage) {
				authorized = true
				break
			}
		}
		if !authorized {
			api_response.Error(c, http.StatusForbidden, "You do not have permission to delete this file")
			return
		}
	}

	if err := storage.GlobalStorage.Delete(c.Request.Context(), req.FileKey); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete file: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"deleted": req.FileKey})
}
