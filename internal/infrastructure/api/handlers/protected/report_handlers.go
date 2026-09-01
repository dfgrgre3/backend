package protected

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

const errReportNotFound = "Report not found"

// ReportWidget represents a single widget in a report
type ReportWidget struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type" binding:"required,oneof=table bar line pie area metric"`
	Title      string                 `json:"title" binding:"required"`
	DataSource string                 `json:"dataSource" binding:"required"`
	Metrics    []ReportMetric         `json:"metrics" binding:"required"`
	Dimensions []ReportDimension      `json:"dimensions,omitempty"`
	Filters    []ReportFilter         `json:"filters,omitempty"`
	Sort       *ReportSort            `json:"sort,omitempty"`
	Limit      int                    `json:"limit,omitempty"`
	Layout     map[string]interface{} `json:"layout" binding:"required"`
}

// ReportMetric represents a metric configuration
type ReportMetric struct {
	Name        string `json:"name" binding:"required"`
	Field       string `json:"field" binding:"required"`
	Aggregation string `json:"aggregation" binding:"required,oneof=count sum avg min max distinct_count"`
	Format      string `json:"format,omitempty" binding:"omitempty,oneof=number currency percentage date"`
}

// ReportDimension represents a dimension configuration
type ReportDimension struct {
	Field  string `json:"field" binding:"required"`
	Label  string `json:"label" binding:"required"`
	Format string `json:"format,omitempty" binding:"omitempty,oneof=date text number"`
}

// ReportFilter represents a filter condition
type ReportFilter struct {
	Field    string      `json:"field" binding:"required"`
	Operator string      `json:"operator" binding:"required,oneof=equals not_equals contains greater_than less_than between in"`
	Value    interface{} `json:"value" binding:"required"`
}

// ReportSort represents sort configuration
type ReportSort struct {
	Field     string `json:"field" binding:"required"`
	Direction string `json:"direction" binding:"required,oneof=asc desc"`
}

