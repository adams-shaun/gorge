#!/usr/bin/env python3
"""test_targets.py: the pure parts of scripts/test-targets.sh (make test-one, test-targeted, test-smart).

  verdict   print the one verdict line for a status and exit with its code
  finish    classify a `go test -json` stream, print failing output and the verdict line
  select    the packages and tests `make test-targeted` runs for a diff (--list prints it)

The shell wrapper owns the cap, the preflight and the processes; everything that decides PASS, FAIL,
ENV, EMPTY or BUDGET, and everything that decides what to run, lives here so scripts/test_targets_test.py
can pin it. Stdlib only.

Verdict line (always the last line on stdout, and the only line that starts with "test-"):

    test-<mode>: PASS|FAIL|ENV|EMPTY|BUDGET|BUSY <sha> <pkgs> <seconds>

Exit codes: 0 PASS, 1 FAIL, 2 usage, 3 ENV, 4 EMPTY, 5 BUDGET, 75 BUSY.
"""
import argparse
import json
import re
import subprocess
import sys

EXIT = {"PASS": 0, "FAIL": 1, "ENV": 3, "EMPTY": 4, "BUDGET": 5, "BUSY": 75}

# A skip whose message says the corpus, the pools or dirac's tables are missing means the environment, not
# the code, decided the run: it is not a pass. "not in the FDN table" or "every pool card is supported" are
# the test's own subject and stay skips, so the table arm names the table's owner ("no FDN table") instead
# of matching any "no ... table" (which would catch "no Bird subtype in the table").
ENV_SKIP = re.compile(
    r"\.cards|fetch-cards|dirac-tables|link-cards|GORGE_POOLS|GORGE_CARDS|KGEN_DECKS|KERNEL_FDN_DECKS"
    r"|\bgorge corpus\b"
    r"|\b(no|missing)\b[^:]{0,30}\bcorpus\b"
    r"|\bcorpus\b[^:]{0,40}\b(missing|unavailable|lacks|has no cardsfolder|not (fetched|present|found))\b"
    r"|\bnot in the corpus\b|\bcorpus fixture\b"
    r"|\bpools?\b[^:]{0,20}\babsent|\bno\b[^:]{0,40}\bpool\b|\bsynthetic pools\b"
    r"|\b(no|missing)\b[^:]{0,20}\b(dirac|fdn|blb|set|card)\s+tables?\b|\bno\s+tables?\b", re.I)
NO_TESTS = "[no tests to run]"
MAX_DETAIL = 40

# Packages that stay out of the reverse-dependency closure: each is over the one-minute budget and runs
# post-merge or sharded (testing-study section 2).
EXCLUDE = ("./server/host", "./cmd/cardfuzz", "./testkit/conformance")
# Packages too big to run whole in the inner loop: only the named tests and the census tests run, unless a
# test helper, import or fixture changed (then only the whole package proves the tests it feeds).
HEAVY = ("./engine/rules", "./dirac/oracle") + EXCLUDE
# Packages that always run (the structural gates).
ALWAYS = ("./internal/archtest", "./internal/codeshape")
CENSUS = ("Census|Covers|IsCurrent|Ratchet|Padded|HasAClass|FileNames|OnlyShrinks|IotaBlock|NoVerbatim|"
          "PublicBoundary|Acceptance|EnumConst|^TestHeads$|^TestSetHeads$")
# Sub-second guards of the committed tables (repro compact encoding, repro glob table, kop ordinals) that
# fail the landing gate and the daemon precheck when a change breaks them. They run for every diff, as
# named tests, whatever the package's weight: forcing ./dirac/kop whole would be far slower.
GUARDS = (("./dirac/kop", "^TestKopOrdinalsUnchanged$"),
          ("./dirac/oracle", "^TestReproCompactRoundTrip$|^TestReproGlobMatchesTable$"))


