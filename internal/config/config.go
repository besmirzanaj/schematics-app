package config

import "os"

type Config struct {
	Addr          string // listen address, e.g. ":8080"
	DataRoot      string // normalized schematic tree on disk
	DBPath        string // sqlite db file
	AdminEmail    string // email auto-provisioned as admin on first run
	SecureCookies bool   // Set-Cookie: Secure
}

func FromEnv() Config {
	return Config{
		Addr:          envOr("SKEMAT_ADDR", ":8080"),
		DataRoot:      envOr("SKEMAT_DATA", "./data/live"),
		DBPath:        envOr("SKEMAT_DB", "./data/skemat.db"),
		AdminEmail:    os.Getenv("SKEMAT_ADMIN_EMAIL"),
		SecureCookies: os.Getenv("SKEMAT_SECURE_COOKIES") == "1",
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}