package handlers

import (
	"context"
	"fmt"
	"log"
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

	// For videos, start background processing and return immediately
	if mediaType == models.MediaTypeVideo {
		log.Printf("🎬 Starting background video processing...")
		
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
		
		c.JSON(http.StatusOK, models.UploadResponse{
			Success: true,
			Message: "File uploaded successfully",
			Media:   media,
		})
		return
	}

	log.Printf("✅ S3 upload successful: %s", uploadedURL)

	// Create media object
	media := &models.Media{
		ID:           mediaID,
		Filename:     finalFilename,
		OriginalName: file.Filename,
		MediaType:    mediaType,
		MimeType:     finalMimeType,
		Size:         file.Size,
		URL:          uploadedURL,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Video processing is now done before upload, so no need for background processing
	// The video is already converted to MP4 format

	log.Printf("✅ Upload completed successfully: %s", media.URL)

	c.JSON(http.StatusOK, models.UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		Media:   media,
	})
}

// processVideoInBackground processes video in background
func (h *MediaHandler) processVideoInBackground(mediaID string, file *multipart.FileHeader, c *gin.Context) error {
	log.Printf("🎬 Starting background video processing for: %s", mediaID)
	
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
	
	// Process video with FFmpeg to optimal MP4 format
	outputFilename := fmt.Sprintf("%s_optimized.mp4", mediaID)
	outputPath := filepath.Join(tempDir, outputFilename)
	
	log.Printf("🎬 Converting to optimal MP4 format...")
	cmd := exec.Command("ffmpeg",
		"-i", tempInputPath,
		"-vcodec", "libx264",        // H.264 video codec
		"-acodec", "aac",            // AAC audio codec
		"-strict", "-2",             // Allow experimental codecs
		"-b:v", "3M",                // Video bitrate 3Mbps
		"-b:a", "192k",              // Audio bitrate 192kbps
		"-f", "mp4",                 // Force MP4 format
		"-movflags", "+faststart",   // Optimize for web streaming
		"-preset", "medium",         // Encoding preset
		"-crf", "23",                // Constant Rate Factor
		"-y",                        // Overwrite output file
		outputPath,
	)
	
	// Set a timeout for FFmpeg processing (5 minutes)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	
	log.Printf("⏱️ Starting FFmpeg processing (timeout: 5 minutes)...")
	log.Printf("📊 Input file size: %d bytes", file.Size)
	
	// Capture FFmpeg output for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("❌ FFmpeg processing timed out after 5 minutes")
			return fmt.Errorf("FFmpeg processing timed out")
		}
		log.Printf("❌ FFmpeg error output: %s", string(output))
		return fmt.Errorf("FFmpeg failed: %v", err)
	}
	
	log.Printf("✅ FFmpeg processing completed successfully")
	
	// Upload processed video to S3
	key := fmt.Sprintf("media/%s/%s", mediaID, outputFilename)
	log.Printf("☁️ Uploading processed video to S3: %s", key)
	
	// Open processed file and upload
	processedFile, err := os.Open(outputPath)
	if err != nil {
		log.Printf("❌ Failed to open processed file: %v", err)
		return err
	}
	defer processedFile.Close()
	
	uploadedURL, err := h.s3Service.UploadFileFromReader(processedFile, outputFilename, "video/mp4", "media/"+mediaID)
	if err != nil {
		log.Printf("❌ S3 upload failed: %v", err)
		return err
	}
	
	log.Printf("✅ Background video processing completed: %s", uploadedURL)
	return nil
}

