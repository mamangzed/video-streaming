package handlers

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
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

	// For videos, check if video processing is enabled
	if mediaType == models.MediaTypeVideo {
		if config.AppConfig.EnableVideoProcessing {
			log.Printf("🎬 Video processing enabled, starting background processing...")
			
			// Start background processing
			go func() {
				if err := h.processVideoInBackground(mediaID, file, c); err != nil {
					log.Printf("❌ Background video processing failed: %v", err)
				}
			}()
			
			// Return immediately with processing status
			c.JSON(http.StatusAccepted, models.UploadResponse{
				Success: true,
				Message: "Video upload started. Processing in background. Check progress at /api/v1/media/" + mediaID + "/progress",
				Media: &models.Media{
					ID:           mediaID,
					Filename:     file.Filename,
					OriginalName: file.Filename,
					MediaType:    mediaType,
					MimeType:     contentType,
					Size:         file.Size,
					URL:          "", // Will be updated when processing completes
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				},
			})
			return
		} else {
			log.Printf("🎬 Video processing disabled, uploading original video file...")
			
			// Upload original video file directly without processing
			key := fmt.Sprintf("media/%s/%s", mediaID, file.Filename)
			log.Printf("☁️ Uploading original video to S3: %s", key)
			
			uploadedURL, err := h.s3Service.UploadFile(file, key)
			if err != nil {
				log.Printf("❌ S3 upload failed: %v", err)
				c.JSON(http.StatusInternalServerError, models.UploadResponse{
					Success: false,
					Message: "Failed to upload video to S3",
				})
				return
			}
			
			// Create media object for original video
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
			
			log.Printf("✅ Original video upload completed successfully: %s", media.URL)
			
			// Add upload information headers
			c.Header("X-Upload-Size", fmt.Sprintf("%d", file.Size))
			c.Header("X-Upload-Time", time.Now().Format(time.RFC3339))
			
			c.JSON(http.StatusOK, models.UploadResponse{
				Success: true,
				Message: "Original video uploaded successfully (no processing)",
				Media:   media,
			})
			return
		}
	} else {
		// For non-video files, upload directly
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
		
		// Create media object for non-video files
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
		
		log.Printf("✅ Upload completed successfully: %s", media.URL)
		
		// Add upload information headers
		c.Header("X-Upload-Size", fmt.Sprintf("%d", file.Size))
		c.Header("X-Upload-Time", time.Now().Format(time.RFC3339))
		
		c.JSON(http.StatusOK, models.UploadResponse{
			Success: true,
			Message: "File uploaded successfully",
			Media:   media,
		})
		return
	}
}