// CustomReportRequest represents a request to create/update a report
type CustomReportRequest struct {
	Name        string         `json:"name" binding:"required,max=100"`
	Description string         `json:"description" binding:"max=500"`
	Widgets     []ReportWidget `json:"widgets" binding:"required,min=1"`
	Filters     []ReportFilter `json:"filters,omitempty"`
	DateRange   *struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"dateRange,omitempty"`
	IsPublic bool `json:"isPublic"`
	Schedule *struct {
		Frequency string   `json:"frequency" binding:"required,oneof=daily weekly monthly"`
		EmailTo   []string `json:"emailTo" binding:"required,min=1"`
	} `json:"schedule,omitempty"`
}

// CreateCustomReport creates a new custom report
// @Summary Create custom report
// @Description Create a new custom report with widgets and filters
// @Tags admin,reports
// @Accept json
// @Produce json
// @Param request body CustomReportRequest true "Report configuration"
// @Success 201 {object} map[string]interface{}
// @Router /api/admin/reports [post]
func CreateCustomReport(c *gin.Context) {
	var req CustomReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	adminID, _ := c.Get("user_id")

	report := models.CustomReport{
		Name:        req.Name,
		Description: req.Description,
		Widgets:     widgetsToJSON(req.Widgets),
		Filters:     filtersToJSON(req.Filters),
		CreatedBy:   adminID.(string),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsPublic:    req.IsPublic,
	}

	if req.DateRange != nil {
		report.DateRangeFrom = &req.DateRange.From
		report.DateRangeTo = &req.DateRange.To
	}

	if req.Schedule != nil {
		report.ScheduleFrequency = req.Schedule.Frequency
		report.ScheduleEmailTo = req.Schedule.EmailTo
	}

	if err := SafeCreate(db.DB, &report); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create report")
		return
	}

	api_response.Success(c, gin.H{
		"message": "Report created successfully",
		"report":  report,
	})
}

// GetCustomReports returns all custom reports
// @Summary Get custom reports
// @Description Get all custom reports with optional filtering
// @Tags admin,reports
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/reports [get]
func GetCustomReports(c *gin.Context) {
	adminID, exists := c.Get("user_id")

	query := db.DB.Model(&models.CustomReport{})

	// Users can see their own reports + public reports
	if exists {
		query = query.Where("created_by = ? OR is_public = ?", adminID, true)
	}

	var reports []models.CustomReport
	if err := query.Order("updated_at DESC").Find(&reports).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch reports")
		return
	}

	api_response.Success(c, gin.H{
		"reports": reports,
	})
}

// GetCustomReport returns a single report
// @Summary Get custom report
// @Description Get a specific custom report by ID
// @Tags admin,reports
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/reports/{id} [get]
func GetCustomReport(c *gin.Context) {
	id := c.Param("id")

	var report models.CustomReport
	if err := db.DB.First(&report, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errReportNotFound)
		return
	}

	api_response.Success(c, gin.H{
		"report": report,
	})
}

// UpdateCustomReport updates a custom report
// @Summary Update custom report
// @Description Update an existing custom report
// @Tags admin,reports
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body CustomReportRequest true "Updated report configuration"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/reports/{id} [patch]
func UpdateCustomReport(c *gin.Context) {
	id := c.Param("id")

	var report models.CustomReport
	if err := db.DB.First(&report, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errReportNotFound)
		return
	}

	var req CustomReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Update fields
	report.Name = req.Name
	report.Description = req.Description
	report.Widgets = widgetsToJSON(req.Widgets)
	report.Filters = filtersToJSON(req.Filters)
	report.IsPublic = req.IsPublic
	report.UpdatedAt = time.Now()

	if req.DateRange != nil {
		report.DateRangeFrom = &req.DateRange.From
		report.DateRangeTo = &req.DateRange.To
	}

	if req.Schedule != nil {
		report.ScheduleFrequency = req.Schedule.Frequency
		report.ScheduleEmailTo = req.Schedule.EmailTo
	}

	if err := db.DB.Save(&report).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update report")
		return
	}

	api_response.Success(c, gin.H{
		"message": "Report updated successfully",
		"report":  report,
	})
}

// DeleteCustomReport deletes a custom report
// @Summary Delete custom report
// @Description Delete a custom report permanently
// @Tags admin,reports
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Success 200 {object} map[string]string
// @Router /api/admin/reports/{id} [delete]
func DeleteCustomReport(c *gin.Context) {
	id := c.Param("id")

	var report models.CustomReport
	if err := db.DB.First(&report, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errReportNotFound)
		return
	}

	if err := db.DB.Delete(&report).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete report")
		return
	}

	api_response.Success(c, gin.H{"message": "Report deleted successfully"})
}

// ExecuteCustomReport executes a custom report and returns results
// @Summary Execute custom report
// @Description Execute a custom report and get the results
// @Tags admin,reports
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/reports/{id}/execute [post]
func ExecuteCustomReport(c *gin.Context) {
	id := c.Param("id")

	var report models.CustomReport
	if err := db.DB.First(&report, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errReportNotFound)
		return
	}

	// Update last run time
	now := time.Now()
	report.LastRunAt = &now
	db.DB.Save(&report)

	// Execute each widget
	results := make(map[string]interface{})
	var widgets []ReportWidget
	if err := json.Unmarshal(report.Widgets, &widgets); err == nil {
		for _, widget := range widgets {
			data, summary, err := executeWidgetQuery(widget)
			if err != nil {
				results[widget.ID] = gin.H{"error": err.Error()}
				continue
			}
			results[widget.ID] = gin.H{
				"data":    data,
				"summary": summary,
			}
		}
	}

	api_response.Success(c, gin.H{
		"result": gin.H{
			"reportId":   report.ID,
			"executedAt": now,
			"results":    results,
		},
	})
}

// ExportCustomReport exports a report in various formats
// @Summary Export custom report
// @Description Export a custom report to PDF, Excel, or CSV
// @Tags admin,reports
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param format query string true "Export format (pdf|excel|csv)"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/reports/{id}/export [get]
func ExportCustomReport(c *gin.Context) {
	id := c.Param("id")
	format := c.Query("format")

	if format == "" {
		format = "csv"
	}

	var report models.CustomReport
	if err := db.DB.First(&report, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errReportNotFound)
		return
	}

	switch format {
	case "csv":
		exportToCSV(c, report)
	case "excel":
		exportToExcel(c, report)
	case "pdf":
		exportToPDF(c, report)
	default:
		api_response.Error(c, http.StatusBadRequest, "Invalid format")
	}
}

// ScheduleCustomReport schedules automatic report generation
// @Summary Schedule report
// @Description Schedule automatic report generation and email delivery
// @Tags admin,reports
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body map[string]interface{} true "Schedule configuration"
// @Success 200 {object} map[string]string
// @Router /api/admin/reports/{id}/schedule [post]
func ScheduleCustomReport(c *gin.Context) {
	id := c.Param("id")

	var report models.CustomReport
	if err := db.DB.First(&report, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errReportNotFound)
		return
	}

	var req struct {
		Frequency string   `json:"frequency" binding:"required,oneof=daily weekly monthly"`
		EmailTo   []string `json:"emailTo" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	report.ScheduleFrequency = req.Frequency
	report.ScheduleEmailTo = req.EmailTo

	if err := db.DB.Save(&report).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to schedule report")
		return
	}

	api_response.Success(c, gin.H{"message": "Report scheduled successfully"})
}

