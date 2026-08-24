package protected

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	models "thanawy-backend/internal/domain/common"
	apiresponse "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

func ExportInstructors(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(maxExportLimit)))
	if limit < 1 {
		limit = 1
	}
	if limit > maxExportLimit {
		limit = maxExportLimit
	}

	search := strings.TrimSpace(c.Query("search"))
	statusParam := strings.TrimSpace(c.Query("status"))
	statusFilter := ""
	if statusParam != "" && !strings.EqualFold(statusParam, "all") {
		normalized, ok := validInstructorStatus(statusParam)
		if !ok {
			apiresponse.Error(c, http.StatusBadRequest, "invalid instructor status")
			return
		}
		statusFilter = normalized
	}

	query := instructorsBaseQuery(database, search)
	if statusFilter != "" {
		query = query.Where("instructor_status = ?", statusFilter)
	}

	var users []models.User
	if err := query.Order("created_at DESC").Limit(limit).Find(&users).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to export instructors")
		return
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{
		"id", "name", "email", "username", "status", "commission_rate", "phone", "country", "created_at",
	}); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to export instructors")
		return
	}

	for i := range users {
		u := users[i]
		commissionRate, _ := u.CommissionRate.Float64()
		record := []string{
			fmt.Sprintf("%v", u.ID),
			stringPtrToString(u.Name),
			u.Email,
			stringPtrToString(u.Username),
			getInstructorStatus(&u),
			strconv.FormatFloat(commissionRate, 'f', -1, 64),
			stringPtrToString(u.Phone),
			stringPtrToString(u.Country),
			fmt.Sprintf("%v", u.CreatedAt),
		}
		if err := writer.Write(record); err != nil {
			apiresponse.Error(c, http.StatusInternalServerError, "Failed to export instructors")
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to export instructors")
		return
	}

	filename := fmt.Sprintf("instructors-%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}
