package main

import (
	"log"
	"net/http"
	"time"

	"github.com/DITLEKS/GO_Project/gateway/internal/api"
)

func main() {
	addr := ":8080"
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Gateway service started on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