def short_pkgs(pkgs, keep=3):
    pkgs = list(pkgs)
    if len(pkgs) <= keep:
        return ",".join(pkgs) or "-"
    return ",".join(pkgs[:keep]) + ",+%dmore" % (len(pkgs) - keep)


def verdict_line(mode, status, sha, pkgs, seconds):
    return "test-%s: %s %s %s %d" % (mode, status, sha or "-", short_pkgs(pkgs), int(seconds))


class Result:
    def __init__(self):
        self.status = "PASS"
        self.ran = 0
        self.skipped = 0
        self.reasons = []
        self.detail = []


def classify(lines, rc=0, named=()):
    """Classify the lines of a `go test -json` run. rc is go's exit status. named are the packages run
    with a -run filter that must match something. Returns a Result."""
    res = Result()
    out = {}  # (package, test) -> output lines
    failed = []  # (package, test) in order
    pkg_fail = []
    raw = []
    budget = env = empty = False
    named = set(named)
    for ln in lines:
        ln = ln.rstrip("\n")
        if not ln.strip():
            continue
        try:
            ev = json.loads(ln)
        except ValueError:
            raw.append(ln)
            continue
        if not isinstance(ev, dict):
            raw.append(ln)
            continue
        act, pkg, test, text = ev.get("Action"), ev.get("Package", ""), ev.get("Test", ""), ev.get("Output", "")
        if act in ("output", "build-output"):
            out.setdefault((pkg, test), []).append(text.rstrip("\n"))
            if "panic: test timed out" in text or "signal: killed" in text or "signal: terminated" in text:
                budget = True
                res.reasons.append("budget: " + text.strip())
            if NO_TESTS in text and not test and pkg in named:
                empty = True
                res.reasons.append("no tests ran in " + pkg)
            continue
        if act in ("pass", "fail", "skip") and test:
            if act == "skip":
                res.skipped += 1
                msg = " ".join(out.get((pkg, test), []))
                if ENV_SKIP.search(msg):
                    env = True
                    res.reasons.append("skipped for the environment: %s %s" % (pkg, test))
            else:
                res.ran += 1
            if act == "fail":
                failed.append((pkg, test))
        elif act in ("fail", "build-fail") and not test:
            pkg_fail.append(pkg)
    for pkg, test in failed:
        res.detail.append("--- FAIL: %s %s" % (pkg, test))
        res.detail.extend(l for l in out.get((pkg, test), []) if l.strip())
    for pkg in pkg_fail:
        if not any(p == pkg for p, _ in failed):
            res.detail.append("--- FAIL: %s" % pkg)
            res.detail.extend(l for l in out.get((pkg, ""), []) if l.strip() and not l.startswith(("FAIL", "ok ")))
    if rc != 0 and not failed and not pkg_fail and raw:
        res.detail.extend(raw[-MAX_DETAIL:])
    if rc in (137, 143, -9, -15):
        budget = True
        res.reasons.append("killed, rc=%d" % rc)
    if budget:
        res.status = "BUDGET"
    elif failed or pkg_fail or rc != 0:
        # A real failure wins over an environment skip: the code, not the box, decided the run.
        res.status = "FAIL"
    elif env:
        res.status = "ENV"
    elif empty or res.ran == 0:
        res.status = "EMPTY"
        if not res.reasons:
            res.reasons.append("no test ran")
    return res


def finish(mode, jsonfile, rc, sha, pkgs, seconds, named, out=sys.stdout):
    with open(jsonfile, errors="replace") as f:
        res = classify(f, rc, named)
    detail = res.detail[:MAX_DETAIL]
    for l in detail:
        print(l, file=out)
    if len(res.detail) > MAX_DETAIL:
        print("... %d more lines in %s" % (len(res.detail) - MAX_DETAIL, jsonfile), file=out)
    for r in res.reasons[:5]:
        print("  note: " + r, file=out)
    print("  tests ran=%d skipped=%d" % (res.ran, res.skipped), file=out)
    print(verdict_line(mode, res.status, sha, pkgs, seconds), file=out)
    return EXIT[res.status]


