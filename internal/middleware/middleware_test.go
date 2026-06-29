package middleware

import "github.com/gin-gonic/gin"

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}
