package query

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// OptimizedEnrollmentCheck checks enrollment status efficiently
func (qo *QueryOptimizer) OptimizedEnrollmentCheck(ctx context.Context, userID, subjectID string) (bool, error) {
	var count int64
	err := qo.db.WithContext(ctx).
		Table("Enrollment").
		Where("user_id = ? AND subject_id = ?", userID, subjectID).
		Count(&count).Error
	return count > 0, err
}

// OptimizedPaymentCheck checks payment status efficiently
func (qo *QueryOptimizer) OptimizedPaymentCheck(ctx context.Context, userID, subjectID string) (bool, error) {
	var count int64
	err := qo.db.WithContext(ctx).
		Table("Payment").
		Where("user_id = ? AND subject_id = ? AND status = ?", userID, subjectID, "COMPLETED").
		Count(&count).Error
	return count > 0, err
}

// BatchEnrollmentCheck checks enrollment for multiple subjects at once
func (qo *QueryOptimizer) BatchEnrollmentCheck(ctx context.Context, userID string, subjectIDs []string) (map[string]bool, error) {
	if len(subjectIDs) == 0 {
		return map[string]bool{}, nil
	}

	type result struct {
		SubjectID string
	}
	var results []result

	err := qo.db.WithContext(ctx).
		Table("Enrollment").
		Select("subject_id").
		Where("user_id = ? AND subject_id IN ?", userID, subjectIDs).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	// Build result map — pre-allocate with full capacity
	enrolled := make(map[string]bool, len(subjectIDs))
	for _, id := range subjectIDs {
		enrolled[id] = false
	}
	for _, r := range results {
		enrolled[r.SubjectID] = true
	}

	return enrolled, nil
}

// QueryPerformanceLogger logs slow queries for monitoring
type QueryPerformanceLogger struct {
	threshold time.Duration
}

func NewQueryPerformanceLogger(threshold time.Duration) *QueryPerformanceLogger {
	return &QueryPerformanceLogger{threshold: threshold}
}

func (qpl *QueryPerformanceLogger) LogSlowQuery(query string, duration time.Duration, args ...interface{}) {
	if duration > qpl.threshold {
		log.Printf("[SLOW QUERY] Duration: %v, Query: %s, Args: %v", duration, query, args)
	}
}

// WithQueryLogging wraps a GORM DB with slow-query logging via GORM's callback system.
func WithQueryLogging(db *gorm.DB, threshold time.Duration) *gorm.DB {

	_ = db.Callback().Query().Before("gorm:query").Register("perf:before_query", func(tx *gorm.DB) {
		tx.Set("perf:start", time.Now())
	})

	_ = db.Callback().Query().After("gorm:query").Register("perf:after_query", func(tx *gorm.DB) {
		startVal, ok := tx.Get("perf:start")
		if !ok {
			return
		}
		start, ok := startVal.(time.Time)
		if !ok {
			return
		}
		elapsed := time.Since(start)
		if elapsed >= threshold {
			sql := tx.Statement.SQL.String()
			log.Printf("[SLOW QUERY] %.2fms — %s", float64(elapsed.Milliseconds()), sql)
		}
	})

	return db
}
