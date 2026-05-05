package app

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectStore struct {
	client *minio.Client
	bucket string
}

func openObjectStore(ctx context.Context, cfg Config) (*ObjectStore, error) {
	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, err
	}
	exists, err := client.BucketExists(ctx, cfg.S3Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("S3 bucket does not exist: %s", cfg.S3Bucket)
	}
	return &ObjectStore{client: client, bucket: cfg.S3Bucket}, nil
}

func OpenObjectStore(ctx context.Context, cfg Config) (*ObjectStore, error) {
	return openObjectStore(ctx, cfg)
}

func (s *ObjectStore) PutText(ctx context.Context, objectName, text string) error {
	_, err := s.client.PutObject(ctx, s.bucket, objectName, strings.NewReader(text), int64(len(text)), minio.PutObjectOptions{
		ContentType: "text/plain; charset=utf-8",
	})
	return err
}

func (s *ObjectStore) PutUpload(ctx context.Context, objectName string, file *multipart.FileHeader) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err = s.client.PutObject(ctx, s.bucket, objectName, src, file.Size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *ObjectStore) PutStream(ctx context.Context, objectName, contentType string, src io.Reader) error {
	_, err := s.client.PutObject(ctx, s.bucket, objectName, src, -1, minio.PutObjectOptions{
		ContentType: contentType,
		PartSize:    10 * 1024 * 1024,
	})
	return err
}

func (s *ObjectStore) GetText(ctx context.Context, objectName string) (string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()
	data, err := io.ReadAll(io.LimitReader(obj, 512*1024))
	return string(data), err
}

func (s *ObjectStore) Get(ctx context.Context, objectName string) (*minio.Object, error) {
	return s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
}

func importCourseZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	cleanDest := filepath.Clean(destDir)
	if err := os.MkdirAll(cleanDest, 0755); err != nil {
		return err
	}

	for _, item := range reader.File {
		target := filepath.Clean(filepath.Join(cleanDest, item.Name))
		if !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) && target != cleanDest {
			return fmt.Errorf("zip entry escapes target directory: %s", item.Name)
		}
		if item.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		src, err := item.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, item.FileInfo().Mode())
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := errorsJoin(src.Close(), dst.Close())
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func saveUploadedZip(file *multipart.FileHeader) (string, func(), error) {
	src, err := file.Open()
	if err != nil {
		return "", nil, err
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "course-*.zip")
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", nil, err
	}
	return tmp.Name(), func() { _ = os.Remove(tmp.Name()) }, nil
}

func errorsJoin(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

func rejectOversizedUpload(r *http.Request, limit int64) {
	r.Body = http.MaxBytesReader(nil, r.Body, limit)
}
