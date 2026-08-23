package protected

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DuplicateCourse duplicates an existing course (Subject) along with its topics, subtopics, and attachments
func DuplicateCourse(c *gin.Context) {
	var input struct {
		CourseID string `json:"courseId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	var oldSubject models.Subject
	if err := db.DB.Preload("Topics.SubTopics.Attachments").First(&oldSubject, "id = ?", input.CourseID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	// Generate unique Code and Slug
	uniqueSuffix := fmt.Sprintf("-%d", time.Now().Unix()%10000)
	var newCode *string
	if oldSubject.Code != nil {
		codeVal := *oldSubject.Code + uniqueSuffix
		newCode = &codeVal
	}
	var newSlug *string
	if oldSubject.Slug != nil {
		slugVal := *oldSubject.Slug + uniqueSuffix
		newSlug = &slugVal
	}

	nameCopy := oldSubject.Name + " (Copy)"
	var nameArCopy *string
	if oldSubject.NameAr != nil {
		valAr := *oldSubject.NameAr + " (نسخة)"
		nameArCopy = &valAr
	}

	newSubject := models.Subject{
		Name:                   nameCopy,
		NameAr:                 nameArCopy,
		Code:                   newCode,
		Slug:                   newSlug,
		Description:            oldSubject.Description,
		Icon:                   oldSubject.Icon,
		Color:                  oldSubject.Color,
		IsActive:               true,
		IsPublished:            false,
		Price:                  oldSubject.Price,
		Level:                  oldSubject.Level,
		InstructorName:         oldSubject.InstructorName,
		InstructorId:           oldSubject.InstructorId,
		CategoryId:             oldSubject.CategoryId,
		DurationHours:          oldSubject.DurationHours,
		Requirements:           oldSubject.Requirements,
		LearningObjectives:     oldSubject.LearningObjectives,
		ThumbnailUrl:           oldSubject.ThumbnailUrl,
		TrailerUrl:             oldSubject.TrailerUrl,
		TrailerDurationMinutes: oldSubject.TrailerDurationMinutes,
		SeoTitle:               oldSubject.SeoTitle,
		SeoDescription:         oldSubject.SeoDescription,
		Language:               oldSubject.Language,
		Type:                   oldSubject.Type,
		CoursePrerequisites:    oldSubject.CoursePrerequisites,
		TargetAudience:         oldSubject.TargetAudience,
		WhatYouLearn:           oldSubject.WhatYouLearn,
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newSubject).Error; err != nil {
			return err
		}

		for _, topic := range oldSubject.Topics {
			newTopic := models.Topic{
				SubjectID:   newSubject.ID,
				Title:       topic.Title,
				Description: topic.Description,
				Order:       topic.Order,
			}
			if err := tx.Create(&newTopic).Error; err != nil {
				return err
			}

			for _, subTopic := range topic.SubTopics {
				newSubTopic := models.SubTopic{
					TopicID:         newTopic.ID,
					Title:           subTopic.Title,
					Description:     subTopic.Description,
					Content:         subTopic.Content,
					VideoUrl:        subTopic.VideoUrl,
					Type:            subTopic.Type,
					ExamID:          subTopic.ExamID,
					Order:           subTopic.Order,
					DurationMinutes: subTopic.DurationMinutes,
					IsFree:          subTopic.IsFree,
				}
				if err := tx.Create(&newSubTopic).Error; err != nil {
					return err
				}

				for _, att := range subTopic.Attachments {
					newAtt := models.LessonAttachment{
						SubTopicID: newSubTopic.ID,
						Title:      att.Title,
						FileUrl:    att.FileUrl,
						FileType:   att.FileType,
						FileSize:   att.FileSize,
					}
					if err := tx.Create(&newAtt).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	}); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to duplicate course: "+err.Error())
		return
	}

	getSubjectRepo().InvalidateSubjectCache(newSubject.ID)
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), newSubject.ID)

	api_response.Success(c, gin.H{
		"message": "Course duplicated successfully",
		"course":  newSubject,
	})
}

// BatchCourseAction performs batch operations on multiple courses.
// Supported actions: publish, unpublish, activate, deactivate, delete,
// archive, unarchive, assign_teacher, remove_teacher.
func BatchCourseAction(c *gin.Context) {
	var input struct {
		IDs       []string `json:"ids" binding:"required"`
		Action    string   `json:"action" binding:"required"`
		TeacherID string   `json:"teacherId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "IDs and Action are required")
		return
	}

	if len(input.IDs) == 0 {
		api_response.Success(c, gin.H{"message": "No courses selected"})
		return
	}

	var err error
	switch input.Action {
	case "publish":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Updates(map[string]interface{}{
			"is_published": true,
			"status":       models.CourseStatusPublished,
			"published_at": time.Now(),
		}).Error
	case "unpublish":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Updates(map[string]interface{}{
			"is_published": false,
			"status":       models.CourseStatusDraft,
		}).Error
	case "activate":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Update("is_active", true).Error
	case "deactivate":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Update("is_active", false).Error
	case "archive":
		updates := map[string]interface{}{
			"is_published": false,
			"is_active":    false,
			"status":       models.CourseStatusArchived,
			"archived_at":  time.Now(),
		}
		if adminID, exists := c.Get("userId"); exists {
			if adminIDStr, ok := adminID.(string); ok && adminIDStr != "" {
				updates["archived_by"] = adminIDStr
			}
		}
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Updates(updates).Error
	case "unarchive":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Updates(map[string]interface{}{
			"is_active":   true,
			"status":      models.CourseStatusDraft,
			"archived_at": nil,
			"archived_by": nil,
		}).Error
	case "assign_teacher":
		if input.TeacherID == "" {
			api_response.Error(c, http.StatusBadRequest, "Teacher ID is required for assign_teacher")
			return
		}
		var teacher models.User
		if err := db.DB.Where("id = ? AND role = ?", input.TeacherID, models.RoleTeacher).First(&teacher).Error; err != nil {
			api_response.Error(c, http.StatusBadRequest, "Teacher not found")
			return
		}
		teacherName := firstNonEmpty(stringOrEmpty(teacher.Name), stringOrEmpty(teacher.Username), teacher.Email)
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Updates(map[string]interface{}{
			"instructor_id":   teacher.ID,
			"instructor_name": teacherName,
		}).Error
	case "remove_teacher":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Updates(map[string]interface{}{
			"instructor_id":   nil,
			"instructor_name": nil,
		}).Error
	case "delete":
		var count int64
		db.DB.Model(&models.Enrollment{}).Where("subject_id IN ?", input.IDs).Count(&count)
		if count > 0 {
			api_response.Error(c, http.StatusBadRequest, "Cannot delete courses with enrolled students")
			return
		}
		err = db.DB.Where("id IN ?", input.IDs).Delete(&models.Subject{}).Error
	default:
		api_response.Error(c, http.StatusBadRequest, "Invalid action")
		return
	}

	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to execute batch action: "+err.Error())
		return
	}

	invalidator := cache.NewCacheInvalidator()
	for _, id := range input.IDs {
		getSubjectRepo().InvalidateSubjectCache(id)
		invalidator.InvalidateSubject(c.Request.Context(), id)
	}

	api_response.Success(c, gin.H{
		"message": "Batch action executed successfully",
	})
}

