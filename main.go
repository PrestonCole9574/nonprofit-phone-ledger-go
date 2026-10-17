package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY required")
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	service := ledger{api: client{key: key, origin: baseURL, http: &http.Client{Timeout: 15 * time.Second}}}
	log.Fatal(http.ListenAndServe(addr, http.HandlerFunc(service.serve)))
}
