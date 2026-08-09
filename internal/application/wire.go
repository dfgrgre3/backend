package application

import (
	"fmt"
	coursedelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	courserepo "thanawy-backend/internal/infrastructure/persistence/repositories"

	commandcourse "thanawy-backend/internal/domain/course/service"
	coursequery "thanawy-backend/internal/domain/course/service"
	courseservice "thanawy-backend/internal/domain/course/service"

	"gorm.io/gorm"
)

type Handlers struct {
	CourseRESTHandler *coursedelivery.CourseRESTHandler
}

func Initialize(database *gorm.DB) (*Handlers, error) {
	if database == nil {
		return nil, fmt.Errorf("initialize application: database is required")
	}

	listCoursesHandler := coursequery.NewListCoursesHandler(database)
	getCourseHandler := coursequery.NewGetCourseHandler(database)
	createCourseHandler := commandcourse.NewCreateCourseHandler(database)
	updateCourseHandler := commandcourse.NewUpdateCourseHandler(database)

	// Import course repository and service
	lmsRepo := courserepo.NewLmsRepository(database)
	courService := courseservice.NewLmsService(lmsRepo)
	searchCoursesHandler := coursequery.NewSearchCoursesHandler(database)
	getEnrollmentHandler := coursequery.NewGetEnrollmentHandler()
	enrollUserHandler := commandcourse.NewEnrollUserHandler(database)
	updateProgressHandler := commandcourse.NewUpdateProgressHandler()

	// Course REST Handler
	courseRESTHandler := coursedelivery.NewCourseRESTHandler(
		courService,
		createCourseHandler,
		updateCourseHandler,
		enrollUserHandler,
		updateProgressHandler,
		getCourseHandler,
		listCoursesHandler,
		searchCoursesHandler,
		getEnrollmentHandler,
		database,
	)

	handlers := &Handlers{
		CourseRESTHandler: courseRESTHandler,
	}

	return handlers, nil
}
