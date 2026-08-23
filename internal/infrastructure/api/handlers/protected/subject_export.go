package protected

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"time"

	"github.com/gin-gonic/gin"
)

// maxSubjectExportRows caps the export so a single request can never stream the
// whole table into memory.
const maxSubjectExportRows = 5000

var subjectExportHeader = []string{
	"id",
	"code",
	"name",
	"nameAr",
	"status",
	"isPublished",
	"isActive",
	"level",
	"price",
	"categoryId",
	"instructorId",
	"instructorName",
	"durationHours",
	"videoCount",
	"enrolledCount",
	"rating",
	"completionRate",
	"language",
	"slug",
	"createdAt",
	"publishedAt",
}

// ExportSubjectsCSV streams the filtered course list as a UTF-8 CSV file.
// It honours exactly the same query parameters as GetSubjects (including `ids`
// for exporting the current admin selection) so the file always matches what
// the admin sees on screen.
func ExportSubjectsCSV(c *gin.Context) {
	readDB, aborted := safeReadDB(c)
	if aborted {
		return
	}

	limit := maxSubjectExportRows
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed < maxSubjectExportRows {
			limit = parsed
		}
	}

	query := buildSubjectFilters(readDB.Model(&models.Subject{}), c)

	var subjects []models.Subject
	if err := query.Order(subjectSortClause(c.Query("sort"))).Limit(limit).Find(&subjects).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to export courses")
		return
	}

	filename := fmt.Sprintf("courses-export-%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	// BOM so Excel renders the Arabic columns correctly.
	if _, err := c.Writer.WriteString("\uFEFF"); err != nil {
		return
	}

	writer := csv.NewWriter(c.Writer)
	if err := writer.Write(subjectExportHeader); err != nil {
		return
	}

	for i := range subjects {
		if err := writer.Write(subjectExportRow(&subjects[i])); err != nil {
			return
		}
	}
	writer.Flush()
}

func subjectExportRow(s *models.Subject) []string {
	return []string{
		s.ID,
		derefString(s.Code),
		s.Name,
		derefString(s.NameAr),
		string(s.Status),
		strconv.FormatBool(s.IsPublished),
		strconv.FormatBool(s.IsActive),
		string(s.Level),
		s.Price.String(),
		derefString(s.CategoryId),
		derefString(s.InstructorId),
		derefString(s.InstructorName),
		strconv.Itoa(s.DurationHours),
		strconv.Itoa(s.VideoCount),
		strconv.Itoa(s.EnrolledCount),
		s.Rating.String(),
		s.CompletionRate.String(),
		s.Language,
		derefString(s.Slug),
		s.CreatedAt.Format(time.RFC3339),
		formatTimePtr(s.PublishedAt),
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func formatTimePtr(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.Format(time.RFC3339)
}
