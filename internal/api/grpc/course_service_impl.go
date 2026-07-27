package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"

	cmd_course "thanawy-backend/internal/app/command/course"
	query_course "thanawy-backend/internal/app/query/course"
	domain_course "thanawy-backend/internal/domain/course"
	coursev1 "thanawy-backend/internal/proto/thanawy/v1"
)

// CourseService implements the gRPC course service
type CourseService struct {
	coursev1.UnimplementedCourseServiceServer

	// Command handlers
	createCourseHandler   *cmd_course.CreateCourseHandler
	updateCourseHandler   *cmd_course.UpdateCourseHandler
	enrollUserHandler     *cmd_course.EnrollUserHandler
	updateProgressHandler *cmd_course.UpdateProgressHandler

	// Query handlers
	getCourseHandler     *query_course.GetCourseHandler
	listCoursesHandler   *query_course.ListCoursesHandler
	getEnrollmentHandler *query_course.GetEnrollmentHandler

	// Service
	courseService *domain_course.CourseService
}

// NewCourseService creates a new gRPC course service
func NewCourseService(
	courseService *domain_course.CourseService,
	createCourseHandler *cmd_course.CreateCourseHandler,
	updateCourseHandler *cmd_course.UpdateCourseHandler,
	enrollUserHandler *cmd_course.EnrollUserHandler,
	updateProgressHandler *cmd_course.UpdateProgressHandler,
	getCourseHandler *query_course.GetCourseHandler,
	listCoursesHandler *query_course.ListCoursesHandler,
	getEnrollmentHandler *query_course.GetEnrollmentHandler,
) *CourseService {
	return &CourseService{
		courseService:         courseService,
		createCourseHandler:   createCourseHandler,
		updateCourseHandler:   updateCourseHandler,
		enrollUserHandler:     enrollUserHandler,
		updateProgressHandler: updateProgressHandler,
		getCourseHandler:      getCourseHandler,
		listCoursesHandler:    listCoursesHandler,
		getEnrollmentHandler:  getEnrollmentHandler,
	}
}

// CreateCourse creates a new course
func (c *CourseService) CreateCourse(ctx context.Context, req *coursev1.CreateCourseRequest) (*coursev1.CreateCourseResponse, error) {
	cmd := cmd_course.CreateCourseCommand{
		Title:                 req.Title,
		Slug:                  req.Slug,
		ShortDescription:      &req.ShortDescription,
		LongDescription:       &req.LongDescription,
		CoverImageURL:         &req.CoverImageUrl,
		PromoVideoURL:         &req.PromoVideoUrl,
		Level:                 domain_course.CourseLevel(req.Level),
		Language:              req.Language,
		EstimatedDurationMins: int(req.EstimatedDurationMins),
		HasCertificate:        req.HasCertificate,
		CertificateTemplate:   &req.CertificateTemplate,
		MaxStudents:           intPtr(int(req.MaxStudents)),
		SEOTitle:              &req.SeoTitle,
		SEODescription:        &req.SeoDescription,
		SEOKeywords:           req.SeoKeywords,
		PrerequisitesText:     &req.PrerequisitesText,
		TargetAudience:        &req.TargetAudience,
		LearningOutcomes:      req.LearningOutcomes,
		PrimaryInstructorID:   req.PrimaryInstructorId,
		CategoryIDs:           req.CategoryIds,
	}

	courseEntity, err := c.createCourseHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return &coursev1.CreateCourseResponse{
		Course: c.domainToProto(courseEntity),
	}, nil
}

// GetCourse retrieves a course
func (c *CourseService) GetCourse(ctx context.Context, req *coursev1.GetCourseRequest) (*coursev1.GetCourseResponse, error) {
	query := query_course.GetCourseQuery{
		ID:   req.Id,
		Slug: req.Slug,
	}

	courseEntity, err := c.getCourseHandler.Handle(ctx, query)
	if err != nil {
		return nil, err
	}

	return &coursev1.GetCourseResponse{
		Course: c.domainToProto(courseEntity),
	}, nil
}

