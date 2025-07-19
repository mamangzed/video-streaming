package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"api-s3/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Service struct {
	client *s3.Client
	bucket string
}

func NewS3Service() (*S3Service, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(config.AppConfig.AWSRegion),
		config.WithCredentialsProvider(config.CredentialsProviderFunc(func(ctx context.Context) (config.Credentials, error) {
			return config.Credentials{
				AccessKeyID:     config.AppConfig.AWSAccessKeyID,
				SecretAccessKey: config.AppConfig.AWSSecretAccessKey,
			}, nil
		})),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	return &S3Service{
		client: client,
		bucket: config.AppConfig.AWSS3Bucket,
	}, nil
}

func (s *S3Service) UploadFile(file *multipart.FileHeader, folder string) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer src.Close()

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%s/%s%s", folder, generateUniqueID(), ext)

	// Upload to S3
	_, err = s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filename),
		Body:   src,
		ACL:    "public-read",
		ContentType: aws.String(file.Header.Get("Content-Type")),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload file to S3: %v", err)
	}

	// Return the S3 URL
	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, config.AppConfig.AWSRegion, filename)
	return url, nil
}

func (s *S3Service) UploadFileFromReader(reader io.Reader, filename, contentType, folder string) (string, error) {
	// Generate unique filename
	ext := filepath.Ext(filename)
	uniqueFilename := fmt.Sprintf("%s/%s%s", folder, generateUniqueID(), ext)

	// Upload to S3
	_, err := s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(uniqueFilename),
		Body:        reader,
		ACL:         "public-read",
		ContentType: aws.String(contentType),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload file to S3: %v", err)
	}

	// Return the S3 URL
	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, config.AppConfig.AWSRegion, uniqueFilename)
	return url, nil
}

func (s *S3Service) DeleteFile(key string) error {
	_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete file from S3: %v", err)
	}

	return nil
}

func (s *S3Service) GeneratePresignedURL(key string, expires time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %v", err)
	}

	return request.URL, nil
}

func (s *S3Service) GetFileURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, config.AppConfig.AWSRegion, key)
}

func (s *S3Service) ExtractKeyFromURL(url string) string {
	// Extract key from S3 URL
	// Format: https://bucket.s3.region.amazonaws.com/key
	parts := strings.Split(url, ".amazonaws.com/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func (s *S3Service) FileExists(key string) (bool, error) {
	_, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		// Check if it's a "not found" error
		var noSuchKey *s3.NoSuchKey
		if strings.Contains(err.Error(), "NoSuchKey") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func generateUniqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
} 