# ---- selection -------------------------------------------------------------------------------------

_DECL = re.compile(r"^(func|var|type|const)\b")
_TESTFN = re.compile(r"^func (Test[A-Za-z0-9_]*)\(")


def segments(lines):
    """Top-level declarations of a gofmt'd Go file: [(start, end, TestName|None)], 1-based inclusive.
    A comment block directly above a declaration belongs to it; lines before the first are a header."""
    starts = [i for i, l in enumerate(lines) if _DECL.match(l)]
    segs = []
    for k, i in enumerate(starts):
        s = i
        while s > 0 and lines[s - 1].startswith("//") and (k == 0 or s - 1 > starts[k - 1]):
            s -= 1
        segs.append([s + 1, 0, None])
        m = _TESTFN.match(lines[i])
        segs[-1][2] = m.group(1) if m else None
    for k in range(len(segs)):
        segs[k][1] = segs[k + 1][0] - 1 if k + 1 < len(segs) else len(lines)
    return segs


def named_tests(lines, ranges):
    """The Test functions a set of changed line ranges lies in. Returns (names, whole): whole is True when
    a range touches anything that is not a Test function (a helper, an import, a type), so the package
    must run whole. ranges None means a new file: every line changed."""
    segs = segments(lines)
    if ranges is None:
        ranges = [(1, max(len(lines), 1))]
    names, whole = [], False
    first = segs[0][0] if segs else len(lines) + 1
    for s, e in ranges:
        if s < first:
            whole = True
        hit = False
        for a, b, t in segs:
            if a <= e and s <= b:
                hit = True
                if t:
                    names.append(t)
                else:
                    whole = True
        if not hit and s >= first:
            whole = True
    return sorted(set(names)), whole


def pkg_of(path, dirmap):
    """The package import path a changed file belongs to, or None. testdata maps to its package."""
    d = path.rsplit("/", 1)[0] if "/" in path else "."
    if "/testdata/" in "/" + path:
        d = ("/" + path).split("/testdata/")[0].lstrip("/") or "."
    return dirmap.get(d)


def plan(changes, dirmap, deps, lines_of):
    """What test-targeted runs.

    changes   {path: ranges}: the changed files; ranges is a list of (start, end) new-file line ranges,
              or None for a file that is new or whose changes are not line-addressable
    dirmap    {repo-relative dir: import path} of every package
    deps      {import path: set of import paths} for every package that has a test binary
    lines_of  path -> lines of the file's current text

    Returns a list of (kind, package, regex): whole, named (regex must match), census (regex may match
    nothing). Package names are ./relative, as `go test` takes them. A package whose test helper, import or
    testdata fixture changed runs whole even when it is heavy: the tests it feeds are otherwise unprovable."""
    rel = {v: k for k, v in dirmap.items()}
    src, touched, named, forced_whole = set(), set(), {}, set()
    for path, ranges in sorted(changes.items()):
        if path.endswith(".md"):
            continue
        p = pkg_of(path, dirmap)
        if p is None:
            continue
        touched.add(p)
        if not path.endswith(".go"):
            if "/testdata/" in "/" + path:
                forced_whole.add(p)  # a fixture can move any test of the package
            else:
                src.add(p)
        elif not path.endswith("_test.go"):
            src.add(p)
        else:
            names, whole = named_tests(lines_of(path), ranges)
            named.setdefault(p, set()).update(names)
            if whole:
                forced_whole.add(p)

    def local(p):
        d = rel[p]
        return "./" + d if d != "." else "."

    out = {}
    skipped = []
    for p in sorted(touched):
        if p not in deps:
            continue  # no test files
        lp = local(p)
        if lp in EXCLUDE:
            if lp not in skipped:
                skipped.append(lp)
            continue
        heavy = lp in HEAVY
        if p in forced_whole:
            # A helper, type, import or testdata fixture can feed any test in the package: only the whole
            # package proves it, heavy or not (the census-only shortcut would print PASS on an untested test).
            out[lp] = ("whole", lp, "")
        elif heavy:
            names = sorted(named.get(p, ()))
            if names:
                out[lp] = ("named", lp, "^(%s)$|%s" % ("|".join(names), CENSUS))
            else:
                out[lp] = ("census", lp, CENSUS)
        elif p in src or p in forced_whole or not named.get(p):
            out[lp] = ("whole", lp, "")
        else:
            out[lp] = ("named", lp, "^(%s)$" % "|".join(sorted(named[p])))
    for p, ds in sorted(deps.items()):
        if not (ds & src) and p not in src:
            continue
        lp = local(p)
        if lp in EXCLUDE:
            if lp not in skipped:
                skipped.append(lp)
            continue
        if lp not in out or out[lp][0] != "whole":
            if lp not in HEAVY:
                out[lp] = ("whole", lp, "")
            elif lp not in out:
                out[lp] = ("census", lp, CENSUS)
    for lp, rx in GUARDS:
        if lp not in out:
            out[lp] = ("named", lp, rx)
        elif out[lp][0] != "whole":
            out[lp] = (out[lp][0], lp, out[lp][2] + "|" + rx)
    for lp in ALWAYS:
        out[lp] = ("whole", lp, "")
    res = [out[k] for k in sorted(out)]
    return res, skipped


