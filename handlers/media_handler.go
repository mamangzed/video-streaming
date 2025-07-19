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

type MediaHandler struct {
	s3Service    *services.S3Service
	videoService *services.VideoService
}

func NewMediaHandler(s3Service *services.S3Service, videoService *services.VideoService) *MediaHandler {
	return &MediaHandler{
		s3Service:    s3Service,
		videoService: videoService,
	}
}

// UploadMedia handles file upload (images and videos)
func (h *MediaHandler) UploadMedia(c *gin.Context) {
	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.UploadResponse{
			Success: false,
			Message: "No file uploaded",
		})
		return
	}

	// Validate file size
	if file.Size > config.AppConfig.MaxFileSize {
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
		c.JSON(http.StatusBadRequest, models.UploadResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	// Generate unique ID for media
	mediaID := uuid.New().String()

	// Create temp directory
	tempDir := "temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, models.UploadResponse{
			Success: false,
			Message: "Failed to create temporary directory",
		})
		return
	}
	defer os.RemoveAll(tempDir)

	// Save file to temp directory
	tempPath := filepath.Join(tempDir, file.Filename)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, models.UploadResponse{
			Success: false,
			Message: "Failed to save uploaded file",
		})
		return
	}

	// Upload original file to S3
	folder := "images"
	if mediaType == models.MediaTypeVideo {
		folder = "videos/original"
	}

	url, err := h.s3Service.UploadFile(file, folder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.UploadResponse{
			Success: false,
			Message: "Failed to upload file to S3",
		})
		return
	}

	// Create media object
	media := &models.Media{
		ID:           mediaID,
		Filename:     filepath.Base(url),
		OriginalName: file.Filename,
		MediaType:    mediaType,
		MimeType:     contentType,
		Size:         file.Size,
		URL:          url,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Process video if it's a video file
	if mediaType == models.MediaTypeVideo && config.AppConfig.EnableVideoProcessing {
		go h.processVideoAsync(mediaID, tempPath, media)
	}

	c.JSON(http.StatusOK, models.UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		Media:   media,
	})
}

// DeleteMedia handles file deletion
func (h *MediaHandler) DeleteMedia(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, models.DeleteResponse{
			Success: false,
			Message: "Media ID is required",
		})
		return
	}

	// For now, we'll delete based on the media ID pattern
	// In a real application, you'd store media metadata in a database
	// and use that to find the actual S3 keys

	// Delete from different folders based on media type
	folders := []string{
		"images",
		"videos/original",
		"videos/144p",
		"videos/240p",
		"videos/360p",
		"videos/480p",
		"videos/720p",
		"videos/1080p",
		"videos/1440p",
		"videos/2160p",
		"thumbnails",
		"playlists",
	}

	for _, folder := range folders {
		// Try to find and delete files with the media ID
		// This is a simplified approach - in production you'd use a database
		pattern := fmt.Sprintf("%s/*%s*", folder, mediaID)
		matches, err := filepath.Glob(pattern)
		if err == nil {
			for _, match := range matches {
				key := h.s3Service.ExtractKeyFromURL(match)
				if key != "" {
					if err := h.s3Service.DeleteFile(key); err != nil {
						log.Printf("Failed to delete file %s: %v", key, err)
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, models.DeleteResponse{
		Success: true,
		Message: "Media deleted successfully",
	})
}

// GetVideoStream returns video streaming information
func (h *MediaHandler) GetVideoStream(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, models.VideoStreamResponse{
			Success: false,
			Message: "Media ID is required",
		})
		return
	}

	// In a real application, you'd fetch variants from a database
	// For now, we'll return a mock response
	variants := []models.VideoVariant{
		{
			ID:        "1",
			MediaID:   mediaID,
			Quality:   models.Quality360p,
			Width:     640,
			Height:    360,
			Bitrate:   500,
			URL:       fmt.Sprintf("https://%s.s3.%s.amazonaws.com/videos/360p/%s_360p.mp4", 
				config.AppConfig.AWSS3Bucket, config.AppConfig.AWSRegion, mediaID),
			Size:      1024000,
			CreatedAt: time.Now(),
		},
		{
			ID:        "2",
			MediaID:   mediaID,
			Quality:   models.Quality720p,
			Width:     1280,
			Height:    720,
			Bitrate:   1500,
			URL:       fmt.Sprintf("https://%s.s3.%s.amazonaws.com/videos/720p/%s_720p.mp4", 
				config.AppConfig.AWSS3Bucket, config.AppConfig.AWSRegion, mediaID),
			Size:      2048000,
			CreatedAt: time.Now(),
		},
	}

	masterURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/playlists/%s.m3u8", 
		config.AppConfig.AWSS3Bucket, config.AppConfig.AWSRegion, mediaID)

	c.JSON(http.StatusOK, models.VideoStreamResponse{
		Success:   true,
		Message:   "Video stream information retrieved",
		Variants:  variants,
		MasterURL: masterURL,
	})
}

