package app

import (
	"fmt"

	"thanawy-backend/internal/api/handlers"
	coursequery "thanawy-backend/internal/app/query/course"

	"gorm.io/gorm"
)

type Handlers struct {
	CourseRESTHandler *handlers.CourseRESTHandler
}

func Initialize(database *gorm.DB) (*Handlers, error) {
	if database == nil {
		return nil, fmt.Errorf("initialize application: database is required")
	}

	listCoursesHandler := coursequery.NewListCoursesHandler(database)
	getCourseHandler := coursequery.NewGetCourseHandler(database)

	// Course REST Handler
	courseRESTHandler := handlers.NewCourseRESTHandler(
		nil, // courseService - will be nil since we removed hexagonal architecture
		nil, // createCourseHandler
		nil, // updateCourseHandler
		nil, // enrollUserHandler
		nil, // updateProgressHandler
		getCourseHandler,
		listCoursesHandler,
		nil, // getEnrollmentHandler
		database,
	)

	handlers := &Handlers{
		CourseRESTHandler: courseRESTHandler,
	}

	return handlers, nil
}
