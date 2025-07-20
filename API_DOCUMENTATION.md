# API S3 - Documentation

## Overview

API S3 adalah layanan upload dan streaming video/foto yang terintegrasi dengan AWS S3. Layanan ini mendukung upload file, delete, dan streaming video dengan berbagai kualitas seperti YouTube.

**Base URL:** `http://localhost:8080` (development) atau domain Anda (production)

## Authentication

Saat ini API tidak memerlukan authentication, namun untuk production disarankan menggunakan API key atau JWT token.

## Response Format

Semua response menggunakan format JSON dengan struktur:

```json
{
  "success": true/false,
  "message": "Response message",
  "data": {}, // Optional data object
  "media": {} // For upload responses
}
```

## Endpoints

### 1. Health Check

**GET** `/health`

Memeriksa status kesehatan API.

**Response:**
```json
{
  "status": "ok",
  "message": "API is running"
}
```

### 2. Upload Media (with Video Optimization)

**POST** `/api/v1/upload`

Upload file dengan optimasi video otomatis menggunakan FFmpeg.

**Request:**
- **Content-Type:** `multipart/form-data`
- **Body:** Form data dengan field `file`

**Parameters:**
- `file` (required): File yang akan diupload (image/video)

**Response Success (Image):**
```json
{
  "success": true,
  "message": "File uploaded successfully",
  "media": {
    "id": "uuid-string",
    "filename": "image.jpg",
    "original_name": "image.jpg",
    "media_type": "image",
    "mime_type": "image/jpeg",
    "size": 1024000,
    "url": "https://bucket.s3.region.amazonaws.com/media/uuid/image.jpg",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

**Response Success (Video - Processing):**
```json
{
  "success": true,
  "message": "Video upload started. Processing in background. Check progress at /api/v1/media/{id}/progress",
  "media": {
    "id": "uuid-string",
    "filename": "video.mp4",
    "original_name": "video.mp4",
    "media_type": "video",
    "mime_type": "video/mp4",
    "size": 52428800,
    "url": "", // Will be populated when processing completes
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

**Response Error:**
```json
{
  "success": false,
  "message": "Error description"
}
```

**Status Codes:**
- `200`: Upload berhasil (image/non-video)
- `202`: Upload diterima, video sedang diproses
- `400`: Bad request (file tidak valid)
- `413`: File terlalu besar
- `500`: Internal server error

### 3. Upload Direct (No Optimization)

**POST** `/api/v1/upload-direct`

Upload file langsung ke S3 tanpa optimasi video.

**Request:**
- **Content-Type:** `multipart/form-data`
- **Body:** Form data dengan field `file`

**Parameters:**
- `file` (required): File yang akan diupload

**Response:** Sama seperti endpoint `/upload` tapi tanpa video processing.

### 4. Upload Large File (No Size Limit)

**POST** `/api/v1/upload-large`

Upload file besar tanpa batasan ukuran.

**Request:**
- **Content-Type:** `multipart/form-data`
- **Body:** Form data dengan field `file`

**Parameters:**
- `file` (required): File yang akan diupload

**Response:** Sama seperti endpoint `/upload-direct`.

### 5. Upload Local (Testing)

**POST** `/api/v1/upload-local`

Upload file ke local storage untuk testing tanpa S3.

**Request:**
- **Content-Type:** `multipart/form-data`
- **Body:** Form data dengan field `file`

**Parameters:**
- `file` (required): File yang akan diupload

**Response:** Sama seperti endpoint `/upload`.

### 6. Get Media Info

**GET** `/api/v1/media/{id}`

Mendapatkan informasi detail tentang media.

**Parameters:**
- `id` (path): ID media

**Response Success:**
```json
{
  "success": true,
  "message": "Media info retrieved",
  "media": {
    "id": "uuid-string",
    "filename": "video.mp4",
    "original_name": "video.mp4",
    "media_type": "video",
    "mime_type": "video/mp4",
    "size": 52428800,
    "url": "https://bucket.s3.region.amazonaws.com/media/uuid/video_optimized.mp4",
    "width": 1920,
    "height": 1080,
    "duration": 120.5,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

**Response Error:**
```json
{
  "success": false,
  "message": "Media not found"
}
```

**Status Codes:**
- `200`: Success
- `404`: Media tidak ditemukan
- `500`: Internal server error

### 7. Get Processing Progress

**GET** `/api/v1/media/{id}/progress`

Mendapatkan progress video processing.

**Parameters:**
- `id` (path): ID media

**Response Success:**
```json
{
  "success": true,
  "message": "Processing video...",
  "progress": 75,
  "status": "processing",
  "estimated_time": "2 minutes remaining"
}
```

**Response Completed:**
```json
{
  "success": true,
  "message": "Video processing completed",
  "progress": 100,
  "status": "completed"
}
```

**Response Failed:**
```json
{
  "success": false,
  "message": "Video processing failed",
  "status": "failed"
}
```

**Status Codes:**
- `200`: Success
- `404`: Media tidak ditemukan
- `500`: Internal server error

### 8. Stream Video

**GET** `/api/v1/media/{id}/stream/{quality}`

Stream video dengan kualitas tertentu.

**Parameters:**
- `id` (path): ID media
- `quality` (path): Kualitas video (144p, 240p, 360p, 480p, 720p, 1080p, 1440p, 2160p)

**Response:**
- **Content-Type:** `video/mp4`
- **Body:** Video stream dengan HTTP Range support

**Headers:**
- `Accept-Ranges: bytes`
- `Content-Length: {file_size}`
- `Content-Range: bytes {start}-{end}/{total}` (untuk range requests)

**Status Codes:**
- `200`: Success
- `206`: Partial content (range request)
- `404`: Video tidak ditemukan
- `500`: Internal server error

### 9. Get Video Stream Info

**GET** `/api/v1/media/{id}/stream`

Mendapatkan informasi streaming video dengan berbagai kualitas.

**Parameters:**
- `id` (path): ID media

**Response Success:**
```json
{
  "success": true,
  "message": "Video streaming information retrieved",
  "variants": [
    {
      "id": "variant-uuid",
      "media_id": "media-uuid",
      "quality": "720p",
      "width": 1280,
      "height": 720,
      "bitrate": 2500,
      "url": "/api/v1/media/uuid/stream/720p",
      "size": 52428800,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

### 10. Get Thumbnail

**GET** `/api/v1/media/{id}/thumbnail`

Mendapatkan thumbnail video.

**Parameters:**
- `id` (path): ID media

**Response:**
- **Content-Type:** `image/jpeg`
- **Body:** Thumbnail image

**Status Codes:**
- `200`: Success
- `404`: Thumbnail tidak ditemukan
- `503`: Video service tidak tersedia

### 11. Delete Media

**DELETE** `/api/v1/media/{id}`

Menghapus media dari S3.

**Parameters:**
- `id` (path): ID media

**Response Success:**
```json
{
  "success": true,
  "message": "Media deleted successfully"
}
```

**Response Error:**
```json
{
  "success": false,
  "message": "Failed to delete media"
}
```

**Status Codes:**
- `200`: Success
- `404`: Media tidak ditemukan
- `500`: Internal server error

## File Types Supported

### Images
- JPEG (.jpg, .jpeg)
- PNG (.png)
- GIF (.gif)
- WebP (.webp)
- BMP (.bmp)

### Videos
- MP4 (.mp4)
- MOV (.mov)
- AVI (.avi)
- MKV (.mkv)
- WebM (.webm)
- FLV (.flv)
- WMV (.wmv)
- 3GP (.3gp)

## Video Processing

### Supported Qualities
- **144p**: 256x144, 200kbps
- **240p**: 426x240, 400kbps
- **360p**: 640x360, 800kbps
- **480p**: 854x480, 1200kbps
- **720p**: 1280x720, 2500kbps
- **1080p**: 1920x1080, 5000kbps
- **1440p**: 2560x1440, 8000kbps
- **2160p**: 3840x2160, 15000kbps

### Processing Settings
- **Codec:** H.264 (libx264)
- **Audio:** AAC, 192kbps
- **Preset:** Berdasarkan ukuran file
  - File kecil (<50MB): `slow` + CRF 18
  - File medium (50-100MB): `medium` + CRF 20
  - File besar (>100MB): `fast` + CRF 22
- **Scaling:** Lanczos algorithm dengan aspect ratio preservation
- **Profile:** High profile, Level 4.1

## Error Codes

| Code | Description |
|------|-------------|
| 400 | Bad Request - File tidak valid atau parameter salah |
| 413 | Payload Too Large - File terlalu besar |
| 404 | Not Found - Media tidak ditemukan |
| 500 | Internal Server Error - Server error |
| 503 | Service Unavailable - S3 service tidak tersedia |

## Rate Limiting

- **API endpoints:** 10 requests/second
- **Upload endpoints:** 2 requests/second
- **Streaming:** Tidak ada limit

## CORS Headers

API mendukung CORS dengan headers:
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization`

## Examples

### Upload Image dengan cURL

```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@image.jpg"
```

### Upload Video dengan cURL

```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@video.mp4"
```

### Check Processing Progress

```bash
curl http://localhost:8080/api/v1/media/{media-id}/progress
```

### Stream Video

```bash
curl -H "Range: bytes=0-" http://localhost:8080/api/v1/media/{media-id}/stream/720p
```

### JavaScript Upload Example

```javascript
const formData = new FormData();
formData.append('file', fileInput.files[0]);

fetch('/api/v1/upload', {
  method: 'POST',
  body: formData
})
.then(response => response.json())
.then(data => {
  if (data.success) {
    console.log('Upload successful:', data.media);
  } else {
    console.error('Upload failed:', data.message);
  }
});
```

### JavaScript Progress Tracking

```javascript
function checkProgress(mediaId) {
  fetch(`/api/v1/media/${mediaId}/progress`)
    .then(response => response.json())
    .then(data => {
      if (data.success) {
        console.log(`Progress: ${data.progress}%`);
        if (data.status === 'completed') {
          console.log('Processing completed!');
        } else if (data.status === 'processing') {
          setTimeout(() => checkProgress(mediaId), 3000);
        }
      }
    });
}
```

## Configuration

### Environment Variables

```env
# AWS Configuration
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
AWS_S3_BUCKET=your-bucket-name

# Server Configuration
PORT=8080
MAX_FILE_SIZE=500MB

# Video Processing
FFMPEG_PATH=/usr/bin/ffmpeg
ENABLE_VIDEO_PROCESSING=true
```

## Monitoring

### Health Check
```bash
curl http://localhost:8080/health
```

### Logs
Aplikasi menampilkan log untuk:
- File upload progress
- Video processing status
- S3 operations
- Error messages

## Security Considerations

1. **File Validation:** Semua file divalidasi berdasarkan MIME type dan extension
2. **Size Limits:** Batasan ukuran file untuk mencegah abuse
3. **CORS:** Konfigurasi CORS yang aman
4. **Rate Limiting:** Pembatasan request rate
5. **S3 Security:** Menggunakan presigned URLs untuk akses file

## Troubleshooting

### Common Issues

1. **Upload Failed (413)**
   - File terlalu besar
   - Periksa `MAX_FILE_SIZE` setting

2. **Video Processing Failed**
   - FFmpeg tidak terinstall
   - Disk space tidak cukup
   - File video corrupt

3. **S3 Upload Failed**
   - AWS credentials salah
   - Bucket tidak ada atau tidak accessible
   - Network connectivity issues

4. **Streaming Not Working**
   - Video belum selesai diproses
   - File tidak ditemukan di S3
   - CORS configuration

### Debug Mode

Untuk debugging, set log level ke debug:
```bash
export LOG_LEVEL=debug
go run main.go
```

## Support

Untuk bantuan dan pertanyaan:
- Create issue di GitHub repository
- Email: support@example.com
- Documentation: [Wiki](https://github.com/username/api-s3/wiki)

## Version History

- **v1.0.0**: Initial release
  - Basic upload functionality
  - Video processing with FFmpeg
  - S3 integration
  - Streaming support

## License

MIT License - see LICENSE file for details 