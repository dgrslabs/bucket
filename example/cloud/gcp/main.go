package main

import (
	"context"
	"fmt"

	"github.com/dgrslabs/bucket"
	"github.com/dgrslabs/bucket/cloud/gcp"
)

func main() {

	cloudConfig := gcp.Config{
		CredentialsFile: "path/to/credentials.json",
		Bucket:          "my-bucket",
	}

	gcpBackend, err := gcp.NewBackend(cloudConfig)
	if err != nil {
		fmt.Println("Failed to create GCP backend:", err)
		return
	}

	if err := gcpBackend.Ping(context.Background()); err != nil {
		fmt.Println("Failed to connect to GCP backend:", err)
		return
	}

	backend := bucket.WithBackend(gcpBackend)

	client, err := bucket.Default(bucket.WithLog(bucket.LogConfig{}), backend)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer client.Close()

	fmt.Println("Bucket Client initialized successfully")
}
