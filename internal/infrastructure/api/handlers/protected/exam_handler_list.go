package protected

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type l1ExamsEntry struct {
	data      gin.H
	expiresAt time.Time
}

var (
	l1ExamsCache sync.Map
)

func GetExams(c *gin.Context) {
	if db.ReadDB() == nil {
		log.Println("[GetExams] Critical: Database connection (db.ReadDB()) is nil")
		api_response.Error(c, http.StatusInternalServerError, "Internal Server Error: Database not initialized")
		return
	}

	page, limit := parseExamsPagination(c)
	search := c.Query("search")
	useCache := cache.Redis != nil && search == ""
	cacheKey := fmt.Sprintf("exams:list:page:%d:limit:%d", page, limit)

	if useCache {
		if tryL1ExamsCache(c, cacheKey) {
			return
		}
		if tryRedisExamsCache(c, cacheKey) {
			return
		}
	}

	query := buildExamsQuery(search)
	total, err := countExams(query)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to count exams")
		return
	}

	items, err := fetchExams(query, page, limit)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch exams")
		return
	}

	// NOTE: previously this fired a live admin WebSocket notification on
	// every single call to this read-only, cacheable listing endpoint
	// ("a user browsed the exams list") — meaning every admin's dashboard
	// got a notification for every page load by any student. Removed:
	// browsing a list is not an admin-actionable event, unlike SubmitExam's
	// "a student completed an exam" notification below, which is kept.

	responseData := buildExamsResponse(items, page, limit, total)
	updateExamsCache(useCache, cacheKey, responseData)

	api_response.Success(c, responseData)
}

func parseExamsPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	return page, limit
}

func tryL1ExamsCache(c *gin.Context, cacheKey string) bool {
	if val, ok := l1ExamsCache.Load(cacheKey); ok {
		entry := val.(*l1ExamsEntry)
		if time.Now().Before(entry.expiresAt) {
			api_response.Success(c, entry.data)
			return true
		}
		l1ExamsCache.Delete(cacheKey)
	}
	return false
}

func tryRedisExamsCache(c *gin.Context, cacheKey string) bool {
	cachedVal, err := cache.Redis.Get(c.Request.Context(), cacheKey).Result()
	if err != nil {
		return false
	}
	var cachedResponse gin.H
	if json.Unmarshal([]byte(cachedVal), &cachedResponse) != nil {
		return false
	}
	l1ExamsCache.Store(cacheKey, &l1ExamsEntry{
		data:      cachedResponse,
		expiresAt: time.Now().Add(15 * time.Second),
	})
	api_response.Success(c, cachedResponse)
	return true
}

func buildExamsQuery(search string) *gorm.DB {
	query := db.ReadDB().Model(&models.Exam{})
	if search != "" {
		query = query.Where("title ILIKE ?", "%"+search+"%")
	}
	return query
}

func countExams(query *gorm.DB) (int64, error) {
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		log.Printf("[GetExams] Error counting exams: %v", err)
		return 0, err
	}
	return total, nil
}

func fetchExams(query *gorm.DB, page, limit int) ([]models.Exam, error) {
	offset := (page - 1) * limit
	var exams []models.Exam
	if err := query.Preload("Subject").Offset(offset).Limit(limit).Find(&exams).Error; err != nil {
		log.Printf("[GetExams] Error fetching exams: %v", err)
		return nil, err
	}
	return exams, nil
}

func buildExamsResponse(exams []models.Exam, page, limit int, total int64) gin.H {
	countMap := getExamResultCounts(exams)
	items := formatExamResponse(exams, countMap)

	return gin.H{
		"items": items,
		"pagination": api_response.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: calculateTotalPages(total, limit),
		},
		"exams": items,
	}
}

func updateExamsCache(useCache bool, cacheKey string, responseData gin.H) {
	if !useCache {
		return
	}
	l1ExamsCache.Store(cacheKey, &l1ExamsEntry{
		data:      responseData,
		expiresAt: time.Now().Add(15 * time.Second),
	})
	go func(key string, data gin.H) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if cacheBytes, err := json.Marshal(data); err == nil {
			cache.Redis.Set(ctx, key, cacheBytes, 5*time.Minute)
		}
	}(cacheKey, responseData)
}

// getExamResultCounts fetches the number of results for each exam
func getExamResultCounts(exams []models.Exam) map[string]int64 {
	countMap := make(map[string]int64)
	if len(exams) == 0 {
		return countMap
	}

	// Collect exam IDs
	examIDs := make([]string, 0, len(exams))
	for _, e := range exams {
		if e.ID != "" {
			examIDs = append(examIDs, e.ID)
		}
	}

	if len(examIDs) == 0 {
		return countMap
	}

	type countResult struct {
		ExamID string `gorm:"column:exam_id"`
		Count  int64  `gorm:"column:count"`
	}
	var counts []countResult

	if err := db.DB.Model(&models.ExamResult{}).
		Select("exam_id, count(*) as count").
		Where("exam_id IN ?", examIDs).
		Group("exam_id").
		Scan(&counts).Error; err != nil {
		log.Printf("[getExamResultCounts] Warning: Error scanning exam result counts: %v", err)
	}

	for _, c := range counts {
		countMap[c.ExamID] = c.Count
	}
	return countMap
}

// formatExamResponse formats the exams for the frontend response
func formatExamResponse(exams []models.Exam, countMap map[string]int64) []gin.H {
	items := make([]gin.H, 0, len(exams))
	for _, exam := range exams {
		// Defensive subject access
		subjectData := gin.H{
			"id":     "",
			"name":   "عام",
			"nameAr": "عام",
		}
		if exam.Subject.ID != "" {
			subjectData = gin.H{
				"id":     exam.Subject.ID,
				"name":   exam.Subject.Name,
				"nameAr": exam.Subject.NameAr,
			}
		}

		items = append(items, gin.H{
			"id":            exam.ID,
			"title":         exam.Title,
			"description":   exam.Description,
			"duration":      exam.Duration,
			"questionCount": exam.QuestionCount,
			"difficulty":    exam.Difficulty,
			"isActive":      exam.IsActive,
			"year":          exam.CreatedAt.Year(),
			"createdAt":     exam.CreatedAt,
			"subject":       subjectData,
			"resultsCount":  countMap[exam.ID],
		})
	}
	return items
}