// Helper functions
func widgetsToJSON(widgets []ReportWidget) []byte {
	data, _ := json.Marshal(widgets)
	return data
}

func filtersToJSON(filters []ReportFilter) []byte {
	data, _ := json.Marshal(filters)
	return data
}

// reportDataSource describes one queryable table for the custom-report
// builder: its real table name and an allow-list of field name -> SQL
// column/expression. Only names present in this map may ever reach a query -
// widget.Metrics/Dimensions/Filters/Sort fields are looked up here and
// rejected otherwise, so no request-controlled string is ever concatenated
// into SQL as an identifier.
type reportDataSource struct {
	table   string
	columns map[string]string
}

var reportDataSources = map[string]reportDataSource{
	// Mirrors admin/src/hooks/use-report-builder.ts's ReportDataSource field lists,
	// mapped onto the real columns in internal/domain/common.
	"users": {
		table: `"User"`,
		columns: map[string]string{
			"id":         "id",
			"name":       "name",
			"email":      "email",
			"role":       "role",
			"created_at": "created_at",
			"last_login": "last_login",
			"is_active":  "(status = 'ACTIVE')",
		},
	},
	"exams": {
		table: `"Exam"`,
		columns: map[string]string{
			"id":         "id",
			"title":      "title",
			"subject_id": "subject_id",
			"duration":   "duration",
			"created_at": "created_at",
		},
	},
	"courses": {
		// "Course" in the admin UI is the Subject entity; enrollment/rating
		// counts are joined in rather than being columns on Subject itself.
		table: `"Subject" LEFT JOIN "SubjectEnrollment" se ON se.subject_id = "Subject".id AND se.deleted_at IS NULL LEFT JOIN "CourseReview" cr ON cr.subject_id = "Subject".id`,
		columns: map[string]string{
			"id":              `"Subject".id`,
			"title":           `"Subject".name`,
			"enrolled_count":  "se.id",
			"avg_rating":      "cr.rating",
			"completion_rate": "se.progress",
		},
	},
	"payments": {
		table: `"Payment"`,
		columns: map[string]string{
			"id":         "id",
			"amount":     "amount",
			"currency":   "currency",
			"status":     "status",
			"created_at": "created_at",
		},
	},
	"activity": {
		table: `"ActivityLog"`,
		columns: map[string]string{
			"user_id":   "user_id",
			"action":    "action",
			"page":      "resource",
			"timestamp": "created_at",
		},
	},
	"content": {
		table: `"BlogPost"`,
		columns: map[string]string{
			"id":         "id",
			"title":      "title",
			"views":      "views",
			"created_at": "created_at",
		},
	},
}

var reportAggregations = map[string]string{
	"count":          "COUNT",
	"sum":            "SUM",
	"avg":            "AVG",
	"min":            "MIN",
	"max":            "MAX",
	"distinct_count": "COUNT(DISTINCT %s)",
}

var reportFilterOperators = map[string]string{
	"equals":       "= ?",
	"not_equals":   "<> ?",
	"contains":     "ILIKE ?",
	"greater_than": "> ?",
	"less_than":    "< ?",
}

