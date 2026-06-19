package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetForumCategories returns all forum categories (public)
func GetForumCategories(c *gin.Context) {
	var cats []models.ForumCategory
	if err := db.DB.Order("\"order\" ASC, created_at DESC").Find(&cats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch forum categories"})
		return
	}
	c.JSON(http.StatusOK, cats)
}

// GetForumPosts returns forum topics/posts with pagination
func GetForumPosts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var topics []models.ForumTopic
	if err := db.DB.WithContext(c.Request.Context()).
		Preload("Author").
		Preload("Category").
		Order("is_pinned DESC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&topics).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch forum posts"})
		return
	}

	posts := make([]gin.H, 0, len(topics))
	for _, t := range topics {
		posts = append(posts, buildForumPostResponse(t))
	}

	c.JSON(http.StatusOK, posts)
}

// GetForumPost returns a single forum topic by ID (public)
func GetForumPost(c *gin.Context) {
	id := c.Param("id")
	var topic models.ForumTopic
	if err := db.DB.Preload("Author").Preload("Category").First(&topic, idQuery, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Forum post not found"})
		return
	}

	c.JSON(http.StatusOK, buildForumPostResponse(topic))
}

func CreateForumPost(c *gin.Context) {
	userID := c.GetString("userId")
	var input struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		CategoryID string `json:"categoryId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	input.CategoryID = strings.TrimSpace(input.CategoryID)
	if input.Title == "" || input.Content == "" || input.CategoryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title, content, and categoryId are required"})
		return
	}

	topic := models.ForumTopic{
		Title:      input.Title,
		Content:    input.Content,
		CategoryID: input.CategoryID,
		AuthorID:   userID,
	}
	if err := SafeCreate(db.DB, &topic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create forum post"})
		return
	}
	db.DB.Preload("Author").Preload("Category").First(&topic, idQuery, topic.ID)
	c.JSON(http.StatusCreated, buildForumPostResponse(topic))
}

func IncrementForumPostView(c *gin.Context) {
	if err := db.DB.WithContext(c.Request.Context()).Model(&models.ForumTopic{}).Where(idQuery, c.Param("id")).
		UpdateColumn("views", gorm.Expr("views + 1")).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update forum post view"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetForumPostReplies(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

func CreateForumPostReply(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"success": true})
}

func buildForumPostResponse(topic models.ForumTopic) gin.H {
	authorName := ""
	if topic.Author != nil && topic.Author.Name != nil {
		authorName = *topic.Author.Name
	}

	categoryName := ""
	if topic.Category != nil {
		categoryName = topic.Category.Name
	}

	return gin.H{
		"id":           topic.ID,
		"title":        topic.Title,
		"content":      topic.Content,
		"authorName":   authorName,
		"categoryId":   topic.CategoryID,
		"categoryName": categoryName,
		"createdAt":    topic.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"views":        topic.Views,
		"repliesCount": 0,
		"isPinned":     topic.IsPinned,
	}
}
