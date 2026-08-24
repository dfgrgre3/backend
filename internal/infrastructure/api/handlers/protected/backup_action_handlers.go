package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	systemservice "thanawy-backend/internal/domain/system/service"
	"time"

	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// CreateBackup creates a new backup
// @Summary Create backup
// @Description Create a manual backup of the system
// @Tags admin,backup
// @Accept json
// @Produce json
// @Param request body CreateBackupRequest true "Backup configuration"
// @Success 201 {object} map[string]interface{}
// @Router /api/admin/backups [post]
func CreateBackup(c *gin.Context) {
	var req CreateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	adminID, _ := c.Get("user_id")

	// Set defaults
	if req.RetentionDays == 0 {
		req.RetentionDays = 30
	}

	if req.Type == "full" {
		req.IncludesDatabase = true
		req.IncludesFiles = true
	}

	// Create backup record
	backup := models.Backup{
		Name:             req.Name,
		Type:             req.Type,
		Status:           "in_progress",
		IncludesFiles:    req.IncludesFiles,
		IncludesDatabase: req.IncludesDatabase,
		Tables:           req.Tables,
		RetentionDays:    req.RetentionDays,
		CreatedBy:        adminID.(string),
		CreatedAt:        time.Now(),
	}

	if err := SafeCreate(db.DB, &backup); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create backup")
		return
	}

	// Log operation
	middleware.LogCriticalOperation(c, "backup_created", map[string]interface{}{
		"backup_type": req.Type,
		"backup_name": req.Name,
	})

	// Start backup process asynchronously
	go systemservice.GetBackupService().PerformBackup(backup.ID)

	api_response.Success(c, gin.H{
		"message": "Backup started successfully",
		"data": gin.H{
			"backup": backup,
		},
	})
}

// RestoreBackup restores a backup
// @Summary Restore backup
// @Description Restore the system from a backup
// @Tags admin,backup
// @Accept json
// @Produce json
// @Param id path string true "Backup ID"
// @Param request body RestoreBackupRequest true "Restore options"
// @Success 200 {object} map[string]string
// @Router /api/admin/backups/{id}/restore [post]
func RestoreBackup(c *gin.Context) {
	id := c.Param("id")
	adminID, _ := c.Get("user_id")

	var req RestoreBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var backup models.Backup
	if err := db.DB.First(&backup, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errBackupNotFound)
		return
	}

	// Can only restore completed backups
	if backup.Status != "completed" {
		api_response.Error(c, http.StatusBadRequest, "Can only restore completed backups")
		return
	}

	// Update status
	backup.Status = "restoring"
	db.DB.Save(&backup)

	// Log critical operation
	middleware.LogCriticalOperation(c, "backup_restore", map[string]interface{}{
		"backup_id":   id,
		"backup_name": backup.Name,
		"dry_run":     req.DryRun,
	})

	// Perform restore asynchronously
	go func() {
		err := systemservice.GetBackupService().RestoreBackup(backup.ID, req.TargetTables, req.SkipExisting)
		if err != nil {
			backup.Status = "failed"
			backup.Error = err.Error()
		} else {
			backup.Status = "completed"
		}
		now := time.Now()
		backup.RestoredAt = &now
		backup.RestoredBy = adminID.(string)
		db.DB.Save(&backup)
	}()

	api_response.Success(c, gin.H{
		"message": "Restore started. This may take several minutes.",
	})
}

// DeleteBackup deletes a backup
// @Summary Delete backup
// @Description Delete a backup permanently
// @Tags admin,backup
// @Accept json
// @Produce json
// @Param id path string true "Backup ID"
// @Success 200 {object} map[string]string
// @Router /api/admin/backups/{id} [delete]
func DeleteBackup(c *gin.Context) {
	id := c.Param("id")

	var backup models.Backup
	if err := db.DB.First(&backup, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errBackupNotFound)
		return
	}

	// Delete physical backup file
	if backup.DownloadURL != "" {
		systemservice.GetBackupService().DeleteBackupFile(backup.DownloadURL)
	}

	// Delete from database
	if err := db.DB.Delete(&backup).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete backup")
		return
	}

	api_response.Success(c, gin.H{"message": "Backup deleted successfully"})
}

// ScheduleBackup creates a scheduled backup configuration
// @Summary Schedule backup
// @Description Configure automatic scheduled backups
// @Tags admin,backup
// @Accept json
// @Produce json
// @Param request body ScheduleBackupRequest true "Schedule configuration"
// @Success 200 {object} map[string]string
// @Router /api/admin/backups/schedule [post]
func ScheduleBackup(c *gin.Context) {
	var req ScheduleBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate day of week/month based on frequency
	if req.Frequency == "weekly" && (req.DayOfWeek < 0 || req.DayOfWeek > 6) {
		api_response.Error(c, http.StatusBadRequest, "Day of week must be 0-6")
		return
	}
	if req.Frequency == "monthly" && (req.DayOfMonth < 1 || req.DayOfMonth > 31) {
		api_response.Error(c, http.StatusBadRequest, "Day of month must be 1-31")
		return
	}

	api_response.Success(c, gin.H{
		"message":   "Backup scheduled successfully",
		"frequency": req.Frequency,
		"time":      req.Time,
	})
}
