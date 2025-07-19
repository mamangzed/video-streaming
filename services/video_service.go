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

type VideoQualityConfig struct {
	Quality models.VideoQuality
	Width   int
	Height  int
	Bitrate string
}

var VideoQualities = []VideoQualityConfig{
	{Quality: models.Quality144p, Width: 256, Height: 144, Bitrate: "100k"},
	{Quality: models.Quality240p, Width: 426, Height: 240, Bitrate: "200k"},
	{Quality: models.Quality360p, Width: 640, Height: 360, Bitrate: "500k"},
	{Quality: models.Quality480p, Width: 854, Height: 480, Bitrate: "800k"},
	{Quality: models.Quality720p, Width: 1280, Height: 720, Bitrate: "1500k"},
	{Quality: models.Quality1080p, Width: 1920, Height: 1080, Bitrate: "3000k"},
	{Quality: models.Quality1440p, Width: 2560, Height: 1440, Bitrate: "6000k"},
	{Quality: models.Quality2160p, Width: 3840, Height: 2160, Bitrate: "12000k"},
}

func (v *VideoService) ProcessVideo(inputPath, mediaID string) ([]models.VideoVariant, error) {
	var variants []models.VideoVariant
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

	// Process each quality
	for _, quality := range VideoQualities {
		// Skip if target quality is higher than source
		if quality.Width > info.Width || quality.Height > info.Height {
			continue
		}

		variant, err := v.createVideoVariant(inputPath, mediaID, quality, tempDir)
		if err != nil {
			log.Printf("Failed to create variant %s: %v", quality.Quality, err)
			continue
		}

		variants = append(variants, *variant)
	}

	return variants, nil
}

func (v *VideoService) createVideoVariant(inputPath, mediaID string, quality VideoQualityConfig, tempDir string) (*models.VideoVariant, error) {
	outputFilename := fmt.Sprintf("%s_%s.mp4", mediaID, quality.Quality)
	outputPath := filepath.Join(tempDir, outputFilename)

	// FFmpeg command for video transcoding
	cmd := exec.Command(v.ffmpegPath,
		"-i", inputPath,
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-vf", fmt.Sprintf("scale=%d:%d", quality.Width, quality.Height),
		"-b:v", quality.Bitrate,
		"-movflags", "+faststart",
		"-y", // Overwrite output file
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

	url, err := v.s3Service.UploadFileFromReader(file, outputFilename, "video/mp4", "videos/"+string(quality.Quality))
	if err != nil {
		return nil, fmt.Errorf("failed to upload variant to S3: %v", err)
	}

	// Create video variant
	variant := &models.VideoVariant{
		ID:        generateVideoUniqueID(),
		MediaID:   mediaID,
		Quality:   quality.Quality,
		Width:     quality.Width,
		Height:    quality.Height,
		Bitrate:   parseBitrate(quality.Bitrate),
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

func (v *VideoService) CreateHLSPlaylist(variants []models.VideoVariant, mediaID string) (string, error) {
	// Create HLS playlist content
	playlist := "#EXTM3U\n"
	playlist += "#EXT-X-VERSION:3\n\n"

	for _, variant := range variants {
		playlist += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n", 
			variant.Bitrate*1000, variant.Width, variant.Height)
		playlist += fmt.Sprintf("%s\n\n", variant.URL)
	}

	// Upload playlist to S3
	playlistFilename := fmt.Sprintf("%s.m3u8", mediaID)
	url, err := v.s3Service.UploadFileFromReader(
		strings.NewReader(playlist),
		playlistFilename,
		"application/vnd.apple.mpegurl",
		"playlists",
	)
	if err != nil {
		return "", fmt.Errorf("failed to upload playlist: %v", err)
	}

	return url, nil
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

// ProcessVideoForStreaming processes a video for streaming with multiple qualities
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
	
	// Process video with different qualities
	qualities := []VideoQualityConfig{
		{Quality: models.Quality360p, Width: 640, Height: 360, Bitrate: "800k"},
		{Quality: models.Quality720p, Width: 1280, Height: 720, Bitrate: "1500k"},
		{Quality: models.Quality1080p, Width: 1920, Height: 1080, Bitrate: "3000k"},
	}
	
	var variants []models.VideoVariant
	
	for _, quality := range qualities {
		variant, err := v.createOptimizedVideoVariant(localVideoPath, media.ID, quality, tempDir)
		if err != nil {
			log.Printf("❌ Failed to create %s variant: %v", quality.Quality, err)
			continue
		}
		
		variants = append(variants, *variant)
		log.Printf("✅ Created %s variant: %s", quality.Quality, variant.URL)
	}
	
	log.Printf("✅ Video processing completed for: %s with %d variants", media.ID, len(variants))
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

// createOptimizedVideoVariant creates an optimized video variant using FFmpeg
func (v *VideoService) createOptimizedVideoVariant(inputPath, mediaID string, quality VideoQualityConfig, tempDir string) (*models.VideoVariant, error) {
	outputFilename := fmt.Sprintf("%s_%s.mp4", mediaID, quality.Quality)
	outputPath := filepath.Join(tempDir, outputFilename)
	
	log.Printf("🎬 Creating %s variant: %s", quality.Quality, outputPath)
	
	// FFmpeg command for optimal video processing
	// Using H.264 for video and AAC for audio as recommended
	cmd := exec.Command(v.ffmpegPath,
		"-i", inputPath,
		"-vcodec", "libx264",        // H.264 video codec
		"-acodec", "aac",            // AAC audio codec
		"-strict", "-2",             // Allow experimental codecs
		"-b:v", quality.Bitrate,     // Video bitrate
		"-b:a", "192k",              // Audio bitrate
		"-vf", fmt.Sprintf("scale=%d:%d", quality.Width, quality.Height), // Scale to target resolution
		"-preset", "medium",         // Encoding preset (balance between speed and quality)
		"-crf", "23",                // Constant Rate Factor (quality setting)
		"-movflags", "+faststart",   // Optimize for web streaming
		"-f", "mp4",                 // Force MP4 format
		"-y",                        // Overwrite output file
		outputPath,
	)
	
	// Capture FFmpeg output for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ FFmpeg error output: %s", string(output))
		return nil, fmt.Errorf("ffmpeg failed: %v", err)
	}
	
	log.Printf("✅ FFmpeg processing completed for %s", quality.Quality)
	
	// Get file size
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}
	
	// Upload processed video to S3
	file, err := os.Open(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open output file: %v", err)
	}
	defer file.Close()
	
	// Upload to S3 with quality-specific path
	url, err := v.s3Service.UploadFileFromReader(file, outputFilename, "video/mp4", "videos/"+string(quality.Quality))
	if err != nil {
		return nil, fmt.Errorf("failed to upload variant to S3: %v", err)
	}
	
	// Create video variant
	variant := &models.VideoVariant{
		ID:        generateVideoUniqueID(),
		MediaID:   mediaID,
		Quality:   quality.Quality,
		Width:     quality.Width,
		Height:    quality.Height,
		Bitrate:   parseBitrate(quality.Bitrate),
		URL:       url,
		Size:      fileInfo.Size(),
		CreatedAt: time.Now(),
	}
	
	return variant, nil
}

// GetVideoVariants returns video variants for streaming
func (v *VideoService) GetVideoVariants(mediaID string) ([]models.VideoVariant, error) {
	log.Printf("📺 Getting video variants for: %s", mediaID)
	
	// For now, return empty slice
	// In a real implementation, you would:
	// 1. Query database for video variants
	// 2. Return the list of available qualities
	
	return []models.VideoVariant{}, nil
} 