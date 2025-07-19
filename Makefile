.PHONY: help build run test clean docker-build docker-run install-deps

# Default target
help:
	@echo "Available commands:"
	@echo "  install-deps  - Install Go dependencies"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run with Docker Compose"

# Install dependencies
install-deps:
	@echo "📦 Installing dependencies..."
	go mod tidy
	go mod download

# Build the application
build:
	@echo "🔨 Building application..."
	go build -o bin/api-s3 main.go

# Run the application
run:
	@echo "🚀 Running application..."
	go run main.go

# Run tests
test:
	@echo "🧪 Running tests..."
	go test ./test/... -v

# Run tests with coverage
test-coverage:
	@echo "📊 Running tests with coverage..."
	go test ./test/... -v -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "📄 Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf temp/
	rm -rf test-temp/
	rm -f coverage.out
	rm -f coverage.html

# Build Docker image
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t api-s3 .

# Run with Docker Compose
docker-run:
	@echo "🐳 Running with Docker Compose..."
	docker-compose up --build

# Stop Docker Compose
docker-stop:
	@echo "🐳 Stopping Docker Compose..."
	docker-compose down

# Format code
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "🔍 Linting code..."
	golangci-lint run

# Install development tools
install-tools:
	@echo "🛠️ Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Setup development environment
setup: install-tools install-deps
	@echo "✅ Development environment setup complete!"

# Create .env file from template
setup-env:
	@echo "📝 Creating .env file from template..."
	cp env.example .env
	@echo "⚠️  Please edit .env file with your AWS credentials"

# Full setup including environment
full-setup: setup setup-env
	@echo "🎉 Full setup complete! Edit .env file and run 'make run'" 