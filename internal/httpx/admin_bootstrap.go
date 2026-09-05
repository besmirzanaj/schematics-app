package httpx

import (
	"log"

	"schematics-app/internal/auth"
	"schematics-app/internal/store"
)

// ensureAdmin provisions the first admin on startup from config.AdminEmail if absent.
func ensureAdmin(st *store.Store, email string) {
	if email == "" {
		return
	}
	if _, err := st.UserByEmail(email); err == nil {
		return
	}
	hash, _ := auth.HashPassword("changeme")
	if _, err := st.CreateUser(email, hash, "admin"); err != nil {
		log.Printf("admin bootstrap failed: %v", err)
		return
	}
	log.Printf("created admin %s with default password 'changeme' (change immediately)", email)
}