package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetBlogCategories(c *gin.Context) {
	var categories []gin.H
	c.JSON(http.StatusOK, categories)
}
