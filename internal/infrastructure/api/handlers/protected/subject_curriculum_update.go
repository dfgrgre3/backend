package protected

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func UpdateCourseCurriculum(c *gin.Context) {
	id := c.Param("id")
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	chaptersRaw, err := extractChaptersRaw(raw)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var chapters []incomingChapter
	if err := json.Unmarshal(chaptersRaw, &chapters); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid curriculum format: "+err.Error())
		return
	}

	// The admin course UI also targets the newer LmsCourse model.  Do not send
	// it through the legacy Subject implementation below: that implementation
	// deletes and recreates the entire tree and cannot find LmsCourse records.
	// Upsert the submitted nodes instead, preserving IDs and leaving omitted
	// nodes for the explicit section/lesson delete endpoints.
	if courseID, parseErr := uuid.Parse(id); parseErr == nil {
		var lmsCourse models.LmsCourse
		if err := db.DB.First(&lmsCourse, "id = ?", courseID).Error; err == nil {
			if lmsCourse.Status == models.CourseStatusPublished {
				api_response.Error(c, http.StatusConflict, "Published course curriculum requires a new version before editing")
				return
			}

			if err := db.DB.Transaction(func(tx *gorm.DB) error {
				return upsertLmsCurriculum(tx, courseID, chapters)
			}); err != nil {
				api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to save curriculum", err)
				return
			}

			var updated models.LmsCourse
			if err := db.DB.Preload("Sections.Lessons", func(query *gorm.DB) *gorm.DB {
				return query.Order("order_index ASC")
			}).First(&updated, "id = ?", courseID).Error; err != nil {
				api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to load updated curriculum", err)
				return
			}
			api_response.Success(c, gin.H{"curriculum": updated.Sections, "sections": updated.Sections})
			return
		}
	}

	// Verify the subject exists before mutating any topics.
	// Without this check, an invalid/mismatched subject ID causes a PostgreSQL
	// FK violation (SQLSTATE 23503) instead of a meaningful 404 response.
	var subject models.Subject
	if err := db.DB.Select("id", "status").First(&subject, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}
	if subject.Status == models.CourseStatusPublished {
		api_response.Error(c, http.StatusConflict, "Published course curriculum requires a new version before editing")
		return
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		_, err := upsertTeachingCurriculum(tx, id, mapIncomingChapters(chapters))
		return err
	}); err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to save curriculum", err)
		return
	}

	getSubjectRepo().InvalidateSubjectCache(id)
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), id)

	if err := db.DB.Preload(preloadTopicsSubTopics).First(&subject, idQuery, id).Error; err != nil {
		api_response.Success(c, gin.H{"success": true, "message": "Curriculum updated"})
		return
	}

	api_response.Success(c, gin.H{"curriculum": subject.Topics})
}

