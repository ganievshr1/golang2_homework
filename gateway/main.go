package main

import (
	"log"
	"net/http"
	"time"

	"example.com/finance-services/gateway/internal/api"
)

func newHandler() http.Handler {
	return api.NewHandler(log.Default())
}

func main() {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Println("Gateway service started on http://localhost:8080")
	log.Fatal(server.ListenAndServe())
}
