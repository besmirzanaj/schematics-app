#!/usr/bin/env python3
"""Normalize the schematics source tree into data/live + a sqlite index.

Stdlib only. Idempotent: catalog tables are wiped and rebuilt; files are
overwritten. Kind map: pdf/jpg/jpeg/png/swf -> pdf/jpg/png/swf, else other.

Disk layout under DEST mirrors the source hierarchy:
  DEST/<dataset_year>/<make>/<model>/[<region>/]<system_code>/<file>
System-code subdirectories are preserved in the live tree so distinct systems
under the same model never collide on filenames.
"""
import argparse
import os
import re
import shutil
import sqlite3
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from aliases import canonical_make

KIND_BY_EXT = {
    ".pdf": "pdf", ".jpg": "jpg", ".jpeg": "jpg",
    ".png": "png", ".swf": "swf",
}
REGION_RE = re.compile(r"^\(([^)]+)\)$")
YEAR_RE = re.compile(r"\[(\d{4})\]")

# (section-name, prefix) for the synthetic reference/misc folders
REFERENCE_FOLDERS = {"optional", "pdf"}


def natural_key(name: str) -> tuple:
    def to_int(tok):
        return int(tok) if tok.isdigit() else tok

    parts = re.split(r"(\d+)", name.lower())
    return tuple(to_int(p) for p in parts)


def kind_of(filename: str) -> str:
    return KIND_BY_EXT.get(os.path.splitext(filename)[1].lower(), "other")