def census_pkgs(changes, dirmap, deps, candidates):
    """The packages test-smart's census step runs, of candidates (./relative, the packages that hold a
    census test).

    A package runs when its test binary imports a package a changed non-test file is in, or when the change
    touches its directory (testdata included), or when it is in ALWAYS (the tree censuses). A changed file
    in a directory with no package belongs to the nearest package above it (a fixture tree, the support
    lists). A file whose nearest package is the module root (the Makefile, scripts/, decks/, docs/ data,
    .github/) can move any census, so then every candidate runs, as before. Markdown is never an input.

    Why (2026-11-12): on a fresh seat worktree go's result cache misses on every test that opens a file
    (it keys on absolute paths and mtimes), so a dirac-only diff re-ran engine/rules', engine/effects' and
    cmd/cardfuzz's censuses (TestTapeMissCensus alone plays 200 games), about 120 of the step's 190
    package-seconds, none of which a dirac change can move."""
    rel = {v: k for k, v in dirmap.items()}
    root = dirmap.get(".")
    src, touched = set(), set()
    for path in sorted(changes):
        if path.endswith(".md"):
            continue
        p = pkg_of(path, dirmap)
        d = path.rsplit("/", 1)[0] if "/" in path else "."
        while p is None and d not in (".", ""):
            d = d.rsplit("/", 1)[0] if "/" in d else "."
            p = dirmap.get(d)
        if p is None or p == root:
            return sorted(set(candidates))
        touched.add(p)
        if "/testdata/" not in "/" + path and not path.endswith("_test.go"):
            src.add(p)
    keep = set()
    for c in candidates:
        ip = dirmap.get(c[2:] if c.startswith("./") else c)
        if ip is None:
            keep.add(c)  # not a package this module lists: keep it rather than guess
            continue
        if ip in touched or (deps.get(ip, set()) & src) or c in ALWAYS:
            keep.add(c)
    keep.update(c for c in ALWAYS if c in candidates)
    return sorted(keep)


def cmd_census(a):
    dirmap, deps = go_graph()
    cands = [ln.strip() for ln in sys.stdin if ln.strip()]
    for p in census_pkgs(diff_changes(a.base), dirmap, deps, cands):
        print(p)
    return 0


def sh(*args, check=True):
    r = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, errors="replace")
    if check and r.returncode:
        sys.exit("test_targets: %s: %s" % (" ".join(args), r.stderr.strip()))
    return r.stdout