// UpdateCourse updates a course
func (c *CourseService) UpdateCourse(ctx context.Context, req *coursev1.UpdateCourseRequest) (*coursev1.UpdateCourseResponse, error) {
	cmd := cmd_course.UpdateCourseCommand{
		CourseID:              req.Id,
		Title:                 &req.Title,
		Slug:                  &req.Slug,
		ShortDescription:      &req.ShortDescription,
		LongDescription:       &req.LongDescription,
		CoverImageURL:         &req.CoverImageUrl,
		PromoVideoURL:         &req.PromoVideoUrl,
		Level:                 (*domain_course.CourseLevel)(&req.Level),
		Language:              &req.Language,
		EstimatedDurationMins: intPtr(int(req.EstimatedDurationMins)),
		HasCertificate:        &req.HasCertificate,
		CertificateTemplate:   &req.CertificateTemplate,
		MaxStudents:           intPtr(int(req.MaxStudents)),
		IsFeatured:            &req.IsFeatured,
		IsTrending:            &req.IsTrending,
		IsNew:                 &req.IsNew,
		SEOTitle:              &req.SeoTitle,
		SEODescription:        &req.SeoDescription,
		SEOKeywords:           req.SeoKeywords,
		PrerequisitesText:     &req.PrerequisitesText,
		TargetAudience:        &req.TargetAudience,
		LearningOutcomes:      req.LearningOutcomes,
		PrimaryInstructorID:   &req.PrimaryInstructorId,
		CategoryIDs:           req.CategoryIds,
	}

	courseEntity, err := c.updateCourseHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return &coursev1.UpdateCourseResponse{
		Course: c.domainToProto(courseEntity),
	}, nil
}

