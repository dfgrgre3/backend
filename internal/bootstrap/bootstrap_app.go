package bootstrap

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// resolveMode determines which of runAPI / runWorkers / runScheduler are
// enabled for this process, based on APP_MODE (or MODE) overriding the
// individual RUN_API / RUN_WORKERS / RUN_SCHEDULER env vars.
func (a *app) resolveMode() {
	a.runAPI = getEnvBool("RUN_API", true)
	a.runWorkers = getEnvBool("RUN_WORKERS", false)
	a.runScheduler = getEnvBool("RUN_SCHEDULER", false)

	appMode := strings.ToLower(os.Getenv("APP_MODE"))
	if appMode == "" {
		appMode = strings.ToLower(os.Getenv("MODE"))
	}

	// Only let APP_MODE/MODE override the individual RUN_API / RUN_WORKERS /
	// RUN_SCHEDULER flags when it's explicitly set to a recognized value.
	// Previously this defaulted to "api" and unconditionally overwrote all
	// three flags on every start, which made RUN_API/RUN_WORKERS/RUN_SCHEDULER
	// (documented in .env.example and set in docker-compose.yml) dead
	// configuration that could never take effect.
	switch appMode {
	case "api":
		a.runAPI = true
		a.runWorkers = false
		a.runScheduler = false
	case "worker", "workers":
		a.runAPI = false
		a.runWorkers = true
		a.runScheduler = false
	case "scheduler":
		a.runAPI = false
		a.runWorkers = false
		a.runScheduler = true
	case "all":
		a.runAPI = true
		a.runWorkers = true
		a.runScheduler = true
	case "":
		// Not set: leave runAPI/runWorkers/runScheduler as resolved from
		// RUN_API/RUN_WORKERS/RUN_SCHEDULER above.
	default:
		log.Printf("[Process Mode] Unrecognized APP_MODE=%q; falling back to RUN_API/RUN_WORKERS/RUN_SCHEDULER values", appMode)
	}

	log.Printf("[Process Mode] APP_MODE=%s (runAPI=%t, runWorkers=%t, runScheduler=%t)",
		appMode, a.runAPI, a.runWorkers, a.runScheduler)
}

// waitForShutdownSignal blocks until SIGINT or SIGTERM is received.
func (a *app) waitForShutdownSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down processes...")
}
