package main

import (
	"context"
	"fmt"

	"github.com/dgrslabs/bucket"
	"github.com/dgrslabs/bucket/cloud/azure"
)

func main() {

	cloudConfig := azure.Config{
		AccountName: "accountName",
		AccountKey:  "accountKey",
		Container:   "container",
	}

	azureBackend, err := azure.NewBackend(cloudConfig)
	if err != nil {
		fmt.Println("Failed to create Azure backend:", err)
		return
	}

	if err := azureBackend.Ping(context.Background()); err != nil {
		fmt.Println("Failed to connect to Azure backend:", err)
		return
	}

	backend := bucket.WithBackend(azureBackend)

	client, err := bucket.Default(bucket.WithLog(bucket.LogConfig{}), backend)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer client.Close()

	fmt.Println("Bucket Client initialized successfully")
}
