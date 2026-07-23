package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/cache"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// =============================================================
// Course Status Workflow
// =============================================================

// SubmitForReview transitions a course from DRAFT to UNDER_REVIEW
func SubmitForReview(c *gin.Context) {
	subjectID := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	// Validate transition
	if !subject.Status.CanTransitionTo(models.CourseStatusUnderReview) {
		api_response.Error(c, http.StatusBadRequest, "Course cannot be submitted for review from current status: "+string(subject.Status))
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":                  models.CourseStatusUnderReview,
		"submitted_for_review_at": now,
		"updated_at":              now,
	}

	if err := db.DB.Model(&subject).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to submit for review")
		return
	}

	// Create review submission record
	submission := models.CourseReviewSubmission{
		ID:          uuid.New().String(),
		SubjectID:   subjectID,
		SubmittedBy: uid,
		Status:      "PENDING",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	db.DB.Create(&submission)

	// Create changelog entry
	recordChangeLog(c, subjectID, "UNDER_REVIEW", uid,
		map[string]interface{}{"from": string(subject.Status), "to": string(models.CourseStatusUnderReview)},
		"Submitted for review")

	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subjectID)
	LogAudit(c, "STATUS_CHANGE", "course", subjectID, gin.H{"action": "submit_for_review"})
	api_response.Success(c, gin.H{
		"message": "Course submitted for review successfully",
		"status":  models.CourseStatusUnderReview,
	})
}

// ApproveCourse transitions a course from UNDER_REVIEW to PUBLISHED
func ApproveCourse(c *gin.Context) {
	subjectID := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	if !subject.Status.CanTransitionTo(models.CourseStatusPublished) {
		api_response.Error(c, http.StatusBadRequest, "Course cannot be approved from current status: "+string(subject.Status))
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	c.ShouldBindJSON(&req)

	now := time.Now()
	updates := map[string]interface{}{
		"status":       models.CourseStatusPublished,
		"is_published": true,
		"reviewed_at":  now,
		"reviewed_by":  uid,
		"published_at": now,
		"updated_at":   now,
	}

	if err := db.DB.Model(&subject).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to approve course")
		return
	}

	// Update review submission
	db.DB.Model(&models.CourseReviewSubmission{}).
		Where("subject_id = ? AND status = ?", subjectID, "PENDING").
		Updates(map[string]interface{}{
			"status":         "APPROVED",
			"reviewer_id":    uid,
			"reviewer_notes": req.Notes,
			"reviewed_at":    now,
			"updated_at":     now,
		})

	recordChangeLog(c, subjectID, "PUBLISH", uid,
		map[string]interface{}{"from": string(subject.Status), "to": string(models.CourseStatusPublished)},
		"Approved and published")

	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subjectID)
	LogAudit(c, "STATUS_CHANGE", "course", subjectID, gin.H{"action": "approve"})
	api_response.Success(c, gin.H{
		"message": "Course approved and published successfully",
		"status":  models.CourseStatusPublished,
	})
}

