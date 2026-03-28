//go:build !s3

package storage

import "errors"

// NewS3Backend is unavailable in this build.
// Rebuild with -tags s3 (and minio-go/v7 in go.mod) to enable S3 support.
func NewS3Backend(_ S3Config) (Backend, error) {
	return nil, errors.New("S3 support not compiled in; rebuild with -tags s3")
}
