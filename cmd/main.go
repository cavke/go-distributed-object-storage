package main

import (
	"context"
	"github.com/cavke/go-distributed-object-storage/internal/gateway"
	"github.com/cavke/go-distributed-object-storage/internal/storage"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	dockercli "github.com/docker/docker/client"
)

const EnvBucketName = "BUCKET_NAME"

func main() {
	log.Println("Starting storage system")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := dockercli.NewClientWithOpts(dockercli.FromEnv)
	checkError(err)

	bucketName := getEnvWithFallback(EnvBucketName, "default")
	storage := storage.NewDistributedStorage(cli, bucketName)
	storage.Init(ctx)

	server := gateway.NewServer(storage)

	log.Println("Starting gateway server")
	go func() {
		if err := server.Start(":3000"); err != nil {
			log.Printf("Server error: %s\n", err)
		}
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	sig := <-sigc
	log.Printf("Received signal: '%s', initiating server shutdown\n", sig.String())
	cancel()

	closeCtx, cancelClose := context.WithTimeout(ctx, time.Second*5)
	defer cancelClose()
	checkError(server.Shutdown(closeCtx))

	log.Println("Storage system shutdown completed successfully")
}

func checkError(err error) {
	if err != nil {
		log.Println(err.Error())
	}
}

func getEnvWithFallback(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Printf("Environment variable %s not set, using default value: %s", key, fallback)
		return fallback
	}
	return value
}
