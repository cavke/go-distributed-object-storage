package itests

import (
	"context"
	"github.com/cavke/go-distributed-object-storage/internal/storage"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testEndpoint    = "localhost:19001"
	testAccessKey   = "ring"
	testSecretKey   = "treepotato"
	testBucketName  = "test-bucket"
	testObjectID    = "1"
	testContentType = "text/plain"
)

func TestMinioIntegration(t *testing.T) {
	cfg := &storage.MinioConfig{
		Endpoint:   testEndpoint,
		AccessKey:  testAccessKey,
		SecretKey:  testSecretKey,
		BucketName: testBucketName,
	}

	mStorage, err := storage.NewMinioStorage(cfg)
	assert.Nil(t, err)

	ctx := context.Background()

	// Test Init
	err = mStorage.Init(ctx)
	assert.Nil(t, err)

	// Test Put
	content := []byte("Hello, Minio!")
	object := storage.Object{
		ID:          testObjectID,
		ContentType: testContentType,
		Content:     content,
	}
	err = mStorage.Put(ctx, &object)
	assert.Nil(t, err)

	// Test Get
	object1, err := mStorage.Get(ctx, testObjectID)
	assert.Nil(t, err)
	assert.Equal(t, testObjectID, object1.ID)
	assert.Equal(t, testContentType, object1.ContentType)
	assert.Equal(t, content, object1.Content)
}
