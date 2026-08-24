package bootstrap

import (
	"log"
	"thanawy-backend/internal/application/services"
	systemservice "thanawy-backend/internal/domain/system/service"
	"thanawy-backend/internal/infrastructure/cache"
	worker "thanawy-backend/internal/infrastructure/workers"
)

// startWorkers starts the background mail queue worker, the automated
// database backup scheduler, and (if Redis is connected) the Asynq worker
// pool and the analytics batch worker. No-op if a.runWorkers is false.
func (a *app) startWorkers() {
	if !a.runWorkers {
		return
	}

	// Start Database-backed Mail Queue Worker
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		log.Println("Starting background mail queue worker...")
		services.GetMailQueueWorker().Start()
	}()

	// Start Automated Database Backup Scheduler
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		log.Println("Starting automated database backup scheduler...")
		systemservice.GetBackupCronWorker().Start()
	}()

	if cache.Redis != nil {
		// Start Asynq Background Worker (shuts down when workerCtx is cancelled)
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			log.Println("Starting background worker...")
			worker.StartWorkerWithContext(a.workerCtx)
		}()

		// Start Analytics Batch Worker (separate from Asynq — uses Redis Stream)
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			log.Println("Starting analytics batch worker (Redis Stream consumer)...")
			worker.StartAnalyticsBatchWorkerWithContext(a.workerCtx)
		}()
	} else {
		log.Println("Redis is not connected; background Redis workers are disabled for this process")
	}
}

// startScheduler starts the periodic task scheduler if enabled and Redis is
// connected; logs why it's disabled otherwise. No-op if a.runScheduler is
// false. Tracked via a.wg and shut down when a.workerCtx is cancelled, same
// as the other background workers started in startWorkers — previously this
// goroutine was untracked and had no way to stop, leaking for the process's
// entire life on every shutdown.
func (a *app) startScheduler() {
	if a.runScheduler && cache.Redis != nil {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			log.Println("Starting periodic task scheduler...")
			worker.StartSchedulerWithContext(a.workerCtx)
		}()
	}
	if a.runScheduler && cache.Redis == nil {
		log.Println("Redis is not connected; periodic scheduler is disabled for this process")
	}
}
