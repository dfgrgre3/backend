package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/cache"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// Course Pricing Management
// =============================================================

// GetCoursePricing returns the pricing configuration for a course
func GetCoursePricing(c *gin.Context) {
	subjectID := c.Param("id")

	var pricing models.CoursePricing
	err := db.DB.Where("subject_id = ?", subjectID).First(&pricing).Error
	if err != nil {
		// Return default pricing if none exists
		pricing = models.CoursePricing{
			SubjectID:  subjectID,
			PricingType: models.PricingOneTime,
			Price:      0,
			Currency:   models.CurrencyEGP,
			IsActive:   true,
		}
	}

	api_response.Success(c, gin.H{"pricing": pricing})
}

// UpdateCoursePricing updates or creates the pricing configuration for a course
func UpdateCoursePricing(c *gin.Context) {
	subjectID := c.Param("id")

	var subject models.Subject
	if err := db.DB.Select("id").First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	var req models.UpdatePricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	// Validate pricing type
	if req.PricingType == "" {
		req.PricingType = models.PricingOneTime
	}
	if !isValidPricingType(req.PricingType) {
		api_response.Error(c, http.StatusBadRequest, "Invalid pricing type")
		return
	}

	// Validate currency
	if req.Currency == "" {
		req.Currency = models.CurrencyEGP
	}
	if !isValidCurrency(req.Currency) {
		api_response.Error(c, http.StatusBadRequest, "Invalid currency")
		return
	}

	// Check if pricing already exists
	var existing models.CoursePricing
	exists := db.DB.Where("subject_id = ?", subjectID).First(&existing).Error == nil

	if exists {
		// Update existing
		updates := map[string]interface{}{
			"pricing_type":   req.PricingType,
			"price":         req.Price,
			"currency":      req.Currency,
			"updated_at":    time.Now(),
		}
		if req.DiscountPrice != nil {
			updates["discount_price"] = *req.DiscountPrice
		}
		if req.DiscountStartAt != nil {
			updates["discount_start_at"] = *req.DiscountStartAt
		}
		if req.DiscountEndAt != nil {
			updates["discount_end_at"] = *req.DiscountEndAt
		}
		if req.SubscriptionPlanID != nil {
			updates["subscription_plan_id"] = *req.SubscriptionPlanID
		}

		if err := db.DB.Model(&models.CoursePricing{}).
			Where("subject_id = ?", subjectID).
			Updates(updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update pricing")
			return
		}
	} else {
		// Create new
		pricing := models.CoursePricing{
			ID:                 uuid.New().String(),
			SubjectID:          subjectID,
			PricingType:        req.PricingType,
			Price:              req.Price,
			Currency:           req.Currency,
			DiscountPrice:     req.DiscountPrice,
			DiscountStartAt:   req.DiscountStartAt,
			DiscountEndAt:     req.DiscountEndAt,
			SubscriptionPlanID: req.SubscriptionPlanID,
			IsActive:           true,
		}

		if err := db.DB.Create(&pricing).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to create pricing: "+err.Error())
			return
		}
		existing = pricing
	}

	// Invalidate cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subjectID)

	LogAudit(c, "UPDATE", "course_pricing", subjectID, req)
	api_response.Success(c, gin.H{"pricing": existing})
}

// =============================================================
// Course Bundle Management
// =============================================================

