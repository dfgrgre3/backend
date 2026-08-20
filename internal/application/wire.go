package application

import (
	"fmt"
	coursedelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	courserepo "thanawy-backend/internal/infrastructure/persistence/repositories"

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

	listCoursesHandler := courseservice.NewListCoursesHandler(database)
	getCourseHandler := courseservice.NewGetCourseHandler(database)
	createCourseHandler := courseservice.NewCreateCourseHandler(database)
	updateCourseHandler := courseservice.NewUpdateCourseHandler(database)

	// Import course repository and service
	lmsRepo := courserepo.NewLmsRepository(database)
	courService := courseservice.NewLmsService(lmsRepo)
	searchCoursesHandler := courseservice.NewSearchCoursesHandler(database)
	getEnrollmentHandler := courseservice.NewGetEnrollmentHandler()
	enrollUserHandler := courseservice.NewEnrollUserHandler(database)
	updateProgressHandler := courseservice.NewUpdateProgressHandler()

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
