#!/usr/bin/env python3
"""queue_precheck.py: the merge queue's head, re-checked against the main of now (operator P0, 11-20).

A pull request's own checks ran against the main of when it was opened or last pushed. By the time Mergify batches
it, main has moved, and a PR that was green alone can fail once merged:
- a duplicate map key (#2514);
- a kgen refusal test (#2579, #2597);
- a pinned census (#2531);
- a game count over the supported set (#2803).
Each such red cost a 10-PR batch plus a bisect, 40-60 min of landing.

On every push to main (.github/workflows/queue-precheck.yml) this script:
1. Lists the queue in Mergify's order: open non-draft same-repo PRs to main whose fast and lint passed; priority
   (hotfix 3000, evl/ 2500, others 2000), then by when the later of fast and lint passed. It takes the first
   HEAD_N, Mergify's next two batches (batch_size 10, max_parallel_checks 2).
2. Merges them, in order, onto the new main in a scratch worktree. A PR that conflicts is left out; Mergify reports
   conflicts itself.
3. Runs the cheap checks once on the combined tree:
   - go vet ./... (compiles every package and its tests);
   - the census, ratchet and pin tests of the packages the queue touches;
   - ./dirac/kgen -short when the queue touches dirac/ or engine/.
4. Green: a success status `queue-precheck` on every checked head. Red: drops what fails on main alone (main itself
   is red, which is pipeline-fix's lane), then re-runs only the failing items on main plus each prefix of the
   queue. The PR whose merge first makes an item fail gets a failure status, which .mergify.yml's queue_conditions
   turn into a dequeue before its batch runs.

Run: python3 scripts/queue_precheck.py [--dry-run] [--head N]   (GITHUB_TOKEN, GITHUB_REPOSITORY in the env)
"""
import argparse
import datetime
import json
import os
import re
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request

CONTEXT = "queue-precheck"
HEAD_N = 20
PRIO_HOTFIX, PRIO_EVL, PRIO_DEFAULT = 3000, 2500, 2000
# test-smart's census family (scripts/test-targets.sh C), the tests that read pinned data and ratchets.
CENSUS = ("Census|Covers|IsCurrent|Unchanged|Ratchet|Padded|HasAClass|FileNames|OnlyShrinks|OnlyFalls|IotaBlock|NoVerbatim|"
          "PublicBoundary|Acceptance|EnumConst|Pinned$|AreRegistered|ParamsAreRead|FilesWellFormed|KnownKeys|OnlyGrows")
# Packages whose census tests are minutes long; the batch runs them (the oracle's lockstep census, engine/rules' heads).
CENSUS_SKIP_PKGS = ("./dirac/oracle", "./engine/rules")
KGEN = "./dirac/kgen"
# The repro replays a dirac change on its own breaks (11-20: dirac/mana*.go moved the stdmeta and MBC replays; #2731
# and #2695 broke TestStdDskReprosLockstep, TestStdPoolReprosLockstep and TestReproManifest). PR fast skips the
# replays (CI_FAST_SKIP_ORACLE_REPLAY), so they first ran in the batch. When the queue touches dirac's Go code outside
# kgen, or a recording, the precheck runs exactly the dirac-repros job's set: replays_rx reads its expression from
# .github/workflows/ci.yml, so the two cannot drift (~7 min under load, once per main push).
ORACLE = "./dirac/oracle"
# dirac's State size and the per-pool bar sum every PR's growth (register cause 96; 11-20 batch #2874: #2708's cold
# store put tail-18 at 45184 B against the 45056 bar). Only a merged check sees the sum; the tests are cheap.
STATE_BAR = "^(TestEveryPoolFitsTheStateBar|TestStateSize|TestColdArraysHoldNoPointers)$"
CI_YML = ".github/workflows/ci.yml"


def replays_rx(ci_yml_text):
    """The -run (or -list) expression of ci.yml's dirac-repros job: the replays a batch runs."""
    job = re.search(r"^  dirac-repros:\n(.*?)(?=^  [A-Za-z0-9_-]+:\n)", ci_yml_text + "\n  end:\n", re.M | re.S)
    if not job:
        raise RuntimeError("ci.yml has no dirac-repros job")
    m = re.search(r"go test [^\n]*?-(?:run|list) '([^']+)' \./dirac/oracle", job.group(1))
    if not m:
        raise RuntimeError("ci.yml's dirac-repros job names no -run/-list expression over ./dirac/oracle")
    return m.group(1)


