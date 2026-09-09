package main

// @title Thanawy API
// @version 1.0
// @description This is the API server for the Thanawy platform.
// @host localhost:8082
// @BasePath /api
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

import "thanawy-backend/internal/bootstrap"

func main() {
	bootstrap.Run()
}