// processVideoInBackground processes video in background - FAST CONVERT ONLY
func (h *MediaHandler) processVideoInBackground(mediaID string, file *multipart.FileHeader, c *gin.Context) error {
	log.Printf("🎬 Starting fast video conversion for: %s", mediaID)
	
	// Check if file is already MP4 - skip processing for speed
	if strings.HasSuffix(strings.ToLower(file.Filename), ".mp4") {
		log.Printf("✅ File is already MP4, uploading directly for speed")
		
		// Upload original MP4 file directly
		key := fmt.Sprintf("media/%s/%s", mediaID, file.Filename)
		log.Printf("☁️ Uploading original MP4 to S3: %s", key)
		
		uploadedURL, err := h.s3Service.UploadFile(file, key)
		if err != nil {
			log.Printf("❌ S3 upload failed: %v", err)
			return err
		}
		
		log.Printf("✅ Original MP4 upload completed: %s", uploadedURL)
		return nil
	}
	
	// Create temp directory for processing with unique timestamp
	timestamp := time.Now().Unix()
	tempDir := fmt.Sprintf("temp_%s_%d", mediaID, timestamp)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("❌ Failed to create temp directory: %v", err)
		return err
	}
	defer os.RemoveAll(tempDir)
	
	// Save uploaded file to temp directory with unique name
	uniqueFilename := fmt.Sprintf("%s_%d_%s", mediaID, timestamp, file.Filename)
	tempInputPath := filepath.Join(tempDir, uniqueFilename)
	if err := c.SaveUploadedFile(file, tempInputPath); err != nil {
		log.Printf("❌ Failed to save temp file: %v", err)
		return err
	}
	
	// Fast convert-only output
	outputFilename := fmt.Sprintf("%s_converted.mp4", mediaID)
	outputPath := filepath.Join(tempDir, outputFilename)
	
	log.Printf("🎬 Converting to MP4 (fast mode)...")
	
	// FAST FFmpeg command - convert only, no scaling
	cmd := exec.Command("ffmpeg",
		"-i", tempInputPath,
		"-c:v", "libx264",           // H.264 video codec
		"-preset", "fast",           // Fast preset (not slow)
		"-crf", "23",                // Good quality, fast encoding
		"-c:a", "aac",               // AAC audio codec
		"-b:a", "128k",              // Standard audio bitrate
		"-movflags", "+faststart",   // Optimize for web streaming
		"-pix_fmt", "yuv420p",       // Standard pixel format
		"-threads", "0",             // Use all CPU threads
		"-f", "mp4",                 // Force MP4 format
		"-y",                        // Overwrite output file
		outputPath,
	)
	
	// Shorter timeout for fast processing
	timeout := 3 * time.Minute
	if file.Size > 100*1024*1024 { // 100MB
		timeout = 5 * time.Minute
		log.Printf("⏱️ Large file detected, extending timeout to 5 minutes")
	} else if file.Size > 50*1024*1024 { // 50MB
		timeout = 4 * time.Minute
		log.Printf("⏱️ Medium file detected, extending timeout to 4 minutes")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	
	log.Printf("⏱️ Starting fast FFmpeg conversion (timeout: %v)...", timeout)
	log.Printf("📊 Input file size: %d bytes (%d MB)", file.Size, file.Size/(1024*1024))
	
	// Capture FFmpeg output for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("❌ FFmpeg conversion timed out")
			return fmt.Errorf("FFmpeg conversion timed out")
		}
		log.Printf("❌ FFmpeg error output: %s", string(output))
		return fmt.Errorf("FFmpeg failed: %v", err)
	}
	
	log.Printf("✅ Fast FFmpeg conversion completed successfully")
	
	// Upload converted video to S3
	key := fmt.Sprintf("media/%s/%s", mediaID, outputFilename)
	log.Printf("☁️ Uploading converted video to S3: %s", key)
	
	// Open converted file and upload
	convertedFile, err := os.Open(outputPath)
	if err != nil {
		log.Printf("❌ Failed to open converted file: %v", err)
		return err
	}
	defer convertedFile.Close()
	
	uploadedURL, err := h.s3Service.UploadFileFromReader(convertedFile, outputFilename, "video/mp4", "media/"+mediaID)
	if err != nil {
		log.Printf("❌ S3 upload failed: %v", err)
		return err
	}
	
	log.Printf("✅ Fast video conversion completed: %s", uploadedURL)
	return nil
}

// UploadMediaDirect handles file upload to S3 without video optimization
func (h *MediaHandler) UploadMediaDirect(c *gin.Context) {
	log.Println("📤 Starting direct S3 file upload (no optimization)...")
	
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
		log.Printf("❌ S3 service not available")
		c.JSON(http.StatusServiceUnavailable, models.UploadResponse{
			Success: false,
			Message: "S3 service not available",
		})
		return
	}

	// Upload directly to S3 without any processing
	key := fmt.Sprintf("media/%s/%s", mediaID, file.Filename)
	log.Printf("☁️ Uploading directly to S3: %s", key)
	
	uploadedURL, err := h.s3Service.UploadFile(file, key)
	if err != nil {
		log.Printf("❌ S3 upload failed: %v", err)
		c.JSON(http.StatusInternalServerError, models.UploadResponse{
			Success: false,
			Message: "Failed to upload file to S3",
		})
		return
	}
	
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
	
	log.Printf("✅ Direct upload completed successfully: %s", media.URL)
	
	c.JSON(http.StatusOK, models.UploadResponse{
		Success: true,
		Message: "File uploaded successfully (no optimization)",
		Media:   media,
	})
}

