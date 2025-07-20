 package models

import (
	"time"
)

type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

type VideoQuality string

const (
	QualityBest VideoQuality = "best_quality"
)

type Media struct {
	ID          string      `json:"id"`
	Filename    string      `json:"filename"`
	OriginalName string     `json:"original_name"`
	MediaType   MediaType   `json:"media_type"`
	MimeType    string      `json:"mime_type"`
	Size        int64       `json:"size"`
	URL         string      `json:"url"`
	ThumbnailURL string     `json:"thumbnail_url,omitempty"`
	Duration    float64     `json:"duration,omitempty"`
	Width       int         `json:"width,omitempty"`
	Height      int         `json:"height,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type VideoVariant struct {
	ID          string      `json:"id"`
	MediaID     string      `json:"media_id"`
	Quality     VideoQuality `json:"quality"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Bitrate     int         `json:"bitrate"`
	URL         string      `json:"url"`
	Size        int64       `json:"size"`
	CreatedAt   time.Time   `json:"created_at"`
}

type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Media   *Media `json:"media,omitempty"`
}

type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type VideoStreamResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Variants []VideoVariant `json:"variants,omitempty"`
	MasterURL string       `json:"master_url,omitempty"`
}

type VideoProcessingJob struct {
	ID        string    `json:"id"`
	MediaID   string    `json:"media_id"`
	Status    string    `json:"status"` // pending, processing, completed, failed
	Progress  int       `json:"progress"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
} 