// RejectCourse transitions a course from UNDER_REVIEW to REJECTED
func RejectCourse(c *gin.Context) {
	subjectID := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	if !subject.Status.CanTransitionTo(models.CourseStatusRejected) {
		api_response.Error(c, http.StatusBadRequest, "Course cannot be rejected from current status: "+string(subject.Status))
		return
	}

	var req struct {
		Reason           string                   `json:"reason" binding:"required"`
		RejectionReasons []models.RejectionReason `json:"rejectionReasons"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Rejection reason is required")
		return
	}

	now := time.Now()
	reasonsJSON, _ := json.Marshal(req.RejectionReasons)

	updates := map[string]interface{}{
		"status":           models.CourseStatusRejected,
		"reviewed_at":      now,
		"reviewed_by":      uid,
		"rejection_reason": req.Reason,
		"updated_at":       now,
	}

	if err := db.DB.Model(&subject).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reject course")
		return
	}

	// Update review submission
	db.DB.Model(&models.CourseReviewSubmission{}).
		Where("subject_id = ? AND status = ?", subjectID, "PENDING").
		Updates(map[string]interface{}{
			"status":            "REJECTED",
			"reviewer_id":       uid,
			"reviewer_notes":    req.Reason,
			"rejection_reasons": reasonsJSON,
			"reviewed_at":       now,
			"updated_at":        now,
		})

	recordChangeLog(c, subjectID, "REJECT", uid,
		map[string]interface{}{"from": string(subject.Status), "to": string(models.CourseStatusRejected), "reason": req.Reason},
		"Rejected: "+req.Reason)

	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subjectID)
	LogAudit(c, "STATUS_CHANGE", "course", subjectID, gin.H{"action": "reject", "reason": req.Reason})
	api_response.Success(c, gin.H{
		"message": "Course rejected",
		"status":  models.CourseStatusRejected,
	})
}

// ArchiveCourse transitions a course to ARCHIVED
func ArchiveCourse(c *gin.Context) {
	subjectID := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	if !subject.Status.CanTransitionTo(models.CourseStatusArchived) {
		api_response.Error(c, http.StatusBadRequest, "Course cannot be archived from current status: "+string(subject.Status))
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       models.CourseStatusArchived,
		"is_published": false,
		"archived_at":  now,
		"archived_by":  uid,
		"updated_at":   now,
	}

	if err := db.DB.Model(&subject).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to archive course")
		return
	}

	recordChangeLog(c, subjectID, "ARCHIVE", uid,
		map[string]interface{}{"from": string(subject.Status), "to": string(models.CourseStatusArchived)},
		"Archived")

	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subjectID)
	LogAudit(c, "STATUS_CHANGE", "course", subjectID, gin.H{"action": "archive"})
	api_response.Success(c, gin.H{
		"message": "Course archived successfully",
		"status":  models.CourseStatusArchived,
	})
}

// UnarchiveCourse restores a course from ARCHIVED to DRAFT
func UnarchiveCourse(c *gin.Context) {
	subjectID := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	if !subject.Status.CanTransitionTo(models.CourseStatusDraft) {
		api_response.Error(c, http.StatusBadRequest, "Course cannot be restored from archived status")
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       models.CourseStatusDraft,
		"is_published": false,
		"archived_at":  nil,
		"archived_by":  nil,
		"updated_at":   now,
	}

	if err := db.DB.Model(&subject).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to restore course")
		return
	}

	recordChangeLog(c, subjectID, "RESTORE", uid,
		map[string]interface{}{"from": string(subject.Status), "to": string(models.CourseStatusDraft)},
		"Restored from archive")

	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subjectID)
	LogAudit(c, "STATUS_CHANGE", "course", subjectID, gin.H{"action": "unarchive"})
	api_response.Success(c, gin.H{
		"message": "Course restored successfully",
		"status":  models.CourseStatusDraft,
	})
}

// GetCoursesPendingReview returns courses awaiting admin review
func GetCoursesPendingReview(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.Subject{}).
		Where("status = ?", models.CourseStatusUnderReview)

	var total int64
	query.Count(&total)

	var courses []models.Subject
	if err := query.Order("submitted_for_review_at ASC").
		Limit(limit).Offset(offset).
		Find(&courses).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	api_response.Success(c, gin.H{
		"courses": courses,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// BulkStatusChange changes status for multiple courses
func BulkStatusChange(c *gin.Context) {
	var req struct {
		IDs    []string `json:"ids" binding:"required"`
		Action string   `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "IDs and action are required")
		return
	}

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	var targetStatus models.CourseStatus
	switch req.Action {
	case "submit_review":
		targetStatus = models.CourseStatusUnderReview
	case "publish":
		targetStatus = models.CourseStatusPublished
	case "archive":
		targetStatus = models.CourseStatusArchived
	case "unarchive":
		targetStatus = models.CourseStatusDraft
	case "reject":
		targetStatus = models.CourseStatusRejected
	default:
		api_response.Error(c, http.StatusBadRequest, "Invalid action: "+req.Action)
		return
	}

	now := time.Now()
	updated := 0
	for _, id := range req.IDs {
		var subject models.Subject
		if err := db.DB.First(&subject, "id = ?", id).Error; err != nil {
			continue
		}

		if !subject.Status.CanTransitionTo(targetStatus) {
			continue
		}

		updates := map[string]interface{}{
			"status":     targetStatus,
			"updated_at": now,
		}
		if targetStatus == models.CourseStatusPublished {
			updates["is_published"] = true
			updates["published_at"] = now
		}
		if targetStatus == models.CourseStatusArchived {
			updates["is_published"] = false
			updates["archived_at"] = now
			updates["archived_by"] = uid
		}

		db.DB.Model(&subject).Updates(updates)
		cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), id)
		recordChangeLog(c, id, string(targetStatus), uid, map[string]interface{}{"action": req.Action}, "")
		updated++
	}

	LogAudit(c, "BULK_STATUS_CHANGE", "course", "", gin.H{"action": req.Action, "count": updated})
	api_response.Success(c, gin.H{
		"message": "Status changed for " + strconv.Itoa(updated) + " courses",
		"updated": updated,
	})
}