// GetProcessingProgress returns the progress of video processing
func (h *MediaHandler) GetProcessingProgress(c *gin.Context) {
	mediaID := c.Param("id")
	log.Printf("📊 Getting processing progress for: %s", mediaID)
	
	// Check if processed video exists in S3
	processedKey := fmt.Sprintf("media/%s/%s_optimized.mp4", mediaID, mediaID)
	exists, err := h.s3Service.FileExists(processedKey)
	if err != nil {
		log.Printf("❌ Error checking file existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking processing status",
		})
		return
	}
	
	if exists {
		// Processing completed
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"media_id": mediaID,
			"status": "completed",
			"progress": 100,
			"message": "Video processing completed successfully!",
		})
	} else {
		// Still processing
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"media_id": mediaID,
			"status": "processing",
			"progress": 75, // Estimate based on typical processing time
			"message": "Video is being processed with FFmpeg...",
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
	
	for _, obj := range objects {
		if strings.Contains(*obj.Key, "_optimized.mp4") {
			// This is the processed video
			url, err := h.s3Service.GeneratePresignedURL(*obj.Key, 24*time.Hour)
			if err != nil {
				log.Printf("❌ Error generating presigned URL: %v", err)
				continue
			}
			mediaURL = url
			filename = filepath.Base(*obj.Key)
			mimeType = "video/mp4"
			break
		}
	}
	
	if mediaURL == "" {
		// Fallback to original file
		for _, obj := range objects {
			url, err := h.s3Service.GeneratePresignedURL(*obj.Key, 24*time.Hour)
			if err != nil {
				log.Printf("❌ Error generating presigned URL: %v", err)
				continue
			}
			mediaURL = url
			filename = filepath.Base(*obj.Key)
			mimeType = "video/mp4" // Default to video
			break
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
	quality := c.Param("quality")
	log.Printf("🎬 Streaming video: %s at quality: %s", mediaID, quality)

	if h.s3Service == nil {
		log.Printf("❌ S3 service not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "S3 service not available",
		})
		return
	}

	// For now, we'll stream the original video since we haven't implemented
	// multi-quality processing yet. In a real implementation, you would:
	// 1. Look up the specific quality variant in database
	// 2. Stream the appropriate video file for that quality
	
	// Try to find the video file in S3 by trying different patterns
	// The upload pattern was: "media/{mediaID}/{filename}"
	// Let's try to find the actual file
	
	// First, try to get filename from query parameter
	filename := c.Query("filename")
	if filename != "" {
		s3Key := fmt.Sprintf("media/%s/%s", mediaID, filename)
		log.Printf("📺 Trying S3 key with filename: %s", s3Key)
		
		if exists, _ := h.s3Service.FileExists(s3Key); exists {
			if err := h.s3Service.StreamFile(c.Writer, c.Request, s3Key); err != nil {
				log.Printf("❌ Failed to stream video: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Failed to stream video",
				})
				return
			}
			log.Printf("✅ Video streamed successfully: %s", s3Key)
			return
		}
	}
	
	// If no filename provided or file not found, try to find the actual file
	// First, try to find processed video variants in the videos/{quality}/ directory
	// The processed videos are stored as: "videos/{quality}/{mediaID}_{quality}.mp4"
	
	// Try to find the specific quality variant
	qualityVariantKey := fmt.Sprintf("videos/%s/%s_%s.mp4", quality, mediaID, quality)
	log.Printf("🔍 Looking for quality variant: %s", qualityVariantKey)
	
	if exists, _ := h.s3Service.FileExists(qualityVariantKey); exists {
		log.Printf("✅ Found quality variant: %s", qualityVariantKey)
		if err := h.s3Service.StreamFile(c.Writer, c.Request, qualityVariantKey); err != nil {
			// Check if it's a broken pipe error (normal for video streaming)
			if strings.Contains(err.Error(), "broken pipe") || 
			   strings.Contains(err.Error(), "connection reset") ||
			   strings.Contains(err.Error(), "write: broken pipe") {
				log.Printf("📺 Client disconnected during streaming (normal): %v", err)
				return // Don't treat as error
			}
			
			log.Printf("❌ Failed to stream video: %v", err)
			// Don't send JSON response if headers already written
			if !c.Writer.Written() {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Failed to stream video",
				})
			}
			return
		}
		log.Printf("✅ Video streamed successfully: %s", qualityVariantKey)
		return
	}
	
	// Try to find the optimized video file (processed with FFmpeg)
	// The processed video is stored as: "media/{mediaID}/{mediaID}_optimized.mp4"
	optimizedKey := fmt.Sprintf("media/%s/%s_optimized.mp4", mediaID, mediaID)
	log.Printf("🔍 Looking for optimized video: %s", optimizedKey)
	
	if exists, _ := h.s3Service.FileExists(optimizedKey); exists {
		log.Printf("✅ Found optimized video: %s", optimizedKey)
		if err := h.s3Service.StreamFile(c.Writer, c.Request, optimizedKey); err != nil {
			// Check if it's a broken pipe error (normal for video streaming)
			if strings.Contains(err.Error(), "broken pipe") || 
			   strings.Contains(err.Error(), "connection reset") ||
			   strings.Contains(err.Error(), "write: broken pipe") {
				log.Printf("📺 Client disconnected during streaming (normal): %v", err)
				return // Don't treat as error
			}
			
			log.Printf("❌ Failed to stream video: %v", err)
			// Don't send JSON response if headers already written
			if !c.Writer.Written() {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Failed to stream video",
				})
			}
			return
		}
		log.Printf("✅ Video streamed successfully: %s", optimizedKey)
		return
	}
	
	// If quality variant not found, try to find the original video
	// Based on the upload pattern, the file is stored as: "media/{mediaID}/{filename}"
	log.Printf("🔍 Quality variant not found, looking for original video...")
	
	// Try to list objects in the media/{mediaID}/ directory
	objects, err := h.s3Service.ListObjects(fmt.Sprintf("media/%s/", mediaID))
	if err != nil {
		log.Printf("❌ Failed to list objects: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to find video file",
		})
		return
	}
	
	// Look for video files (prioritize MP4 files)
	for _, obj := range objects {
		log.Printf("🔍 Found object: %s", obj)
		
		// First, look for optimized MP4 files
		if strings.HasSuffix(strings.ToLower(obj), "_optimized.mp4") {
			log.Printf("✅ Found optimized MP4 file: %s", obj)
			if err := h.s3Service.StreamFile(c.Writer, c.Request, obj); err != nil {
				// Check if it's a broken pipe error (normal for video streaming)
				if strings.Contains(err.Error(), "broken pipe") || 
				   strings.Contains(err.Error(), "connection reset") ||
				   strings.Contains(err.Error(), "write: broken pipe") {
					log.Printf("📺 Client disconnected during streaming (normal): %v", err)
					return // Don't treat as error
				}
				
				log.Printf("❌ Failed to stream video: %v", err)
				// Don't send JSON response if headers already written
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": "Failed to stream video",
					})
				}
				return
			}
			log.Printf("✅ Video streamed successfully: %s", obj)
			return
		}
		
		// Then look for any MP4 files
		if strings.HasSuffix(strings.ToLower(obj), ".mp4") {
			log.Printf("✅ Found MP4 file: %s", obj)
			if err := h.s3Service.StreamFile(c.Writer, c.Request, obj); err != nil {
				// Check if it's a broken pipe error (normal for video streaming)
				if strings.Contains(err.Error(), "broken pipe") || 
				   strings.Contains(err.Error(), "connection reset") ||
				   strings.Contains(err.Error(), "write: broken pipe") {
					log.Printf("📺 Client disconnected during streaming (normal): %v", err)
					return // Don't treat as error
				}
				
				log.Printf("❌ Failed to stream video: %v", err)
				// Don't send JSON response if headers already written
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": "Failed to stream video",
					})
				}
				return
			}
			log.Printf("✅ Video streamed successfully: %s", obj)
			return
		}
		
		// Finally, look for other video formats as fallback
		if strings.HasSuffix(strings.ToLower(obj), ".avi") ||
		   strings.HasSuffix(strings.ToLower(obj), ".mov") ||
		   strings.HasSuffix(strings.ToLower(obj), ".mkv") ||
		   strings.HasSuffix(strings.ToLower(obj), ".webm") {
			
			log.Printf("✅ Found other video file: %s", obj)
			if err := h.s3Service.StreamFile(c.Writer, c.Request, obj); err != nil {
				// Check if it's a broken pipe error (normal for video streaming)
				if strings.Contains(err.Error(), "broken pipe") || 
				   strings.Contains(err.Error(), "connection reset") ||
				   strings.Contains(err.Error(), "write: broken pipe") {
					log.Printf("📺 Client disconnected during streaming (normal): %v", err)
					return // Don't treat as error
				}
				
				log.Printf("❌ Failed to stream video: %v", err)
				// Don't send JSON response if headers already written
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": "Failed to stream video",
					})
				}
				return
			}
			log.Printf("✅ Video streamed successfully: %s", obj)
			return
		}
	}
	
	// If no file found, return error
	log.Printf("❌ No video file found for media ID: %s", mediaID)
	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"message": "Video file not found. Please check the media ID or try uploading again.",
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