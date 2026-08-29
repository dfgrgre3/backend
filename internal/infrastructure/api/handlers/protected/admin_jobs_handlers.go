package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  Scheduled Tasks
// ─────────────────────────────────────────────

func AdminListScheduledTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.ScheduledTask{})
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var tasks []models.ScheduledTask
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch scheduled tasks")
		return
	}

	items := make([]gin.H, 0, len(tasks))
	var activeCount, pausedCount, failedCount, totalRuns int64
	for _, t := range tasks {
		items = append(items, scheduledTaskToGin(t))
		switch t.Status {
		case "ACTIVE":
			activeCount++
		case "PAUSED":
			pausedCount++
		case "FAILED":
			failedCount++
		}
		totalRuns += int64(t.RunCount)
	}

	api_response.Success(c, gin.H{
		"tasks":      items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalTasks": total, "activeTasks": activeCount, "pausedTasks": pausedCount, "failedTasks": failedCount, "totalRuns": totalRuns},
	})
}

func AdminUpdateScheduledTask(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.ScheduledTask{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update scheduled task")
		return
	}
	api_response.Success(c, gin.H{"message": "Scheduled task updated"})
}

func AdminDeleteScheduledTask(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.ScheduledTask{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete scheduled task")
		return
	}
	api_response.Success(c, gin.H{"message": "Scheduled task deleted"})
}

func scheduledTaskToGin(t models.ScheduledTask) gin.H {
	return gin.H{
		"id":           t.ID,
		"name":         t.Name,
		"description":  t.Description,
		"type":         t.Type,
		"status":       t.Status,
		"schedule":     t.Schedule,
		"lastRunAt":    t.LastRunAt,
		"nextRunAt":    t.NextRunAt,
		"runCount":     t.RunCount,
		"successCount": t.SuccessCount,
		"failureCount": t.FailureCount,
		"createdAt":    t.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  Queue Management
// ─────────────────────────────────────────────

func AdminListQueueJobs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.QueueJob{})
	if search != "" {
		query = query.Where("name ILIKE ? OR type ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var jobs []models.QueueJob
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&jobs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch queue jobs")
		return
	}

	items := make([]gin.H, 0, len(jobs))
	var pendingJobs, processingJobs, failedJobs int64
	var totalProcessingTime int64
	for _, j := range jobs {
		items = append(items, queueJobToGin(j))
		switch j.Status {
		case "PENDING":
			pendingJobs++
		case "PROCESSING":
			processingJobs++
		case "FAILED":
			failedJobs++
		}
	}

	avgProcessingTime := int64(0)
	if len(jobs) > 0 {
		avgProcessingTime = totalProcessingTime / int64(len(jobs))
	}

	api_response.Success(c, gin.H{
		"jobs":       items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalJobs": total, "pendingJobs": pendingJobs, "processingJobs": processingJobs, "failedJobs": failedJobs, "avgProcessingTime": avgProcessingTime},
	})
}

func AdminRetryQueueJob(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Model(&models.QueueJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":   "PENDING",
		"attempts": 0,
		"error":    nil,
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to retry job")
		return
	}
	api_response.Success(c, gin.H{"message": "Job retried"})
}

func AdminDeleteQueueJob(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.QueueJob{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete job")
		return
	}
	api_response.Success(c, gin.H{"message": "Job deleted"})
}

func queueJobToGin(j models.QueueJob) gin.H {
	return gin.H{
		"id":          j.ID,
		"name":        j.Name,
		"type":        j.Type,
		"status":      j.Status,
		"priority":    j.Priority,
		"attempts":    j.Attempts,
		"maxAttempts": j.MaxAttempts,
		"error":       j.Error,
		"processedAt": j.ProcessedAt,
		"createdAt":   j.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  Data Import/Export
// ─────────────────────────────────────────────

func AdminListImportExportJobs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	var jobs []models.ImportExportJob
	if err := db.DB.Order("created_at DESC").Offset(offset).Limit(limit).Find(&jobs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch jobs")
		return
	}

	var total int64
	db.DB.Model(&models.ImportExportJob{}).Count(&total)

	var pending, completed, failed int64
	for _, j := range jobs {
		switch j.Status {
		case "PENDING", "PROCESSING":
			pending++
		case "COMPLETED":
			completed++
		case "FAILED":
			failed++
		}
	}

	api_response.Success(c, gin.H{
		"jobs":       jobs,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalJobs": total, "pendingJobs": pending, "completedJobs": completed, "failedJobs": failed},
	})
}

func AdminExportData(c *gin.Context) {
	entity := c.Param("entity")
	job := models.ImportExportJob{
		Type:      "EXPORT",
		Entity:    entity,
		Status:    "PENDING",
		Progress:  0,
		CreatedAt: time.Now(),
	}
	SafeCreate(db.DB, &job)
	api_response.Success(c, gin.H{"message": "Export started", "jobId": job.ID})
}

func AdminImportData(c *gin.Context) {
	entity := c.Param("entity")
	file, _ := c.FormFile("file")
	if file == nil {
		api_response.Error(c, http.StatusBadRequest, "No file uploaded")
		return
	}

	job := models.ImportExportJob{
		Type:      "IMPORT",
		Entity:    entity,
		Status:    "PENDING",
		Progress:  0,
		CreatedAt: time.Now(),
	}
	SafeCreate(db.DB, &job)
	api_response.Success(c, gin.H{"message": "Import started", "jobId": job.ID})
}
