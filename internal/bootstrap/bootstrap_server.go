package bootstrap

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// startAPIServer starts the HTTP server in a background goroutine if
// a.runAPI is true. The *http.Server handle is stored on a.srv so shutdown
// can gracefully stop it later.
func (a *app) startAPIServer(r *gin.Engine) {
	if !a.runAPI {
		return
	}

	cfg := a.cfg

	port := os.Getenv("BACKEND_PORT")
	if port == "" {
		port = os.Getenv("PORT")
	}

	if port == "" {
		port = "8082"
	}

	readTimeout, err := time.ParseDuration(cfg.HTTPReadTimeout)
	if err != nil {
		readTimeout = 10 * time.Second
	}
	writeTimeout, err := time.ParseDuration(cfg.HTTPWriteTimeout)
	if err != nil {
		writeTimeout = 30 * time.Second
	}
	idleTimeout, err := time.ParseDuration(cfg.HTTPIdleTimeout)
	if err != nil {
		idleTimeout = 120 * time.Second
	}

	a.srv = &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Run server in goroutine
	go func() {
		log.Printf("HTTP server starting on port %s", port)
		if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
}
