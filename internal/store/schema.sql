CREATE TABLE IF NOT EXISTS makes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  internal_only INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS models (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  make_id INTEGER NOT NULL REFERENCES makes(id),
  name TEXT NOT NULL,
  display_name TEXT NOT NULL,
  year INTEGER NOT NULL DEFAULT 0,        -- parsed [YYYY] or dataset year
  dataset_year INTEGER NOT NULL DEFAULT 0,
  region TEXT NOT NULL DEFAULT '',
  internal_only INTEGER NOT NULL DEFAULT 0,
  UNIQUE (name, dataset_year)
);

CREATE TABLE IF NOT EXISTS systems (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  model_id INTEGER NOT NULL REFERENCES models(id),
  code TEXT NOT NULL,
  UNIQUE (model_id, code)
);

CREATE TABLE IF NOT EXISTS objects (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  system_id INTEGER NOT NULL REFERENCES systems(id),
  filename TEXT NOT NULL,
  display TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'other',     -- pdf|jpg|png|swf|other
  rel_path TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  UNIQUE (system_id, filename)
);

CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'customer' CHECK (role IN ('admin','staff','customer')),
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS entitlements (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id),
  scope TEXT NOT NULL CHECK (scope IN ('global','make','model','year')),
  ref INTEGER,                           -- make_id / model_id / dataset_year; NULL for global
  UNIQUE (user_id, scope, ref)
);

CREATE TABLE IF NOT EXISTS sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL UNIQUE,       -- sha256 hex of the cookie token
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_models_make ON models(make_id);
CREATE INDEX IF NOT EXISTS idx_systems_model ON systems(model_id);
CREATE INDEX IF NOT EXISTS idx_objects_system ON objects(system_id);
CREATE INDEX IF NOT EXISTS idx_ent_user ON entitlements(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE VIRTUAL TABLE IF NOT EXISTS catalog_fts USING fts5(
  model_name, make_name, system_code
);