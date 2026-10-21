package main

import (
	"log"
	"mcl/internal/catalogrelay"
	"net/http"
	"os"
	"time"
)

func main() {
	key := os.Getenv("MCL_CURSEFORGE_API_KEY")
	if key == "" {
		log.Fatal("publisher server requires MCL_CURSEFORGE_API_KEY")
	}
	addr := os.Getenv("MCL_CATALOG_LISTEN")
	if addr == "" {
		addr = "127.0.0.1:8081"
	}
	server := &http.Server{Addr: addr, Handler: catalogrelay.New(key, nil), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 50 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("MCL catalog relay listening on %s (put an HTTPS reverse proxy in front)", addr)
	log.Fatal(server.ListenAndServe())
}