// =============================================================
// Course Assistants Management
// =============================================================

// AddCourseAssistant adds a co-instructor or assistant to a course
func AddCourseAssistant(c *gin.Context) {
	subjectID := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	var req struct {
		UserID      string                            `json:"userId" binding:"required"`
		Role        models.CourseAssistantRole        `json:"role"`
		Permissions models.CourseAssistantPermissions `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	if req.Role == "" {
		req.Role = models.RoleAssistant
	}

	// Check if user exists
	var user models.User
	if err := db.DB.First(&user, "id = ?", req.UserID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "User not found")
		return
	}

	// Check if already an assistant
	var existing models.CourseAssistant
	if db.DB.Where("subject_id = ? AND user_id = ?", subjectID, req.UserID).First(&existing).Error == nil {
		api_response.Error(c, http.StatusConflict, "User is already an assistant for this course")
		return
	}

	permissionsJSON, _ := json.Marshal(req.Permissions)
	assistant := models.CourseAssistant{
		ID:          uuid.New().String(),
		SubjectID:   subjectID,
		UserID:      req.UserID,
		Role:        req.Role,
		Permissions: req.Permissions,
		InvitedBy:   &uid,
		Status:      "ACTIVE",
		InvitedAt:   time.Now(),
		AcceptedAt:  &time.Time{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Set defaults if empty
	if permissionsJSON != nil && string(permissionsJSON) == "{}" {
		assistant.Permissions = models.CourseAssistantPermissions{
			EditContent:    false,
			ManageStudents: false,
			ViewAnalytics:  true,
			ManageQuizzes:  false,
		}
	}

	if err := db.DB.Create(&assistant).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add assistant")
		return
	}

	LogAudit(c, "CREATE", "course_assistant", assistant.ID, gin.H{"course": subjectID, "user": req.UserID})
	api_response.Created(c, gin.H{"assistant": assistant})
}

// RemoveCourseAssistant removes an assistant from a course
func RemoveCourseAssistant(c *gin.Context) {
	assistantID := c.Param("assistantId")
	subjectID := c.Param("id")

	result := db.DB.Model(&models.CourseAssistant{}).
		Where("id = ? AND subject_id = ?", assistantID, subjectID).
		Updates(map[string]interface{}{
			"status":     "REVOKED",
			"revoked_at": time.Now(),
			"updated_at": time.Now(),
		})

	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Assistant not found")
		return
	}

	LogAudit(c, "DELETE", "course_assistant", assistantID, nil)
	api_response.Success(c, gin.H{"message": "Assistant removed successfully"})
}

// GetCourseAssistants returns all assistants for a course
func GetCourseAssistants(c *gin.Context) {
	subjectID := c.Param("id")

	var assistants []models.CourseAssistant
	if err := db.DB.Preload("User").
		Where("subject_id = ? AND status = ?", subjectID, "ACTIVE").
		Find(&assistants).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch assistants")
		return
	}

	api_response.Success(c, gin.H{"assistants": assistants})
}

// =============================================================
// Course Tag Management
// =============================================================

// GetCourseTags lists all tags available for course categorization.
func GetCourseTags(c *gin.Context) {
	var tags []models.CourseTag
	if err := db.DB.Order("name ASC").Find(&tags).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course tags")
		return
	}
	api_response.Success(c, gin.H{"tags": tags})
}

// CreateCourseTag creates a new course tag.
func CreateCourseTag(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Slug *string `json:"slug"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	tag := models.CourseTag{
		ID:   uuid.New().String(),
		Name: strings.TrimSpace(req.Name),
		Slug: normalizeCourseTagSlug(req.Name, req.Slug),
	}
	if tag.Name == "" {
		api_response.Error(c, http.StatusBadRequest, "Tag name is required")
		return
	}
	if err := db.DB.Create(&tag).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create course tag")
		return
	}
	LogAudit(c, "CREATE", "course_tag", tag.ID, tag)
	api_response.Created(c, gin.H{"tag": tag})
}

