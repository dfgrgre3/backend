package bootstrap

import (
	"context"
	"log"
	"thanawy-backend/internal/application/services"
	systemservice "thanawy-backend/internal/domain/system/service"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"
	"time"
)

// shutdown performs the graceful shutdown sequence in the same order as the
// original inline code in Run(): stop background workers first, wait for
// their goroutines to exit (bounded to 30s), then shut down the HTTP
// server, then close Redis and the database connection pools last.
func (a *app) shutdown() {
	// 1. Stop all background workers first (before closing DB/Redis)
	log.Println("Stopping background workers...")
	a.stopWorkers()

	if a.runWorkers {
		services.GetMailQueueWorker().Stop()
		systemservice.GetBackupCronWorker().Stop()
	}

	// 2. Wait for all worker goroutines to exit (max 30 seconds)
	workerDone := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(workerDone)
	}()
	select {
	case <-workerDone:
		log.Println("All background workers stopped cleanly")
	case <-time.After(30 * time.Second):
		log.Println("[WARN] Timed out waiting for workers to stop; proceeding with shutdown")
	}

	// 3. Shutdown HTTP server after workers are done
	if a.runAPI && a.srv != nil {
		log.Println("Shutting down HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server forced to shutdown: %v", err)
		}
	}

	// 4. Close Redis and DB connections last
	if err := cache.CloseRedis(); err != nil {
		log.Printf("Error closing Redis connection pool: %v", err)
	}

	if err := db.Close(); err != nil {
		log.Printf("Error closing database connection pool: %v", err)
	}

	log.Println("Process exited")
}