// UploadMediaLarge handles large file upload to S3 without any size restrictions
func (h *MediaHandler) UploadMediaLarge(c *gin.Context) {
	log.Println("📤 Starting large file upload (no size limit)...")
	
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

	log.Printf("📁 Large file received: %s, Size: %d bytes (%d MB), Type: %s", 
		file.Filename, file.Size, file.Size/(1024*1024), file.Header.Get("Content-Type"))

	// Validate file type (skip size validation for large files)
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

	log.Printf("✅ Large file validation passed: %s", mediaType)

	// Generate unique ID for media
	mediaID := uuid.New().String()
	log.Printf("🆔 Generated media ID: %s", mediaID)

	// Check if S3 service is available
	if h.s3Service == nil {
		log.Printf("❌ S3 service not available")
		c.JSON(http.StatusServiceUnavailable, models.UploadResponse{
			Success: false,
			Message: "S3 service not available",
		})
		return
	}

	// Upload directly to S3 without any processing
	key := fmt.Sprintf("media/%s/%s", mediaID, file.Filename)
	log.Printf("☁️ Uploading large file to S3: %s", key)
	
	uploadedURL, err := h.s3Service.UploadFile(file, key)
	if err != nil {
		log.Printf("❌ S3 upload failed: %v", err)
		c.JSON(http.StatusInternalServerError, models.UploadResponse{
			Success: false,
			Message: "Failed to upload large file to S3",
		})
		return
	}
	
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
	
	log.Printf("✅ Large file upload completed successfully: %s", media.URL)
	
	c.JSON(http.StatusOK, models.UploadResponse{
		Success: true,
		Message: fmt.Sprintf("Large file uploaded successfully (%d MB)", file.Size/(1024*1024)),
		Media:   media,
	})
}

// GetProcessingProgress returns the progress of video processing
func (h *MediaHandler) GetProcessingProgress(c *gin.Context) {
	mediaID := c.Param("id")
	log.Printf("📊 Getting processing progress for: %s", mediaID)
	
	// List objects in the media directory to find any processed video
	objects, err := h.s3Service.ListObjects("media/" + mediaID)
	if err != nil {
		log.Printf("❌ Error listing objects: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking processing status",
		})
		return
	}
	
	// Check if any MP4 file exists (processed video)
	var hasProcessedVideo bool
	for _, objKey := range objects {
		if strings.HasSuffix(objKey, ".mp4") {
			hasProcessedVideo = true
			log.Printf("✅ Found processed video: %s", objKey)
			break
		}
	}
	
	if hasProcessedVideo {
		// Processing completed
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"media_id": mediaID,
			"status": "completed",
			"progress": 100,
			"message": "Video processing completed successfully!",
		})
	} else {
		// Check if there are any temp files (processing in progress)
		var hasTempFiles bool
		for _, objKey := range objects {
			if strings.Contains(objKey, ".tmp") || strings.Contains(objKey, "temp") {
				hasTempFiles = true
				break
			}
		}
		
		// Calculate progress based on time elapsed (rough estimate)
		// For large files like 198MB, processing can take 5-15 minutes
		progress := 25 // Start at 25% for large files
		message := "Video is being processed with FFmpeg..."
		
		if hasTempFiles {
			progress = 50
			message = "FFmpeg is actively processing your video..."
		}
		
		// Still processing
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"media_id": mediaID,
			"status": "processing",
			"progress": progress,
			"message": message,
		})
	}
}

