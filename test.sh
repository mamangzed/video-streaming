#!/bin/bash

echo "🧪 Running API S3 Tests..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

# Install dependencies
echo "📦 Installing dependencies..."
go mod tidy

# Run tests
echo "🎯 Running tests..."
go test ./test/... -v

# Run tests with coverage
echo "📊 Running tests with coverage..."
go test ./test/... -v -cover

echo "✅ Tests completed!" 