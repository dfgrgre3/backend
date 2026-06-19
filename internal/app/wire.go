package app

import (
	"thanawy-backend/internal/adapters/http"
	"thanawy-backend/internal/adapters/repository"
	"thanawy-backend/internal/cache"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/domain/certificate"
	"thanawy-backend/internal/domain/subject"
	"thanawy-backend/internal/domain/user"

	"gorm.io/gorm"
)

type Services struct {
	UserService        *user.Service
	SubjectService     *subject.Service
	CertificateService *certificate.Service
}

type Handlers struct {
	UserHandler        *http.UserHandler
	SubjectHandler     *http.SubjectHandler
	CertificateHandler *http.CertificateHandler
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

	services := &Services{
		UserService:        userService,
		SubjectService:     subjectService,
		CertificateService: certificateService,
	}

	handlers := &Handlers{
		UserHandler:        http.NewUserHandler(userService),
		SubjectHandler:     http.NewSubjectHandler(subjectService, certificateService),
		CertificateHandler: http.NewCertificateHandler(certificateService),
	}

	return services, handlers
}
