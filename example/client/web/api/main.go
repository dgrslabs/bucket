package main

import (
	"flag"
	"log"
	"os"

	"github.com/dgrslabs/bucket"
	"github.com/dgrslabs/bucket/client/web"
	"github.com/dgrslabs/bucket/pkg/metrics"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Allow overriding via command-line flag
	flagPort := flag.String("port", port, "Port to run the server on")
	flag.Parse()

	// Enable built-in backend metrics so the /metrics endpoint has data to show.
	client, err := bucket.Default(bucket.FlatNameSpaces(true), bucket.WithMetrics(metrics.NewCollector()))
	if err != nil {
		log.Fatalf("Failed to initialize Bucket: %v", err)
	}
	defer client.Close() // Ensure resources are cleaned up

	webClient, err := web.NewClient(client, web.Config{
		Mode: web.WebModeAPI,
	})
	if err != nil {
		log.Fatalf("Failed to create web client: %v", err)
	}

	// Start the router (optional, based on user choice)
	if err := webClient.Run(":" + *flagPort); err != nil {
		log.Fatalf("Failed to start Bucket: %v", err)
	}
}
