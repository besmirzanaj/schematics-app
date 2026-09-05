package store

import (
	"database/sql"
	"embed"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // sqlite WAL: one writer
	for _, stmt := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, err
		}
	}
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(string(schema)); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

var ErrNotFound = errors.New("not found")

type Entitlements struct {
	Global bool
	Makes  map[int64]bool
	Models map[int64]bool
	Years  map[int64]bool
}

func (e Entitlements) Visible(m Model, mk Make, admin bool) bool {
	if admin {
		return true
	}
	if mk.InternalOnly || m.InternalOnly {
		return false
	}
	if e.Global {
		return true
	}
	if e.Makes[mk.ID] || e.Models[m.ID] || e.Years[m.DatasetYear] {
		return true
	}
	return false
}

// -------- models --------

type Make struct {
	ID           int64
	Name         string
	InternalOnly bool
}

type Model struct {
	ID           int64
	MakeID       int64
	Name         string
	DisplayName  string
	Year         int64
	DatasetYear  int64
	Region       string
	InternalOnly bool
}

type System struct {
	ID      int64
	ModelID int64
	Code    string
}

type Object struct {
	ID        int64
	SystemID  int64
	Filename  string
	Display   string
	Kind      string
	RelPath   string
	SortOrder int64
}

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Role         string
}

type ObjectRef struct {
	Obj Object
	Sys System
	Mod Model
	Mk  Make
}

type ModelHit struct {
	Model Model
	Make  Make
}

var _ = time.Now