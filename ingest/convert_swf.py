#!/usr/bin/env python3
"""One-shot migration tool: render every .swf schematic to a Flate PDF.

The archive predates the Flash-player retirement, so its .swf files are
unviewable in any modern browser. This renders each one at 288 dpi (about 4x
the stage size) to a PDF named like the source file (my.swf -> my.pdf),
mirroring the source tree under --out. It is run once, on the migration
machine, never on the VPS. See deploy/DEPLOY.md step 3a.

Prereqs (Mac, one time):
    brew install imagemagick
    ingest/fetch_swfrender.sh     # builds classic swfrender with -r

Usage:
    python3 ingest/convert_swf.py --src "/path/GlobalJig Skemat/Skemat" \
        --out ingest/swfout
"""
import argparse
import glob
import os
import shutil
import subprocess
import sys
import tempfile
from concurrent.futures import ThreadPoolExecutor

def resolve_bin(name, env):
    if name:
        return name
    if os.environ.get("SWFRENDER"):
        return os.environ["SWFRENDER"]
    if env and os.path.exists(env):
        return env
    for cand in ("swfrender", "/opt/homebrew/bin/swfrender", "/usr/local/bin/swfrender"):
        if shutil.which(cand):
            return shutil.which(cand)
    raise SystemExit("swfrender not found; run ingest/fetch_swfrender.sh first")

MAGICK = shutil.which("magick") or shutil.which("convert")
if not MAGICK:
    raise SystemExit("ImageMagick (magick) not found; brew install imagemagick")

def one(args):
    swf, src, out, swfrender, res = args
    rel = os.path.relpath(swf, src)
    dst = os.path.join(out, os.path.splitext(rel)[0] + ".pdf")
    if os.path.exists(dst) and os.path.getsize(dst) > 5000:
        return ("skip", rel, "-")
    tmp = tempfile.mkdtemp(prefix="swfx_")
    png = os.path.join(tmp, "f.png")
    try:
        r = subprocess.run([swfrender, "-r", str(res), swf, "-o", png],
                           capture_output=True)
        if r.returncode != 0 or not os.path.exists(png) or os.path.getsize(png) <= 3000:
            return ("fail", rel, (r.stderr or b"")[-200:])
        mean = 0.0
        try:
            mean = float(subprocess.run(
                ["identify", "-format", "%[fx:mean]", png],
                capture_output=True, text=True).stdout or 0)
        except ValueError:
            pass
        if not (0.005 <= mean <= 0.9999):
            return ("blank", rel, f"mean={mean:.4f}")
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        subprocess.run([MAGICK, png, "-compress", "Zip", dst],
                       check=True, capture_output=True)
        if not os.path.getsize(dst) > 5000:
            return ("tiny", rel, f"{os.path.getsize(dst)}")
        return ("ok", rel, "-")
    except Exception as e:  # noqa
        return ("err", rel, str(e)[:200])
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--src", required=True, help="source tree root (Skemat/)")
    ap.add_argument("--out", required=True, help="mirror output dir (swfout/)")
    ap.add_argument("--swfrender", help="classic swfrender binary (default: ingest/bin/swfrender)")
    ap.add_argument("--res", type=int, default=288, help="ppp render resolution (default 288)")
    ap.add_argument("--jobs", type=int, default=10)
    ap.add_argument("--sample", type=int, help="only convert N random files (smoke test)")
    args = ap.parse_args()

    os.makedirs(args.out, exist_ok=True)
    in_repo = os.path.join(os.path.dirname(os.path.abspath(__file__)), "bin", "swfrender")
    swfrender = resolve_bin(args.swfrender, in_repo)

    swfs = sorted(glob.glob(args.src + "/**/*.swf", recursive=True))
    if args.sample:
        import random
        random.seed(3)
        swfs = random.sample(swfs, args.sample)
    print(f"swf={len(swfs)} renderer={swfrender} res={args.res} jobs={args.jobs}", flush=True)

    counts = {}
    fails = []
    with ThreadPoolExecutor(max_workers=args.jobs) as ex:
        for i, (st, rel, note) in enumerate(
                ex.map(one, ((f, args.src, args.out, swfrender, args.res) for f in swfs)), 1):
            counts[st] = counts.get(st, 0) + 1
            if st in ("fail", "err", "blank", "tiny"):
                fails.append((st, rel, str(note)[:160]))
            if i % 250 == 0:
                print(f"  {i}/{len(swfs)} {dict(counts)}", flush=True)
    print("FINAL", dict(counts), flush=True)
    with open(os.path.join(args.out, "swf_conversion_report.txt"), "w") as fh:
        for st, rel, note in fails:
            fh.write(f"{st}\t{rel}\t{note}\n")
    print("report:", os.path.join(args.out, "swf_conversion_report.txt"), flush=True)
    return 0 if counts.get("fail", 0) + counts.get("err", 0) == 0 else 1

if __name__ == "__main__":
    sys.exit(main())