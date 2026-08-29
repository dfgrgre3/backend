package api

import (
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminCourseRoutes registers Subject, course-alias, curriculum,
// course-students and manual-enroll routes.
func registerAdminCourseRoutes(admin *gin.RouterGroup) {
	// Subject
	admin.GET(adminSubjectsRoute, handlers.GetSubjects)
	admin.POST(adminSubjectsRoute, handlers.CreateSubject)
	admin.PATCH(adminSubjectsRoute, handlers.UpdateSubject)
	admin.DELETE(adminSubjectsRoute, handlers.DeleteSubject)

	// Course aliases for Admin panel compatibility.
	// GET and POST /courses are owned by CourseRESTHandler in hexagonal_routes.go.
	admin.PATCH("/courses", handlers.UpdateSubject)
	admin.DELETE("/courses", handlers.DeleteSubject)
	admin.GET("/courses/:id/curriculum", handlers.GetSubjectCurriculum)
	admin.PUT("/courses/:id/curriculum", handlers.UpdateCourseCurriculum)
	admin.PATCH("/courses/:id/curriculum", handlers.UpdateCourseCurriculum)
	admin.POST("/courses/duplicate", handlers.DuplicateCourse)
	admin.POST("/courses/batch", handlers.BatchCourseAction)

	// Curriculum
	admin.PATCH("/subjects/:id/curriculum", handlers.UpdateCourseCurriculum)
	admin.GET("/subjects/:id/curriculum", handlers.GetSubjectCurriculum)

	// Course Students (view list of enrolled students)
	admin.GET("/courses/:id/students", handlers.GetCourseStudents)
	admin.GET("/courses/:id/students/export", handlers.ExportCourseStudentsCSV)
	admin.GET("/courses/:id/overview-stats", handlers.GetCourseOverviewStats)
	admin.GET("/courses/:id/analytics", handlers.GetCourseAnalytics)

	// Manual Enroll
	admin.GET("/courses/enrollments", handlers.GetCourseEnrollments)
	admin.POST("/courses/enroll", handlers.ManualEnroll)
	admin.POST("/courses/unenroll", handlers.UnenrollUser)
	admin.POST("/courses/lessons/:id/attachments", handlers.AddLessonAttachment)
	admin.DELETE("/courses/lessons/attachments/:attachmentId", handlers.DeleteLessonAttachment)
}
