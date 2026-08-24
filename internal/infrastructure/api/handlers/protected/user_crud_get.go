package protected

import (
	"net/http"
	"thanawy-backend/internal/infrastructure/config"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

func GetGuestUser(c *gin.Context) {
	api_response.Success(c, gin.H{"id": "guest_" + config.Load().Environment})
}

func GetUserByID(c *gin.Context) {
	id := c.Param("id")

	user, err := getUserRepo().FindByID(id)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	api_response.Success(c, buildUserDetailsPayload(*user))
}
