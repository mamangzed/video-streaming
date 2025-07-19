package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"api-s3/config"
	"api-s3/handlers"
	"api-s3/models"
	"api-s3/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestServer() (*gin.Engine, *handlers.MediaHandler) {
	// Load test config
	config.LoadConfig()
	
	// Initialize services
	s3Service, _ := services.NewS3Service()
	videoService := services.NewVideoService(s3Service)
	
	// Create handler
	handler := handlers.NewMediaHandler(s3Service, videoService)
	
	// Setup router
	router := gin.New()
	router.Use(gin.Logger())
	
	// Setup routes
	api := router.Group("/api/v1")
	{
		api.POST("/upload", handler.UploadMedia)
		api.DELETE("/media/:id", handler.DeleteMedia)
		api.GET("/media/:id/stream", handler.GetVideoStream)
		api.GET("/media/:id/stream/:quality", handler.StreamVideo)
		api.GET("/media/:id/thumbnail", handler.GetThumbnail)
	}
	
	return router, handler
}

func createTestFile(filename string, content string) (*os.File, error) {
	// Create temp directory
	tempDir := "test-temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, err
	}
	
	// Create test file
	filePath := filepath.Join(tempDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	
	// Write content
	_, err = file.WriteString(content)
	if err != nil {
		file.Close()
		return nil, err
	}
	
	// Reset file pointer
	file.Seek(0, 0)
	return file, nil
}

func TestHealthCheck(t *testing.T) {
	router, _ := setupTestServer()
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, 200, w.Code)
}

func TestUploadImage(t *testing.T) {
	router, _ := setupTestServer()
	
	// Create test image file
	testFile, err := createTestFile("test.jpg", "fake image content")
	if err != nil {
		t.Fatal(err)
	}
	defer testFile.Close()
	defer os.Remove(testFile.Name())
	
	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", "test.jpg")
	if err != nil {
		t.Fatal(err)
	}
	
	_, err = io.Copy(part, testFile)
	if err != nil {
		t.Fatal(err)
	}
	
	writer.Close()
	
	// Make request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)
	
	// Check response
	assert.Equal(t, 200, w.Code)
	
	var response models.UploadResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "image", string(response.Media.MediaType))
}

func TestUploadVideo(t *testing.T) {
	router, _ := setupTestServer()
	
	// Create test video file
	testFile, err := createTestFile("test.mp4", "fake video content")
	if err != nil {
		t.Fatal(err)
	}
	defer testFile.Close()
	defer os.Remove(testFile.Name())
	
	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", "test.mp4")
	if err != nil {
		t.Fatal(err)
	}
	
	_, err = io.Copy(part, testFile)
	if err != nil {
		t.Fatal(err)
	}
	
	writer.Close()
	
	// Make request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)
	
	// Check response
	assert.Equal(t, 200, w.Code)
	
	var response models.UploadResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "video", string(response.Media.MediaType))
}

func TestGetVideoStream(t *testing.T) {
	router, _ := setupTestServer()
	
	// Test with mock media ID
	mediaID := "test-media-id"
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/media/%s/stream", mediaID), nil)
	router.ServeHTTP(w, req)
	
	// Check response
	assert.Equal(t, 200, w.Code)
	
	var response models.VideoStreamResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.Variants)
}

func TestStreamVideo(t *testing.T) {
	router, _ := setupTestServer()
	
	// Test with mock media ID and quality
	mediaID := "test-media-id"
	quality := "720p"
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/media/%s/stream/%s", mediaID, quality), nil)
	router.ServeHTTP(w, req)
	
	// Should redirect to presigned URL
	assert.Equal(t, 307, w.Code)
}

func TestGetThumbnail(t *testing.T) {
	router, _ := setupTestServer()
	
	// Test with mock media ID
	mediaID := "test-media-id"
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/media/%s/thumbnail", mediaID), nil)
	router.ServeHTTP(w, req)
	
	// Should redirect to presigned URL
	assert.Equal(t, 307, w.Code)
}

func TestDeleteMedia(t *testing.T) {
	router, _ := setupTestServer()
	
	// Test with mock media ID
	mediaID := "test-media-id"
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/media/%s", mediaID), nil)
	router.ServeHTTP(w, req)
	
	// Check response
	assert.Equal(t, 200, w.Code)
	
	var response models.DeleteResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
}

func TestInvalidFileType(t *testing.T) {
	router, _ := setupTestServer()
	
	// Create test file with invalid extension
	testFile, err := createTestFile("test.txt", "text content")
	if err != nil {
		t.Fatal(err)
	}
	defer testFile.Close()
	defer os.Remove(testFile.Name())
	
	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	
	_, err = io.Copy(part, testFile)
	if err != nil {
		t.Fatal(err)
	}
	
	writer.Close()
	
	// Make request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)
	
	// Should return error
	assert.Equal(t, 400, w.Code)
	
	var response models.UploadResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestMissingFile(t *testing.T) {
	router, _ := setupTestServer()
	
	// Make request without file
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/upload", nil)
	router.ServeHTTP(w, req)
	
	// Should return error
	assert.Equal(t, 400, w.Code)
	
	var response models.UploadResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

// Cleanup test files
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	
	// Cleanup
	os.RemoveAll("test-temp")
	os.RemoveAll("temp")
	
	os.Exit(code)
} 