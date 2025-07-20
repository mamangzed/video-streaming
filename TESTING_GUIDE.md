# Testing Guide - API S3

## Overview

Guide ini menjelaskan cara testing API S3 dengan berbagai tools dan skenario.

## 🧪 Testing Tools

### 1. Frontend Interface
- **URL**: `http://localhost:8080`
- **Features**: Drag & drop upload, progress tracking, video player
- **Best for**: Manual testing dan demo

### 2. Postman Collection
- **File**: `postman_collection.json`
- **Environment**: `postman_environment.json`
- **Best for**: API testing dan automation

### 3. cURL Commands
- **Best for**: Quick testing dan scripting

### 4. Automated Tests
- **File**: `test/main_test.go`
- **Command**: `go test ./test`
- **Best for**: Unit testing dan CI/CD

## 📋 Test Scenarios

### 1. Health Check
```bash
# cURL
curl http://localhost:8080/health

# Expected Response
{
  "status": "ok",
  "message": "API is running"
}
```

### 2. Image Upload Test

#### Small Image (< 1MB)
```bash
# cURL
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@test-images/small.jpg"

# Expected Response
{
  "success": true,
  "message": "File uploaded successfully",
  "media": {
    "id": "uuid",
    "media_type": "image",
    "url": "https://bucket.s3.region.amazonaws.com/..."
  }
}
```

#### Large Image (> 10MB)
```bash
# Test with large image
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@test-images/large.jpg"

# Should handle large files properly
```

### 3. Video Upload Test

#### Small Video (< 50MB)
```bash
# cURL
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@test-videos/small.mp4"

# Expected Response (202 Accepted)
{
  "success": true,
  "message": "Video upload started. Processing in background...",
  "media": {
    "id": "uuid",
    "media_type": "video",
    "url": "" // Empty until processing completes
  }
}
```

#### Large Video (> 100MB)
```bash
# Test with large video
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@test-videos/large.mp4"

# Should show progress tracking
```

### 4. Progress Tracking Test

```bash
# 1. Upload video first
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@test-videos/sample.mp4"

# 2. Extract media_id from response
MEDIA_ID="extracted-media-id"

# 3. Check progress
curl http://localhost:8080/api/v1/media/$MEDIA_ID/progress

# Expected Response
{
  "success": true,
  "message": "Processing video...",
  "progress": 45,
  "status": "processing"
}
```

### 5. Video Streaming Test

```bash
# 1. Wait for video processing to complete
# 2. Test streaming
curl -H "Range: bytes=0-" \
  http://localhost:8080/api/v1/media/$MEDIA_ID/stream/720p

# Should return video stream with headers:
# Content-Type: video/mp4
# Accept-Ranges: bytes
# Content-Length: [file_size]
```

### 6. Error Handling Test

#### Invalid File Type
```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@test-files/document.txt"

# Expected Response (400 Bad Request)
{
  "success": false,
  "message": "Unsupported file type"
}
```

#### File Too Large
```bash
# Test with file larger than MAX_FILE_SIZE
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@test-files/very-large.mp4"

# Expected Response (413 Payload Too Large)
{
  "success": false,
  "message": "File size exceeds maximum allowed size"
}
```

#### Non-existent Media
```bash
curl http://localhost:8080/api/v1/media/non-existent-id

# Expected Response (404 Not Found)
{
  "success": false,
  "message": "Media not found"
}
```

## 🚀 Performance Testing

### 1. Upload Speed Test
```bash
# Test with different file sizes
for size in 1MB 10MB 50MB 100MB; do
  echo "Testing $size file upload..."
  time curl -X POST http://localhost:8080/api/v1/upload \
    -F "file=@test-files/${size}.mp4"
done
```

### 2. Concurrent Upload Test
```bash
# Test multiple simultaneous uploads
for i in {1..5}; do
  curl -X POST http://localhost:8080/api/v1/upload \
    -F "file=@test-files/sample$i.mp4" &
done
wait
```

### 3. Video Processing Performance
```bash
# Test video processing time
start_time=$(date +%s)
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@test-videos/large.mp4"
end_time=$(date +%s)
echo "Processing time: $((end_time - start_time)) seconds"
```

## 🔧 Postman Testing

### 1. Import Collection
1. Open Postman
2. Import `postman_collection.json`
3. Import `postman_environment.json`
4. Select "API S3 - Development" environment

