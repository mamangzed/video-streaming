package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"api-s3/config"
	"api-s3/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type VideoService struct {
	s3Service *S3Service
	ffmpegPath string
}

func NewVideoService(s3Service *S3Service) *VideoService {
	return &VideoService{
		s3Service:  s3Service,
		ffmpegPath: config.AppConfig.FFmpegPath,
	}
}

// Single best quality configuration
var BestQualityConfig = struct {
	Width   int
	Height  int
	Bitrate string
	CRF     string
	Preset  string
}{
	Width:   1920,  // 1080p as default
	Height:  1080,
	Bitrate: "5000k",
	CRF:     "18",  // High quality
	Preset:  "slow", // Best quality preset
}

// ProcessVideo processes video to single best quality
func (v *VideoService) ProcessVideo(inputPath, mediaID string) (*models.VideoVariant, error) {
	tempDir := "temp"
	
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Get video info
	info, err := v.getVideoInfo(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %v", err)
	}

	// Use source resolution if it's smaller than target
	targetWidth := BestQualityConfig.Width
	targetHeight := BestQualityConfig.Height
	
	if info.Width < targetWidth || info.Height < targetHeight {
		targetWidth = info.Width
		targetHeight = info.Height
		log.Printf("📊 Using source resolution: %dx%d (smaller than target)", targetWidth, targetHeight)
	} else {
		log.Printf("📊 Using target resolution: %dx%d", targetWidth, targetHeight)
	}

	// Create single high-quality variant
	variant, err := v.createBestQualityVariant(inputPath, mediaID, targetWidth, targetHeight, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create best quality variant: %v", err)
	}

	return variant, nil
}

func (v *VideoService) createBestQualityVariant(inputPath, mediaID string, width, height int, tempDir string) (*models.VideoVariant, error) {
	outputFilename := fmt.Sprintf("%s_best_quality.mp4", mediaID)
	outputPath := filepath.Join(tempDir, outputFilename)

	log.Printf("🎬 Creating best quality video: %dx%d", width, height)

	// FFmpeg command for best quality video transcoding
	cmd := exec.Command(v.ffmpegPath,
		"-i", inputPath,
		"-c:v", "libx264",           // H.264 video codec
		"-preset", BestQualityConfig.Preset, // Best quality preset
		"-crf", BestQualityConfig.CRF,       // High quality CRF
		"-c:a", "aac",               // AAC audio codec
		"-b:a", "192k",              // High audio bitrate
		"-vf", fmt.Sprintf("scale=%d:%d:flags=lanczos:force_original_aspect_ratio=decrease", width, height), // Best scaling with aspect ratio preservation
		"-maxrate", BestQualityConfig.Bitrate, // Max bitrate
		"-bufsize", fmt.Sprintf("%dk", parseBitrate(BestQualityConfig.Bitrate)/2), // Buffer size
		"-movflags", "+faststart",   // Optimize for web streaming
		"-profile:v", "high",        // High profile for best compatibility
		"-level", "4.1",             // H.264 level 4.1
		"-pix_fmt", "yuv420p",       // Standard pixel format
		"-threads", "0",             // Use all available CPU threads
		"-f", "mp4",                 // Force MP4 format
		"-y",                        // Overwrite output file
		outputPath,
	)

	// Run FFmpeg
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %v", err)
	}

	// Get file size
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	// Upload to S3
	file, err := os.Open(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open output file: %v", err)
	}
	defer file.Close()

	// Upload to videos/best_quality/ directory
	url, err := v.s3Service.UploadFileFromReader(file, outputFilename, "video/mp4", "videos/best_quality")
	if err != nil {
		return nil, fmt.Errorf("failed to upload best quality video to S3: %v", err)
	}

	// Create video variant
	variant := &models.VideoVariant{
		ID:        generateVideoUniqueID(),
		MediaID:   mediaID,
		Quality:   "best_quality", // Single quality
		Width:     width,
		Height:    height,
		Bitrate:   parseBitrate(BestQualityConfig.Bitrate),
		URL:       url,
		Size:      fileInfo.Size(),
		CreatedAt: time.Now(),
	}

	return variant, nil
}

func (v *VideoService) getVideoInfo(inputPath string) (*VideoInfo, error) {
	cmd := exec.Command(v.ffmpegPath,
		"-i", inputPath,
		"-f", "null",
		"-",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg probe failed: %v", err)
	}

	// Parse video info from FFmpeg output
	info := &VideoInfo{}
	outputStr := string(output)

	// Extract resolution
	if strings.Contains(outputStr, "Video:") {
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Video:") {
				// Look for resolution pattern like "1920x1080"
				if idx := strings.Index(line, " "); idx != -1 {
					parts := strings.Fields(line[idx:])
					for _, part := range parts {
						if strings.Contains(part, "x") {
							res := strings.Split(part, "x")
							if len(res) == 2 {
								if width, err := strconv.Atoi(res[0]); err == nil {
									info.Width = width
								}
								if height, err := strconv.Atoi(res[1]); err == nil {
									info.Height = height
								}
								break
							}
						}
					}
				}
				break
			}
		}
	}

	// Extract duration
	if strings.Contains(outputStr, "Duration:") {
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Duration:") {
				// Parse duration like "00:01:30.00"
				durationStr := strings.Split(line, "Duration: ")[1]
				durationStr = strings.Split(durationStr, ",")[0]
				info.Duration = parseDuration(durationStr)
				break
			}
		}
	}

	return info, nil
}

