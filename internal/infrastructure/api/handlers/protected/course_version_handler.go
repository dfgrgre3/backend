package protected

import (
	"fmt"
	"net/http"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// Course Version Endpoints
// =============================================================

// CreateCourseVersion creates a new version snapshot of the course
func (h *CourseRESTHandler) CreateCourseVersion(c *gin.Context) {
	id := c.Param("id")

	courseUUID, _ := uuid.Parse(id)
	versionNumber := 1
	version, err := h.courseService.CreateVersion(courseUUID, versionNumber)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create version: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"version": version})
}

// ListCourseVersions lists all versions of a course
func (h *CourseRESTHandler) ListCourseVersions(c *gin.Context) {
	id := c.Param("id")

	courseUUID, _ := uuid.Parse(id)
	versions, err := h.courseService.ListVersions(courseUUID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to list versions: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"versions": versions})
}

// RestoreCourseVersion restores a course to a specific version
func (h *CourseRESTHandler) RestoreCourseVersion(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		VersionNumber int `json:"versionNumber" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	courseUUID, _ := uuid.Parse(id)
	course, err := h.courseService.CloneCourse(courseUUID, fmt.Sprintf("restored-%d", req.VersionNumber))
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to restore version: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"course": course})
}

// GetCourseChangelog returns the changelog for a course
func (h *CourseRESTHandler) GetCourseChangelog(c *gin.Context) {
	id := c.Param("id")

	courseUUID, _ := uuid.Parse(id)
	logs, err := h.courseService.ListChangelogs(courseUUID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to get changelog: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"changelog": logs})
}

// =============================================================
// Admin Wrapper Functions
// =============================================================

// GetCourseVersions returns the changelog for a course (alias for GetCourseChangelog).
func GetCourseVersions(c *gin.Context) {
	// Create a temporary handler to use the method
	handler := &CourseRESTHandler{}
	handler.GetCourseChangelog(c)
}