// ListBundles returns all active bundles
func ListBundles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.CourseBundle{}).
		Where("deleted_at IS NULL")

	if search != "" {
		query = query.Where("name ILIKE ? OR name_ar ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var bundles []models.CourseBundle
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&bundles).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch bundles")
		return
	}

	// Load courses for each bundle
	for i := range bundles {
		var courses []models.Subject
		db.DB.Where("id = ANY(COALESCE(?, '{}'))", bundles[i].CourseIDs).
			Limit(10).Find(&courses)
		bundles[i].Courses = courses
	}

	api_response.Success(c, gin.H{
		"bundles": bundles,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetBundle returns a single bundle by ID
func GetBundle(c *gin.Context) {
	bundleID := c.Param("id")

	var bundle models.CourseBundle
	if err := db.DB.Where("id = ? AND deleted_at IS NULL", bundleID).First(&bundle).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Bundle not found")
		return
	}

	// Load courses
	if len(bundle.CourseIDs) > 0 {
		var courses []models.Subject
		db.DB.Where("id = ANY(?)", bundle.CourseIDs).Find(&courses)
		bundle.Courses = courses
	}

	// Load bundle courses junction for sort order
	var junction []models.BundleCourse
	db.DB.Where("bundle_id = ?", bundleID).Order("sort_order ASC").Find(&junction)

	api_response.Success(c, gin.H{"bundle": bundle, "junction": junction})
}

// CreateBundle creates a new course bundle
func CreateBundle(c *gin.Context) {
	var req models.CreateBundleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if req.Currency == "" {
		req.Currency = models.CurrencyEGP
	}

	bundle := models.CourseBundle{
		ID:             uuid.New().String(),
		Name:           req.Name,
		NameAr:         req.NameAr,
		Description:    req.Description,
		DescriptionAr:  req.DescriptionAr,
		Price:          req.Price,
		Currency:       req.Currency,
		CourseIDs:      req.CourseIDs,
		ThumbnailUrl:   req.ThumbnailUrl,
		IsActive:       true,
		IsFeatured:     req.IsFeatured,
		TotalCourses:   len(req.CourseIDs),
	}

	if req.FeaturedUntil != nil {
		t, err := time.Parse(time.RFC3339, *req.FeaturedUntil)
		if err == nil {
			bundle.FeaturedUntil = &t
		}
	}

	if err := db.DB.Create(&bundle).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create bundle: "+err.Error())
		return
	}

	// Create junction records for courses
	for i, courseID := range req.CourseIDs {
		junction := models.BundleCourse{
			BundleID:  bundle.ID,
			CourseID:  courseID,
			SortOrder: i,
			AddedAt:   time.Now(),
		}
		db.DB.Create(&junction)
	}

	LogAudit(c, "CREATE", "course_bundle", bundle.ID, bundle)
	api_response.Created(c, gin.H{"bundle": bundle})
}

// UpdateBundle updates an existing bundle
func UpdateBundle(c *gin.Context) {
	bundleID := c.Param("id")

	var bundle models.CourseBundle
	if err := db.DB.Where("id = ? AND deleted_at IS NULL", bundleID).First(&bundle).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Bundle not found")
		return
	}

	var req models.UpdateBundleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	updates := map[string]interface{}{"updated_at": time.Now()}

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.NameAr != nil {
		updates["name_ar"] = *req.NameAr
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.DescriptionAr != nil {
		updates["description_ar"] = *req.DescriptionAr
	}
	if req.Price != nil {
		updates["price"] = *req.Price
	}
	if req.Currency != nil {
		updates["currency"] = *req.Currency
	}
	if req.DiscountPrice != nil {
		updates["discount_price"] = *req.DiscountPrice
	}
	if req.ThumbnailUrl != nil {
		updates["thumbnail_url"] = *req.ThumbnailUrl
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsFeatured != nil {
		updates["is_featured"] = *req.IsFeatured
	}
	if req.FeaturedUntil != nil {
		if *req.FeaturedUntil == "" {
			updates["featured_until"] = nil
		} else {
			t, err := time.Parse(time.RFC3339, *req.FeaturedUntil)
			if err == nil {
				updates["featured_until"] = &t
			}
		}
	}

	if err := db.DB.Model(&models.CourseBundle{}).
		Where("id = ?", bundleID).
		Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update bundle")
		return
	}

	db.DB.First(&bundle, "id = ?", bundleID)
	LogAudit(c, "UPDATE", "course_bundle", bundleID, req)
	api_response.Success(c, gin.H{"bundle": bundle})
}

