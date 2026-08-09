package db

import (
	"thanawy-backend/internal/infrastructure/database/query"
)

// Compatibility facade over the database/query package.
// New code should import database/query directly.

// QueryOptimizer provides optimized query patterns for common operations.
type QueryOptimizer = query.QueryOptimizer

type SubjectFilters = query.SubjectFilters

type SubjectListItem = query.SubjectListItem

type SubjectDetail = query.SubjectDetail

type TopicWithSubTopics = query.TopicWithSubTopics

type SubTopicDetail = query.SubTopicDetail

type QueryPerformanceLogger = query.QueryPerformanceLogger

var (
	NewQueryOptimizer         = query.NewQueryOptimizer
	NewQueryPerformanceLogger = query.NewQueryPerformanceLogger
	WithQueryLogging          = query.WithQueryLogging
)
