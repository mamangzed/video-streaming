package routes

import (
	"api-s3/handlers"
	"api-s3/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(s3Service *services.S3Service, videoService *services.VideoService) *gin.Engine {
	// Configure Gin for large file uploads
	gin.SetMode(gin.ReleaseMode)
	
	// Create router with custom configuration
	router := gin.New()
	
	// Add recovery middleware
	router.Use(gin.Recovery())
	
	// Add logger middleware
	router.Use(gin.Logger())
	
	// Configure for large file uploads
	router.MaxMultipartMemory = 1 << 30 // 1GB memory limit for multipart forms
	
	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	// Serve static files
	router.Static("/static", "./public")
	router.LoadHTMLGlob("public/*.html")

	// Serve uploaded files
	router.Static("/uploads", "./uploads")

	// Serve index page
	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	// Create media handler
	mediaHandler := handlers.NewMediaHandler(s3Service, videoService)

	// API routes
	api := router.Group("/api/v1")
	{
		// Media upload with large file support
		api.POST("/upload", func(c *gin.Context) {
			// Set custom limits for this endpoint
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<30) // 1GB
			mediaHandler.UploadMedia(c)
		})
		
		// Direct upload without video optimization
		api.POST("/upload-direct", func(c *gin.Context) {
			// Set custom limits for this endpoint
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<30) // 1GB
			mediaHandler.UploadMediaDirect(c)
		})
		
		// Local upload (for testing without S3)
		api.POST("/upload-local", mediaHandler.UploadMediaLocal)
		
		// Media management
		api.DELETE("/media/:id", mediaHandler.DeleteMedia)
		
		// Video streaming
		api.GET("/media/:id/stream", mediaHandler.GetVideoStream)
		api.GET("/media/:id/stream/:quality", mediaHandler.StreamVideo)
		api.GET("/media/:id/thumbnail", mediaHandler.GetThumbnail)
		api.GET("/media/:id/progress", mediaHandler.GetProcessingProgress)
		api.GET("/media/:id", mediaHandler.GetMediaInfo)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"message": "API is running",
		})
	})

	return router
} 