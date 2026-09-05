package main

import (
	"log"
	"net/http"

	"schematics-app/internal/config"
)

func main() {
	cfg := config.FromEnv()
	log.Printf("skemat server starting on %s (data=%s)", cfg.Addr, cfg.DataRoot)
	if err := http.ListenAndServe(cfg.Addr, nil); err != nil {
		log.Fatal(err)
	}
}