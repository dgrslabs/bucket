package bucket_web_testing

import (
	"flag"
	"log"
	"net/http"
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

	client, err := bucket.Default(
		bucket.FlatNameSpaces(true),
		bucket.WithMetrics(metrics.NewCollector()), // surface backend op metrics at /bucket/metrics
	)
	if err != nil {
		log.Fatalf("Failed to initialize Bucket: %v", err)
	}
	defer client.Close() // Ensure resources are cleaned up

	config := web.Config{
		Mode:  web.WebModeMount,
		Debug: true,
	}

	webClient, err := web.NewClient(client, config)
	if err != nil {
		log.Fatalf("Failed to create web client: %v", err)
	}

	// Create a custom multiplexer
	mux := http.NewServeMux()

	// Mount the Bucket router under /bucket
	mux.Handle("/bucket/", http.StripPrefix("/bucket", webClient.Handler()))

	// Add additional routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to the main application!"))
	})

	// Start the server
	server := &http.Server{
		Addr:    ":" + *flagPort,
		Handler: mux,
	}

	log.Println("Server is running on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
