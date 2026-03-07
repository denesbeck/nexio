package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ParseRemoteURL parses "s3://bucket/prefix" into bucket and prefix.
func ParseRemoteURL(url string) (bucket, prefix string, err error) {
	if !strings.HasPrefix(url, "s3://") {
		return "", "", fmt.Errorf("invalid remote URL: must start with s3://")
	}

	trimmed := strings.TrimPrefix(url, "s3://")
	if trimmed == "" {
		return "", "", fmt.Errorf("invalid remote URL: missing bucket name")
	}

	parts := strings.SplitN(trimmed, "/", 2)
	bucket = parts[0]
	if bucket == "" {
		return "", "", fmt.Errorf("invalid remote URL: empty bucket name")
	}

	if len(parts) > 1 {
		prefix = strings.TrimSuffix(parts[1], "/")
	}

	if prefix == "" {
		return "", "", fmt.Errorf("invalid remote URL: missing prefix/path")
	}

	return bucket, prefix, nil
}

// NewS3Client creates an S3 client using the default AWS credential chain.
func NewS3Client(ctx context.Context) (*s3.Client, error) {
	Debug("Creating S3 client")
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}
	return s3.NewFromConfig(cfg), nil
}

// S3Key joins a prefix and a relative path into an S3 key.
func S3Key(prefix, rel string) string {
	return prefix + "/" + rel
}

// UploadFile uploads a local file to S3.
func UploadFile(ctx context.Context, client *s3.Client, bucket, key, localPath string) error {
	Debug("Uploading %s to s3://%s/%s", localPath, bucket, key)

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", localPath, err)
	}
	defer file.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to s3://%s/%s: %w", bucket, key, err)
	}

	Debug("Upload complete: s3://%s/%s", bucket, key)
	return nil
}

// UploadBytes uploads a byte slice to S3.
func UploadBytes(ctx context.Context, client *s3.Client, bucket, key string, data []byte) error {
	Debug("Uploading %d bytes to s3://%s/%s", len(data), bucket, key)

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to s3://%s/%s: %w", bucket, key, err)
	}

	return nil
}

// DownloadFile downloads an S3 object to a local file.
func DownloadFile(ctx context.Context, client *s3.Client, bucket, key, localPath string) error {
	Debug("Downloading s3://%s/%s to %s", bucket, key, localPath)

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to download s3://%s/%s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	// Ensure parent directory exists
	dir := filepath.Dir(localPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", localPath, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, result.Body); err != nil {
		os.Remove(localPath)
		return fmt.Errorf("failed to write file %s: %w", localPath, err)
	}

	Debug("Download complete: %s", localPath)
	return nil
}

// DownloadBytes downloads an S3 object and returns the content as bytes.
func DownloadBytes(ctx context.Context, client *s3.Client, bucket, key string) ([]byte, error) {
	Debug("Downloading bytes from s3://%s/%s", bucket, key)

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download s3://%s/%s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read s3://%s/%s: %w", bucket, key, err)
	}

	return data, nil
}

// ObjectExists checks if an S3 object exists using HeadObject.
func ObjectExists(ctx context.Context, client *s3.Client, bucket, key string) (bool, error) {
	_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		// Also check for NoSuchKey
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return false, nil
		}
		// HeadObject returns 404 as a generic error sometimes
		if strings.Contains(err.Error(), "StatusCode: 404") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check s3://%s/%s: %w", bucket, key, err)
	}
	return true, nil
}

// ListObjects lists all object keys under a given prefix.
func ListObjects(ctx context.Context, client *s3.Client, bucket, prefix string) ([]string, error) {
	Debug("Listing objects in s3://%s/%s", bucket, prefix)

	var keys []string
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix + "/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in s3://%s/%s: %w", bucket, prefix, err)
		}
		for _, obj := range page.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}
	}

	Debug("Found %d objects", len(keys))
	return keys, nil
}

// DeleteObject deletes an S3 object.
func DeleteObject(ctx context.Context, client *s3.Client, bucket, key string) error {
	Debug("Deleting s3://%s/%s", bucket, key)

	_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete s3://%s/%s: %w", bucket, key, err)
	}

	return nil
}

// GetRemoteURL resolves the remote URL from a flag value or from config.
// The flag value takes precedence over the configured remote.
func GetRemoteURL(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	config := GetConfig()
	if config.Remote == "" {
		return "", fmt.Errorf("no remote configured. Run: nexio config set remote s3://bucket/prefix")
	}

	return config.Remote, nil
}