def safe_dir(s: str) -> str:
    return re.sub(r"[^A-Za-z0-9 _\-]", "", s).strip()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--source", default="../Skemat")
    ap.add_argument("--dest", default="data/live")
    ap.add_argument("--db", default="data/skemat.db")
    ap.add_argument("--schema", default="internal/store/schema.sql")
    args = ap.parse_args()

    os.makedirs(args.dest, exist_ok=True)
    os.makedirs(os.path.dirname(args.db) or ".", exist_ok=True)
    con = sqlite3.connect(args.db)
    schema_sql = open(args.schema).read()
    con.executescript(schema_sql)
    cur = con.cursor()

    # guard: INSERT OR REPLACE silently merges rows if the objects uniqueness
    # key is the legacy (system_id, filename); a stale table must be rebuilt
    # with the current (system_id, rel_path) constraint.
    tbl_sql = cur.execute(
        "SELECT sql FROM sqlite_master WHERE type='table' AND name='objects'"
    ).fetchone()[0]
    if "UNIQUE (system_id, rel_path)" not in tbl_sql:
        cur.execute("DROP TABLE objects")
        con.executescript(schema_sql)

    # wipe catalog tables (idempotency); users/entitlements/sessions survive
    for t in ("objects", "systems", "models", "makes"):
        cur.execute(f"DELETE FROM {t}")
    cur.execute("DELETE FROM catalog_fts")

    def upsert_make(name, internal):
        cur.execute(
            "INSERT OR IGNORE INTO makes (name, internal_only) VALUES (?, ?)",
            (name, int(internal)),
        )
        row = cur.execute("SELECT id, internal_only FROM makes WHERE name = ?", (name,)).fetchone()
        if int(internal) and not row[1]:
            cur.execute("UPDATE makes SET internal_only = 1 WHERE id = ?", (row[0],))
        return row[0]

    def upsert_model(make_id, name, display, year, dataset_year, region, internal):
        cur.execute(
            "INSERT OR IGNORE INTO models (make_id, name, display_name, year, dataset_year, region, internal_only) "
            "VALUES (?, ?, ?, ?, ?, ?, ?)",
            (make_id, name, display, year, dataset_year, region, int(internal)),
        )
        row = cur.execute("SELECT id FROM models WHERE name = ? AND dataset_year = ?", (name, dataset_year)).fetchone()
        return row[0]

    def upsert_system(model_id, code):
        cur.execute(
            "INSERT OR IGNORE INTO systems (model_id, code) VALUES (?, ?)", (model_id, code)
        )
        row = cur.execute(
            "SELECT id FROM systems WHERE model_id = ? AND code = ?", (model_id, code)
        ).fetchone()
        return row[0]

    def add_files(system_id, srcdir, dstbase, recursive):
        if recursive:
            rels = []
            for cwd, dirs, files in os.walk(srcdir):
                dirs[:] = sorted(d for d in dirs if not d.startswith("."))
                for f in files:
                    if not f.startswith("."):
                        rels.append(os.path.relpath(os.path.join(cwd, f), srcdir))
        else:
            rels = [f for f in os.listdir(srcdir)
                    if os.path.isfile(os.path.join(srcdir, f)) and not f.startswith(".")]
        rels = sorted(rels, key=natural_key)
        for order, rel in enumerate(rels, start=1):
            src = os.path.join(srcdir, rel)
            dstrel = os.path.normpath(os.path.join(dstbase, rel))
            dst = os.path.join(args.dest, dstrel)
            os.makedirs(os.path.dirname(dst) or ".", exist_ok=True)
            if os.path.exists(dst):
                os.chmod(dst, 0o644)
            shutil.copy2(src, dst)
            cur.execute(
                "INSERT OR REPLACE INTO objects (system_id, filename, display, kind, rel_path, sort_order) "
                "VALUES (?, ?, ?, ?, ?, ?)",
                (system_id, os.path.basename(rel), os.path.splitext(os.path.basename(rel))[0],
                 kind_of(rel), dstrel, order),
            )

    def add_model_files(make_name, model_dir, dataset_year, year, region, internal, files_root, model_display=None):
        make_id = upsert_make(make_name, False)
        model_display = model_display or model_dir
        model_name = f"{model_dir} ({region})" if region else model_dir
        mid = upsert_model(make_id, model_name, model_display, year, dataset_year, region, internal)
        # files directly under model dir (not in a system subdir) -> system code '0'
        files = [f for f in os.listdir(files_root)
                 if os.path.isfile(os.path.join(files_root, f)) and not f.startswith(".")]
        if files:
            sid = upsert_system(mid, "0")
            add_files(sid, files_root, f"{dataset_year}/{make_name}/{model_dir}/{region}", recursive=False)
        for entry in sorted(os.listdir(files_root)):
            full = os.path.join(files_root, entry)
            if not os.path.isdir(full) or entry.startswith("."):
                continue
            sid = upsert_system(mid, entry)
            add_files(sid, full, f"{dataset_year}/{make_name}/{model_dir}/{region}/{entry}", recursive=True)
        return mid

    def walk_make_dir(make_path, make_name, dataset_year, region=""):
        make_name = canonical_make(make_name)
        for entry in sorted(os.listdir(make_path)):
            full = os.path.join(make_path, entry)
            if not os.path.isdir(full) or entry.startswith("."):
                continue
            yr = YEAR_RE.search(entry)
            year = int(yr.group(1)) if yr else dataset_year
            add_model_files(make_name, entry, dataset_year, year, region, False, full)

    # datasht-YYYY trees
    for root_entry in sorted(os.listdir(args.source)):
        root_path = os.path.join(args.source, root_entry)
        if not os.path.isdir(root_path):
            continue
        if not root_entry.startswith("datasht-"):
            continue
        dataset_year = int(root_entry[len("datasht-"):])
        for entry in sorted(os.listdir(root_path)):
            full = os.path.join(root_path, entry)
            if not os.path.isdir(full) or entry.startswith("."):
                continue
            rm = REGION_RE.match(entry)
            if rm:
                for mk in sorted(os.listdir(full)):
                    walk_make_dir(os.path.join(full, mk), mk, dataset_year, region=rm.group(1))
            elif entry in REFERENCE_FOLDERS:
                make_id = upsert_make("Reference", True)
                display = {"pdf": "Index & Manuals", "optional": "Optional Images"}[entry]
                mid = upsert_model(make_id, f"{entry}-{dataset_year}", display, dataset_year, dataset_year, "", True)
                sid = upsert_system(mid, "0")
                add_files(sid, full, f"{dataset_year}/Reference/{entry}", recursive=False)
            else:
                walk_make_dir(full, entry, dataset_year)

    # Te tjera -> makes, model 'Misc (Te tjera)', staff-only
    misc_root = os.path.join(args.source, "Te tjera")
    if os.path.isdir(misc_root):
        for entry in sorted(os.listdir(misc_root)):
            full = os.path.join(misc_root, entry)
            if not os.path.isdir(full) or entry.startswith("."):
                continue
            yr = YEAR_RE.search(entry)
            make_name = canonical_make(entry)
            year = int(yr.group(1)) if yr else 0
            model_display = "Misc (Te tjera)"
            # model NAME must stay unique per make (UNIQUE(name, dataset_year))
            add_model_files(make_name, f"{make_name} (Te tjera)", year, year, "", True, full,
                            model_display=model_display)

    # rebuild FTS
    cur.execute("DELETE FROM catalog_fts")
    cur.execute(
        """INSERT INTO catalog_fts (rowid, model_name, make_name, system_code)
           SELECT m.id, m.display_name, mk.name, GROUP_CONCAT(sy.code, ' ')
           FROM models m JOIN makes mk ON mk.id = m.make_id
           JOIN systems sy ON sy.model_id = m.id
           GROUP BY m.id"""
    )
    con.commit()
    stats = cur.execute(
        "SELECT (SELECT COUNT(*) FROM makes), (SELECT COUNT(*) FROM models), "
        "(SELECT COUNT(*) FROM systems), (SELECT COUNT(*) FROM objects)"
    ).fetchone()
    print(f"makes={stats[0]} models={stats[1]} systems={stats[2]} objects={stats[3]}")


if __name__ == "__main__":
    main()