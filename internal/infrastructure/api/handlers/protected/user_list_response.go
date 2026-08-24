package protected

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
)

func buildUserListItems(
	users []models.User,
	taskMap, sessionMap, achievementMap, enrollmentMap map[string]int64,
) []gin.H {
	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, gin.H{
			"id":                    user.ID,
			"email":                 user.Email,
			"name":                  user.Name,
			"username":              user.Username,
			"avatar":                user.Avatar,
			"phone":                 user.Phone,
			"phoneVerified":         user.PhoneVerified,
			"twoFactorEnabled":      user.TwoFactorEnabled,
			"role":                  user.Role,
			"status":                user.Status,
			"permissions":           user.GetEffectivePermissions(),
			"emailVerified":         user.EmailVerified,
			"country":               user.Country,
			"gradeLevel":            user.GradeLevel,
			"createdAt":             user.CreatedAt,
			"updatedAt":             user.UpdatedAt,
			"lastLogin":             user.LastLogin,
			"totalXP":               user.TotalXP,
			"level":                 user.Level,
			"currentStreak":         user.CurrentStreak,
			"activeSubscriptionId":  user.ActiveSubscriptionID,
			"subscriptionExpiresAt": user.SubscriptionExpiresAt,
			"_count": gin.H{
				"tasks":              taskMap[user.ID],
				"studySessions":      sessionMap[user.ID],
				"achievements":       achievementMap[user.ID],
				"subjectEnrollments": enrollmentMap[user.ID],
			},
		})
	}
	return items
}