def go_graph():
    """dirmap and test-binary deps from `go list -e -test` (about 2 s)."""
    root = sh("git", "rev-parse", "--show-toplevel").strip()
    mod = sh("go", "list", "-m").strip()
    fmt = '{{.ImportPath}}|{{.Dir}}|{{join .Deps " "}}'
    dirmap, deps = {}, {}
    for ln in sh("go", "list", "-e", "-test", "-f", fmt, "./...").splitlines():
        ip, d, ds = (ln.split("|") + ["", ""])[:3]
        if " " in ip:
            continue
        if ip.endswith(".test"):
            base = ip[:-len(".test")]
            deps[base] = {x.split(" ")[0] for x in ds.split()} | {base}
        elif d.startswith(root) and ip.startswith(mod):
            dirmap[d[len(root):].lstrip("/") or "."] = ip
    return dirmap, deps


def diff_changes(base):
    """{path: ranges} for the working tree against the merge base with base, untracked files included."""
    mb = sh("git", "merge-base", base, "HEAD").strip()
    changes = {}
    for p in sh("git", "diff", "--name-only", mb).splitlines():
        changes[p] = None
    for p in sh("git", "ls-files", "-o", "--exclude-standard").splitlines():
        changes[p] = None
    cur = None
    for ln in sh("git", "diff", "-U0", mb, "--", "*_test.go").splitlines():
        if ln.startswith("+++ b/"):
            cur = ln[6:]
            changes[cur] = []
        elif ln.startswith("+++ "):
            cur = None
        elif ln.startswith("@@") and cur:
            m = re.search(r"\+(\d+)(?:,(\d+))?", ln)
            s, n = int(m.group(1)), int(m.group(2) if m.group(2) is not None else 1)
            changes[cur].append((s, max(s, s + n - 1)))
    return changes


def read_lines(path):
    try:
        with open(path, errors="replace") as f:
            return f.read().splitlines()
    except OSError:
        return []


def cmd_select(a):
    dirmap, deps = go_graph()
    changes = diff_changes(a.base)
    res, skipped = plan(changes, dirmap, deps, read_lines)
    for kind, pkg, rx in res:
        print("%s\t%s\t%s" % (kind, pkg, rx))
    for p in skipped:
        print("excluded\t%s\t" % p)
    return 0


def main(argv=None):
    ap = argparse.ArgumentParser(prog="test_targets.py")
    sub = ap.add_subparsers(dest="cmd", required=True)
    v = sub.add_parser("verdict")
    v.add_argument("--mode", required=True)
    v.add_argument("--status", required=True, choices=sorted(EXIT))
    v.add_argument("--sha", default="-")
    v.add_argument("--pkgs", default="")
    v.add_argument("--seconds", type=float, default=0)
    v.add_argument("--msg", default="")
    f = sub.add_parser("finish")
    f.add_argument("--mode", required=True)
    f.add_argument("--json", required=True)
    f.add_argument("--rc", type=int, default=0)
    f.add_argument("--sha", default="-")
    f.add_argument("--pkgs", default="")
    f.add_argument("--seconds", type=float, default=0)
    f.add_argument("--named", default="")
    s = sub.add_parser("select")
    s.add_argument("--base", default="land")
    c = sub.add_parser("census")
    c.add_argument("--base", default="land")
    a = ap.parse_args(argv)
    pk = [p for p in a.pkgs.split(",") if p] if hasattr(a, "pkgs") else []
    if a.cmd == "verdict":
        if a.msg:
            print("test-targets: %s: %s" % (a.mode, a.msg), file=sys.stderr)
        print(verdict_line(a.mode, a.status, a.sha, pk, a.seconds))
        return EXIT[a.status]
    if a.cmd == "finish":
        return finish(a.mode, a.json, a.rc, a.sha, pk, a.seconds, [p for p in a.named.split(",") if p])
    if a.cmd == "census":
        return cmd_census(a)
    return cmd_select(a)


if __name__ == "__main__":
    sys.exit(main())