def priority(pr):
    head = pr["headRefName"]
    if head.startswith("hotfix/") or "hotfix" in pr.get("labels", ()):
        return PRIO_HOTFIX
    if head.startswith("evl/"):
        return PRIO_EVL
    return PRIO_DEFAULT


def queued(prs, repo):
    """The PRs Mergify queues (fast and lint passed, non-draft, this repo, base main), in its order."""
    out = []
    for pr in prs:
        if pr["isDraft"] or pr["baseRefName"] != "main" or pr.get("headRepo") != repo:
            continue
        if pr["headRefName"].startswith("mergify/"):
            continue
        checks = pr.get("checks", {})
        if checks.get("fast", ("", ""))[0] != "SUCCESS" or checks.get("lint", ("", ""))[0] != "SUCCESS":
            continue
        if pr.get("mergeable") == "CONFLICTING":
            continue  # Mergify leaves a conflicting PR out of its batches too
        if checks.get(CONTEXT, ("", ""))[0] in ("FAILURE", "ERROR"):
            continue  # already dequeued by an earlier run, for this head
        at = max(checks["fast"][1] or "", checks["lint"][1] or "")
        out.append(dict(pr, queuedAt=at))
    out.sort(key=lambda p: (-priority(p), p["queuedAt"], p["number"]))
    return out


# Mergify's own order (11-20: #2744 was in batch #2869 but outside this script's estimate of the head, which also
# held PRs Mergify had not batched). Mergify opens its batch PRs (mergify/merge-queue/*) 1-5 min after a push to
# main, and each body names its PRs. The script waits up to BATCH_WAIT s for them, puts their PRs first, in the
# order of the newest, largest batch, and fills the rest from the estimate.
# 0 since the gate (11-22): a batch forms only after this posts statuses, so waiting for one at the start of a run
# only delayed them (the 20:32Z run waited 6 min for "none yet"). The open batch PRs are read once.
BATCH_WAIT = 0
# The replay set runs in this many concurrent chunks of its listed names: whole, it was a run's longest command
# (448 s on the home box, 11-22; vet 50, kgen 107, the census 93), and a green run must fit inside a batch (~8 min).
REPLAY_CHUNKS = 3


def batch_numbers(pulls, since):
    """The PR numbers of Mergify's batch PRs opened at or after since (ISO time), from the largest newest batch's
    body, in order; [] when none is open yet."""
    best = []
    for b in pulls:
        if not b["head"]["ref"].startswith("mergify/merge-queue/") or b["created_at"] < since:
            continue
        nums = []
        for m in re.findall(r"#(\d+)", b.get("body") or ""):
            if int(m) not in nums and int(m) != b["number"]:
                nums.append(int(m))
        if len(nums) >= len(best):
            best = nums
    return best


FRESH_N = 10  # PRs without a passing status this run checks beyond the batches: one batch's worth


def head_order(batch, estimate, n, fresh=FRESH_N):
    """Mergify's batch PRs first (those still queued, at most n), then the next `fresh` PRs of the estimate that have
    no passing queue-precheck status on their head. queue_conditions require that status (11-21), so the batched
    PRs all carry one already; without the fresh ones a run could check nothing but batches, and the queue would
    drain before any new PR earned its status."""
    by = {p["number"]: p for p in estimate}
    out = [by[x] for x in batch if x in by][:n]
    out += [p for p in estimate
            if p["number"] not in batch and p.get("checks", {}).get(CONTEXT, ("", ""))[0] != "SUCCESS"][:fresh]
    return out


def merge_until(order, target, merge):
    """Merges the PRs of order in turn until target of them merged; a PR whose merge fails (a conflict with main or
    with a PR ahead of it) is skipped and does not count. Returns the merged PRs."""
    merged = []
    for p in order:
        if len(merged) >= target:
            break
        if merge(p):
            merged.append(p)
        else:
            print("#%d conflicts; left to Mergify" % p["number"])
    return merged


QUERY = """query($owner:String!,$name:String!,$after:String){repository(owner:$owner,name:$name){
pullRequests(states:OPEN,baseRefName:"main",first:100,after:$after){pageInfo{hasNextPage endCursor}
nodes{number isDraft mergeable baseRefName headRefName headRefOid headRepository{nameWithOwner} labels(first:20){nodes{name}}
commits(last:1){nodes{commit{statusCheckRollup{contexts(first:100){nodes{
__typename ... on CheckRun{name conclusion completedAt} ... on StatusContext{context state createdAt}}}}}}}}}}}"""


