package protected

import (
	"errors"
	"net/http/httptest"
	"testing"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCourseQuizQueryForViewerHidesUnpublishedQuizzesFromStudents(t *testing.T) {
	originalDB := db.DB
	t.Cleanup(func() { db.DB = originalDB })

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&models.Subject{}, &models.CourseQuiz{}))
	db.DB = database

	instructorID := "instructor-1"
	require.NoError(t, database.Create(&models.Subject{ID: "course-1", Name: "Course", InstructorId: &instructorID}).Error)
	for _, quiz := range []models.CourseQuiz{
		{ID: "quiz-draft", CourseID: "course-1", Title: "Draft", Status: "draft", Questions: []byte("[]"), CreatedBy: instructorID},
		{ID: "quiz-published", CourseID: "course-1", Title: "Published", Status: "published", Questions: []byte("[]"), CreatedBy: instructorID},
		{ID: "quiz-archived", CourseID: "course-1", Title: "Archived", Status: "archived", Questions: []byte("[]"), CreatedBy: instructorID},
	} {
		require.NoError(t, database.Create(&quiz).Error)
	}

	studentContext := quizTestContext("student-1")
	var studentQuizzes []models.CourseQuiz
	require.NoError(t, courseQuizQueryForViewer(studentContext, "course-1", "student-1").Find(&studentQuizzes).Error)
	assert.Equal(t, []string{"quiz-published"}, quizIDs(studentQuizzes))

	instructorContext := quizTestContext(instructorID)
	var instructorQuizzes []models.CourseQuiz
	require.NoError(t, courseQuizQueryForViewer(instructorContext, "course-1", instructorID).Find(&instructorQuizzes).Error)
	assert.ElementsMatch(t, []string{"quiz-draft", "quiz-published", "quiz-archived"}, quizIDs(instructorQuizzes))

	_, err = findCourseQuizForViewer(studentContext, "course-1", "quiz-draft", "student-1")
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))

	quiz, err := findCourseQuizForViewer(instructorContext, "course-1", "quiz-draft", instructorID)
	require.NoError(t, err)
	assert.Equal(t, "quiz-draft", quiz.ID)
}

func TestQuizCourseAccessRejectsForeignStudent(t *testing.T) {
	originalDB := db.DB
	t.Cleanup(func() { db.DB = originalDB })

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&models.Subject{}, &models.Enrollment{}))
	db.DB = database

	instructorID := "instructor-1"
	require.NoError(t, database.Create(&models.Subject{
		ID:           "course-1",
		Name:         "Course",
		InstructorId: &instructorID,
	}).Error)
	require.NoError(t, database.Create(&models.Enrollment{
		UserID:    "student-1",
		SubjectID: "course-1",
	}).Error)

	enrolledStudent := quizTestContext("student-1")
	foreignStudent := quizTestContext("student-2")
	instructor := quizTestContext(instructorID)
	instructor.Set("role", string(models.RoleTeacher))
	admin := quizTestContext("admin-1")
	admin.Set("role", string(models.RoleAdmin))

	assert.True(t, quizCourseAccess(enrolledStudent, "course-1", "student-1"))
	assert.False(t, quizCourseAccess(foreignStudent, "course-1", "student-2"))
	assert.True(t, quizCourseAccess(instructor, "course-1", instructorID))
	assert.True(t, quizCourseAccess(admin, "course-1", "admin-1"))
}

func quizTestContext(userID string) *gin.Context {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("userId", userID)
	return context
}

func quizIDs(quizzes []models.CourseQuiz) []string {
	ids := make([]string, 0, len(quizzes))
	for _, quiz := range quizzes {
		ids = append(ids, quiz.ID)
	}
	return ids
}
