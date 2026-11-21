package main

import (
	"context"
	"fmt"

	"github.com/dgrslabs/bucket"
	"github.com/dgrslabs/bucket/cloud/aws"
)

func main() {

	cloudConfig := aws.Config{
		AccessKey: "accessKey",
		SecretKey: "secretKey",
		Region:    "us-west-2",
		Bucket:    "my-bucket",
	}

	awsBackend, err := aws.NewBackend(cloudConfig)
	if err != nil {
		fmt.Println("Failed to create AWS backend:", err)
		return
	}

	if err := awsBackend.Ping(context.Background()); err != nil {
		fmt.Println("Failed to connect to AWS backend:", err)
		return
	}

	client, err := bucket.Default(bucket.WithLog(bucket.LogConfig{}), bucket.WithBackend(awsBackend))
	if err != nil {
		fmt.Println(err)
		return
	}

	defer client.Close()

	fmt.Println("Bucket Client initialized successfully")
}
