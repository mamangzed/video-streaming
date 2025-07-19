# API S3 - Video & Image Upload Service

API S3 adalah layanan upload dan streaming video/foto yang terintegrasi dengan AWS S3. Layanan ini mendukung upload file, delete, dan streaming video dengan berbagai kualitas seperti YouTube.

## Fitur Utama

- ✅ **Upload File**: Mendukung upload gambar dan video
- ✅ **Delete File**: Hapus file dari S3
- ✅ **Video Processing**: Transcoding video ke berbagai kualitas (144p, 240p, 360p, 480p, 720p, 1080p, 1440p, 2160p)
- ✅ **Video Streaming**: Streaming video dengan HLS (HTTP Live Streaming)
- ✅ **Thumbnail Generation**: Generate thumbnail otomatis untuk video
- ✅ **Presigned URLs**: URL aman untuk akses file
- ✅ **CORS Support**: Cross-origin resource sharing

## Teknologi yang Digunakan

- **Golang** - Backend API
- **Gin Framework** - HTTP router dan middleware
- **AWS SDK v2** - Integrasi dengan AWS S3
- **FFmpeg** - Video processing dan transcoding
- **HLS** - Video streaming protocol

## Prerequisites

1. **Go 1.21+** - [Download Go](https://golang.org/dl/)
2. **FFmpeg** - [Download FFmpeg](https://ffmpeg.org/download.html)
3. **AWS Account** dengan S3 bucket
4. **AWS Credentials** (Access Key ID dan Secret Access Key)

## Instalasi

1. **Clone repository**
```bash
git clone <repository-url>
cd api-s3
```

2. **Install dependencies**
```bash
go mod tidy
```

3. **Setup environment variables**
```bash
cp env.example .env
```

Edit file `.env` dengan konfigurasi AWS Anda:
```env
# AWS Configuration
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your_access_key_here
AWS_SECRET_ACCESS_KEY=your_secret_key_here
AWS_S3_BUCKET=your-bucket-name

# Server Configuration
PORT=8080
MAX_FILE_SIZE=100MB

# Video Processing Configuration
FFMPEG_PATH=/usr/bin/ffmpeg
ENABLE_VIDEO_PROCESSING=true
```

4. **Install FFmpeg**

**Windows:**
```bash
# Download dari https://ffmpeg.org/download.html
# Atau gunakan chocolatey
choco install ffmpeg
```

**macOS:**
```bash
brew install ffmpeg
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt update
sudo apt install ffmpeg
```

5. **Run aplikasi**
```bash
go run main.go
```

## API Endpoints

### 1. Upload File
```http
POST /api/v1/upload
Content-Type: multipart/form-data

file: [file]
```

**Response:**
```json
{
  "success": true,
  "message": "File uploaded successfully",
  "media": {
    "id": "uuid",
    "filename": "filename.ext",
    "original_name": "original_name.ext",
    "media_type": "video|image",
    "mime_type": "video/mp4",
    "size": 1024000,
    "url": "https://bucket.s3.region.amazonaws.com/path/file.ext",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

### 2. Delete File
```http
DELETE /api/v1/media/{id}
```

**Response:**
```json
{
  "success": true,
  "message": "Media deleted successfully"
}
```

### 3. Get Video Stream Info
```http
GET /api/v1/media/{id}/stream
```

**Response:**
```json
{
  "success": true,
  "message": "Video stream information retrieved",
  "variants": [
    {
      "id": "1",
      "media_id": "uuid",
      "quality": "720p",
      "width": 1280,
      "height": 720,
      "bitrate": 1500,
      "url": "https://bucket.s3.region.amazonaws.com/videos/720p/file_720p.mp4",
      "size": 2048000,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "master_url": "https://bucket.s3.region.amazonaws.com/playlists/file.m3u8"
}
```

### 4. Stream Video
```http
GET /api/v1/media/{id}/stream/{quality}
```

**Qualities available:** 144p, 240p, 360p, 480p, 720p, 1080p, 1440p, 2160p

### 5. Get Thumbnail
```http
GET /api/v1/media/{id}/thumbnail
```

### 6. Health Check
```http
GET /health
```

## Struktur S3 Bucket

Setelah upload, file akan disimpan dalam struktur folder berikut:

```
bucket/
├── images/
│   └── [image files]
├── videos/
│   ├── original/
│   │   └── [original video files]
│   ├── 144p/
│   │   └── [144p video variants]
│   ├── 240p/
│   │   └── [240p video variants]
│   ├── 360p/
│   │   └── [360p video variants]
│   ├── 480p/
│   │   └── [480p video variants]
│   ├── 720p/
│   │   └── [720p video variants]
│   ├── 1080p/
│   │   └── [1080p video variants]
│   ├── 1440p/
│   │   └── [1440p video variants]
│   └── 2160p/
│       └── [2160p video variants]
├── thumbnails/
│   └── [video thumbnails]
└── playlists/
    └── [HLS playlists]
```

## Video Processing

Ketika video diupload, sistem akan:

1. **Upload original video** ke folder `videos/original/`
2. **Generate thumbnail** dari frame ke-10 video
3. **Transcode video** ke berbagai kualitas yang didukung
4. **Create HLS playlist** untuk adaptive streaming
5. **Upload semua file** ke S3 dengan struktur yang terorganisir

### Supported Video Formats
- MP4, AVI, MOV, WMV, FLV, WebM, MKV, M4V

### Supported Image Formats
- JPG, JPEG, PNG, GIF, BMP, WebP

## Konfigurasi

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AWS_REGION` | AWS region | `us-east-1` |
| `AWS_ACCESS_KEY_ID` | AWS access key | Required |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | Required |
| `AWS_S3_BUCKET` | S3 bucket name | Required |
| `PORT` | Server port | `8080` |
| `MAX_FILE_SIZE` | Maximum file size | `100MB` |
| `FFMPEG_PATH` | FFmpeg executable path | `/usr/bin/ffmpeg` |
| `ENABLE_VIDEO_PROCESSING` | Enable video processing | `true` |

### Video Quality Settings

Kualitas video dapat dikonfigurasi di `services/video_service.go`:

```go
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
```

## Testing

### Menggunakan cURL

1. **Upload file:**
```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@/path/to/your/video.mp4"
```

2. **Get stream info:**
```bash
curl http://localhost:8080/api/v1/media/{media-id}/stream
```

3. **Delete file:**
```bash
curl -X DELETE http://localhost:8080/api/v1/media/{media-id}
```

### Menggunakan Postman

1. Import collection dari file `postman_collection.json` (jika tersedia)
2. Set environment variables
3. Test semua endpoints

## Deployment

### Docker (Recommended)

1. **Build image:**
```bash
docker build -t api-s3 .
```

2. **Run container:**
```bash
docker run -p 8080:8080 \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=your_key \
  -e AWS_SECRET_ACCESS_KEY=your_secret \
  -e AWS_S3_BUCKET=your_bucket \
  api-s3
```

### Production Deployment

1. **Setup reverse proxy** (Nginx/Apache)
2. **Configure SSL/TLS**
3. **Setup monitoring** (Prometheus/Grafana)
4. **Configure logging** (ELK Stack)
5. **Setup CI/CD** pipeline

## Troubleshooting

### Common Issues

1. **FFmpeg not found**
   - Pastikan FFmpeg terinstall dan path benar
   - Set `FFMPEG_PATH` environment variable

2. **AWS credentials error**
   - Periksa AWS credentials
   - Pastikan bucket ada dan accessible

3. **File upload failed**
   - Periksa file size limit
   - Pastikan file format didukung

4. **Video processing failed**
   - Periksa FFmpeg installation
   - Periksa disk space untuk temp files

### Logs

Aplikasi akan menampilkan log untuk:
- File upload progress
- Video processing status
- S3 operations
- Error messages

## Contributing

1. Fork repository
2. Create feature branch
3. Commit changes
4. Push to branch
5. Create Pull Request

## License

MIT License - see LICENSE file for details

## Support

Untuk bantuan dan pertanyaan:
- Create issue di GitHub
- Email: support@example.com
- Documentation: [Wiki](https://github.com/username/api-s3/wiki) 