// upsertLmsCurriculum performs a non-destructive compatibility update for the
// legacy aggregate curriculum payload. Missing sections/lessons are not
// deleted; deletion remains available through the granular APIs.
func upsertLmsCurriculum(tx *gorm.DB, courseID uuid.UUID, chapters []incomingChapter) error {
	for chapterIndex, chapter := range chapters {
		title := strings.TrimSpace(chapter.Name)
		if title == "" {
			title = strings.TrimSpace(chapter.Title)
		}

		section := models.LmsSection{CourseID: courseID, Title: title, OrderIndex: chapterIndex}
		if sectionID, err := uuid.Parse(chapter.ID); err == nil {
			section.ID = sectionID
			if err := tx.Where("id = ? AND course_id = ?", sectionID, courseID).First(&section).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				section = models.LmsSection{ID: sectionID, CourseID: courseID}
			}
		}
		section.Title = title
		section.OrderIndex = chapterIndex
		if section.ID == uuid.Nil {
			if err := tx.Create(&section).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&models.LmsSection{}).Where("id = ? AND course_id = ?", section.ID, courseID).
			Updates(map[string]interface{}{"title": section.Title, "order_index": section.OrderIndex}).Error; err != nil {
			return err
		}

		for lessonIndex, lesson := range chapter.SubTopics {
			lessonTitle := strings.TrimSpace(lesson.Name)
			if lessonTitle == "" {
				lessonTitle = strings.TrimSpace(lesson.Title)
			}
			durationMinutes := lesson.Duration
			if durationMinutes == 0 {
				durationMinutes = lesson.DurationMin
			}
			lmsLesson := models.LmsLesson{
				SectionID: section.ID, Title: lessonTitle, Type: models.LessonType(lesson.Type),
				MediaURL: lesson.VideoUrl, DurationSeconds: durationMinutes * 60,
				IsFreePreview: lesson.IsFree, OrderIndex: lessonIndex, Content: lesson.Description,
			}
			if lessonID, err := uuid.Parse(lesson.ID); err == nil {
				lmsLesson.ID = lessonID
				if err := tx.Where("id = ? AND section_id = ?", lessonID, section.ID).First(&lmsLesson).Error; err != nil {
					if !errors.Is(err, gorm.ErrRecordNotFound) {
						return err
					}
					lmsLesson = models.LmsLesson{ID: lessonID, SectionID: section.ID}
				}
			}
			lmsLesson.Title, lmsLesson.OrderIndex = lessonTitle, lessonIndex
			lmsLesson.Type, lmsLesson.MediaURL = models.LessonType(lesson.Type), lesson.VideoUrl
			lmsLesson.DurationSeconds, lmsLesson.IsFreePreview = durationMinutes*60, lesson.IsFree
			lmsLesson.Content = lesson.Description
			if lmsLesson.ID == uuid.Nil {
				if err := tx.Create(&lmsLesson).Error; err != nil {
					return err
				}
			} else if err := tx.Model(&models.LmsLesson{}).Where("id = ? AND section_id = ?", lmsLesson.ID, section.ID).Updates(map[string]interface{}{
				"title": lmsLesson.Title, "type": lmsLesson.Type, "media_url": lmsLesson.MediaURL,
				"duration_seconds": lmsLesson.DurationSeconds, "is_free_preview": lmsLesson.IsFreePreview,
				"order_index": lmsLesson.OrderIndex, "content": lmsLesson.Content,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

type incomingAttachment struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	FileUrl  string  `json:"fileUrl"`
	FileType *string `json:"fileType"`
	FileSize *int64  `json:"fileSize"`
}

type incomingLesson struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Title       string               `json:"title"`
	Order       int                  `json:"order"`
	Type        string               `json:"type"`
	VideoUrl    *string              `json:"videoUrl"`
	Duration    int                  `json:"duration"`
	DurationMin int                  `json:"durationMinutes"`
	IsFree      bool                 `json:"isFree"`
	Description *string              `json:"description"`
	Attachments []incomingAttachment `json:"attachments"`
}

type incomingChapter struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Title     string           `json:"title"`
	Order     int              `json:"order"`
	SubTopics []incomingLesson `json:"subTopics"`
}

func mapIncomingChapters(chapters []incomingChapter) []teachingChapterInput {
	result := make([]teachingChapterInput, 0, len(chapters))
	for _, chapter := range chapters {
		title := chapter.Name
		if title == "" {
			title = chapter.Title
		}
		mapped := teachingChapterInput{ID: chapter.ID, Title: title, Lessons: make([]teachingLessonInput, 0, len(chapter.SubTopics))}
		for _, lesson := range chapter.SubTopics {
			title := lesson.Name
			if title == "" {
				title = lesson.Title
			}
			attachments := make([]teachingAttachmentInput, 0, len(lesson.Attachments))
			for _, attachment := range lesson.Attachments {
				fileType := ""
				if attachment.FileType != nil {
					fileType = *attachment.FileType
				}
				fileSize := int64(0)
				if attachment.FileSize != nil {
					fileSize = *attachment.FileSize
				}
				attachments = append(attachments, teachingAttachmentInput{
					ID:       attachment.ID,
					Title:    attachment.Title,
					FileURL:  attachment.FileUrl,
					FileType: fileType,
					FileSize: fileSize,
				})
			}
			durationMinutes := lesson.Duration
			if durationMinutes == 0 {
				durationMinutes = lesson.DurationMin
			}
			mapped.Lessons = append(mapped.Lessons, teachingLessonInput{
				ID:              lesson.ID,
				Title:           title,
				VideoURL:        lesson.VideoUrl,
				DurationMinutes: &durationMinutes,
				Duration:        "",
				Type:            lesson.Type,
				Preview:         lesson.IsFree,
				Description:     lesson.Description,
				Attachments:     attachments,
			})
		}
		result = append(result, mapped)
	}
	return result
}

func extractChaptersRaw(raw map[string]json.RawMessage) (json.RawMessage, error) {
	if v, ok := raw["curriculum"]; ok {
		return v, nil
	}
	if v, ok := raw["topics"]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("missing curriculum or topics field")
}
