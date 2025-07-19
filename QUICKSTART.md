# 🚀 Quick Start Guide - API S3

## Prerequisites

- **Go 1.21+** - [Download](https://golang.org/dl/)
- **FFmpeg** - [Download](https://ffmpeg.org/download.html)
- **AWS Account** dengan S3 bucket
- **Docker** (optional) - [Download](https://docker.com/)

## 🏃‍♂️ Quick Start (5 minutes)

### 1. Clone & Setup

```bash
# Clone repository
git clone <your-repo-url>
cd api-s3

# Copy environment file
cp env.example .env
```

### 2. Configure AWS

Edit file `.env`:
```env
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your_access_key_here
AWS_SECRET_ACCESS_KEY=your_secret_key_here
AWS_S3_BUCKET=your-bucket-name
```

### 3. Install Dependencies

```bash
# Install Go dependencies
go mod tidy

# Install FFmpeg (if not installed)
# Windows: choco install ffmpeg
# macOS: brew install ffmpeg
# Linux: sudo apt install ffmpeg
```

### 4. Run Application

```bash
# Option 1: Direct run
go run main.go

# Option 2: Using script
./run.sh  # Linux/macOS
run.bat   # Windows

# Option 3: Using Make
make run

# Option 4: Using Docker
docker-compose up --build
```

### 5. Test API

```bash
# Health check
curl http://localhost:8080/health

# Upload file
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@/path/to/your/video.mp4"

# Open web interface
open http://localhost:8080
```

## 🎯 What You Get

✅ **Upload API** - Upload images and videos  
✅ **Video Processing** - Auto-transcode to multiple qualities  
✅ **Streaming** - HLS video streaming like YouTube  
✅ **Thumbnails** - Auto-generate video thumbnails  
✅ **Web Interface** - Beautiful upload interface  
✅ **S3 Integration** - Direct AWS S3 storage  

## 📁 Project Structure

```
api-s3/
├── config/          # Configuration management
├── handlers/        # HTTP request handlers
├── models/          # Data models
├── services/        # Business logic
├── routes/          # API routes
├── public/          # Static files
├── test/            # Test files
├── monitoring/      # Monitoring configs
├── nginx/           # Nginx config
├── k8s/             # Kubernetes configs
└── main.go          # Application entry point
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AWS_REGION` | AWS region | `us-east-1` |
| `AWS_ACCESS_KEY_ID` | AWS access key | Required |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | Required |
| `AWS_S3_BUCKET` | S3 bucket name | Required |
| `PORT` | Server port | `8080` |
| `MAX_FILE_SIZE` | Max file size | `100MB` |
| `FFMPEG_PATH` | FFmpeg path | `/usr/bin/ffmpeg` |

### Video Qualities

The API automatically creates video variants in these qualities:
- 144p, 240p, 360p, 480p, 720p, 1080p, 1440p, 2160p

## 🧪 Testing

```bash
# Run tests
go test ./test/... -v

# Run with coverage
go test ./test/... -v -cover

# Using script
./test.sh  # Linux/macOS
test.bat   # Windows

# Using Make
make test
make test-coverage
```

## 🐳 Docker

```bash
# Build image
docker build -t api-s3 .

# Run container
docker run -p 8080:8080 \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=your_key \
  -e AWS_SECRET_ACCESS_KEY=your_secret \
  -e AWS_S3_BUCKET=your_bucket \
  api-s3

# Using Docker Compose
docker-compose up --build
```

## 📊 Monitoring

```bash
# Start with monitoring stack
docker-compose -f docker-compose.prod.yml up

# Access monitoring tools
# Prometheus: http://localhost:9090
# Grafana: http://localhost:3000 (admin/admin)
# Kibana: http://localhost:5601
```

## 🚀 Production Deployment

### Option 1: Docker Compose
```bash
# Production deployment
docker-compose -f docker-compose.prod.yml up -d
```

### Option 2: Kubernetes
```bash
# Deploy to Kubernetes
kubectl apply -f k8s/deployment.yml
```

### Option 3: Manual Deployment
```bash
# Build and deploy
make build
./deploy.sh deploy
```

## 🔍 Troubleshooting

### Common Issues

1. **FFmpeg not found**
   ```bash
   # Check FFmpeg installation
   ffmpeg -version
   
   # Set custom path in .env
   FFMPEG_PATH=/usr/local/bin/ffmpeg
   ```

2. **AWS credentials error**
   ```bash
   # Verify credentials
   aws sts get-caller-identity
   
   # Check .env file
   cat .env
   ```

3. **Port already in use**
   ```bash
   # Change port in .env
   PORT=8081
   ```

4. **File upload fails**
   ```bash
   # Check file size limit
   # Increase MAX_FILE_SIZE in .env
   MAX_FILE_SIZE=500MB
   ```

### Logs

```bash
# View application logs
docker-compose logs api-s3

# View nginx logs
docker-compose logs nginx

# View all logs
docker-compose logs -f
```

## 📚 Next Steps

1. **Customize Video Qualities** - Edit `services/video_service.go`
2. **Add Authentication** - Implement JWT middleware
3. **Database Integration** - Add PostgreSQL for metadata
4. **Caching** - Add Redis for performance
5. **CDN Integration** - Use CloudFront for global delivery
6. **Webhook Support** - Notify on upload completion

## 🆘 Need Help?

- 📖 [Full Documentation](README.md)
- 🐛 [Report Issues](https://github.com/your-repo/issues)
- 💬 [Discussions](https://github.com/your-repo/discussions)
- 📧 Email: support@example.com

---

**Happy Coding! 🎉** 