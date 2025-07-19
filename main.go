package main

import (
	"log"
	"net/http"

	"api-s3/config"
	"api-s3/routes"
	"api-s3/services"
)

func main() {
	// Set log format for better debugging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	// Load configuration
	config.LoadConfig()
	log.Println("✅ Configuration loaded successfully")

	// Initialize S3 service with better error handling
	var s3Service *services.S3Service
	var videoService *services.VideoService
	
	s3Service, err := services.NewS3Service()
	if err != nil {
		log.Printf("⚠️  Failed to initialize S3 service: %v", err)
		log.Println("   Running in local mode only")
		log.Println("   Use /api/v1/upload-local for testing without S3")
		s3Service = nil
	} else {
		log.Println("✅ S3 service initialized successfully")
	}

	// Initialize video service
	if s3Service != nil {
		videoService = services.NewVideoService(s3Service)
		log.Println("✅ Video service initialized successfully")
	} else {
		log.Println("⚠️  Video service disabled (no S3 connection)")
	}

	// Setup routes
	router := routes.SetupRoutes(s3Service, videoService)
	log.Println("✅ Routes configured successfully")

	// Start server
	port := ":" + config.AppConfig.Port
	log.Printf(" Starting server on port %s", port)
	log.Printf("📋 API endpoints:")
	log.Printf("  POST   /api/v1/upload          (requires S3)")
	log.Printf("  POST   /api/v1/upload-local    (local storage)")
	log.Printf("  DELETE /api/v1/media/:id")
	log.Printf("  GET    /api/v1/media/:id/stream")
	log.Printf("  GET    /api/v1/media/:id/stream/:quality")
	log.Printf("  GET    /api/v1/media/:id/thumbnail")
	log.Printf("  GET    /health")
	log.Printf("  GET    /")

	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}