#!/bin/bash

# Production deployment script for API S3

set -e

# Configuration
APP_NAME="api-s3"
DOCKER_IMAGE="api-s3:latest"
DOCKER_REGISTRY="your-registry.com"
PRODUCTION_TAG="v1.0.0"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if required tools are installed
check_requirements() {
    log_info "Checking requirements..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose is not installed"
        exit 1
    fi
    
    if [ ! -f .env ]; then
        log_error ".env file not found. Please create it from env.example"
        exit 1
    fi
    
    log_info "Requirements check passed"
}

# Run tests
run_tests() {
    log_info "Running tests..."
    go test ./test/... -v
    if [ $? -ne 0 ]; then
        log_error "Tests failed"
        exit 1
    fi
    log_info "Tests passed"
}

# Build application
build_app() {
    log_info "Building application..."
    go build -o bin/api-s3 main.go
    if [ $? -ne 0 ]; then
        log_error "Build failed"
        exit 1
    fi
    log_info "Build successful"
}

# Build Docker image
build_docker() {
    log_info "Building Docker image..."
    docker build -t $DOCKER_IMAGE .
    if [ $? -ne 0 ]; then
        log_error "Docker build failed"
        exit 1
    fi
    log_info "Docker image built successfully"
}

# Tag Docker image
tag_docker() {
    log_info "Tagging Docker image..."
    docker tag $DOCKER_IMAGE $DOCKER_REGISTRY/$APP_NAME:$PRODUCTION_TAG
    docker tag $DOCKER_IMAGE $DOCKER_REGISTRY/$APP_NAME:latest
    log_info "Docker image tagged"
}

# Push Docker image
push_docker() {
    log_info "Pushing Docker image to registry..."
    docker push $DOCKER_REGISTRY/$APP_NAME:$PRODUCTION_TAG
    docker push $DOCKER_REGISTRY/$APP_NAME:latest
    log_info "Docker image pushed successfully"
}

# Deploy to production
deploy_production() {
    log_info "Deploying to production..."
    
    # Pull latest image
    docker-compose pull
    
    # Deploy with zero downtime
    docker-compose up -d --no-deps --build
    
    # Health check
    log_info "Performing health check..."
    sleep 10
    
    # Check if service is healthy
    if curl -f http://localhost:8080/health > /dev/null 2>&1; then
        log_info "Health check passed"
    else
        log_error "Health check failed"
        docker-compose logs
        exit 1
    fi
    
    log_info "Deployment successful"
}

# Rollback deployment
rollback() {
    log_warn "Rolling back deployment..."
    docker-compose down
    docker-compose up -d
    log_info "Rollback completed"
}

# Main deployment process
main() {
    log_info "Starting deployment process..."
    
    # Check requirements
    check_requirements
    
    # Run tests
    run_tests
    
    # Build application
    build_app
    
    # Build Docker image
    build_docker
    
    # Tag Docker image
    tag_docker
    
    # Push Docker image (uncomment if using remote registry)
    # push_docker
    
    # Deploy to production
    deploy_production
    
    log_info "Deployment completed successfully!"
}

# Handle script arguments
case "$1" in
    "deploy")
        main
        ;;
    "rollback")
        rollback
        ;;
    "test")
        run_tests
        ;;
    "build")
        build_app
        build_docker
        ;;
    "push")
        tag_docker
        push_docker
        ;;
    *)
        echo "Usage: $0 {deploy|rollback|test|build|push}"
        echo "  deploy   - Full deployment process"
        echo "  rollback - Rollback to previous version"
        echo "  test     - Run tests only"
        echo "  build    - Build application and Docker image"
        echo "  push     - Push Docker image to registry"
        exit 1
        ;;
esac 