def api(path, token, data=None, method=None):
    req = urllib.request.Request("https://api.github.com" + path, method=method,
                                 data=json.dumps(data).encode() if data is not None else None,
                                 headers={"Authorization": "Bearer " + token, "Accept": "application/vnd.github+json"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read() or b"null")


def list_prs(repo, token):
    owner, name = repo.split("/")
    prs, after = [], None
    while True:
        d = api("/graphql", token, {"query": QUERY, "variables": {"owner": owner, "name": name, "after": after}})
        page = d["data"]["repository"]["pullRequests"]
        for n in page["nodes"]:
            checks = {}
            for c in ((n["commits"]["nodes"] or [{}])[0].get("commit", {}).get("statusCheckRollup") or {}).get(
                    "contexts", {}).get("nodes", []):
                if c["__typename"] == "CheckRun":
                    checks[c["name"]] = (c["conclusion"] or "", c["completedAt"])
                else:
                    checks[c["context"]] = (c["state"], c["createdAt"])
            prs.append({"number": n["number"], "isDraft": n["isDraft"], "mergeable": n.get("mergeable"), "baseRefName": n["baseRefName"],
                        "headRefName": n["headRefName"], "sha": n["headRefOid"],
                        "headRepo": (n["headRepository"] or {}).get("nameWithOwner"),
                        "labels": [l["name"] for l in n["labels"]["nodes"]], "checks": checks})
        if not page["pageInfo"]["hasNextPage"]:
            return prs
        after = page["pageInfo"]["endCursor"]


def sh(*args, cwd=None, check=True):
    r = subprocess.run(args, cwd=cwd, capture_output=True, text=True)
    if check and r.returncode != 0:
        raise RuntimeError("%s: %s" % (" ".join(args), r.stderr[-2000:]))
    return r


class Tree:
    """A scratch worktree at a base commit, onto which PR heads are merged."""

    def __init__(self, path, base):
        self.path = path
        sh("git", "worktree", "remove", "--force", path, check=False)
        sh("git", "worktree", "add", "--detach", "-q", path, base)
        cards = os.path.abspath(".cards")
        if os.path.exists(cards) and not os.path.exists(os.path.join(path, ".cards")):
            os.symlink(os.path.realpath(cards), os.path.join(path, ".cards"))

    def merge(self, pr):
        sh("git", "fetch", "-q", "origin", "pull/%d/head" % pr["number"], cwd=self.path)
        r = sh("git", "-c", "user.email=ci@invalid", "-c", "user.name=ci", "merge", "-q", "--no-edit",
               "FETCH_HEAD", cwd=self.path, check=False)
        if r.returncode != 0:
            sh("git", "merge", "--abort", cwd=self.path, check=False)
            return False
        return True

    def close(self):
        sh("git", "worktree", "remove", "--force", self.path, check=False)


def touched(path, base):
    """The changed files of the tree at path against base."""
    return sh("git", "diff", "--name-only", base, "HEAD", cwd=path).stdout.split()


def plan(files, census_pkgs):
    """The checks for a set of changed files: (census packages, run kgen, run the replays, run the State bar)."""
    dirs = {"./" + f.rsplit("/", 1)[0] for f in files if "/" in f}
    pk = sorted(p for p in census_pkgs if p in dirs and p not in CENSUS_SKIP_PKGS)
    kgen = any(f.startswith(("dirac/", "engine/", "tablesrc.go")) for f in files)
    # Generated files and ordinals (11-20 #2744: ask ordinal 157 taken twice): their IsCurrent/Unchanged pins live
    # in ./dirac (TestAskKindsUnchanged, TestManifestGenIsCurrent) and in the generator's own package.
    if any(f.startswith("dirac/manifest/") or f.endswith("_gen.go") for f in files):
        gen = {"./dirac"} | {"./" + f.rsplit("/", 1)[0] for f in files if f.endswith("_gen.go") and "/" in f}
        pk = sorted(set(pk) | {p for p in census_pkgs if p in gen and p not in CENSUS_SKIP_PKGS})
    code = any(f.startswith("dirac/") and f.endswith(".go") and not f.endswith("_test.go")
               and not f.startswith("dirac/kgen/") for f in files)
    replays = code or any(f.startswith("dirac/oracle/testdata/repro/") for f in files)
    # The State bar also follows kgen's output: lowering more cards into a pool grows its cold arrays without any
    # dirac/*.go change. So it runs whenever kgen does (dirac/, engine/, tablesrc.go); it takes about a second.
    return pk, kgen, replays, kgen


def census_packages(path):
    out = sh("git", "grep", "-lE", "^func Test[A-Za-z0-9_]*(%s)" % CENSUS, "--", "*_test.go", cwd=path,
             check=False).stdout.split()
    return sorted({"./" + f.rsplit("/", 1)[0] for f in out if "/" in f})


def chunk_names(names, n):
    """-run expressions for n round-robin chunks of the sorted test names (empty chunks dropped)."""
    names = sorted(set(names))
    return ["^(%s)$" % "|".join(names[k::n]) for k in range(n) if names[k::n]]


def replay_chunks(path, env, rx):
    """The replay set as REPLAY_CHUNKS -run expressions over the names go test -list finds; the one expression when
    listing fails (the test run then reports why)."""
    r = subprocess.run(["go", "test", "-list", rx, ORACLE], cwd=path, capture_output=True, text=True, env=env)
    names = [ln for ln in r.stdout.splitlines() if ln.startswith("Test")]
    return chunk_names(names, REPLAY_CHUNKS) if r.returncode == 0 and names else [rx]


# Tree-wide ratchets: tests that measure the whole module, so a change anywhere can trip them, but they live in their
# own packages, which "the touched packages' census" never selects (11-21 batch #3304: #3171 grew dirac
# (*State).scriptOffers to complexity 106 against internal/codeshape's ceiling of 105). They run whenever any Go file
# changes: internal/codeshape whole (~10 s: function length and complexity, code shape, strings), internal/archtest
# without its 386 cross-build (~25 s: import boundaries, the only-shrinks boards, the public boundary).
TREE_WIDE = ((["./internal/codeshape"], "", ""), (["./internal/archtest"], "", "^TestEngineCompilesFor32Bit$"))


def tree_wide(files):
    """Whether the tree-wide ratchets run: any Go file changed."""
    return any(f.endswith(".go") for f in files)


def run_checks(path, pk, kgen, replays=False, state=False, only=None, tree=False):
    """Runs the checks in the tree at path; returns the failed items, each a (kind, package, test) tuple.
    only restricts the run to those items (attribution)."""
    env = dict(os.environ, GOFLAGS="-trimpath")
    t0 = time.time()
    if os.environ.get("QUEUE_PRECHECK_LOCAL"):  # a maintainer's dry run: the linked .cards, its own tables
        sh("make", "-s", "dirac-tables", cwd=path, check=False)
    else:
        sh("bash", "scripts/ci-corpus.sh", cwd=path, check=False)
    print("%s phase corpus+tables %.0f s" % (time.strftime("%H:%M:%S"), time.time() - t0), flush=True)
    fails = []
    vet_pkgs = sorted({p for k, p, _ in only if k == "vet"}) if only is not None else ["./..."]
    groups = []
    if only is None:
        if pk:
            groups.append((pk, CENSUS))
        if kgen:
            groups.append(([KGEN], ""))
        if state:
            groups.append((["./dirac"], STATE_BAR))
        if replays:
            with open(os.path.join(path, CI_YML)) as f:
                rx = replays_rx(f.read())
            groups.extend(([ORACLE], c) for c in replay_chunks(path, env, rx))
        if tree:
            groups.extend(TREE_WIDE)
    else:
        tests = {}
        for k, p, t in only:
            if k == "test":
                tests.setdefault(p, set()).add(t)
        groups = [([p], "^(%s)$" % "|".join(sorted(ts))) for p, ts in sorted(tests.items())]
    # Every command starts at once and the run waits for all of them: run one after another, vet, the censuses,
    # kgen, the State bar and the replays took 9-16 min, longer than the ~8 min batch the check is meant to gate
    # (11-21). Together the run is bound by its longest command (the replays, ~5-6 min).
    # Output goes to files, not pipes: a pipe the parent reads only in its turn fills (the replays' -json stream is
    # megabytes) and stalls its writer, which would serialise the commands again.
    def start(args):
        out, err = tempfile.TemporaryFile("w+"), tempfile.TemporaryFile("w+")
        p = subprocess.Popen(args, cwd=path, env=env, stdout=out, stderr=err, text=True)
        ended = {}
        threading.Thread(target=lambda: (p.wait(), ended.setdefault("t", time.time())), daemon=True).start()
        return p, out, err, time.time(), ended

    def finish(job):  # prints each command's own wall time (its end, not when its turn to be read came)
        p, out, err, began, ended = job
        p.wait()
        while "t" not in ended:
            time.sleep(0.05)
        print("%s phase %s: %.0f s, rc %d" % (time.strftime("%H:%M:%S"), " ".join(p.args[:2] + p.args[-2:])[:120],
                                              ended["t"] - began, p.returncode), flush=True)
        out.seek(0)
        err.seek(0)
        r = subprocess.CompletedProcess(p.args, p.returncode, out.read(), err.read())
        out.close()
        err.close()
        return r
    vet = start(["go", "vet", *vet_pkgs]) if vet_pkgs else None
    def test_args(g):  # (packages, -run regex[, -skip regex])
        pkgs, rx, skip = g[0], g[1], (g[2] if len(g) > 2 else "")
        return (["go", "test", "-short", "-json", "-timeout", "15m"] + (["-run", rx] if rx else [])
                + (["-skip", skip] if skip else []) + pkgs)
    procs = [(g[0], start(test_args(g))) for g in groups]
    if vet is not None:
        rv = finish(vet)
        if rv.returncode != 0:
            bad = sorted(set(re.findall(r"^# (\S+)", rv.stderr, re.M))) or ["./..."]
            for b in bad:
                fails.append(("vet", b if b.startswith(".") or b == "./..." else "./" + b.split("/gorge/", 1)[-1], ""))
            print(rv.stderr[-4000:], file=sys.stderr)
    for pkgs, job in procs:
        r = finish(job)
        for line in r.stdout.splitlines():
            try:
                e = json.loads(line)
            except ValueError:
                continue
            if e.get("Action") == "fail" and e.get("Test") and "/" not in e["Test"]:
                pkg = "./" + e["Package"].split("/gorge/", 1)[-1]
                fails.append(("test", pkg, e["Test"]))
        if r.returncode != 0 and not any(f[0] == "test" for f in fails):
            print(r.stdout[-3000:] + r.stderr[-3000:], file=sys.stderr)
            fails.append(("build", ",".join(pkgs), ""))
    return fails


def attribute(fails, order, on_main, on_prefix):
    """Who to blame for each failed item: on_main(items) -> items failing on main alone (dropped: main is red);
    on_prefix(k, items) -> items failing on main plus the first k PRs. The first k at which an item fails blames
    order[k-1]. Each item's first failing k is found by bisection (an item that fails at k fails at every later
    k, as the full queue showed), and one run at a k checks every item still open, so a red run builds about
    log2(len(order)) merged trees instead of one per PR (11-21: 24 min for a 9-PR head). Returns {pr: [items]}."""
    left = [f for f in fails if f not in set(on_main(fails))]
    n = len(order)
    if not left or n == 0:
        return {}

    def fails_at(k, items):
        need = [f for f in items if (k, f) not in checked]
        if need:
            bad = set(on_prefix(k, need))
            for f in need:
                checked[(k, f)] = f in bad
        return {f for f in items if checked[(k, f)]}

    checked = {(n, f): True for f in left}
    lo = {f: 1 for f in left}   # the first failing k is >= lo
    hi = {f: n for f in left}   # and <= hi
    while True:
        open_ = [f for f in left if lo[f] < hi[f]]
        if not open_:
            break
        # One k for the items whose ranges share it: the midpoint of the first open item's range.
        f0 = open_[0]
        k = (lo[f0] + hi[f0]) // 2
        group = [f for f in open_ if lo[f] <= k < hi[f]]
        bad = fails_at(k, group)
        for f in group:
            if f in bad:
                hi[f] = k
            else:
                lo[f] = k + 1
    blame = {}
    for f in left:
        blame.setdefault(order[hi[f] - 1]["number"], []).append(f)
    return blame


def set_status(repo, token, sha, state, desc, dry):
    print("status %s %s %s: %s" % (CONTEXT, sha[:10], state, desc))
    if not dry:
        api("/repos/%s/statuses/%s" % (repo, sha), token,
            {"state": state, "context": CONTEXT, "description": desc[:140]})


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--head", type=int, default=HEAD_N)
    ap.add_argument("--base", default="origin/main")
    a = ap.parse_args()
    repo, token = os.environ["GITHUB_REPOSITORY"], os.environ["GITHUB_TOKEN"]
    base = sh("git", "rev-parse", a.base).stdout.strip()
    since = sh("git", "show", "-s", "--format=%cI", base).stdout.strip()
    since = datetime.datetime.fromisoformat(since).astimezone(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    batch = []
    est = queued(list_prs(repo, token), repo)
    # A batch forms only from PRs that already passed here (queue_conditions, 11-21): with none passed there is no
    # batch to wait for, and waiting would only delay the statuses the queue needs.
    expect_batch = any(p["checks"].get(CONTEXT, ("", ""))[0] == "SUCCESS" for p in est)
    for _ in range(BATCH_WAIT // 30 + 1):
        pulls = api("/repos/%s/pulls?state=open&base=main&per_page=100" % repo, token)
        batch = batch_numbers(pulls, since)
        if batch or a.dry_run or not expect_batch:
            break
        time.sleep(30)
    print("Mergify's batches since %s: %s" % (since, " ".join("#%d" % x for x in batch) or "none yet"))
    # With no batch open, the next two batches' worth (Mergify runs two at once) earn their statuses in this run.
    fresh = FRESH_N if batch else 2 * FRESH_N
    # Every candidate, not a capped head: a PR that conflicts is skipped and does not use a slot (11-22: GitHub's
    # mergeable read UNKNOWN for 18 conflicting PRs at the head, every run merged none of them and posted no status,
    # and the gated queue merged nothing for 5.5 h).
    order = head_order(batch, est, a.head, fresh=len(est))
    print(time.strftime("%H:%M:%S"), "candidates (%d): %s" % (len(order), " ".join("#%d" % p["number"] for p in order)))
    if not order:
        return 0
    wt = os.path.abspath(".worktrees/queue-precheck")
    t = Tree(wt, base)
    nbatched = sum(1 for p in order if p["number"] in batch)
    merged = merge_until(order, nbatched + fresh, t.merge)
    print(time.strftime("%H:%M:%S"), "merged (%d): %s" % (len(merged), " ".join("#%d" % p["number"] for p in merged)))
    files = touched(wt, base)
    pk, kgen, replays, state = plan(files, census_packages(wt))
    tree = tree_wide(files)
    print(time.strftime("%H:%M:%S"), "checks: go vet ./...; census in %s; kgen=%s; replays=%s; state bar=%s; tree-wide=%s"
          % (" ".join(pk) or "-", kgen, replays, state, tree))
    fails = run_checks(wt, pk, kgen, replays, state, tree=tree)
    t.close()
    short = base[:10]
    if not fails:
        for p in merged:
            set_status(repo, token, p["sha"], "success", "green merged onto main %s with the queue ahead" % short, a.dry_run)
        return 0
    print("%s combined tree failed: %s" % (time.strftime("%H:%M:%S"), fails))

    def on_main(items):
        tm = Tree(wt, base)
        try:
            return run_checks(wt, pk, kgen, replays, state, only=items)
        finally:
            tm.close()

    def on_prefix(k, items):
        tp = Tree(wt, base)
        try:
            for p in merged[:k]:
                tp.merge(p)
            return run_checks(wt, pk, kgen, replays, state, only=items)
        finally:
            tp.close()

    blame = attribute(fails, merged, on_main, on_prefix)
    culprits = set(blame)
    for p in merged:
        if p["number"] in culprits:
            what = "; ".join("%s %s %s" % f for f in blame[p["number"]])
            set_status(repo, token, p["sha"], "failure", "red merged onto main %s: %s" % (short, what), a.dry_run)
        else:
            # queue_conditions require this status (11-21): a PR the attribution did not blame (the red was main's
            # alone, or another PR's) must not wait for a later run to batch.
            set_status(repo, token, p["sha"], "success",
                       "not the cause of a red merged onto main %s (culprits: %s)"
                       % (short, " ".join("#%d" % c for c in sorted(culprits)) or "main itself"), a.dry_run)
    return 0


if __name__ == "__main__":
    sys.exit(main())