func (v *VideoService) CreateThumbnail(inputPath, mediaID string) (string, error) {
	tempDir := "temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	thumbnailFilename := fmt.Sprintf("%s_thumb.jpg", mediaID)
	thumbnailPath := filepath.Join(tempDir, thumbnailFilename)

	// FFmpeg command to create thumbnail at 10 seconds
	cmd := exec.Command(v.ffmpegPath,
		"-i", inputPath,
		"-ss", "00:00:10",
		"-vframes", "1",
		"-vf", "scale=320:180",
		"-y",
		thumbnailPath,
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create thumbnail: %v", err)
	}

	// Upload thumbnail to S3
	file, err := os.Open(thumbnailPath)
	if err != nil {
		return "", fmt.Errorf("failed to open thumbnail: %v", err)
	}
	defer file.Close()

	url, err := v.s3Service.UploadFileFromReader(file, thumbnailFilename, "image/jpeg", "thumbnails")
	if err != nil {
		return "", fmt.Errorf("failed to upload thumbnail: %v", err)
	}

	return url, nil
}

// ProcessVideoForStreaming processes a video for streaming with best quality only
func (v *VideoService) ProcessVideoForStreaming(media *models.Media) error {
	log.Printf("🎥 Starting video processing for streaming: %s", media.ID)
	
	// Create temp directory for processing
	tempDir := "temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Download video from S3 to local temp storage
	localVideoPath := filepath.Join(tempDir, "original_video.mp4")
	if err := v.downloadVideoFromS3(media.URL, localVideoPath); err != nil {
		return fmt.Errorf("failed to download video from S3: %v", err)
	}
	
	log.Printf("📥 Downloaded video to: %s", localVideoPath)
	
	// Get video info to determine target resolution
	info, err := v.getVideoInfo(localVideoPath)
	if err != nil {
		return fmt.Errorf("failed to get video info: %v", err)
	}
	
	// Use source resolution if it's smaller than target
	targetWidth := BestQualityConfig.Width
	targetHeight := BestQualityConfig.Height
	
	if info.Width < targetWidth || info.Height < targetHeight {
		targetWidth = info.Width
		targetHeight = info.Height
		log.Printf("📊 Using source resolution: %dx%d (smaller than target)", targetWidth, targetHeight)
	} else {
		log.Printf("📊 Using target resolution: %dx%d", targetWidth, targetHeight)
	}
	
	// Create single best quality variant
	variant, err := v.createBestQualityVariant(localVideoPath, media.ID, targetWidth, targetHeight, tempDir)
	if err != nil {
		log.Printf("❌ Failed to create best quality variant: %v", err)
		return err
	}
	
	log.Printf("✅ Created best quality variant: %s", variant.URL)
	log.Printf("✅ Video processing completed for: %s", media.ID)
	return nil
}

// downloadVideoFromS3 downloads a video from S3 to local storage
func (v *VideoService) downloadVideoFromS3(s3URL, localPath string) error {
	// Extract S3 key from URL
	// URL format: https://bucket.s3.region.amazonaws.com/key
	key := v.s3Service.ExtractKeyFromURL(s3URL)
	if key == "" {
		return fmt.Errorf("failed to extract S3 key from URL: %s", s3URL)
	}
	
	// Get object from S3
	result, err := v.s3Service.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(v.s3Service.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object from S3: %v", err)
	}
	defer result.Body.Close()
	
	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer file.Close()
	
	// Copy content from S3 to local file
	_, err = io.Copy(file, result.Body)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}
	
	return nil
}

// GetVideoVariants returns video variants for streaming (now only best quality)
func (v *VideoService) GetVideoVariants(mediaID string) ([]models.VideoVariant, error) {
	log.Printf("📺 Getting video variants for: %s", mediaID)
	
	// For single quality, return empty slice
	// The best quality video is accessed directly via /stream endpoint
	return []models.VideoVariant{}, nil
}

type VideoInfo struct {
	Width    int
	Height   int
	Duration float64
}

func parseBitrate(bitrateStr string) int {
	// Remove 'k' suffix and convert to int
	bitrateStr = strings.TrimSuffix(bitrateStr, "k")
	if bitrate, err := strconv.Atoi(bitrateStr); err == nil {
		return bitrate
	}
	return 0
}

func parseDuration(durationStr string) float64 {
	// Parse duration like "00:01:30.00"
	parts := strings.Split(durationStr, ":")
	if len(parts) >= 3 {
		hours, _ := strconv.Atoi(parts[0])
		minutes, _ := strconv.Atoi(parts[1])
		seconds, _ := strconv.ParseFloat(parts[2], 64)
		return float64(hours*3600 + minutes*60) + seconds
	}
	return 0
}

func generateVideoUniqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
} 