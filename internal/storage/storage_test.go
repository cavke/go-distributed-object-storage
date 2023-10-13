package storage

import (
	"context"
	"github.com/buraksezer/consistent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocking the Storage behavior
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Init(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockStorage) Put(ctx context.Context, object *Object) error {
	args := m.Called(ctx, object)
	return args.Error(0)
}

func (m *MockStorage) Get(ctx context.Context, id string) (*Object, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*Object), args.Error(1)
}

func TestDistributedStorage_Get(t *testing.T) {
	// Setup mockStorage and nodes
	mockStorage, nodes := setupMocksAndNodes()

	// Create DistributedStorage instance
	ds := createDistributedStorage(mockStorage, nodes)

	// Table-driven tests
	tests := []struct {
		name     string
		objectID string
		expected *Object
	}{
		{
			name:     "Get object-1",
			objectID: "object-1",
			expected: &Object{ID: "object-1", Content: []byte("data1")},
		},
		{
			name:     "Get object-2",
			objectID: "object-2",
			expected: &Object{ID: "object-2", Content: []byte("data2")},
		},
		{
			name:     "Get object-3",
			objectID: "543b8e0ef09346689eb33adbbbee452a",
			expected: &Object{ID: "543b8e0ef09346689eb33adbbbee452a", Content: []byte("data444")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := ds.Get(context.TODO(), tt.objectID)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, obj)
		})
	}
}

func setupMocksAndNodes() (*MockStorage, map[string]Node) {
	mockStorage := new(MockStorage)
	mockStorage.On("Get", mock.Anything, "object-1").Return(&Object{ID: "object-1", Content: []byte("data1")}, nil)
	mockStorage.On("Get", mock.Anything, "object-2").Return(&Object{ID: "object-2", Content: []byte("data2")}, nil)
	mockStorage.On("Get", mock.Anything, "543b8e0ef09346689eb33adbbbee452a").Return(&Object{ID: "543b8e0ef09346689eb33adbbbee452a", Content: []byte("data444")}, nil)

	nodes := map[string]Node{
		"node1#1": {ID: "node1", Name: "1", Endpoint: "1.1.1.1", AccessKey: "key", SecretKey: "secret"},
		"node2#2": {ID: "node2", Name: "2", Endpoint: "2.2.2.2", AccessKey: "key", SecretKey: "secret"},
		"node3#3": {ID: "node3", Name: "3", Endpoint: "3.3.3.3", AccessKey: "key", SecretKey: "secret"},
	}

	return mockStorage, nodes
}

func createDistributedStorage(mockStorage *MockStorage, nodes map[string]Node) *DistributedStorage {
	ds := &DistributedStorage{
		circle: consistent.New(nil, consistent.Config{
			Hasher:            hasher{},
			PartitionCount:    len(nodes),
			ReplicationFactor: 0,
			Load:              1.25,
		}),
		availableStorages: map[string]Storage{"node1#1": mockStorage, "node2#2": mockStorage, "node3#3": mockStorage},
	}

	for _, node := range nodes {
		ds.circle.Add(node)
	}

	return ds
}
