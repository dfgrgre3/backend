package admin

import (
	"math"
	"net/http"
	"strconv"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// parseFlexibleDate accepts either a full RFC3339 timestamp or a bare
// YYYY-MM-DD date string.
func parseFlexibleDate(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", value)
}

// ---------------------------------------------------------------------
// Attendance
// ---------------------------------------------------------------------

// AdminListUserAttendance returns a user's attendance history.
func AdminListUserAttendance(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var total, presentCount, absentCount, lateCount int64
	db.DB.Model(&models.Attendance{}).Where("user_id = ?", userID).Count(&total)
	db.DB.Model(&models.Attendance{}).Where("user_id = ? AND status = ?", userID, models.AttendancePresent).Count(&presentCount)
	db.DB.Model(&models.Attendance{}).Where("user_id = ? AND status = ?", userID, models.AttendanceAbsent).Count(&absentCount)
	db.DB.Model(&models.Attendance{}).Where("user_id = ? AND status = ?", userID, models.AttendanceLate).Count(&lateCount)

	var records []models.Attendance
	if err := db.DB.Preload("Subject").
		Where("user_id = ?", userID).
		Order("session_date DESC").
		Limit(limit).
		Find(&records).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch attendance")
		return
	}

	items := make([]gin.H, 0, len(records))
	for _, r := range records {
		subjectName := ""
		if r.Subject != nil {
			subjectName = r.Subject.Name
		}
		items = append(items, gin.H{
			"id":          r.ID,
			"subjectId":   r.SubjectID,
			"subjectName": subjectName,
			"sessionDate": r.SessionDate,
			"status":      r.Status,
			"notes":       r.Notes,
			"createdAt":   r.CreatedAt,
		})
	}

	attendanceRate := 0.0
	if total > 0 {
		attendanceRate = (float64(presentCount) / float64(total)) * 100
	}

	api_response.Success(c, gin.H{
		"userId": userID,
		"items":  items,
		"summary": gin.H{
			"total":          total,
			"presentCount":   presentCount,
			"absentCount":    absentCount,
			"lateCount":      lateCount,
			"attendanceRate": attendanceRate,
		},
	})
}

// AdminRecordAttendance creates a new attendance record for a user.
func AdminRecordAttendance(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	var req struct {
		SubjectID   string `json:"subjectId"`
		SessionDate string `json:"sessionDate" binding:"required"` // RFC3339 or YYYY-MM-DD
		Status      string `json:"status" binding:"required,oneof=present absent late excused"`
		Notes       string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid attendance payload")
		return
	}

	sessionDate, err := parseFlexibleDate(req.SessionDate)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid sessionDate")
		return
	}

	var subjectID *string
	if req.SubjectID != "" {
		subjectID = &req.SubjectID
	}

	record := models.Attendance{
		UserID:      userID,
		SubjectID:   subjectID,
		SessionDate: sessionDate,
		Status:      models.AttendanceStatus(req.Status),
		Notes:       req.Notes,
	}
	if adminID, exists := c.Get("userId"); exists {
		if s, ok := adminID.(string); ok && s != "" {
			record.RecordedBy = &s
		}
	}

	if err := db.DB.Create(&record).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to record attendance")
		return
	}

	api_response.Success(c, gin.H{"message": "Attendance recorded", "id": record.ID})
}

// ---------------------------------------------------------------------
// Student notes
// ---------------------------------------------------------------------

// AdminListStudentNotes returns internal notes attached to a user.
func AdminListStudentNotes(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	db.DB.Model(&models.StudentNote{}).Where("user_id = ?", userID).Count(&total)

	var notes []models.StudentNote
	if err := db.DB.Preload("Author").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&notes).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch notes")
		return
	}

	items := make([]gin.H, 0, len(notes))
	for _, n := range notes {
		authorName := "الإدارة"
		if n.Author != nil && n.Author.Name != nil {
			authorName = *n.Author.Name
		}
		items = append(items, gin.H{
			"id":         n.ID,
			"category":   n.Category,
			"note":       n.Note,
			"authorName": authorName,
			"createdAt":  n.CreatedAt,
		})
	}

	api_response.Success(c, gin.H{
		"userId": userID,
		"items":  items,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

// AdminCreateStudentNote adds a new internal note to a user's profile.
func AdminCreateStudentNote(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	var req struct {
		Category string `json:"category"`
		Note     string `json:"note" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid note payload")
		return
	}

	note := models.StudentNote{
		UserID:   userID,
		Category: req.Category,
		Note:     req.Note,
	}
	if adminID, exists := c.Get("userId"); exists {
		if s, ok := adminID.(string); ok && s != "" {
			note.AuthorID = &s
		}
	}

	if err := db.DB.Create(&note).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create note")
		return
	}

	api_response.Success(c, gin.H{"message": "Note added", "id": note.ID})
}

// AdminDeleteStudentNote removes a note.
func AdminDeleteStudentNote(c *gin.Context) {
	noteID := c.Param("noteId")
	result := db.DB.Where("id = ?", noteID).Delete(&models.StudentNote{})
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete note")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Note not found")
		return
	}
	api_response.Success(c, gin.H{"message": "Note deleted"})
}
