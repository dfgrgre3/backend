package protected

import (
	"fmt"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	systemservice "thanawy-backend/internal/domain/system/service"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetBackups returns all backups
// @Summary Get backups
// @Description Get all backups with optional filtering
// @Tags admin,backup
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/backups [get]
func GetBackups(c *gin.Context) {
	query := db.DB.Model(&models.Backup{}).Order("created_at DESC")

	var backups []models.Backup
	if err := query.Find(&backups).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch backups")
		return
	}

	api_response.Success(c, gin.H{
		"data": gin.H{
			"backups": backups,
			"count":   len(backups),
		},
	})
}

// DownloadBackup downloads a backup file
// @Summary Download backup
// @Description Download a backup file
// @Tags admin,backup
// @Accept json
// @Produce json
// @Param id path string true "Backup ID"
// @Success 200 {file} application/octet-stream
// @Router /api/admin/backups/{id}/download [get]
func DownloadBackup(c *gin.Context) {
	id := c.Param("id")

	var backup models.Backup
	if err := db.DB.First(&backup, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errBackupNotFound)
		return
	}

	if backup.Status != "completed" {
		api_response.Error(c, http.StatusBadRequest, "Backup not ready for download")
		return
	}

	// Generate signed URL or serve file directly
	filePath, err := systemservice.GetBackupService().GetBackupFilePath(backup.ID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to locate backup file")
		return
	}

	c.FileAttachment(filePath, fmt.Sprintf("backup-%s-%s.sql", backup.Name, backup.CreatedAt.Format("2006-01-02")))
}

// VerifyBackup verifies backup integrity
// @Summary Verify backup
// @Description Verify the integrity of a backup
// @Tags admin,backup
// @Accept json
// @Produce json
// @Param id path string true "Backup ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/backups/{id}/verify [post]
func VerifyBackup(c *gin.Context) {
	id := c.Param("id")

	var backup models.Backup
	if err := db.DB.First(&backup, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errBackupNotFound)
		return
	}

	isValid, err := systemservice.GetBackupService().VerifyBackup(backup.ID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Verification failed")
		return
	}

	api_response.Success(c, gin.H{
		"valid": isValid,
	})
}

// GetBackupStats returns backup statistics
// @Summary Get backup statistics
// @Description Get statistics about backups
// @Tags admin,backup
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/backups/stats [get]
func GetBackupStats(c *gin.Context) {
	var stats struct {
		TotalBackups   int64   `json:"totalBackups"`
		TotalSize      int64   `json:"totalSize"`
		LastBackupAt   *string `json:"lastBackupAt,omitempty"`
		ScheduledCount int     `json:"scheduledBackups"`
	}

	db.DB.Model(&models.Backup{}).Count(&stats.TotalBackups)
	db.DB.Model(&models.Backup{}).Select("COALESCE(SUM(size), 0)").Scan(&stats.TotalSize)

	var lastBackup models.Backup
	if err := db.DB.Where("status = ?", "completed").Order("created_at DESC").First(&lastBackup).Error; err == nil {
		lastAt := lastBackup.CreatedAt.Format(time.RFC3339)
		stats.LastBackupAt = &lastAt
	}

	// Storage usage calculations
	storageUsed := stats.TotalSize
	storageLimit := int64(10 * 1024 * 1024 * 1024) // 10 GB

	api_response.Success(c, gin.H{
		"overview":          stats,
		"storageUsed":       storageUsed,
		"storageLimit":      storageLimit,
		"storagePercentage": float64(storageUsed) / float64(storageLimit) * 100,
	})
}

// GetDatabaseTables returns all database tables
// @Summary Get database tables
// @Description Get a list of all database tables available for backup
// @Tags admin,backup
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/backups/tables [get]
func GetDatabaseTables(c *gin.Context) {
	tables, err := systemservice.GetBackupService().GetDatabaseTables()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch tables")
		return
	}

	api_response.Success(c, gin.H{
		"tables": tables,
	})
}

// GetBackupProgress returns the progress of an in-progress backup
// @Summary Get backup progress
// @Description Get the progress of a running backup operation
// @Tags admin,backup
// @Accept json
// @Produce json
// @Param id path string true "Backup ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/backups/{id}/progress [get]
func GetBackupProgress(c *gin.Context) {
	id := c.Param("id")

	var backup models.Backup
	if err := db.DB.First(&backup, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errBackupNotFound)
		return
	}

	// Get progress from service
	progress := systemservice.GetBackupService().GetProgress(backup.ID)

	var percent int
	var message string
	var eta int

	if progress != nil {
		percent = progress.Percent
		message = progress.Message
		eta = progress.ETA
	} else {
		// Fallback based on DB status
		switch backup.Status {
		case "completed":
			percent = 100
			message = "Completed"
			eta = 0
		case "failed":
			percent = 0
			message = "Failed"
			eta = 0
		default:
			percent = 0
			message = "Backup status: " + backup.Status
			eta = 0
		}
	}

	api_response.Success(c, gin.H{
		"backupId": id,
		"status":   backup.Status,
		"percent":  percent,
		"message":  message,
		"eta":      eta,
	})
}
