package main

import (
	"log"
	"net/http"

	"schematics-app/internal/config"
	"schematics-app/internal/httpx"
	"schematics-app/internal/store"
)

func main() {
	cfg := config.FromEnv()
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer st.Close()
	srv, err := httpx.NewServer(st, cfg)
	if err != nil {
		log.Fatalf("httpx: %v", err)
	}
	log.Printf("skemat server on %s (data=%s db=%s)", cfg.Addr, cfg.DataRoot, cfg.DBPath)
	if err := http.ListenAndServe(cfg.Addr, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}