// UpdateCourseTag updates an existing course tag.
func UpdateCourseTag(c *gin.Context) {
	tagID := c.Param("id")
	var req struct {
		Name string  `json:"name"`
		Slug *string `json:"slug"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(req.Name) != "" {
		updates["name"] = strings.TrimSpace(req.Name)
	}
	if req.Slug != nil {
		updates["slug"] = normalizeCourseTagSlug(req.Name, req.Slug)
	} else if req.Name != "" {
		updates["slug"] = normalizeCourseTagSlug(req.Name, nil)
	}
	if len(updates) == 0 {
		api_response.Error(c, http.StatusBadRequest, "Nothing to update")
		return
	}

	if err := db.DB.Model(&models.CourseTag{}).Where("id = ?", tagID).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update course tag")
		return
	}
	LogAudit(c, "UPDATE", "course_tag", tagID, updates)
	api_response.Success(c, gin.H{"message": "Tag updated"})
}

// DeleteCourseTag removes a course tag.
func DeleteCourseTag(c *gin.Context) {
	tagID := c.Param("id")
	if err := db.DB.Where("id = ?", tagID).Delete(&models.CourseTag{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete course tag")
		return
	}
	LogAudit(c, "DELETE", "course_tag", tagID, nil)
	api_response.Success(c, gin.H{"message": "Tag deleted"})
}

// AssignTagsToCourse replaces tag associations for a course.
func AssignTagsToCourse(c *gin.Context) {
	subjectID := c.Param("id")
	var req struct {
		TagIDs []string `json:"tagIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("subject_id = ?", subjectID).Delete(&models.SubjectTag{}).Error; err != nil {
			return err
		}
		for _, tagID := range req.TagIDs {
			if tagID == "" {
				continue
			}
			if err := tx.Create(&models.SubjectTag{SubjectID: subjectID, TagID: tagID}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to assign tags")
		return
	}
	LogAudit(c, "ASSIGN_TAGS", "course", subjectID, req)
	api_response.Success(c, gin.H{"message": "Tags assigned"})
}

// =============================================================
// Related Course Management
// =============================================================

// GetRelatedCourses lists related or prerequisite courses.
func GetRelatedCourses(c *gin.Context) {
	subjectID := c.Param("id")
	var related []models.RelatedCourse
	if err := db.DB.Where("course_id = ?", subjectID).Order("relation_type ASC").Find(&related).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch related courses")
		return
	}
	api_response.Success(c, gin.H{"relatedCourses": related})
}

