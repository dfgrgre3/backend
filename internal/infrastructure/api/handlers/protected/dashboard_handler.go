package protected

import (
	"log/slog"
	"net/http"

	"thanawy-backend/internal/application/cqrs/queries"
	apiresponse "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// Package-level query services backing the learner dashboard endpoints.
// Free functions are used to match the routing style used across the
// protected handlers package (see progress_handler.go & gamification_handler.go).
var (
	performanceQuery     = queries.NewPerformanceQueryService()
	predictionsQuery     = queries.NewPredictionsQueryService()
	recommendationsQuery = queries.NewRecommendationsQueryService()
	tipsQuery            = queries.NewTipsQueryService()
	courseProgressQuery  = queries.NewCourseProgressQueryService()
)

// --- Response DTOs ---

type PerformanceResponse struct {
	Metrics []queries.PerformanceMetricReadModel `json:"metrics"`
}

type PredictionsResponse struct {
	Predictions []queries.PredictionReadModel `json:"predictions"`
}

type RecommendationsResponse struct {
	Recommendations []queries.RecommendationReadModel `json:"recommendations"`
}

type TipsResponse struct {
	Tips []queries.TipReadModel `json:"tips"`
}

// CoursesProgressResponse uses the new summary model from the service layer.
// Fix #4: AveragePercent is now float64.
type CoursesProgressResponse struct {
	Courses        []queries.CourseProgressReadModel `json:"courses"`
	TotalCourses   int                               `json:"totalCourses"`
	Completed      int                               `json:"completed"`
	InProgress     int                               `json:"inProgress"`
	NotStarted     int                               `json:"notStarted"`     // Fix #2
	AveragePercent float64                           `json:"averagePercent"` // Fix #4
}

// --- HTTP Handlers ---

func GetPerformanceMetrics(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		apiresponse.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	metrics, err := performanceQuery.GetPerformanceMetrics(userID)
	if err != nil {
		slog.Error("failed to compute performance metrics", "userID", userID, "error", err)
		apiresponse.Error(c, mapErrorToHTTPStatus(err), "Failed to compute performance metrics")
		return
	}

	apiresponse.Success(c, PerformanceResponse{Metrics: metrics})
}

func GetPredictions(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		apiresponse.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	predictions, err := predictionsQuery.GetPredictions(userID)
	if err != nil {
		slog.Error("failed to compute predictions", "userID", userID, "error", err)
		apiresponse.Error(c, mapErrorToHTTPStatus(err), "Failed to compute predictions")
		return
	}

	apiresponse.Success(c, PredictionsResponse{Predictions: predictions})
}

func GetRecommendations(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		apiresponse.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	recommendations, err := recommendationsQuery.GetRecommendations(userID)
	if err != nil {
		slog.Error("failed to build recommendations", "userID", userID, "error", err)
		apiresponse.Error(c, mapErrorToHTTPStatus(err), "Failed to build recommendations")
		return
	}

	apiresponse.Success(c, RecommendationsResponse{Recommendations: recommendations})
}

func GetTips(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		apiresponse.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	tips, err := tipsQuery.GetTips(userID)
	if err != nil {
		slog.Error("failed to generate tips", "userID", userID, "error", err)
		apiresponse.Error(c, mapErrorToHTTPStatus(err), "Failed to generate tips")
		return
	}

	apiresponse.Success(c, TipsResponse{Tips: tips})
}

// GetUserCoursesProgress now delegates ALL business logic to the service layer. (Fix #5)
func GetUserCoursesProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		apiresponse.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	summary, err := courseProgressQuery.GetCourseProgressSummary(userID)
	if err != nil {
		slog.Error("failed to fetch course progress summary", "userID", userID, "error", err)
		apiresponse.Error(c, mapErrorToHTTPStatus(err), "Failed to fetch course progress")
		return
	}

	apiresponse.Success(c, CoursesProgressResponse{
		Courses:        summary.Courses,
		TotalCourses:   summary.TotalCourses,
		Completed:      summary.Completed,
		InProgress:     summary.InProgress,
		NotStarted:     summary.NotStarted,
		AveragePercent: summary.AveragePercent,
	})
}
