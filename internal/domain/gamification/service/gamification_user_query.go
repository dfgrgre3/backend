package gamificationservice

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"
	"time"

	"gorm.io/gorm"
)

type GamificationQueryService struct {
}

func NewGamificationQueryService() *GamificationQueryService {
	return &GamificationQueryService{}
}

// readDBOrFallback dynamically retrieves the read DB connection.
func (s *GamificationQueryService) readDBOrFallback() *gorm.DB {
	return db.ReadDB()
}

type l1AchievementsEntry struct {
	entries   []UserAchievementReadModel
	expiresAt time.Time
}

var (
	l1AchievementsCache sync.Map
)

const (
	achievementsL1TTL    = time.Minute
	achievementsRedisTTL = 10 * time.Minute
)

func (s *GamificationQueryService) GetUserProgress(userID string) (*UserProgressReadModel, error) {
	rdb := s.readDBOrFallback()
	if rdb == nil {
		return NewDefaultUserProgress(userID), nil
	}

	var user models.User
	loadUser := func(conn *gorm.DB) error {
		return conn.
			Select("id", "total_xp", "level", "current_streak", "longest_streak", "total_study_time", "created_at", "updated_at").
			Where("id = ?", userID).
			First(&user).Error
	}

	if err := loadUser(rdb); err != nil {
		if err == gorm.ErrRecordNotFound && rdb != db.DB && db.DB != nil {
			err = loadUser(db.DB)
		}
		if err != nil {
			return nil, err
		}
	}

	achievements, err := s.GetUserAchievements(userID)
	if err != nil {
		return nil, err
	}

	achievementKeys := make([]string, 0, len(achievements))
	for _, achievement := range achievements {
		if achievement.Key != "" {
			achievementKeys = append(achievementKeys, achievement.Key)
		} else {
			achievementKeys = append(achievementKeys, achievement.ID)
		}
	}

	progress := &UserProgressReadModel{
		ID:             user.ID,
		UserID:         user.ID,
		TotalXP:        user.TotalXP,
		Level:          user.Level,
		CurrentStreak:  user.CurrentStreak,
		LongestStreak:  user.LongestStreak,
		TotalStudyTime: user.TotalStudyTime,
		Achievements:   achievementKeys,
		CustomGoals:    []any{},
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
	}
	progress.applyLevelThresholds()
	return progress, nil
}

func (s *GamificationQueryService) GetUserAchievements(userID string) ([]UserAchievementReadModel, error) {
	cacheKey := fmt.Sprintf("achievements:%s", userID)

	if achievements, ok := getAchievementsFromL1(userID); ok {
		return achievements, nil
	}
	if achievements, ok := getAchievementsFromRedis(userID, cacheKey); ok {
		return achievements, nil
	}

	rdb := s.readDBOrFallback()
	if rdb == nil {
		return nil, nil
	}

	var userAchievements []models.UserAchievement
	if err := rdb.Preload("Achievement").Where("user_id = ?", userID).Find(&userAchievements).Error; err != nil {
		return nil, err
	}

	achievements := buildUserAchievementEntries(userAchievements)
	cacheAchievements(userID, cacheKey, achievements)

	return achievements, nil
}

func getAchievementsFromL1(userID string) ([]UserAchievementReadModel, bool) {
	val, ok := l1AchievementsCache.Load(userID)
	if !ok {
		return nil, false
	}

	entry := val.(*l1AchievementsEntry)
	if time.Now().Before(entry.expiresAt) {
		return entry.entries, true
	}

	l1AchievementsCache.Delete(userID)
	return nil, false
}

func getAchievementsFromRedis(userID, cacheKey string) ([]UserAchievementReadModel, bool) {
	if cache.Redis == nil {
		return nil, false
	}

	redisCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	cachedVal, err := cache.Redis.Get(redisCtx, cacheKey).Result()
	cancel()
	if err != nil {
		return nil, false
	}

	var cachedAchievements []UserAchievementReadModel
	if json.Unmarshal([]byte(cachedVal), &cachedAchievements) != nil {
		return nil, false
	}

	storeAchievementsInL1(userID, cachedAchievements)
	return cachedAchievements, true
}

func buildUserAchievementEntries(userAchievements []models.UserAchievement) []UserAchievementReadModel {
	achievements := make([]UserAchievementReadModel, 0, len(userAchievements))
	for _, ua := range userAchievements {
		if ua.Achievement == nil {
			continue
		}

		achievements = append(achievements, UserAchievementReadModel{
			ID:          ua.Achievement.ID,
			Key:         ua.Achievement.Key,
			Title:       ua.Achievement.Title,
			Description: ua.Achievement.Description,
			Icon:        ua.Achievement.Icon,
			UnlockedAt:  ua.UnlockedAt,
			Rarity:      ua.Achievement.Rarity,
			XpReward:    ua.Achievement.XpReward,
		})
	}
	return achievements
}

func cacheAchievements(userID, cacheKey string, achievements []UserAchievementReadModel) {
	storeAchievementsInL1(userID, achievements)

	if cache.Redis == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if cacheBytes, err := json.Marshal(achievements); err == nil {
			cache.Redis.Set(ctx, cacheKey, cacheBytes, achievementsRedisTTL)
		}
	}()
}

func storeAchievementsInL1(userID string, achievements []UserAchievementReadModel) {
	l1AchievementsCache.Store(userID, &l1AchievementsEntry{
		entries:   achievements,
		expiresAt: time.Now().Add(achievementsL1TTL),
	})
}