// AddRelatedCourse adds a related or prerequisite course link.
func AddRelatedCourse(c *gin.Context) {
	subjectID := c.Param("id")
	var req struct {
		RelatedCourseID string `json:"relatedCourseId" binding:"required"`
		RelationType    string `json:"relationType"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}
	if req.RelatedCourseID == subjectID {
		api_response.Error(c, http.StatusBadRequest, "A course cannot link to itself")
		return
	}

	entry := models.RelatedCourse{
		CourseID:        subjectID,
		RelatedCourseID: req.RelatedCourseID,
		RelationType:   normalizeRelationType(req.RelationType),
	}
	if err := db.DB.Create(&entry).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add related course")
		return
	}
	LogAudit(c, "CREATE", "related_course", entry.RelatedCourseID, entry)
	api_response.Created(c, gin.H{"relation": entry})
}

// RemoveRelatedCourse removes a related course link.
func RemoveRelatedCourse(c *gin.Context) {
	subjectID := c.Param("id")
	relatedID := c.Param("relatedId")
	if err := db.DB.Where("course_id = ? AND related_course_id = ?", subjectID, relatedID).Delete(&models.RelatedCourse{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to remove related course")
		return
	}
	LogAudit(c, "DELETE", "related_course", relatedID, gin.H{"courseId": subjectID})
	api_response.Success(c, gin.H{"message": "Related course removed"})
}

// =============================================================
// Review Comments
// =============================================================

// GetReviewComments lists review comments for a course.
func GetReviewComments(c *gin.Context) {
	subjectID := c.Param("id")
	var comments []models.CourseReviewComment
	if err := db.DB.Where("subject_id = ?", subjectID).Order("created_at DESC").Find(&comments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch review comments")
		return
	}
	api_response.Success(c, gin.H{"comments": comments})
}

// AddReviewComment adds a review comment.
func AddReviewComment(c *gin.Context) {
	subjectID := c.Param("id")
	userID, _ := c.Get("userId")
	uid, _ := userID.(string)
	var req struct {
		Comment string `json:"comment" binding:"required"`
		Status  string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}
	comment := models.CourseReviewComment{
		ID:         uuid.New().String(),
		SubjectID:  subjectID,
		ReviewerID: uid,
		Comment:    strings.TrimSpace(req.Comment),
		Status:     normalizeReviewCommentStatus(req.Status),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := db.DB.Create(&comment).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create review comment")
		return
	}
	LogAudit(c, "CREATE", "review_comment", comment.ID, comment)
	api_response.Created(c, gin.H{"comment": comment})
}

// UpdateReviewComment updates an existing review comment.
func UpdateReviewComment(c *gin.Context) {
	commentID := c.Param("commentId")
	var req struct {
		Comment string `json:"comment"`
		Status  string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}
	updates := map[string]interface{}{}
	if strings.TrimSpace(req.Comment) != "" {
		updates["comment"] = strings.TrimSpace(req.Comment)
	}
	if req.Status != "" {
		updates["status"] = normalizeReviewCommentStatus(req.Status)
	}
	if len(updates) == 0 {
		api_response.Error(c, http.StatusBadRequest, "Nothing to update")
		return
	}
	if err := db.DB.Model(&models.CourseReviewComment{}).Where("id = ?", commentID).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update review comment")
		return
	}
	LogAudit(c, "UPDATE", "review_comment", commentID, updates)
	api_response.Success(c, gin.H{"message": "Comment updated"})
}

// =============================================================
// Course Changelog
// =============================================================

// GetCourseChangelog returns the change history for a course
func GetCourseChangelog(c *gin.Context) {
	subjectID := c.Param("id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	var total int64
	db.DB.Model(&models.CourseChangelog{}).
		Where("subject_id = ?", subjectID).
		Count(&total)

	var changes []models.CourseChangelog
	if err := db.DB.Where("subject_id = ?", subjectID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&changes).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch changelog")
		return
	}

	api_response.Success(c, gin.H{
		"changelog": changes,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// =============================================================
// Advanced Lesson Management
// =============================================================

// GetLessonDripSchedule returns the drip schedule for a lesson
func GetLessonDripSchedule(c *gin.Context) {
	subTopicID := c.Param("lessonId")

	var schedule models.LessonDripSchedule
	if err := db.DB.Where("sub_topic_id = ?", subTopicID).First(&schedule).Error; err != nil {
		api_response.Success(c, gin.H{"schedule": nil})
		return
	}

	api_response.Success(c, gin.H{"schedule": schedule})
}

// SetLessonDripSchedule sets or updates the drip schedule
func SetLessonDripSchedule(c *gin.Context) {
	subTopicID := c.Param("lessonId")

	var subTopic models.SubTopic
	if err := db.DB.First(&subTopic, "id = ?", subTopicID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Lesson not found")
		return
	}

	var req struct {
		DripType            string  `json:"dripType"`
		ReleaseDate         *string `json:"releaseDate,omitempty"`
		DaysAfterEnrollment *int    `json:"daysAfterEnrollment,omitempty"`
		IsActive            bool    `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	if req.DripType == "" {
		req.DripType = "ABSOLUTE"
	}

	var releaseDate *time.Time
	if req.ReleaseDate != nil {
		t, err := time.Parse(time.RFC3339, *req.ReleaseDate)
		if err == nil {
			releaseDate = &t
		}
	}

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	// Check if schedule exists
	var existing models.LessonDripSchedule
	exists := db.DB.Where("sub_topic_id = ?", subTopicID).First(&existing).Error == nil

	if exists {
		db.DB.Model(&existing).Updates(map[string]interface{}{
			"drip_type":             req.DripType,
			"release_date":          releaseDate,
			"days_after_enrollment": req.DaysAfterEnrollment,
			"is_active":             req.IsActive,
			"updated_at":            time.Now(),
		})
		existing = models.LessonDripSchedule{}
		db.DB.Where("sub_topic_id = ?", subTopicID).First(&existing)
	} else {
		existing = models.LessonDripSchedule{
			ID:                  uuid.New().String(),
			SubTopicID:          subTopicID,
			DripType:            models.DripScheduleType(req.DripType),
			ReleaseDate:         releaseDate,
			DaysAfterEnrollment: req.DaysAfterEnrollment,
			IsActive:            req.IsActive,
			CreatedBy:           &uid,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}
		db.DB.Create(&existing)

		// Enable drip on subtopic
		db.DB.Model(&subTopic).Update("is_drip_enabled", true)
	}

	LogAudit(c, "UPDATE", "lesson_drip", subTopicID, req)
	api_response.Success(c, gin.H{"schedule": existing})
}

// GetLessonSubtitles returns all subtitle tracks for a lesson
func GetLessonSubtitles(c *gin.Context) {
	subTopicID := c.Param("lessonId")

	var subtitles []models.LessonSubtitle
	if err := db.DB.Where("sub_topic_id = ?", subTopicID).
		Order("is_default DESC, language ASC").
		Find(&subtitles).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch subtitles")
		return
	}

	api_response.Success(c, gin.H{"subtitles": subtitles})
}

