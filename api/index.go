package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[vercel] Starting up...")
}

// Handler is the Vercel Serverless Function entry point.
func Handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[vercel] %s %s", r.Method, r.URL.Path)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP"}`))
}

// Local dev (ignored by Vercel)
func main() {
	http.HandleFunc("/", Handler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Starting local server on :%s\n", port)
	_ = http.ListenAndServe(":"+port, nil)
}