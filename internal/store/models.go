package store

import (
	"database/sql"
	"strings"
)

// -------- catalog --------

func (s *Store) Makes(e Entitlements, admin bool) ([]Make, error) {
	rows, err := s.db.Query(`SELECT id, name, internal_only FROM makes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []Make
	for rows.Next() {
		var m Make
		if err := rows.Scan(&m.ID, &m.Name, &m.InternalOnly); err != nil {
			return nil, err
		}
		all = append(all, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if admin {
		return all, nil
	}
	out := make([]Make, 0, len(all))
	for _, m := range all {
		if m.InternalOnly {
			continue
		}
		if e.Global || e.Makes[m.ID] || s.makeReachable(m.ID, e) {
			out = append(out, m)
		}
	}
	return out, nil
}

// makeReachable reports whether the user's model- or year-scoped entitlements
// cover at least one model that belongs to the given make.
func (s *Store) makeReachable(makeID int64, e Entitlements) bool {
	if len(e.Models) == 0 && len(e.Years) == 0 {
		return false
	}
	var conds []string
	var args []any
	if len(e.Models) > 0 {
		conds = append(conds, "id IN ("+placeholders(len(e.Models))+")")
		for id := range e.Models {
			args = append(args, id)
		}
	}
	if len(e.Years) > 0 {
		conds = append(conds, "dataset_year IN ("+placeholders(len(e.Years))+")")
		for y := range e.Years {
			args = append(args, y)
		}
	}
	q := "SELECT EXISTS(SELECT 1 FROM models WHERE make_id = ? AND (" + strings.Join(conds, " OR ") + "))"
	args = append([]any{makeID}, args...)
	var hit bool
	err := s.db.QueryRow(q, args...).Scan(&hit)
	return err == nil && hit
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func (s *Store) Models(mkID int64, e Entitlements, admin bool) ([]Model, error) {
	rows, err := s.db.Query(`SELECT id, make_id, name, display_name, year, dataset_year, region, internal_only FROM models WHERE make_id = ? ORDER BY display_name`, mkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []Model
	for rows.Next() {
		var m Model
		if err := rows.Scan(&m.ID, &m.MakeID, &m.Name, &m.DisplayName, &m.Year, &m.DatasetYear, &m.Region, &m.InternalOnly); err != nil {
			return nil, err
		}
		all = append(all, m)
	}
	out := make([]Model, 0, len(all))
	for _, m := range all {
		mk := Make{ID: m.MakeID}
		if e.Visible(m, mk, admin) {
			out = append(out, m)
		}
	}
	return out, rows.Err()
}

func (s *Store) Systems(modelID int64) ([]System, error) {
	rows, err := s.db.Query(`SELECT id, model_id, code FROM systems WHERE model_id = ? ORDER BY CAST(code AS INTEGER)`, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []System
	for rows.Next() {
		var sy System
		if err := rows.Scan(&sy.ID, &sy.ModelID, &sy.Code); err != nil {
			return nil, err
		}
		out = append(out, sy)
	}
	return out, rows.Err()
}

func (s *Store) Objects(systemID int64) ([]Object, error) {
	rows, err := s.db.Query(`SELECT id, system_id, filename, display, kind, rel_path, sort_order FROM objects WHERE system_id = ? ORDER BY sort_order, filename`, systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		var o Object
		if err := rows.Scan(&o.ID, &o.SystemID, &o.Filename, &o.Display, &o.Kind, &o.RelPath, &o.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) Object(id int64) (ObjectRef, error) {
	var r ObjectRef
	err := s.db.QueryRow(`SELECT o.id, o.system_id, o.filename, o.display, o.kind, o.rel_path, o.sort_order,
		 sy.id, sy.model_id, sy.code,
		 m.id, m.make_id, m.name, m.display_name, m.year, m.dataset_year, m.region, m.internal_only,
		 mk.id, mk.name, mk.internal_only
		 FROM objects o
		 JOIN systems sy ON sy.id = o.system_id
		 JOIN models m ON m.id = sy.model_id
		 JOIN makes mk ON mk.id = m.make_id
		 WHERE o.id = ?`, id).
		Scan(&r.Obj.ID, &r.Obj.SystemID, &r.Obj.Filename, &r.Obj.Display, &r.Obj.Kind, &r.Obj.RelPath, &r.Obj.SortOrder,
			&r.Sys.ID, &r.Sys.ModelID, &r.Sys.Code,
			&r.Mod.ID, &r.Mod.MakeID, &r.Mod.Name, &r.Mod.DisplayName, &r.Mod.Year, &r.Mod.DatasetYear, &r.Mod.Region, &r.Mod.InternalOnly,
			&r.Mk.ID, &r.Mk.Name, &r.Mk.InternalOnly)
	if err == sql.ErrNoRows {
		return r, ErrNotFound
	}
	return r, err
}

// -------- users --------

func (s *Store) UserByEmail(email string) (User, error) {
	return s.scanUser(`SELECT id, email, password_hash, role FROM users WHERE email = ?`, email)
}

func (s *Store) UserByID(id int64) (User, error) {
	return s.scanUser(`SELECT id, email, password_hash, role FROM users WHERE id = ?`, id)
}

func (s *Store) scanUser(q string, a any) (User, error) {
	var u User
	err := s.db.QueryRow(q, a).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role)
	if err == sql.ErrNoRows {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) CreateUser(email, passwordHash, role string) (User, error) {
	res, err := s.db.Exec(`INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)`, email, passwordHash, role)
	if err != nil {
		return User{}, err
	}
	id, _ := res.LastInsertId()
	return User{ID: id, Email: email, PasswordHash: passwordHash, Role: role}, nil
}

func (s *Store) UpdateUserRole(id int64, role string) error {
	_, err := s.db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, id)
	return err
}

func (s *Store) UpdateUserPassword(id int64, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, email, password_hash, role FROM users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// -------- entitlements --------

func (s *Store) EntitlementsForUser(id int64) (Entitlements, error) {
	rows, err := s.db.Query(`SELECT scope, COALESCE(ref,0) FROM entitlements WHERE user_id = ?`, id)
	if err != nil {
		return Entitlements{}, err
	}
	defer rows.Close()
	e := Entitlements{Makes: map[int64]bool{}, Models: map[int64]bool{}, Years: map[int64]bool{}}
	for rows.Next() {
		var scope string
		var ref int64
		if err := rows.Scan(&scope, &ref); err != nil {
			return e, err
		}
		switch scope {
		case "global":
			e.Global = true
		case "make":
			e.Makes[ref] = true
		case "model":
			e.Models[ref] = true
		case "year":
			e.Years[ref] = true
		}
	}
	return e, rows.Err()
}

func (s *Store) SetEntitlement(userID int64, scope string, ref int64) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO entitlements (user_id, scope, ref) VALUES (?, ?, ?)`, userID, scope, ref)
	return err
}

