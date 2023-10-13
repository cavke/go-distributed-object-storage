package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const MinioKeyNotExistErrString = "The specified key does not exist."

type MinioConfig struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	BucketName string
}

type MinioStorage struct {
	client     *minio.Client
	endpoint   string
	bucketName string
}

func NewMinioStorage(cfg *MinioConfig) (Storage, error) {
	log.Printf("NewMinioStorage: %v\n", cfg.Endpoint)

	// Set the timeout values in HTTP transport
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second, // Connection timeout
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:    false,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create minio storage instance: %w", err)
	}

	return &MinioStorage{
		client:     client,
		endpoint:   cfg.Endpoint,
		bucketName: cfg.BucketName,
	}, nil
}

func (s *MinioStorage) Init(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucketName)
	if err != nil {
		return fmt.Errorf("error init bucket (%s): unable to check bucket: %w", s.endpoint, err)
	}
	if exists {
		return nil
	}

	if err = s.client.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("error init bucket (%s): unable to create bucket: %w", s.endpoint, err)
	}

	log.Printf("MinioStorage(%s) Init completed: created bucket %s\n", s.endpoint, s.bucketName)
	return nil
}

func (s *MinioStorage) Get(ctx context.Context, id string) (*Object, error) {
	mObj, err := s.client.GetObject(ctx, s.bucketName, id, minio.GetObjectOptions{})
	if err != nil {
		return s.handleKeyDoesNotExistError(err, "error get object", id)
	}
	defer mObj.Close()

	info, err := mObj.Stat()
	if err != nil {
		return s.handleKeyDoesNotExistError(err, "error get object: unable to read stat", id)
	}

	body, err := io.ReadAll(mObj)
	if err != nil {
		return nil, fmt.Errorf("error get object (%s | %s): unable to read body: %w", s.endpoint, id, err)
	}

	object := Object{
		ID:          id,
		ContentType: info.ContentType,
		Content:     body,
	}

	return &object, nil
}

func (s *MinioStorage) Put(ctx context.Context, object *Object) error {
	_, err := s.client.PutObject(ctx, s.bucketName, object.ID, bytes.NewReader(object.Content), int64(len(object.Content)), minio.PutObjectOptions{
		ContentType: object.ContentType,
	})
	if err != nil {
		return fmt.Errorf("error put object (%s | %s): %w", s.endpoint, object.ID, err)
	}
	return nil
}

func (s *MinioStorage) handleKeyDoesNotExistError(err error, prefix, id string) (*Object, error) {
	if keyDoesNotExist(err) {
		return nil, nil
	}
	return nil, fmt.Errorf("%s (%s | %s): %w", prefix, s.endpoint, id, err)
}

func keyDoesNotExist(err error) bool {
	return strings.Contains(err.Error(), MinioKeyNotExistErrString)
}
