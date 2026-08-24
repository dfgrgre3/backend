package gamificationservice

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"gorm.io/gorm"
)

type l1LeaderboardEntry struct {
	entries   []LeaderboardEntryReadModel
	expiresAt time.Time
}

var (
	l1LeaderboardCache sync.Map
)

const (
	leaderboardL1TTL    = time.Minute
	leaderboardRedisTTL = 10 * time.Minute
)

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
	if cache.Redis == nil {
		return nil, false
	}

	redisCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	cachedVal, err := cache.Redis.Get(redisCtx, cacheKey).Result()
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

	if cache.Redis == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if cacheBytes, err := json.Marshal(leaderboard); err == nil {
			cache.Redis.Set(ctx, cacheKey, cacheBytes, leaderboardRedisTTL)
		}
	}()
}

func storeLeaderboardInL1(limit int, leaderboard []LeaderboardEntryReadModel) {
	l1LeaderboardCache.Store(limit, &l1LeaderboardEntry{
		entries:   leaderboard,
		expiresAt: time.Now().Add(leaderboardL1TTL),
	})
}
