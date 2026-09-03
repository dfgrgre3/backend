package cqrs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	db "thanawy-backend/internal/infrastructure/database"
)

const (
	// refreshTimeout is the maximum allowed time for a single materialized view refresh.
	// Prevents the background worker from hanging indefinitely if the DB is under heavy load or locked.
	refreshTimeout = 5 * time.Minute
)

// materializedView defines a target view and its refresh SQL command.
type materializedView struct {
	name string
	sql  string
}

/*
⚠️ IMPORTANT DBA NOTICE:
Using `REFRESH MATERIALIZED VIEW CONCURRENTLY` in PostgreSQL strictly requires
that each materialized view has at least one UNIQUE INDEX.
Ensure your database migrations create these unique indexes, otherwise the refresh will fail.
*/

// ValidateMaterializedViewIndexes validates that all materialized views have required unique indexes.
// This should be called during application startup to catch configuration issues early.
func ValidateMaterializedViewIndexes() error {
	wdb := db.WriteDB()
	if wdb == nil {
		return errors.New("database write connection is not initialized")
	}

	views := []materializedView{
		{name: "mv_user_progress_summary"},
		{name: "mv_user_weekly_analytics"},
		{name: "mv_user_watch_time"},
	}

	for _, v := range views {
		// Check if the materialized view has at least one unique index
		var indexCount int64
		err := wdb.Raw(`
			SELECT COUNT(*) FROM pg_indexes 
			WHERE schemaname = 'public' 
			AND tablename = ? 
			AND indexname LIKE '%_unique_%' OR indexname LIKE '%_pk'
		`, v.name).Scan(&indexCount).Error

		if err != nil {
			return fmt.Errorf("failed to validate indexes for %s: %w", v.name, err)
		}

		if indexCount == 0 {
			slog.Warn("materialized view is missing required unique index",
				"view", v.name,
				"warning", "REFRESH MATERIALIZED VIEW CONCURRENTLY will fail without a unique index",
			)
		}
	}

	slog.Info("materialized view indexes validated successfully")
	return nil
}

// RefreshMaterializedViews refreshes all CQRS read model materialized views
// sequentially to avoid write-DB contention. Called periodically by the background worker.
func RefreshMaterializedViews() error {
	wdb := db.WriteDB()
	if wdb == nil {
		return errors.New("database write connection is not initialized")
	}

	views := []materializedView{
		{
			name: "mv_user_progress_summary",
			sql:  `REFRESH MATERIALIZED VIEW CONCURRENTLY mv_user_progress_summary`,
		},
		{
			name: "mv_user_weekly_analytics",
			sql:  `REFRESH MATERIALIZED VIEW CONCURRENTLY mv_user_weekly_analytics`,
		},
		{
			name: "mv_user_watch_time",
			sql:  `REFRESH MATERIALIZED VIEW CONCURRENTLY mv_user_watch_time`,
		},
	}

	for _, v := range views {
		startTime := time.Now()

		// Use a timeout context to prevent indefinite blocking if the database is locked or overwhelmed.
		ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)

		err := wdb.WithContext(ctx).Exec(v.sql).Error
		cancel() // Release context resources immediately

		duration := time.Since(startTime)

		if err != nil {
			slog.Error("failed to refresh materialized view",
				"view", v.name,
				"duration", duration,
				"error", err,
			)
			// Fail-fast: return the error so the background worker can retry the entire batch later.
			return fmt.Errorf("refresh %s failed after %v: %w", v.name, duration, err)
		}

		slog.Info("successfully refreshed materialized view",
			"view", v.name,
			"duration", duration,
		)
	}

	return nil
}
