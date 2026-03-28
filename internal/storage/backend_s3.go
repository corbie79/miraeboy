//go:build s3

package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type s3Backend struct {
	client *minio.Client
	bucket string
}

// NewS3Backend creates an S3-compatible backend using the minio client.
// Requires building with -tags s3 and the minio-go/v7 dependency.
func NewS3Backend(cfg S3Config) (Backend, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client init: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("s3 bucket check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("s3 bucket create: %w", err)
		}
	}

	return &s3Backend{client: client, bucket: cfg.Bucket}, nil
}

func (b *s3Backend) Get(key string) ([]byte, error) {
	obj, err := b.client.GetObject(context.Background(), b.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, b.wrapErr(err)
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, b.wrapErr(err)
	}
	return data, nil
}

func (b *s3Backend) Put(key string, data []byte) error {
	_, err := b.client.PutObject(
		context.Background(), b.bucket, key,
		bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
	return err
}

func (b *s3Backend) GetStream(key string) (io.ReadCloser, int64, error) {
	obj, err := b.client.GetObject(context.Background(), b.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, b.wrapErr(err)
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, b.wrapErr(err)
	}
	return obj, info.Size, nil
}

func (b *s3Backend) PutStream(key string, r io.Reader, size int64) error {
	_, err := b.client.PutObject(
		context.Background(), b.bucket, key,
		r, size,
		minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
	return err
}

func (b *s3Backend) Delete(key string) error {
	return b.client.RemoveObject(context.Background(), b.bucket, key, minio.RemoveObjectOptions{})
}

func (b *s3Backend) DeletePrefix(prefix string) error {
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for obj := range b.client.ListObjects(context.Background(), b.bucket,
			minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			objectsCh <- obj
		}
	}()
	for err := range b.client.RemoveObjects(context.Background(), b.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if err.Err != nil {
			return err.Err
		}
	}
	return nil
}

func (b *s3Backend) List(prefix string) ([]string, error) {
	var keys []string
	for obj := range b.client.ListObjects(context.Background(), b.bucket,
		minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		keys = append(keys, obj.Key)
	}
	return keys, nil
}

func (b *s3Backend) Exists(key string) bool {
	_, err := b.client.StatObject(context.Background(), b.bucket, key, minio.StatObjectOptions{})
	return err == nil
}

func (b *s3Backend) wrapErr(err error) error {
	var resp minio.ErrorResponse
	if errors.As(err, &resp) && resp.Code == "NoSuchKey" {
		return ErrNotFound
	}
	return err
}