### 2. Test Workflow
1. **Health Check** - Verify API is running
2. **Upload Media** - Upload a test file
3. **Get Media Info** - Check uploaded file details
4. **Get Progress** - Monitor processing (for videos)
5. **Stream Video** - Test video streaming
6. **Delete Media** - Clean up test files

### 3. Environment Variables
Postman will automatically populate:
- `media_id` - From upload responses
- `uploaded_file_url` - File URL after upload
- `processing_status` - Current processing status

## 🧪 Automated Testing

### 1. Run All Tests
```bash
go test ./test -v
```

### 2. Run Specific Test
```bash
go test ./test -v -run TestUploadImage
```

### 3. Run Tests with Coverage
```bash
go test ./test -v -cover
```

### 4. Test Categories

#### Unit Tests
- `TestHealthCheck` - Health endpoint
- `TestUploadImage` - Image upload
- `TestUploadVideo` - Video upload
- `TestInvalidFileType` - Error handling
- `TestMissingFile` - Error handling

#### Integration Tests
- `TestGetVideoStream` - Video streaming
- `TestStreamVideo` - Video quality selection
- `TestGetThumbnail` - Thumbnail generation
- `TestDeleteMedia` - File deletion

## 📊 Test Data

### Sample Files for Testing

#### Images
- `test-images/small.jpg` (100KB)
- `test-images/medium.jpg` (1MB)
- `test-images/large.jpg` (10MB)

#### Videos
- `test-videos/small.mp4` (5MB, 30 seconds)
- `test-videos/medium.mp4` (50MB, 2 minutes)
- `test-videos/large.mp4` (200MB, 5 minutes)

#### Invalid Files
- `test-files/document.txt` (text file)
- `test-files/script.js` (JavaScript file)

## 🐛 Debugging

### 1. Enable Debug Logs
```bash
export LOG_LEVEL=debug
go run main.go
```

### 2. Check Application Logs
```bash
# Monitor logs in real-time
tail -f logs/app.log

# Check error logs
grep "ERROR" logs/app.log
```

### 3. Check S3 Logs
```bash
# AWS CLI to check S3 operations
aws s3 ls s3://your-bucket/media/ --recursive
```

### 4. Check FFmpeg Logs
```bash
# Monitor FFmpeg processing
tail -f logs/ffmpeg.log
```

## 📈 Performance Benchmarks

### Expected Performance

#### Upload Speed
- **Small files (< 10MB)**: 5-10 MB/s
- **Medium files (10-100MB)**: 3-7 MB/s
- **Large files (> 100MB)**: 2-5 MB/s

#### Video Processing
- **Small videos (< 50MB)**: 1-3 minutes
- **Medium videos (50-200MB)**: 3-10 minutes
- **Large videos (> 200MB)**: 10-30 minutes

#### Response Times
- **Health check**: < 100ms
- **Upload response**: < 1s
- **Progress check**: < 500ms
- **Streaming start**: < 2s

## 🔒 Security Testing

### 1. File Type Validation
```bash
# Test with malicious files
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@malicious.exe"
```

### 2. File Size Limits
```bash
# Test with oversized files
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@very-large-file.mp4"
```

### 3. CORS Testing
```bash
# Test CORS headers
curl -H "Origin: http://malicious-site.com" \
  -H "Access-Control-Request-Method: POST" \
  -X OPTIONS http://localhost:8080/api/v1/upload
```

## 📝 Test Report Template

### Test Report
```
Test Date: [Date]
Tester: [Name]
Environment: [Development/Staging/Production]

## Test Results
- Health Check: ✅/❌
- Image Upload: ✅/❌
- Video Upload: ✅/❌
- Progress Tracking: ✅/❌
- Video Streaming: ✅/❌
- Error Handling: ✅/❌

## Performance Metrics
- Average Upload Speed: [X] MB/s
- Video Processing Time: [X] minutes
- Response Time: [X] ms

## Issues Found
- [List any issues]

## Recommendations
- [List recommendations]
```

## 🚀 Continuous Testing

### 1. GitHub Actions
```yaml
# .github/workflows/test.yml
name: API Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.21'
      - run: go test ./test -v
```

### 2. Local CI
```bash
#!/bin/bash
# test-ci.sh
set -e

echo "Running API tests..."
go test ./test -v

echo "Running performance tests..."
./scripts/performance-test.sh

echo "All tests passed! ✅"
```

## 📞 Support

Jika menemukan masalah saat testing:
1. Check logs untuk error details
2. Verify environment configuration
3. Test dengan file yang berbeda
4. Create issue di GitHub repository 