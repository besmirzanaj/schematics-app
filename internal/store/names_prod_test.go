package store

import (
	"os"
	"testing"
)

func TestDumpProdNames(t *testing.T) {
	p := os.Getenv("PROD_DB")
	if p == "" {
		t.Skip("PROD_DB not set")
	}
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.db.Query(`SELECT DISTINCT display_name FROM models ORDER BY display_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var raw, clean string
	type pair struct{ raw, clean string }
	var pairs []pair
	unchanged := 0
	for rows.Next() {
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		clean = CleanDisplay(raw)
		pairs = append(pairs, pair{raw, clean})
		if clean == raw {
			unchanged++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("total distinct=%d unchanged=%d", len(pairs), unchanged)
	for _, p := range pairs {
		t.Logf("%-40s -> %s", p.raw, p.clean)
	}
}