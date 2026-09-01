package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

// =============================================================
// Constants & Task Types
// =============================================================

const (
	// TypeServiceHealthCheck is the task type for the periodic service-health
	// probe that persists a real historical record instead of only answering
	// the live dashboard request.
	TypeServiceHealthCheck = "system:health_check"
)

// ServiceHealthSnapshotPersister runs every known service probe and stores
// one row per service into service_health_checks.
//
// The actual probes (database, cache, storage, search, queue, scheduler,
// api) live in the admin handlers package, which already imports this
// package (for exam generation, etc.) — importing it back here would create
// an import cycle. admin.init() sets this variable instead, so the dependency
// runs handlers -> workers only, the same direction as everywhere else.
var ServiceHealthSnapshotPersister func(ctx context.Context) error

// HealthCheckHandler processes TypeServiceHealthCheck tasks.
type HealthCheckHandler struct{}

// ProcessTask runs every known service probe and stores one row per service
// in service_health_checks. Failures are logged, not retried aggressively —
// a missed minute is not worth asynq's retry/backoff noise since the next
// scheduled run is one minute away regardless.
func (h *HealthCheckHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if ServiceHealthSnapshotPersister == nil {
		return fmt.Errorf("service health snapshot persister is not registered")
	}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := ServiceHealthSnapshotPersister(probeCtx); err != nil {
		log.Printf("[HealthScheduler] Failed to persist service health snapshot: %v", err)
		return err
	}
	return nil
}
