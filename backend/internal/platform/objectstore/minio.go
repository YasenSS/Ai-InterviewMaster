package objectstore

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/internal/platform/appconfig"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func New(settings appconfig.ObjectStore) (*minio.Client, error) {
	endpoint, secure, err := endpoint(settings.Endpoint)
	if err != nil {
		return nil, err
	}
	return minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(settings.AccessKey, settings.SecretKey, ""), Secure: secure, Region: settings.Region})
}

func EnsureBucket(ctx context.Context, client *minio.Client, bucket, region string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check object bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
		return fmt.Errorf("create object bucket: %w", err)
	}
	return nil
}

func endpoint(raw string) (string, bool, error) {
	if !strings.Contains(raw, "://") {
		return raw, false, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", false, fmt.Errorf("invalid object storage endpoint")
	}
	return parsed.Host, parsed.Scheme == "https", nil
}