// GetMediaInfo returns information about a specific media file
func (h *MediaHandler) GetMediaInfo(c *gin.Context) {
	mediaID := c.Param("id")
	log.Printf("📋 Getting media info for: %s", mediaID)
	
	// List objects in the media directory to find the file
	objects, err := h.s3Service.ListObjects("media/" + mediaID)
	if err != nil {
		log.Printf("❌ Error listing objects: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error retrieving media info",
		})
		return
	}
	
	if len(objects) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Media not found",
		})
		return
	}
	
	// Find the processed video file
	var mediaURL string
	var filename string
	var mimeType string
	
	// First try to find any MP4 file (processed video)
	for _, objKey := range objects {
		if strings.HasSuffix(objKey, ".mp4") {
			// This is the processed video
			url, err := h.s3Service.GeneratePresignedURL(objKey, 24*time.Hour)
			if err != nil {
				log.Printf("❌ Error generating presigned URL: %v", err)
				continue
			}
			mediaURL = url
			filename = filepath.Base(objKey)
			mimeType = "video/mp4"
			log.Printf("✅ Found processed video: %s", objKey)
			break
		}
	}
	
	// If no MP4 found, try any other video file
	if mediaURL == "" {
		for _, objKey := range objects {
			if strings.Contains(objKey, ".mp4") || strings.Contains(objKey, ".mov") || 
			   strings.Contains(objKey, ".avi") || strings.Contains(objKey, ".mkv") {
				url, err := h.s3Service.GeneratePresignedURL(objKey, 24*time.Hour)
				if err != nil {
					log.Printf("❌ Error generating presigned URL: %v", err)
					continue
				}
				mediaURL = url
				filename = filepath.Base(objKey)
				mimeType = "video/mp4" // Default to video
				log.Printf("✅ Found video file: %s", objKey)
				break
			}
		}
	}
	
	media := &models.Media{
		ID:           mediaID,
		Filename:     filename,
		OriginalName: filename,
		MediaType:    models.MediaTypeVideo,
		MimeType:     mimeType,
		Size:         0, // We don't have size info from listing
		URL:          mediaURL,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Media info retrieved successfully",
		"media":   media,
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
	log.Printf("🎬 Streaming best quality video: %s", mediaID)

	if h.s3Service == nil {
		log.Printf("❌ S3 service not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "S3 service not available",
		})
		return
	}

	// Try to find the best quality video
	bestQualityKey := fmt.Sprintf("videos/best_quality/%s_best_quality.mp4", mediaID)
	log.Printf("🔍 Looking for best quality video: %s", bestQualityKey)
	
	if exists, _ := h.s3Service.FileExists(bestQualityKey); exists {
		log.Printf("✅ Found best quality video: %s", bestQualityKey)
		if err := h.s3Service.StreamFile(c.Writer, c.Request, bestQualityKey); err != nil {
			// Handle broken pipe errors gracefully
			if strings.Contains(err.Error(), "broken pipe") || 
			   strings.Contains(err.Error(), "connection reset") ||
			   strings.Contains(err.Error(), "write: broken pipe") {
				log.Printf("📺 Client disconnected during streaming (normal): %v", err)
				return
			}
			
			log.Printf("❌ Failed to stream video: %v", err)
			if !c.Writer.Written() {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Failed to stream video",
				})
			}
			return
		}
		log.Printf("✅ Video streamed successfully: %s", bestQualityKey)
		return
	}
	
	// Fallback to converted video
	convertedKey := fmt.Sprintf("media/%s/%s_converted.mp4", mediaID, mediaID)
	log.Printf("🔍 Looking for converted video: %s", convertedKey)
	
	if exists, _ := h.s3Service.FileExists(convertedKey); exists {
		log.Printf("✅ Found converted video: %s", convertedKey)
		if err := h.s3Service.StreamFile(c.Writer, c.Request, convertedKey); err != nil {
			// Handle broken pipe errors gracefully
			if strings.Contains(err.Error(), "broken pipe") || 
			   strings.Contains(err.Error(), "connection reset") ||
			   strings.Contains(err.Error(), "write: broken pipe") {
				log.Printf("📺 Client disconnected during streaming (normal): %v", err)
				return
			}
			
			log.Printf("❌ Failed to stream video: %v", err)
			if !c.Writer.Written() {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Failed to stream video",
				})
			}
			return
		}
		log.Printf("✅ Video streamed successfully: %s", convertedKey)
		return
	}
	
	// Try to find original video file
	originalKey := fmt.Sprintf("media/%s/", mediaID)
	log.Printf("🔍 Looking for original video in: %s", originalKey)
	
	// List objects in the media directory
	objects, err := h.s3Service.ListObjects(originalKey)
	if err != nil {
		log.Printf("❌ Failed to list objects: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to find video file",
		})
		return
	}
	
	// Look for any video file
	for _, obj := range objects {
		if strings.HasSuffix(strings.ToLower(obj), ".mp4") {
			log.Printf("✅ Found original video: %s", obj)
			if err := h.s3Service.StreamFile(c.Writer, c.Request, obj); err != nil {
				// Handle broken pipe errors gracefully
				if strings.Contains(err.Error(), "broken pipe") || 
				   strings.Contains(err.Error(), "connection reset") ||
				   strings.Contains(err.Error(), "write: broken pipe") {
					log.Printf("📺 Client disconnected during streaming (normal): %v", err)
					return
				}
				
				log.Printf("❌ Failed to stream video: %v", err)
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": "Failed to stream video",
					})
				}
				return
			}
			log.Printf("✅ Original video streamed successfully: %s", obj)
			return
		}
	}
	
	// Video not found
	log.Printf("❌ Video not found for: %s", mediaID)
	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"message": "Video not found",
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