@echo off
echo 🧪 Running API S3 Tests...

REM Check if Go is installed
go version >nul 2>&1
if errorlevel 1 (
    echo ❌ Go is not installed. Please install Go 1.21+ first.
    pause
    exit /b 1
)

REM Install dependencies
echo 📦 Installing dependencies...
go mod tidy

REM Run tests
echo 🎯 Running tests...
go test ./test/... -v

REM Run tests with coverage
echo 📊 Running tests with coverage...
go test ./test/... -v -cover

echo ✅ Tests completed!
pause 