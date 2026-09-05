import os
import shutil
import sqlite3
import subprocess
import sys
import tempfile
import unittest

THIS = os.path.dirname(os.path.abspath(__file__))


def make_fixture(root: str):
    src = os.path.join(root, "Skemat")
    os.makedirs(os.path.join(src, "datasht-2013", "audi", "a3 [2012]", "2058"), exist_ok=True)
    os.makedirs(os.path.join(src, "datasht-2005", "mitsbshi", "lancer", "700"), exist_ok=True)
    os.makedirs(os.path.join(src, "datasht-2005", "(AUS)", "ford", "Escape", "1114"), exist_ok=True)
    os.makedirs(os.path.join(src, "datasht-2005", "pdf"), exist_ok=True)
    os.makedirs(os.path.join(src, "Te tjera", "bmw"), exist_ok=True)
    for p, files in {
        os.path.join(src, "datasht-2013", "audi", "a3 [2012]", "2058"): ["2058_1.pdf", "2058_2.pdf"],
        os.path.join(src, "datasht-2005", "mitsbshi", "lancer", "700"): ["700_1.jpg"],
        os.path.join(src, "datasht-2005", "(AUS)", "ford", "Escape", "1114"): ["1114_1.pdf"],
        os.path.join(src, "datasht-2005", "pdf"): ["indice_en.pdf", "indice_it.pdf"],
        os.path.join(src, "Te tjera", "bmw"): ["1.pdf"],
    }.items():
        for f in files:
            with open(os.path.join(p, f), "wb") as fh:
                fh.write(b"%PDF-fake")
    return src


class TestIngest(unittest.TestCase):
    def test_golden_and_idempotent(self):
        root = tempfile.mkdtemp()
        src = make_fixture(root)
        dest = os.path.join(root, "live")
        db = os.path.join(root, "i.db")
        run = ["python3", os.path.join(THIS, "ingest.py"), "--source", src,
               "--dest", dest, "--db", db,
               "--schema", os.path.normpath(os.path.join(THIS, "..", "internal", "store", "schema.sql"))]
        first = subprocess.run(run, capture_output=True, text=True, check=True)
        con = sqlite3.connect(db)
        stats = con.execute(
            "SELECT (SELECT COUNT(*) FROM makes), (SELECT COUNT(*) FROM models), "
            "(SELECT COUNT(*) FROM systems), (SELECT COUNT(*) FROM objects)"
        ).fetchone()
        self.assertEqual(stats, (5, 5, 5, 7), first.stderr)
        rows = con.execute(
            "SELECT mk.name, m.name, m.display_name, m.dataset_year, m.region, m.internal_only "
            "FROM models m JOIN makes mk ON mk.id=m.make_id ORDER BY mk.name, m.display_name"
        ).fetchall()
        by_display = [(r[0], r[2]) for r in rows]
        by_model = [(r[0], r[1]) for r in rows]
        self.assertIn(("Audi", "a3 [2012]"), by_display)       # alias not used, titlecase
        self.assertIn(("Mitsubishi", "lancer"), by_display)    # aliased make
        self.assertIn(("Ford", "Escape (AUS)"), by_model)      # region suffix on model name
        self.assertTrue(any(r[0] == "Ford" and r[2] == "Escape" and r[4] == "AUS" for r in rows))
        self.assertIn(("Reference", "Index & Manuals"), by_display)
        self.assertTrue(any(r[0] == "Reference" and r[5] == 1 for r in rows))
        self.assertIn(("BMW", "Misc (Te tjera)"), by_display)  # acronym make stays BMW
        self.assertTrue(any(r[0] == "BMW" and r[5] == 1 for r in rows))  # Te tjera internal
        fts = con.execute("SELECT COUNT(*) FROM catalog_fts").fetchone()[0]
        self.assertEqual(fts, 5)
        # file copied to dest with the system-code dir preserved
        self.assertTrue(os.path.exists(
            os.path.join(dest, "2013", "Audi", "a3 [2012]", "2058", "2058_1.pdf")))
        con.close()
        # idempotency: run again, same stats
        subprocess.run(run, capture_output=True, check=True)
        con = sqlite3.connect(db)
        stats2 = con.execute(
            "SELECT (SELECT COUNT(*) FROM makes), (SELECT COUNT(*) FROM models), "
            "(SELECT COUNT(*) FROM systems), (SELECT COUNT(*) FROM objects)"
        ).fetchone()
        self.assertEqual(stats, stats2)

    def test_recursive_subtrees_no_doubleentry(self):
        # A model with root files AND subdir systems AND nested content in a system
        # must not double-ingest (system '0' stays flat) and must preserve paths.
        root = tempfile.mkdtemp()
        src = os.path.join(root, "Skemat")
        m = os.path.join(src, "datasht-2005", "daihatsu", "cuore")
        os.makedirs(os.path.join(m, "1237", "pics"), exist_ok=True)
        os.makedirs(os.path.join(m, "99 968"), exist_ok=True)
        os.makedirs(os.path.join(src, "datasht-2005", "audi", "a4", "2WD-4WD", "1013"), exist_ok=True)
        for rel in ("1237_1.PDF", "1238_1.PDF", "1237/1237_1.PDF", "1237/1238_1.PDF",
                    "1237/pics/breakout.jpg", "99 968/1-6.swf"):
            with open(os.path.join(m, rel), "wb") as fh:
                fh.write(b"x")
        for rel in ("1013/1013_1a.pdf",):
            with open(os.path.join(src, "datasht-2005", "audi", "a4", "2WD-4WD", rel), "wb") as fh:
                fh.write(b"x")
        dest = os.path.join(root, "live")
        db = os.path.join(root, "i.db")
        run = ["python3", os.path.join(THIS, "ingest.py"), "--source", src,
               "--dest", dest, "--db", db,
               "--schema", os.path.normpath(os.path.join(THIS, "..", "internal", "store", "schema.sql"))]
        subprocess.run(run, capture_output=True, check=True)
        con = sqlite3.connect(db)
        stats = con.execute(
            "SELECT (SELECT COUNT(*) FROM makes), (SELECT COUNT(*) FROM models), "
            "(SELECT COUNT(*) FROM systems), (SELECT COUNT(*) FROM objects)"
        ).fetchone()
        self.assertEqual(stats, (2, 2, 4, 7), f"expected 2/2/4/7 got {stats}")
        dup = con.execute(
            "SELECT COUNT(*) FROM (SELECT rel_path FROM objects GROUP BY rel_path HAVING COUNT(*) > 1)"
        ).fetchone()[0]
        self.assertEqual(dup, 0)
        for rel in ("2005/Daihatsu/cuore/1237/1237_1.PDF",
                    "2005/Daihatsu/cuore/1237/pics/breakout.jpg",
                    "2005/Daihatsu/cuore/99 968/1-6.swf",
                    "2005/Audi/a4/2WD-4WD/1013/1013_1a.pdf"):
            self.assertTrue(os.path.exists(os.path.join(dest, rel)), rel)
        con.close()


if __name__ == "__main__":
    unittest.main()