package response

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    data,
	})
}

func Message(c *gin.Context, status int, message string, data interface{}) {
	payload := gin.H{
		"success": status < 400,
		"message": message,
	}
	if data != nil {
		payload["data"] = data
	}
	c.JSON(status, payload)
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error":   message,
	})
}

// isDevelopment mirrors the same env-var check used elsewhere in this repo
// (secrets_validator.go, csrf_protection.go) to decide dev-only behavior.
// Defaults to "not development" (the safe side) unless explicitly opted in,
// so an unset/misconfigured environment never accidentally leaks details.
func isDevelopment() bool {
	env := os.Getenv("NODE_ENV")
	if env == "" {
		env = os.Getenv("GO_ENV")
	}
	return env == "development"
}

// ErrorDetail responds with publicMessage to the client and always logs the
// full err server-side. In development, err's text is appended to the
// response too (matching this repo's long-standing "Failed to X: <err>"
// convention, useful for local debugging); in every other environment the
// client only ever sees publicMessage, since err may contain raw database
// error text (column/constraint names, driver internals) that shouldn't
// reach an API client in production.
func ErrorDetail(c *gin.Context, status int, publicMessage string, err error) {
	if err != nil {
		log.Printf("[%s %s] %s: %v", c.Request.Method, c.FullPath(), publicMessage, err)
	}

	message := publicMessage
	if isDevelopment() && err != nil {
		message = publicMessage + ": " + err.Error()
	}

	Error(c, status, message)
}

func List(c *gin.Context, items interface{}, pagination Pagination, aliases gin.H) {
	data := gin.H{
		"items":      items,
		"pagination": pagination,
	}
	for key, value := range aliases {
		data[key] = value
	}

	Success(c, data)
}

// AdminList responds with admin-specific list format including stats
func AdminList(c *gin.Context, items interface{}, pagination Pagination, stats gin.H) {
	data := gin.H{
		"items":      items,
		"pagination": pagination,
		"stats":      stats,
	}

	Success(c, data)
}