// DeleteCourse deletes a course
func (c *CourseService) DeleteCourse(ctx context.Context, req *coursev1.DeleteCourseRequest) (*coursev1.DeleteCourseResponse, error) {
	err := c.courseService.DeleteCourse(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &coursev1.DeleteCourseResponse{
		Success: true,
	}, nil
}

// ListCourses lists courses
func (c *CourseService) ListCourses(ctx context.Context, req *coursev1.ListCoursesRequest) (*coursev1.ListCoursesResponse, error) {
	var status *domain_course.CourseStatus
	if req.Status != "" {
		s := domain_course.CourseStatus(req.Status)
		status = &s
	}

	var level *domain_course.CourseLevel
	if req.Level != "" {
		l := domain_course.CourseLevel(req.Level)
		level = &l
	}

	query := query_course.ListCoursesQuery{
		Status:       status,
		Level:        level,
		Language:     &req.Language,
		CategoryID:   &req.CategoryId,
		InstructorID: &req.InstructorId,
		IsFeatured:   &req.IsFeatured,
		IsTrending:   &req.IsTrending,
		IsNew:        &req.IsNew,
		SearchQuery:  &req.SearchQuery,
		Page:         int(req.Page),
		Limit:        int(req.Limit),
	}

	courses, total, err := c.listCoursesHandler.Handle(ctx, query)
	if err != nil {
		return nil, err
	}

	protoCourses := make([]*coursev1.Course, len(courses))
	for i, course := range courses {
		protoCourses[i] = c.domainToProto(course)
	}

	totalPages := int32((total + int(req.Limit) - 1) / int(req.Limit))

	return &coursev1.ListCoursesResponse{
		Courses:    protoCourses,
		Total:      int32(total),
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: totalPages,
	}, nil
}

// SubmitForReview submits a course for review
func (c *CourseService) SubmitForReview(ctx context.Context, req *coursev1.SubmitForReviewRequest) (*coursev1.SubmitForReviewResponse, error) {
	err := c.courseService.SubmitForReview(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &coursev1.SubmitForReviewResponse{
		Success: true,
		Status:  string(domain_course.CourseStatusUnderReview),
	}, nil
}

// ApproveCourse approves a course
func (c *CourseService) ApproveCourse(ctx context.Context, req *coursev1.ApproveCourseRequest) (*coursev1.ApproveCourseResponse, error) {
	err := c.courseService.ApproveCourse(ctx, req.Id, req.ReviewerId, req.Notes)
	if err != nil {
		return nil, err
	}

	return &coursev1.ApproveCourseResponse{
		Success: true,
		Status:  string(domain_course.CourseStatusPublished),
	}, nil
}

// RejectCourse rejects a course
func (c *CourseService) RejectCourse(ctx context.Context, req *coursev1.RejectCourseRequest) (*coursev1.RejectCourseResponse, error) {
	err := c.courseService.RejectCourse(ctx, req.Id, req.ReviewerId, req.Reason)
	if err != nil {
		return nil, err
	}

	return &coursev1.RejectCourseResponse{
		Success: true,
		Status:  string(domain_course.CourseStatusRejected),
	}, nil
}

// ArchiveCourse archives a course
func (c *CourseService) ArchiveCourse(ctx context.Context, req *coursev1.ArchiveCourseRequest) (*coursev1.ArchiveCourseResponse, error) {
	err := c.courseService.ArchiveCourse(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &coursev1.ArchiveCourseResponse{
		Success: true,
		Status:  string(domain_course.CourseStatusArchived),
	}, nil
}

// UnarchiveCourse unarchives a course
func (c *CourseService) UnarchiveCourse(ctx context.Context, req *coursev1.UnarchiveCourseRequest) (*coursev1.UnarchiveCourseResponse, error) {
	err := c.courseService.UnarchiveCourse(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &coursev1.UnarchiveCourseResponse{
		Success: true,
		Status:  string(domain_course.CourseStatusDraft),
	}, nil
}

// EnrollUser enrolls a user in a course
func (c *CourseService) EnrollUser(ctx context.Context, req *coursev1.EnrollUserRequest) (*coursev1.EnrollUserResponse, error) {
	cmd := cmd_course.EnrollUserCommand{
		CourseID: req.CourseId,
		UserID:   req.UserId,
	}

	enrollment, err := c.enrollUserHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return &coursev1.EnrollUserResponse{
		Enrollment: c.domainEnrollmentToProto(enrollment),
	}, nil
}

// GetEnrollment retrieves user enrollment
func (c *CourseService) GetEnrollment(ctx context.Context, req *coursev1.GetEnrollmentRequest) (*coursev1.GetEnrollmentResponse, error) {
	query := query_course.GetEnrollmentQuery{
		CourseID: req.CourseId,
		UserID:   req.UserId,
	}

	enrollment, err := c.getEnrollmentHandler.Handle(ctx, query)
	if err != nil {
		return nil, err
	}

	return &coursev1.GetEnrollmentResponse{
		Enrollment: c.domainEnrollmentToProto(enrollment),
	}, nil
}

// UpdateProgress updates enrollment progress
func (c *CourseService) UpdateProgress(ctx context.Context, req *coursev1.UpdateProgressRequest) (*coursev1.UpdateProgressResponse, error) {
	cmd := cmd_course.UpdateProgressCommand{
		CourseID: req.CourseId,
		UserID:   req.UserId,
		Progress: req.Progress,
	}

	err := c.updateProgressHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, err
	}

	// Fetch updated enrollment
	query := query_course.GetEnrollmentQuery{
		CourseID: req.CourseId,
		UserID:   req.UserId,
	}
	enrollment, _ := c.getEnrollmentHandler.Handle(ctx, query)

	return &coursev1.UpdateProgressResponse{
		Enrollment: c.domainEnrollmentToProto(enrollment),
	}, nil
}

// ListEnrollments lists enrollments
func (c *CourseService) ListEnrollments(ctx context.Context, req *coursev1.ListEnrollmentsRequest) (*coursev1.ListEnrollmentsResponse, error) {
	filter := domain_course.EnrollmentFilter{
		CourseID: &req.CourseId,
		UserID:   &req.UserId,
		Status:   &req.Status,
		Page:     int(req.Page),
		Limit:    int(req.Limit),
	}

	enrollments, total, err := c.courseService.ListEnrollments(ctx, filter)
	if err != nil {
		return nil, err
	}

	protoEnrollments := make([]*coursev1.Enrollment, len(enrollments))
	for i, e := range enrollments {
		protoEnrollments[i] = c.domainEnrollmentToProto(e)
	}

	return &coursev1.ListEnrollmentsResponse{
		Enrollments: protoEnrollments,
		Total:       int32(total),
		Page:        req.Page,
		Limit:       req.Limit,
	}, nil
}

// SetPricing sets course pricing
func (c *CourseService) SetPricing(ctx context.Context, req *coursev1.SetPricingRequest) (*coursev1.SetPricingResponse, error) {
	durationDays := int(req.SubscriptionDurationDays)
	pricing := &domain_course.Pricing{
		Type:                     domain_course.PricingType(req.Type),
		Amount:                   req.Amount,
		CurrencyCode:             req.CurrencyCode,
		SubscriptionDurationDays: &durationDays,
		DiscountPrice:            &req.DiscountPrice,
		IsActive:                 true,
	}

	if req.DiscountStartAt > 0 {
		t := time.Unix(req.DiscountStartAt, 0)
		pricing.DiscountStartAt = &t
	}

	if req.DiscountEndAt > 0 {
		t := time.Unix(req.DiscountEndAt, 0)
		pricing.DiscountEndAt = &t
	}

	result, err := c.courseService.SetPricing(ctx, req.CourseId, pricing)
	if err != nil {
		return nil, err
	}

	return &coursev1.SetPricingResponse{
		Pricing: c.domainPricingToProto(result),
	}, nil
}

// GetPricing gets course pricing
func (c *CourseService) GetPricing(ctx context.Context, req *coursev1.GetPricingRequest) (*coursev1.GetPricingResponse, error) {
	pricing, err := c.courseService.GetPricing(ctx, req.CourseId)
	if err != nil {
		return nil, err
	}

	return &coursev1.GetPricingResponse{
		Pricing: c.domainPricingToProto(pricing),
	}, nil
}

// Helper methods to convert domain models to protobuf

func (c *CourseService) domainToProto(courseEntity *domain_course.Course) *coursev1.Course {
	sections := make([]*coursev1.Section, len(courseEntity.Sections))
	for i := range courseEntity.Sections {
		sections[i] = c.domainSectionToProto(&courseEntity.Sections[i])
	}

	pricings := make([]*coursev1.Pricing, len(courseEntity.Pricings))
	for i := range courseEntity.Pricings {
		pricings[i] = c.domainPricingToProto(&courseEntity.Pricings[i])
	}

	return &coursev1.Course{
		Id:                    courseEntity.ID.String(),
		Title:                 courseEntity.Title,
		Slug:                  courseEntity.Slug,
		ShortDescription:      stringPtr(courseEntity.ShortDescription),
		LongDescription:       stringPtr(courseEntity.LongDescription),
		CoverImageUrl:         stringPtr(courseEntity.CoverImageURL),
		PromoVideoUrl:         stringPtr(courseEntity.PromoVideoURL),
		Status:                string(courseEntity.Status),
		Level:                 string(courseEntity.Level),
		Language:              courseEntity.Language,
		EstimatedDurationMins: int32(courseEntity.EstimatedDurationMins),
		HasCertificate:        courseEntity.HasCertificate,
		CertificateTemplate:   stringPtr(courseEntity.CertificateTemplate),
		MaxStudents:           int32Ptr(courseEntity.MaxStudents),
		Version:               int32(courseEntity.Version),
		IsFeatured:            courseEntity.IsFeatured,
		IsTrending:            courseEntity.IsTrending,
		IsNew:                 courseEntity.IsNew,
		SeoTitle:              stringPtr(courseEntity.SEOTitle),
		SeoDescription:        stringPtr(courseEntity.SEODescription),
		SeoKeywords:           courseEntity.SEOKeywords,
		PrerequisitesText:     stringPtr(courseEntity.PrerequisitesText),
		TargetAudience:        stringPtr(courseEntity.TargetAudience),
		LearningOutcomes:      courseEntity.LearningOutcomes,
		PrimaryInstructorId:   courseEntity.PrimaryInstructorID.String(),
		CreatedAt:             courseEntity.CreatedAt.Unix(),
		UpdatedAt:             courseEntity.UpdatedAt.Unix(),
		Sections:              sections,
		Pricings:              pricings,
	}
}

func (c *CourseService) domainSectionToProto(section *domain_course.Section) *coursev1.Section {
	lessons := make([]*coursev1.Lesson, len(section.Lessons))
	for i := range section.Lessons {
		lessons[i] = c.domainLessonToProto(&section.Lessons[i])
	}

	return &coursev1.Section{
		Id:            section.ID.String(),
		CourseId:      section.CourseID.String(),
		Title:         section.Title,
		OrderIndex:    int32(section.OrderIndex),
		AvailableFrom: timePtrToUnix(section.AvailableFrom),
		DripDelayDays: int32Ptr(section.DripDelayDays),
		CreatedAt:     section.CreatedAt.Unix(),
		UpdatedAt:     section.UpdatedAt.Unix(),
		Lessons:       lessons,
	}
}

func (c *CourseService) domainLessonToProto(lesson *domain_course.Lesson) *coursev1.Lesson {
	attachments := make([]*coursev1.Attachment, len(lesson.Attachments))
	for i := range lesson.Attachments {
		attachments[i] = c.domainAttachmentToProto(&lesson.Attachments[i])
	}

	return &coursev1.Lesson{
		Id:               lesson.ID.String(),
		SectionId:        lesson.SectionID.String(),
		Title:            lesson.Title,
		Type:             string(lesson.Type),
		Content:          stringPtr(lesson.Content),
		MediaUrl:         stringPtr(lesson.MediaURL),
		DurationSeconds:  int32(lesson.DurationSeconds),
		IsFreePreview:    lesson.IsFreePreview,
		OrderIndex:       int32(lesson.OrderIndex),
		AvailabilityType: string(lesson.AvailabilityType),
		AvailableFrom:    timePtrToUnix(lesson.AvailableFrom),
		DripDelayDays:    int32Ptr(lesson.DripDelayDays),
		CreatedAt:        lesson.CreatedAt.Unix(),
		UpdatedAt:        lesson.UpdatedAt.Unix(),
		Attachments:      attachments,
	}
}

func (c *CourseService) domainAttachmentToProto(attachment *domain_course.Attachment) *coursev1.Attachment {
	return &coursev1.Attachment{
		Id:        attachment.ID.String(),
		LessonId:  attachment.LessonID.String(),
		Title:     attachment.Title,
		FileUrl:   attachment.FileURL,
		FileType:  attachment.FileType,
		FileSize:  int64Ptr(attachment.FileSize),
		CreatedAt: attachment.CreatedAt.Unix(),
	}
}

func (c *CourseService) domainEnrollmentToProto(enrollment *domain_course.Enrollment) *coursev1.Enrollment {
	return &coursev1.Enrollment{
		Id:          enrollment.ID.String(),
		CourseId:    enrollment.CourseID.String(),
		UserId:      enrollment.UserID.String(),
		Progress:    enrollment.Progress,
		EnrolledAt:  enrollment.EnrolledAt.Unix(),
		CompletedAt: timePtrToUnix(enrollment.CompletedAt),
		BundleId:    uuidPtrToString(enrollment.BundleID),
		CreatedAt:   enrollment.CreatedAt.Unix(),
		UpdatedAt:   enrollment.UpdatedAt.Unix(),
	}
}

func (c *CourseService) domainPricingToProto(pricing *domain_course.Pricing) *coursev1.Pricing {
	return &coursev1.Pricing{
		Id:                       pricing.ID.String(),
		CourseId:                 pricing.CourseID.String(),
		Type:                     string(pricing.Type),
		Amount:                   pricing.Amount,
		CurrencyCode:             pricing.CurrencyCode,
		SubscriptionDurationDays: int32Ptr(pricing.SubscriptionDurationDays),
		DiscountPrice:            float64Ptr(pricing.DiscountPrice),
		DiscountStartAt:          timePtrToUnix(pricing.DiscountStartAt),
		DiscountEndAt:            timePtrToUnix(pricing.DiscountEndAt),
		SubscriptionPlanId:       stringPtr(pricing.SubscriptionPlanID),
		IsActive:                 pricing.IsActive,
		CreatedAt:                pricing.CreatedAt.Unix(),
		UpdatedAt:                pricing.UpdatedAt.Unix(),
	}
}

// Helper functions
func stringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intPtr(i int) *int {
	return &i
}

func int32Ptr(i *int) int32 {
	if i == nil {
		return 0
	}
	return int32(*i)
}

func int64Ptr(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func float64Ptr(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func uuidPtrToString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func timePtrToUnix(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}
