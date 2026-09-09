package protected

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTeachingUpdateCourseRejectsCurriculumChangesToPublishedCourse(t *testing.T) {
	originalDB := db.DB
	t.Cleanup(func() { db.DB = originalDB })

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&models.Subject{}, &models.Topic{}, &models.SubTopic{}))
	db.DB = database

	instructorID := "instructor-1"
	courseID := "course-1"
	require.NoError(t, database.Create(&models.Subject{
		ID:           courseID,
		Name:         "Published course",
		InstructorId: &instructorID,
		Status:       models.CourseStatusPublished,
	}).Error)
	require.NoError(t, database.Create(&models.Topic{ID: "topic-1", SubjectID: courseID, Title: "Original", Order: 1}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: courseID}}
	context.Set("userId", instructorID)
	context.Request = httptest.NewRequest(http.MethodPatch, "/courses/"+courseID, bytes.NewBufferString(`{"chapters":[{"id":"topic-1","title":"Changed"}]}`))
	context.Request.Header.Set("Content-Type", "application/json")

	TeachingUpdateCourse(context)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	var topic models.Topic
	require.NoError(t, database.First(&topic, "id = ?", "topic-1").Error)
	assert.Equal(t, "Original", topic.Title)
}

func TestUpsertTeachingCurriculumPreservesExistingIDsAndDoesNotDeleteOmittedItems(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&models.Topic{}, &models.SubTopic{}, &models.LessonAttachment{}))

	courseID := uuid.NewString()
	keptTopicID := uuid.NewString()
	keptLessonID := uuid.NewString()
	omittedTopicID := uuid.NewString()
	omittedLessonID := uuid.NewString()
	attachmentID := uuid.NewString()

	require.NoError(t, database.Create(&models.Topic{ID: keptTopicID, SubjectID: courseID, Title: "Old chapter", Order: 1}).Error)
	require.NoError(t, database.Create(&models.Topic{ID: omittedTopicID, SubjectID: courseID, Title: "Omitted chapter", Order: 2}).Error)
	require.NoError(t, database.Create(&models.SubTopic{ID: keptLessonID, TopicID: keptTopicID, Title: "Old lesson", Type: models.SubTopicVideo, Order: 1}).Error)
	require.NoError(t, database.Create(&models.SubTopic{ID: omittedLessonID, TopicID: omittedTopicID, Title: "Omitted lesson", Type: models.SubTopicVideo, Order: 1}).Error)
	require.NoError(t, database.Create(&models.LessonAttachment{ID: attachmentID, SubTopicID: keptLessonID, Title: "Old file", FileUrl: "https://example.com/old.pdf"}).Error)

	chapters := []teachingChapterInput{{
		ID:    keptTopicID,
		Title: "Updated chapter",
		Lessons: []teachingLessonInput{{
			ID:    keptLessonID,
			Title: "Updated lesson",
			Type:  string(models.SubTopicVideo),
			Attachments: []teachingAttachmentInput{{
				ID:       attachmentID,
				Title:    "Updated file",
				FileURL:  "https://example.com/new.pdf",
				FileType: "application/pdf",
			}},
		}},
	}}

	response, err := upsertTeachingCurriculum(database, courseID, chapters)
	require.NoError(t, err)
	assert.Equal(t, keptTopicID, response[0]["id"])

	var topic models.Topic
	require.NoError(t, database.First(&topic, "id = ?", keptTopicID).Error)
	assert.Equal(t, "Updated chapter", topic.Title)

	var lesson models.SubTopic
	require.NoError(t, database.First(&lesson, "id = ?", keptLessonID).Error)
	assert.Equal(t, "Updated lesson", lesson.Title)

	var omittedTopic models.Topic
	require.NoError(t, database.First(&omittedTopic, "id = ?", omittedTopicID).Error)
	var omittedLesson models.SubTopic
	require.NoError(t, database.First(&omittedLesson, "id = ?", omittedLessonID).Error)
	assert.Equal(t, "Omitted chapter", omittedTopic.Title)
	assert.Equal(t, "Omitted lesson", omittedLesson.Title)

	var attachment models.LessonAttachment
	require.NoError(t, database.First(&attachment, "id = ?", attachmentID).Error)
	assert.Equal(t, "Updated file", attachment.Title)
	assert.Equal(t, "https://example.com/new.pdf", attachment.FileUrl)
}
