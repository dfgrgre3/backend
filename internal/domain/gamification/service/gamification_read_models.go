package gamificationservice

import "time"

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

	// Level thresholds, so clients never have to guess the XP curve.
	CurrentLevelXP int `json:"currentLevelXP"`
	NextLevelXP    int `json:"nextLevelXP"`
	XPIntoLevel    int `json:"xpIntoLevel"`
	XPToNextLevel  int `json:"xpToNextLevel"`
}

// xpThresholdForLevel returns the cumulative XP required to reach a level.
// The curve is quadratic: level n starts at 500 * (n-1) * n.
func xpThresholdForLevel(level int) int {
	if level <= 1 {
		return 0
	}
	return 500 * (level - 1) * level
}

// applyLevelThresholds fills the XP-curve fields from the learner's total XP.
func (m *UserProgressReadModel) applyLevelThresholds() {
	if m.Level < 1 {
		m.Level = 1
	}

	m.CurrentLevelXP = xpThresholdForLevel(m.Level)
	m.NextLevelXP = xpThresholdForLevel(m.Level + 1)

	m.XPIntoLevel = m.TotalXP - m.CurrentLevelXP
	if m.XPIntoLevel < 0 {
		m.XPIntoLevel = 0
	}

	m.XPToNextLevel = m.NextLevelXP - m.TotalXP
	if m.XPToNextLevel < 0 {
		m.XPToNextLevel = 0
	}
}

func NewDefaultUserProgress(userID string) *UserProgressReadModel {
	now := time.Now()
	progress := &UserProgressReadModel{
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
	progress.applyLevelThresholds()
	return progress
}
