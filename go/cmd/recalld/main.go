package main

import (
    "flag"
	"fmt"
	"log"
	"net/http"
    "os"
    "path/filepath"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, World!")
}

func main() {
    homeDir, err := os.UserHomeDir()

	if err != nil {
		log.Fatalf("$HOME directory not found: %w", err)
	}

    dataDir := filepath.Join(homeDir, ".config", "recall")

	dataDirPtr := flag.String("data-dir", dataDir, "recall data directory")
	portPtr := flag.Int("port", 22550, "HTTP port to listen on")

    flag.Parse()
    fmt.Println("--data-dir=", *dataDirPtr)
    fmt.Println("--port=", *portPtr)

	// Register the route and handler
	http.HandleFunc("/", helloHandler)

	// fmt.Println("Server starting on :8080...")

	// Start the server and block
	err = http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", *portPtr), nil)

	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
