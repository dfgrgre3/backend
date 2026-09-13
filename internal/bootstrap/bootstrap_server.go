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

	// Warm the public homepage endpoints right after boot so the FIRST real
	// visitor doesn't pay for a cold cache. Without this, /settings,
	// /categories, /navigation/menu, /homepage, /teachers, /courses and
	// /blog each do their expensive query + cache-populate on whichever
	// request happens to land first (see [PERF] SLOW REQUEST log lines at
	// startup, ~500-950ms), then serve every subsequent hit in a few ms
	// from cache. Firing these ourselves moves that one-time cost from a
	// real user's page load to server startup.
	go warmHomepageCache(port)
}

// warmHomepageCache issues best-effort GET requests to the public endpoints
// the storefront/homepage loads on first paint, so their response caches
// (settings, categories, navigation menu, homepage aggregate, teachers,
// courses, blog) are already populated before real traffic arrives.
func warmHomepageCache(port string) {
	// Give the listener a moment to actually accept connections.
	time.Sleep(300 * time.Millisecond)

	base := "http://127.0.0.1:" + port
	targets := []string{
		"/api/v1/settings",
		"/api/v1/categories?limit=8",
		"/api/v1/categories?limit=12",
		"/api/v1/navigation/menu",
		"/api/v1/teachers?limit=6",
		"/api/v1/homepage",
		"/api/v1/blog?limit=4&published=true",
		"/api/v1/courses?isActive=true&isPublished=true&limit=8&order=desc&sort=enrolledCount",
	}

	client := &http.Client{Timeout: 15 * time.Second}
	start := time.Now()
	for _, path := range targets {
		reqStart := time.Now()
		resp, err := client.Get(base + path)
		if err != nil {
			log.Printf("[Cache WarmUp] %s failed: %v", path, err)
			continue
		}
		resp.Body.Close()
		log.Printf("[Cache WarmUp] %s -> %d in %s", path, resp.StatusCode, time.Since(reqStart).Round(time.Millisecond))
	}
	log.Printf("[Cache WarmUp] Done warming %d endpoint(s) in %s", len(targets), time.Since(start).Round(time.Millisecond))
}
