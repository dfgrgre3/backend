package app

import (
	"thanawy-backend/internal/adapters/http"
	"thanawy-backend/internal/adapters/repository"
	"thanawy-backend/internal/api/handlers"
	coursecmd "thanawy-backend/internal/app/command/course"
	coursequery "thanawy-backend/internal/app/query/course"
	"thanawy-backend/internal/cache"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/domain/certificate"
	coursedomain "thanawy-backend/internal/domain/course"
	"thanawy-backend/internal/domain/subject"
	"thanawy-backend/internal/domain/user"

	"gorm.io/gorm"
)

type Services struct {
	UserService        *user.Service
	SubjectService     *subject.Service
	CertificateService *certificate.Service
	CourseService      *coursedomain.CourseService
}

type Handlers struct {
	UserHandler        *http.UserHandler
	SubjectHandler     *http.SubjectHandler
	CertificateHandler *http.CertificateHandler
	CourseRESTHandler  *handlers.CourseRESTHandler
}

func Initialize(database *gorm.DB) (*Services, *Handlers) {
	if database == nil {
		database = db.DB
	}

	userRepo := repository.NewUserRepository(database)
	userHasher := repository.NewBcryptHasher()
	userPublisher := repository.NewNoOpPublisher()
	userService := user.NewService(userRepo, userHasher, userPublisher)

	// Build Subject repository stack:
	// 1. Base repository (GORM)
	baseSubjectRepo := repository.NewSubjectRepository(database)
	
	// 2. Caching layer (Redis) wraps the base repository
	invalidator := cache.NewCacheInvalidator()
	subjectRepo := repository.NewSubjectCachedRepository(baseSubjectRepo, db.Redis, invalidator)

	subjectPublisher := repository.NewNoOpSubjectPublisher()
	subjectService := subject.NewService(subjectRepo, subjectPublisher)

	// Certificate Service
	certificateRepo := repository.NewCertificateRepository(database)
	certificatePublisher := repository.NewNoOpCertificatePublisher()
	certificateService := certificate.NewService(certificateRepo, certificatePublisher)

	// Course Domain (new architecture)
	courseRepo := repository.NewCourseRepositoryImpl(database)
	courseService := coursedomain.NewCourseService(courseRepo)

	// Course Command Handlers
	createCourseHandler := coursecmd.NewCreateCourseHandler(courseService)
	updateCourseHandler := coursecmd.NewUpdateCourseHandler(courseService)
	enrollUserHandler := coursecmd.NewEnrollUserHandler(courseService)
	updateProgressHandler := coursecmd.NewUpdateProgressHandler(courseService)

	// Course Query Handlers
	getCourseHandler := coursequery.NewGetCourseHandler(courseService)
	listCoursesHandler := coursequery.NewListCoursesHandler(courseService)
	getEnrollmentHandler := coursequery.NewGetEnrollmentHandler(courseService)

	// Course REST Handler
	courseRESTHandler := handlers.NewCourseRESTHandler(
		courseService,
		createCourseHandler,
		updateCourseHandler,
		enrollUserHandler,
		updateProgressHandler,
		getCourseHandler,
		listCoursesHandler,
		getEnrollmentHandler,
		database,
	)

	services := &Services{
		UserService:        userService,
		SubjectService:     subjectService,
		CertificateService: certificateService,
		CourseService:      courseService,
	}

	handlers := &Handlers{
		UserHandler:       http.NewUserHandler(userService),
		SubjectHandler:    http.NewSubjectHandler(subjectService, certificateService),
		CertificateHandler: http.NewCertificateHandler(certificateService),
		CourseRESTHandler:  courseRESTHandler,
	}

	return services, handlers
}
