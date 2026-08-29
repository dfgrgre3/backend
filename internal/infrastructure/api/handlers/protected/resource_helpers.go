package protected

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type resourceInput struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description *string  `json:"description"`
	URL         string   `json:"url"`
	Type        string   `json:"type"`
	Source      *string  `json:"source"`
	Free        *bool    `json:"free"`
	SubjectID   string   `json:"subjectId"`
	IDs         []string `json:"ids"`
}

type l1ResourceEntry struct {
	items     []gin.H
	expiresAt time.Time
}

var (
	l1ResourceCache sync.Map
	l1ResourceTTL   = 20 * time.Second
)

func InvalidateResourcesCache() {
	l1ResourceCache.Range(func(key, value interface{}) bool {
		l1ResourceCache.Delete(key)
		return true
	})

	if cache.Redis != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			iter := cache.Redis.Scan(ctx, 0, "resources:public:*", 100).Iterator()
			for iter.Next(ctx) {
				cache.Redis.Del(ctx, iter.Val())
			}
		}()
	}
}

type listResourcesParams struct {
	page         int
	limit        int
	subjectID    string
	resourceType string
	admin        bool
	cacheKey     string
}

func parseListResourcesParams(c *gin.Context, admin bool) listResourcesParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	subjectID := c.Query("subjectId")
	resourceType := c.Query("type")

	cacheKey := fmt.Sprintf("resources:public:page:%d:limit:%d:subject:%s:type:%s", page, limit, subjectID, resourceType)

	return listResourcesParams{
		page: page, limit: limit,
		subjectID: subjectID, resourceType: resourceType,
		admin: admin, cacheKey: cacheKey,
	}
}

func collectResourceIDs(input resourceInput) []string {
	ids := input.IDs
	if input.ID != "" {
		ids = append(ids, input.ID)
	}
	return ids
}

type resourceUpdates struct {
	Title       *string `gorm:"column:title"`
	Description *string `gorm:"column:description"`
	URL         *string `gorm:"column:url"`
	Type        *string `gorm:"column:type"`
	Source      *string `gorm:"column:source"`
	Free        *bool   `gorm:"column:free"`
	SubjectID   *string `gorm:"column:subject_id"`
}

type resourceUpdatesResult struct {
	structVal  resourceUpdates
	hasUpdates bool
}

func buildResourceUpdates(input resourceInput) resourceUpdatesResult {
	updates := resourceUpdates{}
	hasUpdates := false

	if input.Title != "" {
		updates.Title = &input.Title
		hasUpdates = true
	}
	if input.Description != nil {
		updates.Description = input.Description
		hasUpdates = true
	}
	if input.URL != "" {
		updates.URL = &input.URL
		hasUpdates = true
	}
	if input.Type != "" {
		updates.Type = &input.Type
		hasUpdates = true
	}
	if input.Source != nil {
		updates.Source = input.Source
		hasUpdates = true
	}
	if input.Free != nil {
		updates.Free = input.Free
		hasUpdates = true
	}
	if input.SubjectID != "" {
		updates.SubjectID = &input.SubjectID
		hasUpdates = true
	}

	return resourceUpdatesResult{structVal: updates, hasUpdates: hasUpdates}
}

func buildResourceQuery(params listResourcesParams) *gorm.DB {
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	var activeDB *gorm.DB
	if params.admin {
		activeDB = db.DB
	} else {
		activeDB = readDB
	}

	query := activeDB.Model(&models.Resource{}).Preload("Subject")
	if params.subjectID != "" {
		query = query.Where("subject_id = ?", params.subjectID)
	}
	if params.resourceType != "" && params.resourceType != "all" {
		query = query.Where("type = ?", params.resourceType)
	}
	if !params.admin {
		query = query.Where("free = ?", true)
	}
	return query
}

func formatResourceItem(resource models.Resource, admin bool) gin.H {
	subjectName := resource.Subject.Name
	if resource.Subject.NameAr != nil && *resource.Subject.NameAr != "" {
		subjectName = *resource.Subject.NameAr
	}

	item := gin.H{
		"id":          resource.ID,
		"title":       resource.Title,
		"description": resource.Description,
		"url":         resource.URL,
		"type":        resource.Type,
		"source":      resource.Source,
		"free":        resource.Free,
		"createdAt":   resource.CreatedAt,
		"subject":     subjectName,
		"subjectId":   resource.SubjectID,
		"subjectName": subjectName,
	}

	if admin {
		item["subject"] = gin.H{
			"id":     resource.Subject.ID,
			"name":   resource.Subject.Name,
			"nameAr": resource.Subject.NameAr,
			"color":  resource.Subject.Color,
		}
	}

	return item
}
