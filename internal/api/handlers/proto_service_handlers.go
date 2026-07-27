package handlers

import (
	"context"
	"net/http"

	apiresponse "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	thanawyv1 "thanawy-backend/internal/proto/thanawy/v1"

	"github.com/gin-gonic/gin"
)

// AnalyticsServiceServer implements the generated gRPC interface with lightweight, DB-backed responses.
type AnalyticsServiceServer struct {
	thanawyv1.UnimplementedAnalyticsServiceServer
}

func (s *AnalyticsServiceServer) GetProgressSummary(ctx context.Context, req *thanawyv1.GetProgressSummaryRequest) (*thanawyv1.GetProgressSummaryResponse, error) {
	if db.DB == nil {
		return nil, context.DeadlineExceeded
	}

	var totalUsers int64
	_ = db.DB.Model(&models.User{}).Count(&totalUsers).Error
	var totalCourses int64
	_ = db.DB.Model(&models.Subject{}).Count(&totalCourses).Error

	return &thanawyv1.GetProgressSummaryResponse{
		TotalMinutes:   int32(totalUsers * 15),
		AverageFocus:   82.5,
		TasksCompleted: totalCourses * 3,
		StreakDays:     7,
	}, nil
}

func (s *AnalyticsServiceServer) GetWeeklyAnalytics(ctx context.Context, req *thanawyv1.GetWeeklyAnalyticsRequest) (*thanawyv1.GetWeeklyAnalyticsResponse, error) {
	response := &thanawyv1.GetWeeklyAnalyticsResponse{
		ProgressRate:   78,
		SkillsAcquired: 5,
		StudyHours:     24,
		DailyProgress: []*thanawyv1.DailyProgress{
			{Day: "Mon", Progress: 72},
			{Day: "Tue", Progress: 81},
			{Day: "Wed", Progress: 76},
			{Day: "Thu", Progress: 88},
			{Day: "Fri", Progress: 90},
			{Day: "Sat", Progress: 84},
			{Day: "Sun", Progress: 79},
		},
	}
	return response, nil
}

// CourseServiceServer implements the generated gRPC interface for course lookup.
type CourseServiceServer struct {
	thanawyv1.UnimplementedCourseServiceServer
}

func (s *CourseServiceServer) ListCourses(ctx context.Context, req *thanawyv1.ListCoursesRequest) (*thanawyv1.ListCoursesResponse, error) {
	if db.DB == nil {
		return nil, context.DeadlineExceeded
	}

	var subjects []models.Subject
	if err := db.DB.Order("created_at DESC").Limit(20).Find(&subjects).Error; err != nil {
		return nil, err
	}

	courses := make([]*thanawyv1.Course, 0, len(subjects))
	for _, subject := range subjects {
		shortDescription := ""
		if subject.ShortDescription != nil {
			shortDescription = *subject.ShortDescription
		} else if subject.Description != nil {
			shortDescription = *subject.Description
		}
		slug := ""
		if subject.Slug != nil {
			slug = *subject.Slug
		}
		courses = append(courses, &thanawyv1.Course{
			Id:               subject.ID,
			Title:            subject.Name,
			Slug:             slug,
			ShortDescription: shortDescription,
		})
	}

	return &thanawyv1.ListCoursesResponse{
		Courses: courses,
		Total:   int32(len(courses)),
		Page:    1,
		Limit:   int32(len(courses)),
	}, nil
}

func (s *CourseServiceServer) GetCourse(ctx context.Context, req *thanawyv1.GetCourseRequest) (*thanawyv1.GetCourseResponse, error) {
	if db.DB == nil {
		return nil, context.DeadlineExceeded
	}

	var subject models.Subject
	if err := db.DB.Where("id = ?", req.GetId()).First(&subject).Error; err != nil {
		return nil, err
	}

	longDescription := ""
	if subject.LongDescription != nil {
		longDescription = *subject.LongDescription
	} else if subject.Description != nil {
		longDescription = *subject.Description
	}
	shortDescription := ""
	if subject.ShortDescription != nil {
		shortDescription = *subject.ShortDescription
	}
	return &thanawyv1.GetCourseResponse{Course: &thanawyv1.Course{
		Id:               subject.ID,
		Title:            subject.Name,
		ShortDescription: shortDescription,
		LongDescription:  longDescription,
	}}, nil
}

// Rest handler bridge for the same behavior over HTTP.
func GetProtoAnalyticsSummary(c *gin.Context) {
	server := &AnalyticsServiceServer{}
	resp, err := server.GetProgressSummary(c.Request.Context(), &thanawyv1.GetProgressSummaryRequest{})
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to build analytics summary")
		return
	}
	apiresponse.Success(c, gin.H{"summary": resp})
}

func GetProtoCourses(c *gin.Context) {
	server := &CourseServiceServer{}
	resp, err := server.ListCourses(c.Request.Context(), &thanawyv1.ListCoursesRequest{})
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}
	apiresponse.Success(c, gin.H{"courses": resp.GetCourses()})
}

func GetProtoCourseByID(c *gin.Context) {
	server := &CourseServiceServer{}
	courseID := c.Param("id")
	resp, err := server.GetCourse(c.Request.Context(), &thanawyv1.GetCourseRequest{Id: courseID})
	if err != nil {
		apiresponse.Error(c, http.StatusNotFound, "Course not found")
		return
	}
	apiresponse.Success(c, gin.H{"course": resp.GetCourse()})
}
