package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"api-s3/config"
	"api-s3/models"
	"api-s3/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MediaHandler handles media-related HTTP requests
type MediaHandler struct {
	s3Service    *services.S3Service
	videoService *services.VideoService
}

// NewMediaHandler creates a new MediaHandler instance
func NewMediaHandler(s3Service *services.S3Service, videoService *services.VideoService) *MediaHandler {
	return &MediaHandler{
		s3Service:    s3Service,
		videoService: videoService,
	}
}

// UploadMedia handles file upload to S3
func (h *MediaHandler) UploadMedia(c *gin.Context) {
	log.Println("📤 Starting S3 file upload...")
	
	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("❌ No file uploaded: %v", err)
		c.JSON(http.StatusBadRequest, models.UploadResponse{
			Success: false,
			Message: "No file uploaded",
		})
		return
	}

	log.Printf("📁 File received: %s, Size: %d bytes, Type: %s", 
		file.Filename, file.Size, file.Header.Get("Content-Type"))

	// Validate file size
	if file.Size > config.AppConfig.MaxFileSize {
		log.Printf("❌ File too large: %d > %d", file.Size, config.AppConfig.MaxFileSize)
		c.JSON(http.StatusBadRequest, models.UploadResponse{
			Success: false,
			Message: fmt.Sprintf("File size exceeds maximum allowed size of %d bytes", config.AppConfig.MaxFileSize),
		})
		return
	}

	// Validate file type
	contentType := file.Header.Get("Content-Type")
	mediaType, err := h.validateFileType(contentType, file.Filename)
	if err != nil {
		log.Printf("❌ Invalid file type: %v", err)
		c.JSON(http.StatusBadRequest, models.UploadResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	log.Printf("✅ File validation passed: %s", mediaType)

	// Generate unique ID for media
	mediaID := uuid.New().String()
	log.Printf("🆔 Generated media ID: %s", mediaID)

	// Check if S3 service is available
	if h.s3Service == nil {
		log.Printf("❌ S3 service not available, falling back to local upload")
		c.JSON(http.StatusServiceUnavailable, models.UploadResponse{
			Success: false,
			Message: "S3 service not available. Please use /upload-local endpoint for local uploads.",
		})
		return
	}

	// Upload to S3
	key := fmt.Sprintf("media/%s/%s", mediaID, file.Filename)
	log.Printf("☁️ Uploading to S3: %s", key)
	
	uploadedURL, err := h.s3Service.UploadFile(file, key)
	if err != nil {
		log.Printf("❌ S3 upload failed: %v", err)
		c.JSON(http.StatusInternalServerError, models.UploadResponse{
			Success: false,
			Message: "Failed to upload file to S3",
		})
		return
	}

	log.Printf("✅ S3 upload successful: %s", uploadedURL)

	// Create media object
	media := &models.Media{
		ID:           mediaID,
		Filename:     file.Filename,
		OriginalName: file.Filename,
		MediaType:    mediaType,
		MimeType:     contentType,
		Size:         file.Size,
		URL:          uploadedURL,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// If it's a video, start processing for streaming
	if mediaType == models.MediaTypeVideo {
		log.Printf("🎥 Starting video processing for streaming...")
		go h.videoService.ProcessVideoForStreaming(media)
	}

	log.Printf("✅ Upload completed successfully: %s", media.URL)

	c.JSON(http.StatusOK, models.UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		Media:   media,
	})
}

// UploadMediaLocal handles file upload to local storage (for testing without S3)
func (h *MediaHandler) UploadMediaLocal(c *gin.Context) {
	log.Println("📤 Starting local file upload...")
	
	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("❌ No file uploaded: %v", err)
		c.JSON(http.StatusBadRequest, models.UploadResponse{
			Success: false,
			Message: "No file uploaded",
		})
		return
	}

	log.Printf("📁 File received: %s, Size: %d bytes, Type: %s", 
		file.Filename, file.Size, file.Header.Get("Content-Type"))

	// Validate file size
	if file.Size > config.AppConfig.MaxFileSize {
		log.Printf("❌ File too large: %d > %d", file.Size, config.AppConfig.MaxFileSize)
		c.JSON(http.StatusBadRequest, models.UploadResponse{
			Success: false,
			Message: fmt.Sprintf("File size exceeds maximum allowed size of %d bytes", config.AppConfig.MaxFileSize),
		})
		return
	}

	// Validate file type
	contentType := file.Header.Get("Content-Type")
	mediaType, err := h.validateFileType(contentType, file.Filename)
	if err != nil {
		log.Printf("❌ Invalid file type: %v", err)
		c.JSON(http.StatusBadRequest, models.UploadResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	log.Printf("✅ File validation passed: %s", mediaType)

	// Generate unique ID for media
	mediaID := uuid.New().String()
	log.Printf("🆔 Generated media ID: %s", mediaID)

	// Create uploads directory
	uploadDir := "uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("❌ Failed to create uploads directory: %v", err)
		c.JSON(http.StatusInternalServerError, models.UploadResponse{
			Success: false,
			Message: "Failed to create uploads directory",
		})
		return
	}

	// Save file locally
	filename := fmt.Sprintf("%s_%s", mediaID, file.Filename)
	filePath := filepath.Join(uploadDir, filename)
	
	log.Printf("💾 Saving file to: %s", filePath)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		log.Printf("❌ Failed to save file: %v", err)
		c.JSON(http.StatusInternalServerError, models.UploadResponse{
			Success: false,
			Message: "Failed to save uploaded file",
		})
		return
	}

	// Verify file was saved
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("❌ File not found after save: %s", filePath)
		c.JSON(http.StatusInternalServerError, models.UploadResponse{
			Success: false,
			Message: "File was not saved properly",
		})
		return
	}

	log.Printf("✅ File saved successfully: %s", filePath)

	// Create media object
	media := &models.Media{
		ID:           mediaID,
		Filename:     filename,
		OriginalName: file.Filename,
		MediaType:    mediaType,
		MimeType:     contentType,
		Size:         file.Size,
		URL:          fmt.Sprintf("/uploads/%s", filename),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	log.Printf("✅ Upload completed successfully: %s", media.URL)

	c.JSON(http.StatusOK, models.UploadResponse{
		Success: true,
		Message: "File uploaded successfully (local mode)",
		Media:   media,
	})
}

