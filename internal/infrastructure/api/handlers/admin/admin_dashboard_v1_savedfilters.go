package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SaveDashboardFilter يحفظ حالة فلاتر مسماة للمستخدم الحالي.
//
// POST /api/admin/dashboard/saved-filters
func SaveDashboardFilter(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardSaveFilters) {
		return
	}
	conn, abort := safeDB(c)
	if abort {
		return
	}
	callerID, authenticated := getAuthenticatedUserID(c)
	if !authenticated {
		return
	}

	var body struct {
		Name        string          `json:"name"`
		Description *string         `json:"description"`
		FilterState json.RawMessage `json:"filterState"`
		IsDefault   bool            `json:"isDefault"`
		Visibility  string          `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		api_response.Error(c, http.StatusBadRequest, "name is required")
		return
	}
	if len(name) > 120 {
		api_response.Error(c, http.StatusUnprocessableEntity, "name must not exceed 120 characters")
		return
	}

	if len(body.FilterState) == 0 {
		body.FilterState = json.RawMessage(`{}`)
	}
	// filterState يُخزَّن كـ jsonb، فيجب رفض أي حمولة غير صالحة قبل الكتابة.
	var probe map[string]interface{}
	if err := json.Unmarshal(body.FilterState, &probe); err != nil {
		api_response.Error(c, http.StatusUnprocessableEntity, "filterState must be a valid JSON object")
		return
	}

	visibility := strings.ToLower(strings.TrimSpace(body.Visibility))
	if visibility == "" {
		visibility = models.DashboardFilterVisibilityPrivate
	}
	if visibility != models.DashboardFilterVisibilityPrivate &&
		visibility != models.DashboardFilterVisibilityShared {
		api_response.Error(c, http.StatusBadRequest, "visibility must be private or shared")
		return
	}
	// المشاركة تجعل الفلتر مرئيًا لبقية الإداريين، لذا تتطلب إذن الحفظ العام.
	if visibility == models.DashboardFilterVisibilityShared &&
		!dashboardCan(c, models.PermDashboardManage) {
		api_response.Error(c, http.StatusForbidden, "Sharing a filter requires dashboard:manage")
		return
	}

	var duplicate int64
	conn.Model(&models.DashboardSavedFilter{}).
		Where("user_id = ? AND LOWER(name) = LOWER(?)", callerID, name).
		Count(&duplicate)
	if duplicate > 0 {
		api_response.Error(c, http.StatusConflict, "A filter with this name already exists")
		return
	}

	filter := models.DashboardSavedFilter{
		UserID:      callerID,
		Name:        name,
		Description: body.Description,
		FilterState: body.FilterState,
		IsDefault:   body.IsDefault,
		Visibility:  visibility,
	}

	err := conn.Transaction(func(tx *gorm.DB) error {
		// الافتراضي واحد فقط لكل مستخدم.
		if filter.IsDefault {
			if err := tx.Model(&models.DashboardSavedFilter{}).
				Where("user_id = ? AND is_default = true", callerID).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&filter).Error
	})
	if err != nil {
		if IsDuplicateKeyError(err) {
			api_response.Error(c, http.StatusConflict, "A filter with this name already exists")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to save filter")
		return
	}

	api_response.Created(c, gin.H{
		"id":          filter.ID,
		"name":        filter.Name,
		"description": filter.Description,
		"filterState": filter.FilterState,
		"isDefault":   filter.IsDefault,
		"visibility":  filter.Visibility,
		"createdAt":   filter.CreatedAt,
		"updatedAt":   filter.UpdatedAt,
	})
}

// DeleteDashboardSavedFilter يحذف فلترًا محفوظًا حذفًا منطقيًا.
//
// DELETE /api/admin/dashboard/saved-filters/:filterId
func DeleteDashboardSavedFilter(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardDeleteSavedFilters) {
		return
	}
	conn, abort := safeDB(c)
	if abort {
		return
	}
	callerID, authenticated := getAuthenticatedUserID(c)
	if !authenticated {
		return
	}

	filterID := strings.TrimSpace(c.Param("filterId"))
	if filterID == "" {
		api_response.Error(c, http.StatusBadRequest, "filterId is required")
		return
	}

	var filter models.DashboardSavedFilter
	if err := conn.Where("id = ?", filterID).First(&filter).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Filter not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to load filter")
		return
	}

	// المالك يحذف فلتره؛ الفلاتر المشتركة تحتاج إذن إدارة اللوحة.
	if filter.UserID != callerID && !dashboardCan(c, models.PermDashboardManage) {
		api_response.Error(c, http.StatusForbidden, "You cannot delete this filter")
		return
	}

	// الفلتر الافتراضي يجب إلغاء تعيينه أولًا حتى لا تفقد اللوحة حالتها الأولية.
	if filter.IsDefault {
		api_response.Error(c, http.StatusConflict,
			"This filter is set as default; unset it before deleting")
		return
	}

	now := time.Now()
	if err := conn.Delete(&models.DashboardSavedFilter{}, "id = ?", filterID).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete filter")
		return
	}

	api_response.Success(c, gin.H{"success": true, "deletedAt": now})
}
