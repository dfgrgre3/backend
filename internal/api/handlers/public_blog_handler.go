package handlers

import (
	api_response "thanawy-backend/internal/api/response"

	"github.com/gin-gonic/gin"
)

func GetBlogCategories(c *gin.Context) {
	var categories []gin.H
	api_response.Success(c, categories)
}
