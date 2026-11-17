#!/usr/bin/env bash
# ci-stable-mtimes.sh DIR...: set every file's mtime from its content, so Go's test cache hits in CI.
# Measured by the first queued PR after e885cdd2f8: compare its "(cached)" count with the 0-25% before.
# Go keys a cached test result on the size and mtime of every file the test opens. A fresh checkout, a restored corpus
# and freshly built dirac tables give each file a new mtime, so no test that reads testdata, .cards or the tables ever
# reuses the Go cache restored from main. Same content gives the same mtime (a hit); changed content gives a new one.
set -euo pipefail
python3 - "$@" <<'PY'
import os, sys, zlib
base = 946684800  # 2000-01-01: far from "now", which Go refuses to cache
n = 0
for root in sys.argv[1:]:
    for d, dirs, files in os.walk(root):
        dirs[:] = [x for x in dirs if x not in (".git", "node_modules")]
        for f in files:
            p = os.path.join(d, f)
            if os.path.islink(p):
                continue
            h = 0
            with open(p, "rb") as fh:
                while chunk := fh.read(1 << 20):
                    h = zlib.crc32(chunk, h)
            t = base + h % 500_000_000
            os.utime(p, (t, t)); n += 1
print(f"ci-stable-mtimes: {n} files")
PY
