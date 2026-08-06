// Package storage wraps the MinIO SDK behind a small interface tailored to
// this app's needs: upload, short-lived presigned reads, and delete. It's
// generic infra (like the store package wraps *sql.DB) — modules that need
// object storage (currently just couple's photo feature) depend on this
// rather than importing minio-go directly.
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// presignExpiry bounds how long a presigned GET URL stays valid. Short-lived
// on purpose: the bucket is never public, so this URL is the only way a
// photo reaches the frontend, and it's regenerated fresh on every list/
// upload request rather than cached or stored — see couple's ListPhotos.
const presignExpiry = 15 * time.Minute

// Client wraps a *minio.Client bound to a single bucket. A nil *Client means
// the photo feature is disabled (MINIO_ENDPOINT unset in config) — callers
// must check for nil themselves before calling in; see couple's handlers for
// the 503 "not configured" pattern instead of a nil-pointer panic.
type Client struct {
	mc     *minio.Client
	bucket string
}

// New builds a Client from explicit config values (endpoint host:port, no
// scheme; access/secret keys; bucket name; whether to speak TLS). This only
// constructs the SDK client — it doesn't touch the network, so it can't tell
// you MinIO is actually reachable. Call EnsureBucket for that.
func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return &Client{mc: mc, bucket: bucket}, nil
}

// EnsureBucket creates the configured bucket if it doesn't already exist.
// Idempotent, so it's safe to call on every boot — same idempotent-on-every-
// boot spirit as store.Migrate and gym.SeedExercises, but this is cheap/
// local so there's no version tracking or caching needed.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("bucket exists check: %w", err)
	}
	if exists {
		return nil
	}
	if err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("make bucket: %w", err)
	}
	return nil
}

// Upload stores an object at key, reading exactly size bytes from r.
func (c *Client) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

// PresignedGetURL returns a short-lived URL (see presignExpiry) for reading
// the object at key. The bucket itself stays private — this is the only way
// a caller without MinIO credentials can ever read an object.
func (c *Client) PresignedGetURL(ctx context.Context, key string) (string, error) {
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, key, presignExpiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// Delete removes the object at key.
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}