// GetPopularCourses returns the most popular published courses for the homepage,
// ordered by enrolled_count descending, then by rating descending.
// Limit and offset can be passed via query params; default limit is 8.
// Requires isPublished=true; also filters to isActive=true.
func GetPopularCourses(c *gin.Context) {
	const SubjectCacheTTL = 2 * time.Hour
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "8"))
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}

	cacheKey := fmt.Sprintf("subject:popular:limit=%d:offset=%d", limit, offset)

	if cache.Redis != nil {
		cached, err := cache.Redis.Get(c.Request.Context(), cacheKey).Result()
		if err == nil {
			var cachedResponse gin.H
			if json.Unmarshal([]byte(cached), &cachedResponse) == nil {
				api_response.Success(c, cachedResponse)
				return
			}
		}
	}

	readDB, aborted := safeReadDB(c)
	if aborted {
		return
	}

	var subjects []models.Subject
	if err := readDB.Model(&models.Subject{}).
		Where("is_published = ? AND is_active = ?", true, true).
		Order("enrolled_count DESC").
		Order("rating DESC").
		Offset(offset).
		Limit(limit).
		Find(&subjects).Error; err != nil {

		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch popular courses")
		return
	}

	subjectIDs := make([]string, len(subjects))
	for i, s := range subjects {
		subjectIDs[i] = s.ID
	}

	topicCountMap := fetchTopicCounts(c.Request.Context(), subjectIDs)

	items := buildSubjectListResponse(subjects, topicCountMap)

	response := gin.H{
		"items": items,
		"total": len(items),
	}
	if cache.Redis != nil {
		data, _ := json.Marshal(response)
		go func(key string, payload []byte) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cache.Redis.Set(ctx, key, payload, SubjectCacheTTL)
		}(cacheKey, data)
	}

	api_response.Success(c, response)
}
