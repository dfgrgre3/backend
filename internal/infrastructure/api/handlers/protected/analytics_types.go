package protected

import "time"

const startedAtGteQuery = "started_at >= ?"
const countDistinctUserQuery = "COUNT(DISTINCT user_id)"

// UserJourneyRequest represents a user journey tracking request
type UserJourneyRequest struct {
	UserID         string            `json:"userId" binding:"required"`
	SessionID      string            `json:"sessionId" binding:"required"`
	StartedAt      time.Time         `json:"startedAt" binding:"required"`
	EndedAt        *time.Time        `json:"endedAt,omitempty"`
	Steps          []UserJourneyStep `json:"steps" binding:"required"`
	TotalDuration  int64             `json:"totalDuration"`
	ConversionGoal string            `json:"conversionGoal,omitempty"`
	Completed      bool              `json:"completed"`
}

// UserJourneyStep represents a single step in the user journey
type UserJourneyStep struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"userId"`
	SessionID string                 `json:"sessionId"`
	Page      string                 `json:"page"`
	Action    string                 `json:"action"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  *int64                 `json:"duration,omitempty"`
}

// ConversionEventRequest represents a conversion event
type ConversionEventRequest struct {
	UserID       string    `json:"userId" binding:"required"`
	SessionID    string    `json:"sessionId" binding:"required"`
	Goal         string    `json:"goal" binding:"required"`
	Value        float64   `json:"value,omitempty"`
	Timestamp    time.Time `json:"timestamp" binding:"required"`
	JourneySteps int       `json:"journeySteps"`
}

// ActivityMetrics holds aggregated activity metrics
type ActivityMetrics struct {
	DailyActiveUsers       int64              `json:"dailyActiveUsers"`
	WeeklyActiveUsers      int64              `json:"weeklyActiveUsers"`
	MonthlyActiveUsers     int64              `json:"monthlyActiveUsers"`
	AverageSessionDuration float64            `json:"averageSessionDuration"`
	BounceRate             float64            `json:"bounceRate"`
	TopPages               []PageStats        `json:"topPages"`
	UserFlows              []FlowStats        `json:"userFlows"`
	ConversionRates        map[string]float64 `json:"conversionRates"`
}

// PageStats represents aggregated stats for a single page
type PageStats struct {
	Page           string  `json:"page"`
	Views          int64   `json:"views"`
	UniqueVisitors int64   `json:"uniqueVisitors"`
	AvgDuration    float64 `json:"avgDuration"`
}

// FlowStats represents a page-to-page user flow transition
type FlowStats struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int64  `json:"count"`
}
