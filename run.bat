@echo off
echo 🚀 Starting API S3 Application...

REM Check if Go is installed
go version >nul 2>&1
if errorlevel 1 (
    echo ❌ Go is not installed. Please install Go 1.21+ first.
    pause
    exit /b 1
)

REM Check if FFmpeg is installed
ffmpeg -version >nul 2>&1
if errorlevel 1 (
    echo ⚠️  FFmpeg is not installed. Video processing will be disabled.
    echo    Install FFmpeg from: https://ffmpeg.org/download.html
)

REM Check if .env file exists
if not exist .env (
    echo ⚠️  .env file not found. Creating from template...
    copy env.example .env
    echo 📝 Please edit .env file with your AWS credentials before running again.
    pause
    exit /b 1
)

REM Install dependencies
echo 📦 Installing dependencies...
go mod tidy

REM Create temp directory
if not exist temp mkdir temp

REM Run the application
echo 🎯 Starting server...
go run main.go

pause 