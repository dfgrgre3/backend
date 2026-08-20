package protected

import (
	"context"
	"encoding/json"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetHomepageData(c *gin.Context) {
	const cacheTTL = 5 * time.Minute
	cacheKey := "homepage:data:v1"

	if cache.Redis != nil {
		cached, err := cache.Redis.Get(c.Request.Context(), cacheKey).Result()
		if err == nil {
			var cachedResponse gin.H
			if json.Unmarshal([]byte(cached), &cachedResponse) == nil {
				api_response.Success(c, cachedResponse)
				return
			}
		}
	}

	if _, aborted := safeReadDB(c); aborted {
		return
	}

	ctx := c.Request.Context()
	// freshDB returns a fully independent GORM session from the root DB for
	// every query. This prevents clause/statement leakage when switching between
	// different models (e.g. Subject → User) in the same handler.
	freshDB := func() *gorm.DB { return db.ReadDB(ctx) }

	// Fetch popular courses (limit 8)
	var popularCourses []models.Subject
	if err := freshDB().Model(&models.Subject{}).
		Where("is_published = ? AND is_active = ?", true, true).
		Order("enrolled_count DESC").
		Order("rating DESC").
		Limit(8).
		Find(&popularCourses).Error; err != nil {
		popularCourses = []models.Subject{}
	}

	// Fetch featured courses (limit 6)
	var featuredCourses []models.Subject
	if err := freshDB().Model(&models.Subject{}).
		Where("is_published = ? AND is_active = ? AND is_featured = ?", true, true, true).
		Order("created_at DESC").
		Limit(6).
		Find(&featuredCourses).Error; err != nil {
		featuredCourses = []models.Subject{}
	}

	// Fetch new courses (limit 6)
	var newCourses []models.Subject
	if err := freshDB().Model(&models.Subject{}).
		Where("is_published = ? AND is_active = ? AND is_new = ?", true, true, true).
		Order("created_at DESC").
		Limit(6).
		Find(&newCourses).Error; err != nil {
		newCourses = []models.Subject{}
	}

	// Fetch system stats
	var totalCourses int64
	freshDB().Model(&models.Subject{}).Where("is_published = ? AND is_active = ?", true, true).Count(&totalCourses)

	var totalStudents int64
	freshDB().Model(&models.User{}).Where("role = ?", models.RoleStudent).Count(&totalStudents)

	var totalTeachers int64
	freshDB().Model(&models.User{}).Where("role = ?", models.RoleTeacher).Count(&totalTeachers)

	var totalEnrollments int64
	freshDB().Model(&models.Enrollment{}).Count(&totalEnrollments)

	// Fetch featured teachers (limit 4)
	var featuredTeachers []models.User
	if err := freshDB().Model(&models.User{}).
		Where("role = ? AND status = ?", models.RoleTeacher, models.StatusActive).
		Order("total_xp DESC").
		Limit(4).
		Find(&featuredTeachers).Error; err != nil {
		featuredTeachers = []models.User{}
	}

	// Build topic counts for all courses
	allCourseIDs := make([]string, 0, len(popularCourses)+len(featuredCourses)+len(newCourses))
	for _, s := range popularCourses {
		allCourseIDs = append(allCourseIDs, s.ID)
	}
	for _, s := range featuredCourses {
		allCourseIDs = append(allCourseIDs, s.ID)
	}
	for _, s := range newCourses {
		allCourseIDs = append(allCourseIDs, s.ID)
	}

	topicCountMap := fetchTopicCounts(c.Request.Context(), allCourseIDs)

	// Format course responses
	popularItems := buildSubjectListResponse(popularCourses, topicCountMap)
	featuredItems := buildSubjectListResponse(featuredCourses, topicCountMap)
	newItems := buildSubjectListResponse(newCourses, topicCountMap)

	// Format teacher responses
	teacherItems := make([]gin.H, 0, len(featuredTeachers))
	for _, t := range featuredTeachers {
		teacherItems = append(teacherItems, gin.H{
			"id":          t.ID,
			"name":        t.GetName(),
			"bio":         stringPtrToString(t.Bio),
			"avatar":      stringPtrToString(t.Avatar),
			"totalXP":     t.TotalXP,
			"specialties": []string{},
		})
	}

	response := gin.H{
		"popularCourses": gin.H{
			"items": popularItems,
			"total": len(popularItems),
		},
		"featuredCourses": gin.H{
			"items": featuredItems,
			"total": len(featuredItems),
		},
		"newCourses": gin.H{
			"items": newItems,
			"total": len(newItems),
		},
		"stats": gin.H{
			"totalCourses":     totalCourses,
			"totalStudents":    totalStudents,
			"totalTeachers":    totalTeachers,
			"totalEnrollments": totalEnrollments,
		},
		"featuredTeachers": gin.H{
			"items": teacherItems,
			"total": len(teacherItems),
		},
	}

	if cache.Redis != nil {
		data, _ := json.Marshal(response)
		go func(key string, payload []byte) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cache.Redis.Set(ctx, key, payload, cacheTTL)
		}(cacheKey, data)
	}

	api_response.Success(c, response)
}