// executeWidgetQuery runs one report widget's aggregation against the real
// database, using only allow-listed table/column names from
// reportDataSources. Any field the widget references that isn't in the
// allow-list for its data source is rejected with an error rather than
// silently ignored or faked.
func executeWidgetQuery(widget ReportWidget) ([]map[string]interface{}, map[string]float64, error) {
	source, ok := reportDataSources[widget.DataSource]
	if !ok {
		return nil, nil, fmt.Errorf("unknown data source %q", widget.DataSource)
	}

	resolveColumn := func(field string) (string, error) {
		col, ok := source.columns[field]
		if !ok {
			return "", fmt.Errorf("field %q is not available on data source %q", field, widget.DataSource)
		}
		return col, nil
	}

	// Build SELECT list: dimensions as plain grouped columns, metrics as
	// aggregation(column) AS alias.
	var selects []string
	var groupBy []string
	metricAliases := make([]string, 0, len(widget.Metrics))

	for _, dim := range widget.Dimensions {
		col, err := resolveColumn(dim.Field)
		if err != nil {
			return nil, nil, err
		}
		alias := "dim_" + dim.Field
		selects = append(selects, fmt.Sprintf("%s AS %s", col, alias))
		groupBy = append(groupBy, col)
	}

	for _, m := range widget.Metrics {
		col, err := resolveColumn(m.Field)
		if err != nil {
			return nil, nil, err
		}
		aggTpl, ok := reportAggregations[m.Aggregation]
		if !ok {
			return nil, nil, fmt.Errorf("unknown aggregation %q", m.Aggregation)
		}
		var expr string
		if m.Aggregation == "distinct_count" {
			expr = fmt.Sprintf(aggTpl, col)
		} else {
			expr = fmt.Sprintf("%s(%s)", aggTpl, col)
		}
		alias := "metric_" + m.Name
		selects = append(selects, fmt.Sprintf("%s AS %s", expr, alias))
		metricAliases = append(metricAliases, alias)
	}

	if len(selects) == 0 {
		return nil, nil, fmt.Errorf("widget has no metrics or dimensions to query")
	}

	query := db.DB.Table(source.table)

	for _, f := range widget.Filters {
		col, err := resolveColumn(f.Field)
		if err != nil {
			return nil, nil, err
		}
		opTpl, ok := reportFilterOperators[f.Operator]
		if !ok {
			// "between" and "in" need multi-value handling; reject cleanly
			// rather than guess at the shape of f.Value.
			return nil, nil, fmt.Errorf("filter operator %q is not supported yet", f.Operator)
		}
		value := f.Value
		if f.Operator == "contains" {
			value = fmt.Sprintf("%%%v%%", f.Value)
		}
		query = query.Where(fmt.Sprintf("%s %s", col, opTpl), value)
	}

	query = query.Select(strings.Join(selects, ", "))
	if len(groupBy) > 0 {
		query = query.Group(strings.Join(groupBy, ", "))
	}

	if widget.Sort != nil {
		sortCol := "dim_" + widget.Sort.Field
		if _, isDim := source.columns[widget.Sort.Field]; !isDim {
			return nil, nil, fmt.Errorf("cannot sort by unknown field %q", widget.Sort.Field)
		}
		direction := "ASC"
		if widget.Sort.Direction == "desc" {
			direction = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", sortCol, direction))
	}

	limit := widget.Limit
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	query = query.Limit(limit)

	var rows []map[string]interface{}
	if err := query.Find(&rows).Error; err != nil {
		return nil, nil, fmt.Errorf("query failed: %w", err)
	}

	// data: relabel dim_/metric_ prefixed keys back to the widget's own
	// field/metric names so the frontend sees exactly what it asked for.
	data := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		out := make(map[string]interface{}, len(row))
		for _, dim := range widget.Dimensions {
			out[dim.Field] = row["dim_"+dim.Field]
		}
		for _, m := range widget.Metrics {
			out[m.Name] = row["metric_"+m.Name]
		}
		data = append(data, out)
	}

	// summary: same metrics aggregated over the whole filtered set, ignoring
	// dimension grouping, so the widget can show a total/avg alongside the
	// breakdown.
	summary := make(map[string]float64, len(metricAliases))
	if len(metricAliases) > 0 {
		summaryQuery := db.DB.Table(source.table)
		for _, f := range widget.Filters {
			col, _ := resolveColumn(f.Field) // already validated above
			opTpl := reportFilterOperators[f.Operator]
			value := f.Value
			if f.Operator == "contains" {
				value = fmt.Sprintf("%%%v%%", f.Value)
			}
			summaryQuery = summaryQuery.Where(fmt.Sprintf("%s %s", col, opTpl), value)
		}
		var summarySelects []string
		for _, m := range widget.Metrics {
			col, _ := resolveColumn(m.Field)
			aggTpl := reportAggregations[m.Aggregation]
			var expr string
			if m.Aggregation == "distinct_count" {
				expr = fmt.Sprintf(aggTpl, col)
			} else {
				expr = fmt.Sprintf("%s(%s)", aggTpl, col)
			}
			summarySelects = append(summarySelects, fmt.Sprintf("%s AS metric_%s", expr, m.Name))
		}
		var summaryRow map[string]interface{}
		if err := summaryQuery.Select(strings.Join(summarySelects, ", ")).Find(&summaryRow).Error; err == nil {
			for _, m := range widget.Metrics {
				if v, ok := summaryRow["metric_"+m.Name]; ok && v != nil {
					if f, ok := toFloat64(v); ok {
						summary[m.Name] = f
					}
				}
			}
		}
	}

	return data, summary, nil
}

// toFloat64 converts the numeric types the postgres driver may hand back for
// an aggregate (int64, float64, or a decimal-as-string/[]byte) into float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case []byte:
		f, err := strconv.ParseFloat(string(n), 64)
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func exportToCSV(c *gin.Context, report models.CustomReport) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", report.Name))
	c.String(http.StatusOK, "label,value\nItem 1,100\nItem 2,200\n")
}

func exportToExcel(c *gin.Context, _ models.CustomReport) {
	// In production, use a library like excelize to generate Excel files
	api_response.Error(c, http.StatusNotImplemented, "Excel export not yet implemented")
}

func exportToPDF(c *gin.Context, _ models.CustomReport) {
	// In production, use a library like gofpdf or headless Chrome to generate PDFs
	api_response.Error(c, http.StatusNotImplemented, "PDF export not yet implemented")
}
