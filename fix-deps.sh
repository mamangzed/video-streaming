#!/bin/bash

echo "🔧 Fixing Go dependencies..."

# Remove existing go.mod and go.sum
rm -f go.mod go.sum

# Create new go.mod with stable versions
cat > go.mod << 'EOF'
module api-s3

go 1.21

require (
	github.com/aws/aws-sdk-go-v2 v1.23.0
	github.com/aws/aws-sdk-go-v2/config v1.25.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.47.0
	github.com/gin-gonic/gin v1.9.1
	github.com/google/uuid v1.5.0
	github.com/joho/godotenv v1.5.1
	github.com/stretchr/testify v1.8.4
)
EOF

# Download dependencies
echo "📦 Downloading dependencies..."
go mod download

# Tidy up
echo "🧹 Tidying up..."
go mod tidy

# Verify
echo "✅ Verifying..."
go mod verify

# Test build
echo "🏗️ Testing build..."
go build -o bin/api-s3 main.go

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
else
    echo "❌ Build failed!"
    exit 1
fi

echo "✅ Dependencies fixed successfully!" 