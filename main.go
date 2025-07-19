package main

import (
	"log"
	"net/http"

	"api-s3/config"
	"api-s3/routes"
	"api-s3/services"
)

func main() {
	// Load configuration
	config.LoadConfig()
	log.Println("Configuration loaded successfully")

	// Initialize S3 service
	s3Service, err := services.NewS3Service()
	if err != nil {
		log.Fatalf("Failed to initialize S3 service: %v", err)
	}
	log.Println("S3 service initialized successfully")

	// Initialize video service
	videoService := services.NewVideoService(s3Service)
	log.Println("Video service initialized successfully")

	// Setup routes
	router := routes.SetupRoutes(s3Service, videoService)
	log.Println("Routes configured successfully")

	// Start server
	port := ":" + config.AppConfig.Port
	log.Printf("Starting server on port %s", port)
	log.Printf("API endpoints:")
	log.Printf("  POST   /api/v1/upload")
	log.Printf("  DELETE /api/v1/media/:id")
	log.Printf("  GET    /api/v1/media/:id/stream")
	log.Printf("  GET    /api/v1/media/:id/stream/:quality")
	log.Printf("  GET    /api/v1/media/:id/thumbnail")
	log.Printf("  GET    /health")

	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
} 