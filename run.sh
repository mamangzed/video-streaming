#!/bin/bash

echo "🚀 Starting API S3 Application..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

# Check if FFmpeg is installed
if ! command -v ffmpeg &> /dev/null; then
    echo "⚠️  FFmpeg is not installed. Video processing will be disabled."
    echo "   Install FFmpeg from: https://ffmpeg.org/download.html"
fi

# Check if .env file exists
if [ ! -f .env ]; then
    echo "⚠️  .env file not found. Creating from template..."
    cp env.example .env
    echo "📝 Please edit .env file with your AWS credentials before running again."
    exit 1
fi

# Install dependencies
echo "📦 Installing dependencies..."
go mod tidy

# Create temp directory
mkdir -p temp

# Run the application
echo "🎯 Starting server..."
go run main.go 