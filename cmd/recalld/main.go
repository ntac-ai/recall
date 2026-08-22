package main

import (
	"fmt"
	"log"
	"net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, World!")
}

func main() {
	// Register the route and handler
	http.HandleFunc("/", helloHandler)

	fmt.Println("Server starting on :8080...")

	// Start the server and block
	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