// DeleteMedia handles media deletion
func (h *MediaHandler) DeleteMedia(c *gin.Context) {
	mediaID := c.Param("id")
	log.Printf("🗑️ Deleting media: %s", mediaID)

	if h.s3Service != nil {
		// Delete from S3
		if err := h.s3Service.DeleteFile(mediaID); err != nil {
			log.Printf("❌ Failed to delete from S3: %v", err)
			c.JSON(http.StatusInternalServerError, models.DeleteResponse{
				Success: false,
				Message: "Failed to delete file from S3",
			})
			return
		}
	} else {
		// Delete from local storage
		filePath := filepath.Join("uploads", mediaID)
		if err := os.Remove(filePath); err != nil {
			log.Printf("❌ Failed to delete local file: %v", err)
			c.JSON(http.StatusInternalServerError, models.DeleteResponse{
				Success: false,
				Message: "Failed to delete local file",
			})
			return
		}
	}

	log.Printf("✅ Media deleted successfully: %s", mediaID)
	c.JSON(http.StatusOK, models.DeleteResponse{
		Success: true,
		Message: "Media deleted successfully",
	})
}

// GetVideoStream returns video streaming information
func (h *MediaHandler) GetVideoStream(c *gin.Context) {
	mediaID := c.Param("id")
	log.Printf("🎥 Getting video stream info: %s", mediaID)

	if h.videoService == nil {
		c.JSON(http.StatusServiceUnavailable, models.VideoStreamResponse{
			Success: false,
			Message: "Video service not available",
		})
		return
	}

	variants, err := h.videoService.GetVideoVariants(mediaID)
	if err != nil {
		log.Printf("❌ Failed to get video variants: %v", err)
		c.JSON(http.StatusInternalServerError, models.VideoStreamResponse{
			Success: false,
			Message: "Failed to get video streaming information",
		})
		return
	}

	c.JSON(http.StatusOK, models.VideoStreamResponse{
		Success:  true,
		Message:  "Video streaming information retrieved",
		Variants: variants,
	})
}

// StreamVideo streams video at specific quality
func (h *MediaHandler) StreamVideo(c *gin.Context) {
	mediaID := c.Param("id")
	quality := c.Param("quality")
	log.Printf("🎬 Streaming video: %s at quality: %s", mediaID, quality)

	if h.videoService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "Video service not available",
		})
		return
	}

	// This would typically serve the video file or redirect to the appropriate URL
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Streaming video %s at %s quality", mediaID, quality),
		"url":     fmt.Sprintf("/media/%s/stream/%s", mediaID, quality),
	})
}

// GetThumbnail returns video thumbnail
func (h *MediaHandler) GetThumbnail(c *gin.Context) {
	mediaID := c.Param("id")
	log.Printf("🖼️ Getting thumbnail: %s", mediaID)

	if h.videoService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "Video service not available",
		})
		return
	}

	// This would typically serve the thumbnail image
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Thumbnail retrieved",
		"url":     fmt.Sprintf("/media/%s/thumbnail", mediaID),
	})
}

// validateFileType validates the uploaded file type
func (h *MediaHandler) validateFileType(contentType, filename string) (models.MediaType, error) {
	// Check content type
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return models.MediaTypeImage, nil
	case strings.HasPrefix(contentType, "video/"):
		return models.MediaTypeVideo, nil
	}

	// Check file extension as fallback
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return models.MediaTypeImage, nil
	case ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv":
		return models.MediaTypeVideo, nil
	}

	return "", fmt.Errorf("unsupported file type: %s", contentType)
}