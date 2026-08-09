package protected

import (
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

func GetBlogCategories(c *gin.Context) {
	var categories []gin.H
	api_response.Success(c, categories)
}
