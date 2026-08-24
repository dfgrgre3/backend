package protected

import (
	courseservice "thanawy-backend/internal/domain/course/service"

	"gorm.io/gorm"
)

// CourseRESTHandler handles REST API requests for courses.
//
// Its methods are split across several files in this package (all sharing
// the protected package and the CourseRESTHandler receiver), grouped by
// area: this file (construction/wiring), course_handler_create.go,
// course_handler_update.go, course_handler_delete.go,
// course_handler_list.go (list/search), course_handler_get.go
// (get/meta/slug-check/pending-review) course_handler_reviews.go and
// course_handler_bulk.go.
type CourseRESTHandler struct {
	createCourseHandler   *courseservice.CreateCourseHandler
	updateCourseHandler   *courseservice.UpdateCourseHandler
	enrollUserHandler     *courseservice.EnrollUserHandler
	updateProgressHandler *courseservice.UpdateProgressHandler
	getCourseHandler      *courseservice.GetCourseHandler
	listCoursesHandler    *courseservice.ListCoursesHandler
	searchCoursesHandler  *courseservice.SearchCoursesHandler
	getEnrollmentHandler  *courseservice.GetEnrollmentHandler
	courseService         *courseservice.LmsService
	db                    *gorm.DB
}

// NewCourseRESTHandler creates a new REST course handler
func NewCourseRESTHandler(
	courseService *courseservice.LmsService,
	createCourseHandler *courseservice.CreateCourseHandler,
	updateCourseHandler *courseservice.UpdateCourseHandler,
	enrollUserHandler *courseservice.EnrollUserHandler,
	updateProgressHandler *courseservice.UpdateProgressHandler,
	getCourseHandler *courseservice.GetCourseHandler,
	listCoursesHandler *courseservice.ListCoursesHandler,
	searchCoursesHandler *courseservice.SearchCoursesHandler,
	getEnrollmentHandler *courseservice.GetEnrollmentHandler,
	db *gorm.DB,
) *CourseRESTHandler {
	return &CourseRESTHandler{
		courseService:         courseService,
		createCourseHandler:   createCourseHandler,
		updateCourseHandler:   updateCourseHandler,
		enrollUserHandler:     enrollUserHandler,
		updateProgressHandler: updateProgressHandler,
		getCourseHandler:      getCourseHandler,
		listCoursesHandler:    listCoursesHandler,
		searchCoursesHandler:  searchCoursesHandler,
		getEnrollmentHandler:  getEnrollmentHandler,
		db:                    db,
	}
}