// AddLessonSubtitle adds a subtitle track to a lesson
func AddLessonSubtitle(c *gin.Context) {
	subTopicID := c.Param("lessonId")

	var subTopic models.SubTopic
	if err := db.DB.First(&subTopic, "id = ?", subTopicID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Lesson not found")
		return
	}

	var req struct {
		Language             string  `json:"language" binding:"required"`
		LanguageName         *string `json:"languageName"`
		SubtitleUrl          string  `json:"subtitleUrl" binding:"required"`
		SubtitleFormat       string  `json:"subtitleFormat"`
		IsDefault            bool    `json:"isDefault"`
		IsForHearingImpaired bool    `json:"isForHearingImpaired"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	if req.SubtitleFormat == "" {
		req.SubtitleFormat = "vtt"
	}

	// If this is default, unset other defaults
	if req.IsDefault {
		db.DB.Model(&models.LessonSubtitle{}).
			Where("sub_topic_id = ?", subTopicID).
			Update("is_default", false)
	}

	subtitle := models.LessonSubtitle{
		ID:                   uuid.New().String(),
		SubTopicID:           subTopicID,
		Language:             req.Language,
		LanguageName:         req.LanguageName,
		SubtitleUrl:          req.SubtitleUrl,
		SubtitleFormat:       models.SubtitleFormat(req.SubtitleFormat),
		IsDefault:            req.IsDefault,
		IsForHearingImpaired: req.IsForHearingImpaired,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if err := db.DB.Create(&subtitle).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add subtitle: "+err.Error())
		return
	}

	LogAudit(c, "CREATE", "lesson_subtitle", subtitle.ID, nil)
	api_response.Created(c, gin.H{"subtitle": subtitle})
}

// DeleteLessonSubtitle removes a subtitle track
func DeleteLessonSubtitle(c *gin.Context) {
	subtitleID := c.Param("subtitleId")

	result := db.DB.Where("id = ?", subtitleID).Delete(&models.LessonSubtitle{})
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Subtitle not found")
		return
	}

	LogAudit(c, "DELETE", "lesson_subtitle", subtitleID, nil)
	api_response.Success(c, gin.H{"message": "Subtitle deleted"})
}

// GetVideoChapters returns chapters for a lesson
func GetVideoChapters(c *gin.Context) {
	subTopicID := c.Param("lessonId")

	var chapters []models.VideoChapter
	if err := db.DB.Where("sub_topic_id = ? AND is_active = ?", subTopicID, true).
		Order("sort_order ASC").
		Find(&chapters).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch chapters")
		return
	}

	api_response.Success(c, gin.H{"chapters": chapters})
}

// SetVideoChapters sets or replaces chapters for a lesson
func SetVideoChapters(c *gin.Context) {
	subTopicID := c.Param("lessonId")

	var req []struct {
		Title       string  `json:"title" binding:"required"`
		TitleAr     *string `json:"titleAr"`
		TimeSeconds int     `json:"timeSeconds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Delete existing chapters
		if err := tx.Where("sub_topic_id = ?", subTopicID).
			Delete(&models.VideoChapter{}).Error; err != nil {
			return err
		}

		// Create new chapters
		for i, ch := range req {
			chapter := models.VideoChapter{
				ID:          uuid.New().String(),
				SubTopicID:  subTopicID,
				Title:       ch.Title,
				TitleAr:     ch.TitleAr,
				TimeSeconds: ch.TimeSeconds,
				SortOrder:   i,
				IsActive:    true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			if err := tx.Create(&chapter).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save chapters")
		return
	}

	LogAudit(c, "UPDATE", "video_chapters", subTopicID, nil)
	api_response.Success(c, gin.H{"message": "Chapters updated"})
}

// GetLessonViewStats returns view statistics for a lesson
func GetLessonViewStats(c *gin.Context) {
	subTopicID := c.Param("lessonId")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	// Aggregate stats
	var stats struct {
		TotalViews       int
		TotalCompletions int
		AvgWatchTime     float64
		CompletionRate   float64
	}
	db.DB.Model(&models.LessonViewStat{}).
		Where("sub_topic_id = ?", subTopicID).
		Select("COUNT(*) as total_views, SUM(CASE WHEN completed THEN 1 ELSE 0 END) as total_completions, AVG(watch_time_seconds) as avg_watch_time").
		Scan(&stats)

	var total int64
	db.DB.Model(&models.LessonViewStat{}).
		Where("sub_topic_id = ?", subTopicID).
		Count(&total)

	var views []models.LessonViewStat
	db.DB.Preload("User").
		Where("sub_topic_id = ?", subTopicID).
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&views)

	api_response.Success(c, gin.H{
		"aggregate": gin.H{
			"totalViews":       stats.TotalViews,
			"totalCompletions": stats.TotalCompletions,
			"avgWatchTime":     stats.AvgWatchTime,
			"completionRate":   stats.CompletionRate,
		},
		"views": views,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// TrackLessonView records a lesson view (student endpoint)
func TrackLessonView(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	subTopicID := c.Param("lessonId")

	var req struct {
		WatchTimeSeconds    int    `json:"watchTimeSeconds"`
		LastPositionSeconds int    `json:"lastPositionSeconds"`
		Completed           bool   `json:"completed"`
		DeviceType          string `json:"deviceType"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Silent fail for tracking
		api_response.Success(c, gin.H{"tracked": false})
		return
	}

	var stat models.LessonViewStat
	exists := db.DB.Where("sub_topic_id = ? AND user_id = ?", subTopicID, userID).First(&stat).Error == nil

	now := time.Now()
	if exists {
		updates := map[string]interface{}{
			"watch_time_seconds":    req.WatchTimeSeconds,
			"last_position_seconds": req.LastPositionSeconds,
			"updated_at":            now,
			"attempts":              stat.Attempts + 1,
		}
		if req.LastPositionSeconds > stat.MaxPositionSeconds {
			updates["max_position_seconds"] = req.LastPositionSeconds
		}
		if req.Completed && !stat.Completed {
			updates["completed"] = true
			updates["completed_at"] = now
		}
		db.DB.Model(&stat).Updates(updates)
	} else {
		stat = models.LessonViewStat{
			ID:                  uuid.New().String(),
			SubTopicID:          subTopicID,
			UserID:              userID,
			WatchTimeSeconds:    req.WatchTimeSeconds,
			LastPositionSeconds: req.LastPositionSeconds,
			MaxPositionSeconds:  req.LastPositionSeconds,
			Completed:           req.Completed,
			CompletedAt:         func() *time.Time { t := now; return &t }(),
			DeviceType:          &req.DeviceType,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if req.Completed {
			stat.CompletedAt = &now
		}
		db.DB.Create(&stat)
	}

	// Update denormalized stats on SubTopic
	db.DB.Model(&models.SubTopic{}).
		Where("id = ?", subTopicID).
		Updates(map[string]interface{}{
			"view_count":             gorm.Expr("view_count + 1"),
			"completion_count":       gorm.Expr("completion_count + ?", boolToInt(req.Completed && !exists)),
			"avg_watch_time_seconds": req.WatchTimeSeconds,
		})

	api_response.Success(c, gin.H{"tracked": true})
}

// =============================================================
// Availability Windows
// =============================================================

// GetAvailabilityWindows returns scheduled windows for a course
func GetAvailabilityWindows(c *gin.Context) {
	subjectID := c.Param("id")

	var windows []models.CourseAvailabilityWindow
	if err := db.DB.Where("subject_id = ? AND is_active = ?", subjectID, true).
		Order("starts_at ASC").
		Find(&windows).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch windows")
		return
	}

	api_response.Success(c, gin.H{"windows": windows})
}

// CreateAvailabilityWindow creates a new availability window
func CreateAvailabilityWindow(c *gin.Context) {
	subjectID := c.Param("id")

	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	var req struct {
		WindowType    string  `json:"windowType"`
		StartsAt      string  `json:"startsAt" binding:"required"`
		EndsAt        *string `json:"endsAt"`
		IsRepeating   bool    `json:"isRepeating"`
		RepeatPattern *string `json:"repeatPattern"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	if req.WindowType == "" {
		req.WindowType = "ENROLLMENT"
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid startsAt format")
		return
	}

	var endsAt *time.Time
	if req.EndsAt != nil {
		t, err := time.Parse(time.RFC3339, *req.EndsAt)
		if err == nil {
			endsAt = &t
		}
	}

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	window := models.CourseAvailabilityWindow{
		ID:            uuid.New().String(),
		SubjectID:     subjectID,
		WindowType:    models.AvailabilityWindowType(req.WindowType),
		StartsAt:      startsAt,
		EndsAt:        endsAt,
		IsRepeating:   req.IsRepeating,
		RepeatPattern: req.RepeatPattern,
		IsActive:      true,
		CreatedBy:     &uid,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := db.DB.Create(&window).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create window")
		return
	}

	LogAudit(c, "CREATE", "availability_window", window.ID, window)
	api_response.Created(c, gin.H{"window": window})
}

// DeleteAvailabilityWindow removes an availability window
func DeleteAvailabilityWindow(c *gin.Context) {
	windowID := c.Param("windowId")

	result := db.DB.Where("id = ?", windowID).Delete(&models.CourseAvailabilityWindow{})
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Window not found")
		return
	}

	LogAudit(c, "DELETE", "availability_window", windowID, nil)
	api_response.Success(c, gin.H{"message": "Window deleted"})
}

// =============================================================
// Helper functions
// =============================================================

func recordChangeLog(c *gin.Context, subjectID, changeType, userID string, changes map[string]interface{}, summary string) {
	changesJSON, _ := json.Marshal(changes)
	cl := models.SubjectChangelog{
		ID:         uuid.New().String(),
		SubjectID:  subjectID,
		Version:    time.Now().Format("2006-01-02"),
		ChangeType: changeType,
		Changes:    changesJSON,
		ChangedBy:  &userID,
		IPAddress:  func() *string { ip := c.ClientIP(); return &ip }(),
		UserAgent:  func() *string { ua := c.GetHeader("User-Agent"); return &ua }(),
		CreatedAt:  time.Now(),
	}
	if summary != "" {
		cl.Version = summary
	}
	db.DB.Create(&cl)
}

func normalizeCourseTagSlug(name string, explicit *string) string {
	if explicit != nil && strings.TrimSpace(*explicit) != "" {
		return strings.TrimSpace(*explicit)
	}
	return buildSlug(name, nil)
}

func normalizeReviewCommentStatus(status string) string {
	s := strings.TrimSpace(strings.ToUpper(status))
	switch s {
	case "APPROVED", "REJECTED", "PENDING":
		return s
	default:
		return "PENDING"
	}
}

func normalizeRelationType(relationType string) string {
	s := strings.TrimSpace(strings.ToLower(relationType))
	if s == "prerequisite" {
		return "prerequisite"
	}
	return "related"
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
