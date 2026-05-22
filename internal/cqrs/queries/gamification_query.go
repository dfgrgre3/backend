package queries

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"time"

	"gorm.io/gorm"
)

type LeaderboardEntryReadModel struct {
	Rank    int    `json:"rank"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Avatar  string `json:"avatar"`
	TotalXP int    `json:"totalXP"`
	Level   int    `json:"level"`
	Role    string `json:"role"`
}

type UserAchievementReadModel struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	UnlockedAt  time.Time `json:"unlockedAt"`
	Rarity      string    `json:"rarity"`
	XpReward    int       `json:"xpReward"`
}

type UserProgressReadModel struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userId"`
	TotalXP        int       `json:"totalXP"`
	Level          int       `json:"level"`
	CurrentStreak  int       `json:"currentStreak"`
	LongestStreak  int       `json:"longestStreak"`
	TotalStudyTime int       `json:"totalStudyTime"`
	Achievements   []string  `json:"achievements"`
	CustomGoals    []any     `json:"customGoals"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type GamificationQueryService struct {
}

type l1LeaderboardEntry struct {
	entries   []LeaderboardEntryReadModel
	expiresAt time.Time
}

type l1AchievementsEntry struct {
	entries   []UserAchievementReadModel
	expiresAt time.Time
}

var (
	l1LeaderboardCache  sync.Map
	l1AchievementsCache sync.Map
)

const (
	leaderboardL1TTL     = time.Minute
	leaderboardRedisTTL  = 10 * time.Minute
	achievementsL1TTL    = time.Minute
	achievementsRedisTTL = 10 * time.Minute
)

func NewGamificationQueryService() *GamificationQueryService {
	return &GamificationQueryService{}
}

func NewDefaultUserProgress(userID string) *UserProgressReadModel {
	now := time.Now()
	return &UserProgressReadModel{
		ID:             userID,
		UserID:         userID,
		TotalXP:        0,
		Level:          1,
		CurrentStreak:  0,
		LongestStreak:  0,
		TotalStudyTime: 0,
		Achievements:   []string{},
		CustomGoals:    []any{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// readDBOrFallback dynamically retrieves the read DB connection.
func (s *GamificationQueryService) readDBOrFallback() *gorm.DB {
	return db.ReadDB()
}

func (s *GamificationQueryService) GetLeaderboard(limit int) ([]LeaderboardEntryReadModel, error) {
	cacheKey := fmt.Sprintf("leaderboard:%d", limit)

	if leaderboard, ok := getLeaderboardFromL1(limit); ok {
		return leaderboard, nil
	}
	if leaderboard, ok := getLeaderboardFromRedis(limit, cacheKey); ok {
		return leaderboard, nil
	}

	rdb := s.readDBOrFallback()
	if rdb == nil {
		return nil, nil
	}

	users, err := loadLeaderboardUsers(rdb, limit)
	if err != nil {
		return nil, err
	}

	leaderboard := buildLeaderboardEntries(users)
	cacheLeaderboard(limit, cacheKey, leaderboard)

	return leaderboard, nil
}

func getLeaderboardFromL1(limit int) ([]LeaderboardEntryReadModel, bool) {
	val, ok := l1LeaderboardCache.Load(limit)
	if !ok {
		return nil, false
	}

	entry := val.(*l1LeaderboardEntry)
	if time.Now().Before(entry.expiresAt) {
		return entry.entries, true
	}

	l1LeaderboardCache.Delete(limit)
	return nil, false
}

func getLeaderboardFromRedis(limit int, cacheKey string) ([]LeaderboardEntryReadModel, bool) {
	if db.Redis == nil {
		return nil, false
	}

	redisCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	cachedVal, err := db.Redis.Get(redisCtx, cacheKey).Result()
	cancel()
	if err != nil {
		return nil, false
	}

	var cachedLeaderboard []LeaderboardEntryReadModel
	if json.Unmarshal([]byte(cachedVal), &cachedLeaderboard) != nil {
		return nil, false
	}

	storeLeaderboardInL1(limit, cachedLeaderboard)
	return cachedLeaderboard, true
}

func loadLeaderboardUsers(rdb *gorm.DB, limit int) ([]models.User, error) {
	var users []models.User
	err := rdb.
		Select("id", "email", "name", "username", "avatar", "total_xp", "level", "role").
		Where("status = ?", models.StatusActive).
		Order("total_xp DESC").
		Limit(limit).
		Find(&users).Error
	if err != nil {
		return nil, err
	}

	if len(users) < limit {
		users = append(users, loadRemainingLeaderboardUsers(rdb, limit-len(users))...)
	}

	return users, nil
}

func loadRemainingLeaderboardUsers(rdb *gorm.DB, remainingCount int) []models.User {
	var remainingUsers []models.User
	rdb.
		Select("id", "email", "name", "username", "avatar", "total_xp", "level", "role").
		Where("status != ?", models.StatusActive).
		Order("total_xp DESC").
		Limit(remainingCount).
		Find(&remainingUsers)
	return remainingUsers
}

func buildLeaderboardEntries(users []models.User) []LeaderboardEntryReadModel {
	leaderboard := make([]LeaderboardEntryReadModel, 0, len(users))
	for i, u := range users {
		leaderboard = append(leaderboard, LeaderboardEntryReadModel{
			Rank:    i + 1,
			ID:      u.ID,
			Name:    leaderboardDisplayName(u),
			Avatar:  leaderboardAvatar(u),
			TotalXP: u.TotalXP,
			Level:   u.Level,
			Role:    string(u.Role),
		})
	}
	return leaderboard
}

func leaderboardDisplayName(user models.User) string {
	if user.Name != nil && *user.Name != "" {
		return *user.Name
	}
	if user.Username != nil && *user.Username != "" {
		return *user.Username
	}
	return user.Email
}

func leaderboardAvatar(user models.User) string {
	if user.Avatar == nil {
		return ""
	}
	return *user.Avatar
}

func cacheLeaderboard(limit int, cacheKey string, leaderboard []LeaderboardEntryReadModel) {
	storeLeaderboardInL1(limit, leaderboard)

	if db.Redis == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if cacheBytes, err := json.Marshal(leaderboard); err == nil {
			db.Redis.Set(ctx, cacheKey, cacheBytes, leaderboardRedisTTL)
		}
	}()
}

func storeLeaderboardInL1(limit int, leaderboard []LeaderboardEntryReadModel) {
	l1LeaderboardCache.Store(limit, &l1LeaderboardEntry{
		entries:   leaderboard,
		expiresAt: time.Now().Add(leaderboardL1TTL),
	})
}

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

	return &UserProgressReadModel{
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
	}, nil
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
	if db.Redis == nil {
		return nil, false
	}

	redisCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	cachedVal, err := db.Redis.Get(redisCtx, cacheKey).Result()
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

	if db.Redis != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if cacheBytes, err := json.Marshal(achievements); err == nil {
				db.Redis.Set(ctx, cacheKey, cacheBytes, achievementsRedisTTL)
			}
		}()
	}
}

func storeAchievementsInL1(userID string, achievements []UserAchievementReadModel) {
	l1AchievementsCache.Store(userID, &l1AchievementsEntry{
		entries:   achievements,
		expiresAt: time.Now().Add(achievementsL1TTL),
	})
}
