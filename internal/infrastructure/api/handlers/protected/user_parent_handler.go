package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
)

func GetParentStudents(c *gin.Context) {
	parentID := c.Param("id")
	if parentID == "" {
		api_response.Error(c, http.StatusBadRequest, "Parent ID is required")
		return
	}

	type studentInfo struct {
		ID           string
		Name         string
		Email        string
		GradeLevel   string
		Level        int
		Progress     float64
		Attendance   float64
		CurrentGPA   float64
		LastActivity *time.Time
	}

	var students []studentInfo
	err := db.DB.Raw(`
		SELECT 
			u.id,
			u.name,
			u.email,
			u.grade_level,
			u.level,
			COALESCE(u.progress, 0) as progress,
			COALESCE(u.attendance, 0) as attendance,
			COALESCE(u.current_gpa, 0) as current_gpa,
			u.last_login as last_activity
		FROM users u
		INNER JOIN student_parents sp ON u.id = sp.student_id
		WHERE sp.parent_id = ? AND u.deleted_at IS NULL
		ORDER BY u.name ASC
	`, parentID).Scan(&students).Error

	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch parent students")
		return
	}

	api_response.Success(c, gin.H{
		"students": students,
		"total":    len(students),
	})
}

// LinkStudentToParent links a student to a parent
func LinkStudentToParent(c *gin.Context) {
	parentID := c.Param("id")
	if parentID == "" {
		api_response.Error(c, http.StatusBadRequest, "Parent ID is required")
		return
	}

	var req struct {
		StudentID string `json:"studentId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Verify parent exists and has PARENT role
	var parent models.User
	if err := db.DB.Where("id = ? AND role = ? AND deleted_at IS NULL", parentID, models.RoleParent).First(&parent).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Parent not found")
		return
	}

	// Verify student exists
	var student models.User
	if err := db.DB.Where("id = ? AND deleted_at IS NULL", req.StudentID).First(&student).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Student not found")
		return
	}

	// Check if already linked
	var existingLink int64
	db.DB.Table("student_parents").
		Where("parent_id = ? AND student_id = ?", parentID, req.StudentID).
		Count(&existingLink)

	if existingLink > 0 {
		api_response.Error(c, http.StatusConflict, "Student is already linked to this parent")
		return
	}

	// Create the link
	if err := db.DB.Exec(`
		INSERT INTO student_parents (student_id, parent_id, created_at)
		VALUES (?, ?, NOW())
	`, req.StudentID, parentID).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to link student")
		return
	}

	api_response.Success(c, gin.H{
		"message": "Student linked successfully",
	})
}

// UnlinkStudentFromParent unlinks a student from a parent
func UnlinkStudentFromParent(c *gin.Context) {
	parentID := c.Param("id")
	studentID := c.Query("studentId")

	if parentID == "" || studentID == "" {
		api_response.Error(c, http.StatusBadRequest, "Parent ID and Student ID are required")
		return
	}

	result := db.DB.Exec(`
		DELETE FROM student_parents
		WHERE parent_id = ? AND student_id = ?
	`, parentID, studentID)

	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to unlink student")
		return
	}

	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Link not found")
		return
	}

	api_response.Success(c, gin.H{
		"message": "Student unlinked successfully",
	})
}

// GetParentStatistics returns statistics for parents
func GetParentStatistics(c *gin.Context) {
	var stats struct {
		TotalParents        int64
		ActiveParents       int64
		SuspendedParents    int64
		PendingApproval     int64
		OnlineParents       int64
		NewParentsToday     int64
		NewParentsThisMonth int64
	}

	today := time.Now().Truncate(24 * time.Hour)
	monthStart := time.Now().AddDate(0, 0, -time.Now().Day()+1).Truncate(24 * time.Hour)

	db.DB.Model(&models.User{}).
		Where("role = ? AND deleted_at IS NULL", models.RoleParent).
		Count(&stats.TotalParents)

	db.DB.Model(&models.User{}).
		Where("role = ? AND status = ? AND deleted_at IS NULL", models.RoleParent, models.StatusActive).
		Count(&stats.ActiveParents)

	db.DB.Model(&models.User{}).
		Where("role = ? AND status = ? AND deleted_at IS NULL", models.RoleParent, models.StatusSuspended).
		Count(&stats.SuspendedParents)

	db.DB.Model(&models.User{}).
		Where("role = ? AND status = ? AND deleted_at IS NULL", models.RoleParent, "PENDING").
		Count(&stats.PendingApproval)

	// Online parents (last login within 15 minutes)
	fifteenMinutesAgo := time.Now().Add(-15 * time.Minute)
	db.DB.Model(&models.User{}).
		Where("role = ? AND last_login >= ? AND deleted_at IS NULL", models.RoleParent, fifteenMinutesAgo).
		Count(&stats.OnlineParents)

	db.DB.Model(&models.User{}).
		Where("role = ? AND created_at >= ? AND deleted_at IS NULL", models.RoleParent, today).
		Count(&stats.NewParentsToday)

	db.DB.Model(&models.User{}).
		Where("role = ? AND created_at >= ? AND deleted_at IS NULL", models.RoleParent, monthStart).
		Count(&stats.NewParentsThisMonth)

	api_response.Success(c, stats)
}