// DeleteBundle soft-deletes a bundle
func DeleteBundle(c *gin.Context) {
	bundleID := c.Param("id")

	result := db.DB.Model(&models.CourseBundle{}).
		Where("id = ?", bundleID).
		Update("deleted_at", time.Now())

	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Bundle not found")
		return
	}

	LogAudit(c, "DELETE", "course_bundle", bundleID, nil)
	api_response.Success(c, gin.H{"message": "Bundle deleted successfully"})
}

// AddCoursesToBundle adds courses to an existing bundle
func AddCoursesToBundle(c *gin.Context) {
	bundleID := c.Param("id")

	var bundle models.CourseBundle
	if err := db.DB.Where("id = ? AND deleted_at IS NULL", bundleID).First(&bundle).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Bundle not found")
		return
	}

	var req models.AddBundleCoursesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	// Verify courses exist
	var count int64
	db.DB.Model(&models.Subject{}).Where("id IN ?", req.CourseIDs).Count(&count)
	if count != int64(len(req.CourseIDs)) {
		api_response.Error(c, http.StatusBadRequest, "One or more courses not found")
		return
	}

	// Get current max sort order
	var maxOrder int
	db.DB.Model(&models.BundleCourse{}).
		Where("bundle_id = ?", bundleID).
		Select("COALESCE(MAX(sort_order), -1)").
		Scan(&maxOrder)

	added := 0
	for _, courseID := range req.CourseIDs {
		// Check if already in bundle
		var existing int64
		db.DB.Model(&models.BundleCourse{}).
			Where("bundle_id = ? AND course_id = ?", bundleID, courseID).
			Count(&existing)
		if existing > 0 {
			continue
		}

		maxOrder++
		junction := models.BundleCourse{
			BundleID:  bundleID,
			CourseID:  courseID,
			SortOrder: maxOrder,
			AddedAt:   time.Now(),
		}
		if err := db.DB.Create(&junction).Error; err == nil {
			added++
		}
	}

	// Update bundle's course_ids array and total_courses
	var allCourses []string
	db.DB.Model(&models.BundleCourse{}).
		Where("bundle_id = ?", bundleID).
		Select("course_id").
		Scan(&allCourses)

	db.DB.Model(&bundle).Updates(map[string]interface{}{
		"course_ids":    allCourses,
		"total_courses": len(allCourses),
		"updated_at":    time.Now(),
	})

	LogAudit(c, "ADD_COURSES", "course_bundle", bundleID, gin.H{"added": added})
	api_response.Success(c, gin.H{
		"message": "Courses added successfully",
		"added":   added,
	})
}

