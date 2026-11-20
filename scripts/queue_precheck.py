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
import json
import os
import re
import subprocess
import sys
import urllib.request

CONTEXT = "queue-precheck"
HEAD_N = 20
PRIO_HOTFIX, PRIO_EVL, PRIO_DEFAULT = 3000, 2500, 2000
# test-smart's census family (scripts/test-targets.sh C), the tests that read pinned data and ratchets.
CENSUS = ("Census|Covers|IsCurrent|Ratchet|Padded|HasAClass|FileNames|OnlyShrinks|OnlyFalls|IotaBlock|NoVerbatim|"
          "PublicBoundary|Acceptance|EnumConst|Pinned$|AreRegistered|ParamsAreRead|FilesWellFormed|KnownKeys|OnlyGrows")
# Packages whose census tests are minutes long; the batch runs them (the oracle's lockstep census, engine/rules' heads).
CENSUS_SKIP_PKGS = ("./dirac/oracle", "./engine/rules")
KGEN = "./dirac/kgen"


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
        if checks.get(CONTEXT, ("", ""))[0] in ("FAILURE", "ERROR"):
            continue  # already dequeued by an earlier run, for this head
        at = max(checks["fast"][1] or "", checks["lint"][1] or "")
        out.append(dict(pr, queuedAt=at))
    out.sort(key=lambda p: (-priority(p), p["queuedAt"], p["number"]))
    return out


QUERY = """query($owner:String!,$name:String!,$after:String){repository(owner:$owner,name:$name){
pullRequests(states:OPEN,baseRefName:"main",first:100,after:$after){pageInfo{hasNextPage endCursor}
nodes{number isDraft baseRefName headRefName headRefOid headRepository{nameWithOwner} labels(first:20){nodes{name}}
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
            prs.append({"number": n["number"], "isDraft": n["isDraft"], "baseRefName": n["baseRefName"],
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
    """The checks for a set of changed files: (vet, census packages, run kgen)."""
    dirs = {"./" + f.rsplit("/", 1)[0] for f in files if "/" in f}
    pk = sorted(p for p in census_pkgs if p in dirs and p not in CENSUS_SKIP_PKGS)
    kgen = any(f.startswith(("dirac/", "engine/", "tablesrc.go")) for f in files)
    return pk, kgen


def census_packages(path):
    out = sh("git", "grep", "-lE", "^func Test[A-Za-z0-9_]*(%s)" % CENSUS, "--", "*_test.go", cwd=path,
             check=False).stdout.split()
    return sorted({"./" + f.rsplit("/", 1)[0] for f in out if "/" in f})


def run_checks(path, pk, kgen, only=None):
    """Runs the checks in the tree at path; returns the failed items, each a (kind, package, test) tuple.
    only restricts the run to those items (attribution)."""
    env = dict(os.environ, GOFLAGS="-trimpath")
    if os.environ.get("QUEUE_PRECHECK_LOCAL"):  # a maintainer's dry run: the linked .cards, its own tables
        sh("make", "-s", "dirac-tables", cwd=path, check=False)
    else:
        sh("bash", "scripts/ci-corpus.sh", cwd=path, check=False)
    fails = []
    vet_pkgs = sorted({p for k, p, _ in only if k == "vet"}) if only is not None else ["./..."]
    if vet_pkgs:
        r = subprocess.run(["go", "vet", *vet_pkgs], cwd=path, capture_output=True, text=True, env=env)
        if r.returncode != 0:
            bad = sorted(set(re.findall(r"^# (\S+)", r.stderr, re.M))) or ["./..."]
            for b in bad:
                fails.append(("vet", b if b.startswith(".") or b == "./..." else "./" + b.split("/gorge/", 1)[-1], ""))
            print(r.stderr[-4000:], file=sys.stderr)
    groups = []
    if only is None:
        if pk:
            groups.append((pk, CENSUS))
        if kgen:
            groups.append(([KGEN], ""))
    else:
        tests = {}
        for k, p, t in only:
            if k == "test":
                tests.setdefault(p, set()).add(t)
        groups = [([p], "^(%s)$" % "|".join(sorted(ts))) for p, ts in sorted(tests.items())]
    for pkgs, rx in groups:
        args = ["go", "test", "-short", "-json", "-timeout", "15m"] + (["-run", rx] if rx else []) + pkgs
        r = subprocess.run(args, cwd=path, capture_output=True, text=True, env=env)
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
    order[k-1]. Returns {pr number: [items]}."""
    left = [f for f in fails if f not in set(on_main(fails))]
    blame = {}
    for k in range(1, len(order) + 1):
        if not left:
            break
        bad = set(on_prefix(k, left))
        for f in [f for f in left if f in bad]:
            blame.setdefault(order[k - 1]["number"], []).append(f)
        left = [f for f in left if f not in bad]
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
    order = queued(list_prs(repo, token), repo)[:a.head]
    print("queue head (%d): %s" % (len(order), " ".join("#%d" % p["number"] for p in order)))
    if not order:
        return 0
    wt = os.path.abspath(".worktrees/queue-precheck")
    t = Tree(wt, base)
    merged = [p for p in order if t.merge(p) or print("#%d conflicts; left to Mergify" % p["number"])]
    files = touched(wt, base)
    pk, kgen = plan(files, census_packages(wt))
    print("checks: go vet ./...; census in %s; kgen=%s" % (" ".join(pk) or "-", kgen))
    fails = run_checks(wt, pk, kgen)
    t.close()
    short = base[:10]
    if not fails:
        for p in merged:
            set_status(repo, token, p["sha"], "success", "green merged onto main %s with the queue ahead" % short, a.dry_run)
        return 0
    print("combined tree failed: %s" % fails)

    def on_main(items):
        tm = Tree(wt, base)
        try:
            return run_checks(wt, pk, kgen, only=items)
        finally:
            tm.close()

    def on_prefix(k, items):
        tp = Tree(wt, base)
        try:
            for p in merged[:k]:
                tp.merge(p)
            return run_checks(wt, pk, kgen, only=items)
        finally:
            tp.close()

    blame = attribute(fails, merged, on_main, on_prefix)
    culprits = set(blame)
    for p in merged:
        if p["number"] in culprits:
            what = "; ".join("%s %s %s" % f for f in blame[p["number"]])
            set_status(repo, token, p["sha"], "failure", "red merged onto main %s: %s" % (short, what), a.dry_run)
        elif not culprits:
            pass  # red on main alone, or only in combination we could not pin: the batch decides
    return 0


if __name__ == "__main__":
    sys.exit(main())