func (s *Store) ClearEntitlements(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM entitlements WHERE user_id = ?`, userID)
	return err
}

// -------- sessions --------

func (s *Store) CreateSession(userID int64, tokenHash, expiresAt string) error {
	_, err := s.db.Exec(`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES (?, ?, ?)`, userID, tokenHash, expiresAt)
	return err
}

func (s *Store) UserByToken(tokenHash string) (User, error) {
	var u User
	err := s.db.QueryRow(`SELECT u.id, u.email, u.password_hash, u.role
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > datetime('now')`, tokenHash).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role)
	if err == sql.ErrNoRows {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// -------- search (FTS5) --------

func (s *Store) Search(q string, e Entitlements, admin bool) ([]ModelHit, error) {
	match := buildMatch(q)
	rows, err := s.db.Query(`SELECT
		mk.id, mk.name, mk.internal_only,
		m.id, m.make_id, m.name, m.display_name, m.year, m.dataset_year, m.region, m.internal_only
		FROM catalog_fts f
		JOIN models m ON m.id = f.rowid
		JOIN makes mk ON mk.id = m.make_id
		WHERE catalog_fts MATCH ?
		ORDER BY rank LIMIT 100`, match)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []ModelHit
	for rows.Next() {
		var mk Make
		var m Model
		if err := rows.Scan(&mk.ID, &mk.Name, &mk.InternalOnly,
			&m.ID, &m.MakeID, &m.Name, &m.DisplayName, &m.Year, &m.DatasetYear, &m.Region, &m.InternalOnly); err != nil {
			return nil, err
		}
		if e.Visible(m, mk, admin) {
			hits = append(hits, ModelHit{Model: m, Make: mk})
		}
	}
	return hits, rows.Err()
}

func buildMatch(q string) string {
	var words []string
	cur := ""
	for _, c := range q {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			cur += string(c)
		} else if cur != "" {
			words = append(words, cur)
			cur = ""
		}
	}
	if cur != "" {
		words = append(words, cur)
	}
	match := ""
	for i, w := range words {
		if i > 0 {
			match += " AND "
		}
		match += `"` + w + `"`
	}
	return match
}

// -------- ACL lookup helpers used by the HTTP layer --------

func (s *Store) ModelByID(id int64) (Model, error) {
	var m Model
	err := s.db.QueryRow(`SELECT id, make_id, name, display_name, year, dataset_year, region, internal_only FROM models WHERE id = ?`, id).
		Scan(&m.ID, &m.MakeID, &m.Name, &m.DisplayName, &m.Year, &m.DatasetYear, &m.Region, &m.InternalOnly)
	if err == sql.ErrNoRows {
		return m, ErrNotFound
	}
	return m, err
}

func (s *Store) MakeByID(id int64) (Make, error) {
	var mk Make
	err := s.db.QueryRow(`SELECT id, name, internal_only FROM makes WHERE id = ?`, id).Scan(&mk.ID, &mk.Name, &mk.InternalOnly)
	if err == sql.ErrNoRows {
		return mk, ErrNotFound
	}
	return mk, err
}

func (s *Store) SystemWithContext(id int64) (ObjectRef, error) {
	var r ObjectRef
	err := s.db.QueryRow(`SELECT sy.id, sy.model_id, sy.code,
		 m.id, m.make_id, m.name, m.display_name, m.year, m.dataset_year, m.region, m.internal_only,
		 mk.id, mk.name, mk.internal_only
		 FROM systems sy
		 JOIN models m ON m.id = sy.model_id
		 JOIN makes mk ON mk.id = m.make_id
		 WHERE sy.id = ?`, id).
		Scan(&r.Sys.ID, &r.Sys.ModelID, &r.Sys.Code,
			&r.Mod.ID, &r.Mod.MakeID, &r.Mod.Name, &r.Mod.DisplayName, &r.Mod.Year, &r.Mod.DatasetYear, &r.Mod.Region, &r.Mod.InternalOnly,
			&r.Mk.ID, &r.Mk.Name, &r.Mk.InternalOnly)
	if err == sql.ErrNoRows {
		return r, ErrNotFound
	}
	return r, err
}