// RemoveCoursesFromBundle removes courses from a bundle
func RemoveCoursesFromBundle(c *gin.Context) {
	bundleID := c.Param("id")

	var req struct {
		CourseIDs []string `json:"courseIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	result := db.DB.Where("bundle_id = ? AND course_id IN ?", bundleID, req.CourseIDs).
		Delete(&models.BundleCourse{})

	// Update bundle's course_ids array
	var allCourses []string
	db.DB.Model(&models.BundleCourse{}).
		Where("bundle_id = ?", bundleID).
		Select("course_id").
		Scan(&allCourses)

	db.DB.Model(&models.CourseBundle{}).
		Where("id = ?", bundleID).
		Updates(map[string]interface{}{
			"course_ids":    allCourses,
			"total_courses": len(allCourses),
			"updated_at":    time.Now(),
		})

	LogAudit(c, "REMOVE_COURSES", "course_bundle", bundleID, gin.H{"removed": result.RowsAffected})
	api_response.Success(c, gin.H{
		"message":  "Courses removed successfully",
		"removed": result.RowsAffected,
	})
}

// =============================================================
// Bundle Enrollments
// =============================================================

// GetBundleEnrollments returns enrollments for a specific bundle
func GetBundleEnrollments(c *gin.Context) {
	bundleID := c.Param("id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	db.DB.Model(&models.BundleEnrollment{}).
		Where("bundle_id = ?", bundleID).
		Count(&total)

	var enrollments []models.BundleEnrollment
	db.DB.Preload("User").
		Where("bundle_id = ?", bundleID).
		Order("enrolled_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&enrollments)

	api_response.Success(c, gin.H{
		"enrollments": enrollments,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// =============================================================
// Helper functions
// =============================================================

func isValidPricingType(t models.PricingType) bool {
	switch t {
	case models.PricingFree, models.PricingOneTime, models.PricingSubscription, models.PricingBundle:
		return true
	}
	return false
}

func isValidCurrency(cur models.Currency) bool {
	switch cur {
	case models.CurrencyEGP, models.CurrencyUSD, models.CurrencyEUR,
		models.CurrencySAR, models.CurrencyAED, models.CurrencyGBP:
		return true
	}
	return false
}

// MapCourseIDs maps subject IDs to bundle course IDs for frontend
func mapBundleCourses(bundle *models.CourseBundle, courses []models.Subject) {
	bundle.Courses = courses
	if len(courses) > 0 {
		ids := make([]string, len(courses))
		for i, c := range courses {
			ids[i] = c.ID
		}
		bundle.CourseIDs = ids
	}
}

// =============================================================
// Course Versioning
// =============================================================

// CreateCourseVersion creates a new version snapshot of a course
func CreateCourseVersion(c *gin.Context) {
	subjectID := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	var subject models.Subject
	if err := db.DB.Preload("Topics.SubTopics.Attachments").First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	var req struct {
		Version        string `json:"version" binding:"required"`
		ChangeSummary  string `json:"changeSummary"`
		ChangeSummaryAr string `json:"changeSummaryAr"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	// Get next version number
	var maxVersion int
	db.DB.Model(&models.CourseVersion{}).
		Where("subject_id = ?", subjectID).
		Select("COALESCE(MAX(version_number), 0)").
		Scan(&maxVersion)

	// Serialize curriculum
	curriculumJSON, _ := json.Marshal(subject.Topics)

	version := models.CourseVersion{
		ID:                 uuid.New().String(),
		SubjectID:          subjectID,
		Version:            req.Version,
		VersionNumber:      maxVersion + 1,
		ChangeSummary:      &req.ChangeSummary,
		ChangeSummaryAr:    &req.ChangeSummaryAr,
		CurriculumSnapshot: curriculumJSON,
		CreatedBy:          &uid,
		CreatedAt:          time.Now(),
	}

	if err := db.DB.Create(&version).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create version: "+err.Error())
		return
	}

	// Update subject's current version
	db.DB.Model(&subject).Updates(map[string]interface{}{
		"current_version": req.Version,
		"version_number":  maxVersion + 1,
	})

	LogAudit(c, "CREATE", "course_version", version.ID, version)
	api_response.Created(c, gin.H{"version": version})
}

// GetCourseVersions returns all versions for a course
func GetCourseVersions(c *gin.Context) {
	subjectID := c.Param("id")

	var versions []models.CourseVersion
	if err := db.DB.Where("subject_id = ?", subjectID).
		Order("version_number DESC").
		Find(&versions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch versions")
		return
	}

	api_response.Success(c, gin.H{"versions": versions})
}

// RestoreCourseVersion restores a course to a specific version
func RestoreCourseVersion(c *gin.Context) {
	subjectID := c.Param("id")
	versionID := c.Param("versionId")

	var version models.CourseVersion
	if err := db.DB.Where("id = ? AND subject_id = ?", versionID, subjectID).First(&version).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Version not found")
		return
	}

	if len(version.CurriculumSnapshot) == 0 {
		api_response.Error(c, http.StatusBadRequest, "No curriculum data in this version")
		return
	}

	var topics []models.Topic
	if err := json.Unmarshal(version.CurriculumSnapshot, &topics); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to parse curriculum")
		return
	}

	// This is a simplified restore - in production you'd want a full transaction
	// to handle the complete restoration with proper ID mapping
	api_response.Success(c, gin.H{
		"message":      "Version restore is not fully implemented - requires transaction-based ID remapping",
		"version":      version,
		"topic_count": len(topics),
	})
}