// StreamVideo handles video streaming with range requests
func (h *MediaHandler) StreamVideo(c *gin.Context) {
	mediaID := c.Param("id")
	quality := c.Param("quality")
	
	if mediaID == "" || quality == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Media ID and quality are required",
		})
		return
	}

	// Construct S3 key
	s3Key := fmt.Sprintf("videos/%s/%s_%s.mp4", quality, mediaID, quality)
	
	// Generate presigned URL for streaming
	presignedURL, err := h.s3Service.GeneratePresignedURL(s3Key, 1*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate streaming URL",
		})
		return
	}

	// Redirect to presigned URL
	c.Redirect(http.StatusTemporaryRedirect, presignedURL)
}

// GetThumbnail returns video thumbnail
func (h *MediaHandler) GetThumbnail(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Media ID is required",
		})
		return
	}

	// Construct S3 key for thumbnail
	s3Key := fmt.Sprintf("thumbnails/%s_thumb.jpg", mediaID)
	
	// Generate presigned URL
	presignedURL, err := h.s3Service.GeneratePresignedURL(s3Key, 1*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate thumbnail URL",
		})
		return
	}

	// Redirect to presigned URL
	c.Redirect(http.StatusTemporaryRedirect, presignedURL)
}

// validateFileType validates the uploaded file type
func (h *MediaHandler) validateFileType(contentType, filename string) (models.MediaType, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	
	// Image types
	imageTypes := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp"}
	for _, imgExt := range imageTypes {
		if ext == imgExt {
			return models.MediaTypeImage, nil
		}
	}

	// Video types
	videoTypes := []string{".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv", ".m4v"}
	for _, vidExt := range videoTypes {
		if ext == vidExt {
			return models.MediaTypeVideo, nil
		}
	}

	return "", fmt.Errorf("unsupported file type: %s", ext)
}

// processVideoAsync processes video in background
func (h *MediaHandler) processVideoAsync(mediaID, tempPath string, media *models.Media) {
	log.Printf("Starting video processing for media ID: %s", mediaID)

	// Process video variants
	variants, err := h.videoService.ProcessVideo(tempPath, mediaID)
	if err != nil {
		log.Printf("Failed to process video %s: %v", mediaID, err)
		return
	}

	// Create thumbnail
	thumbnailURL, err := h.videoService.CreateThumbnail(tempPath, mediaID)
	if err != nil {
		log.Printf("Failed to create thumbnail for %s: %v", mediaID, err)
	} else {
		media.ThumbnailURL = thumbnailURL
	}

	// Create HLS playlist
	playlistURL, err := h.videoService.CreateHLSPlaylist(variants, mediaID)
	if err != nil {
		log.Printf("Failed to create playlist for %s: %v", mediaID, err)
	}

	log.Printf("Video processing completed for media ID: %s. Variants: %d, Playlist: %s", 
		mediaID, len(variants), playlistURL)
} 