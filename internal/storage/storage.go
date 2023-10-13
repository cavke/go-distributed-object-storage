package storage

import (
	"context"
	"errors"
	"fmt"
	dockercli "github.com/docker/docker/client"
	"log"
	"strings"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

const (
	ContainerNamePattern = "amazin-object-storage-node-"
	MinioAccessKeyEnv    = "MINIO_ACCESS_KEY"
	MinioSecretKeyEnv    = "MINIO_SECRET_KEY"
	MinioApiPort         = 9000
)

type Object struct {
	ID          string
	ContentType string
	Content     []byte
}

type Node struct {
	ID        string
	Name      string
	Endpoint  string
	AccessKey string
	SecretKey string
}

type Storage interface {
	Init(ctx context.Context) error
	Put(ctx context.Context, object *Object) error
	Get(ctx context.Context, id string) (*Object, error)
}

func (n Node) String() string {
	return fmt.Sprintf("%s#%s", n.ID, n.Name)
}

func (n Node) Debug() string {
	return fmt.Sprintf("%s#%s#%s#%s#%s", n.ID, n.Name, n.Endpoint, n.AccessKey, maskSecret(n.SecretKey))
}

type hasher struct{}

func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

type DistributedStorage struct {
	client            *dockercli.Client
	bucketName        string
	circle            *consistent.Consistent
	availableStorages map[string]Storage
}

func NewDistributedStorage(cli *dockercli.Client, bucketName string) Storage {
	return &DistributedStorage{
		client:     cli,
		bucketName: bucketName,
	}
}

func (s *DistributedStorage) Init(ctx context.Context) error {
	nodes, err := s.getAvailableStorageNodes(ctx)
	if err != nil {
		return fmt.Errorf("retrieve storage nodes: %w", err)
	}

	if err := s.initStorages(ctx, nodes); err != nil {
		return err
	}

	s.initHashCircle(nodes)
	log.Println("DistributedStorage initialized successfully")
	return nil
}

// initStorages initializes all storage nodes.
func (s *DistributedStorage) initStorages(ctx context.Context, nodes []Node) error {
	s.availableStorages = make(map[string]Storage, len(nodes))

	for _, node := range nodes {
		storage, err := s.initStorageNode(ctx, node)
		if err != nil {
			return err
		}
		s.availableStorages[node.String()] = storage
	}
	return nil
}

// initStorageNode initializes a single storage node.
func (s *DistributedStorage) initStorageNode(ctx context.Context, node Node) (Storage, error) {
	storage, err := NewMinioStorage(&MinioConfig{
		Endpoint:   node.Endpoint,
		AccessKey:  node.AccessKey,
		SecretKey:  node.SecretKey,
		BucketName: s.bucketName,
	})
	if err != nil {
		return nil, fmt.Errorf("create Minio storage for node %s: %w", node.Debug(), err)
	}

	if err := storage.Init(ctx); err != nil {
		return nil, fmt.Errorf("initialize storage for node %s: %w", node.Debug(), err)
	}
	return storage, nil
}

// initHashCircle initializes the hash circle for node distribution.
func (s *DistributedStorage) initHashCircle(nodes []Node) {
	s.circle = consistent.New(nil, consistent.Config{
		Hasher:            hasher{},
		PartitionCount:    len(nodes),
		ReplicationFactor: 0,
		Load:              1.25,
	})
	for _, node := range nodes {
		s.circle.Add(node)
	}
}

func (s *DistributedStorage) Put(ctx context.Context, object *Object) error {
	if object == nil || object.ID == "" {
		return errors.New("object is empty")
	}
	// locate key on hash ring
	key := s.circle.LocateKey([]byte(object.ID)).String()
	log.Printf("DistributedStorage.Put: %s | %s\n", key, object.ID)

	// resolve storage
	storage, ok := s.availableStorages[key]
	if !ok {
		return fmt.Errorf("failed to push data: storage node not available (%s)", key)
	}

	// store object to node
	if err := storage.Put(ctx, object); err != nil {
		return fmt.Errorf("failed to put data using node (%s): %w", key, err)
	}
	return nil
}

func (s *DistributedStorage) Get(ctx context.Context, id string) (*Object, error) {
	// locate key on hash ring
	key := s.circle.LocateKey([]byte(id)).String()
	log.Printf("DistributedStorage.Get: %s | %s\n", key, id)

	// resolve storage
	storage, ok := s.availableStorages[key]
	if !ok {
		return nil, fmt.Errorf("failed to get data: storage node not available (%s)", key)
	}

	// retrieve object from node
	object, err := storage.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get data using node (%s): %w", key, err)
	}
	return object, nil
}

// getAvailableStorageNodes returns map od Nodes that correspond to minio docker containers in running status
func (s *DistributedStorage) getAvailableStorageNodes(ctx context.Context) ([]Node, error) {
	containers, err := s.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key: "status", Value: "running",
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list containers: %w", err)
	}
	// log.Printf("getAvailableStorageNodes: %v\n", containers)

	var storageNodes []Node
	for _, container := range containers {
		if container.ID == "" || container.NetworkSettings == nil {
			continue
		}
		node := Node{ID: container.ID}

		// parse name
		if len(container.Names) > 0 {
			node.Name = container.Names[0]
		}
		// check if it's relevant storage node
		if !strings.Contains(node.Name, ContainerNamePattern) {
			continue
		}

		// log.Printf("getAvailableStorageNodes: node: %v\n", node)

		// resolve IPAddress of storage node
		var addr string
		if container.NetworkSettings != nil && container.NetworkSettings.Networks != nil {
			for _, n := range container.NetworkSettings.Networks {
				if n.IPAddress != "" {
					addr = n.IPAddress
					break
				}
			}
		}
		if addr == "" {
			log.Fatalf("unable to resolve ip address of storage node: %v", node)
			continue
		}
		node.Endpoint = fmt.Sprintf("%s:%d", addr, MinioApiPort)

		// resolve access credentials
		inspectData, err := s.client.ContainerInspect(ctx, container.ID)
		if err != nil {
			return nil, fmt.Errorf("unable to inspect container %s: %w", container.ID, err)
		}

		containerEnv := make(map[string]string)
		for _, env := range inspectData.Config.Env {
			splitted := strings.Split(env, "=")
			if len(splitted) > 1 {
				containerEnv[splitted[0]] = splitted[1]
			}
		}

		node.AccessKey = containerEnv[MinioAccessKeyEnv]
		node.SecretKey = containerEnv[MinioSecretKeyEnv]

		// add storage node
		storageNodes = append(storageNodes, node)
		log.Printf("getAvailableStorageNodes: node added %s", node.Debug())
	}

	return storageNodes, nil
}

func maskSecret(secret string) string {
	return strings.Repeat("X", len(secret))
}
