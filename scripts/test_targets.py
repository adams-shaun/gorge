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
import os
import re
import subprocess
import sys
import zlib

# The script directory, which `python3 -I` does not put on sys.path: the deal CI's oracle shards and the
# precheck's lanes share lives there (scripts/oracle_shard_deal.py), one implementation, not a copy.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import oracle_shard_deal  # noqa: E402

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
# The per-test wall budget (docs/site/contributing/testing.md): one minute at 4 cores. A test the diff adds or
# changes that runs longer makes the verdict BUDGET; one it leaves alone is only reported (docs/agents/known-slow.md).
TEST_BUDGET_S = 60.0
MAX_SLOW_LINES = 10

# Packages that stay out of the reverse-dependency closure: each is over the one-minute budget and runs
# post-merge or sharded (testing-study section 2).
EXCLUDE = ("./server/host", "./cmd/cardfuzz", "./testkit/conformance")
# Packages too big to run whole in the inner loop: only the named tests and the census tests run, unless a
# test helper, import or fixture changed (then only the whole package proves the tests it feeds).
HEAVY = ("./engine/rules", "./dirac/oracle") + EXCLUDE
# Packages that always run (the structural gates).
ALWAYS = ("./internal/archtest", "./internal/codeshape")
# A changed recording under dirac/oracle/testdata/repro runs the tests that read it (repro_named finds them
# by the recording's basename in the package's _test.go files) plus the fallback families below: the table
# tests that glob the directory and the per-pool lockstep replays (about a minute, not the whole package).
REPRO_DIR = "dirac/oracle/testdata/repro/"
# The Go tests that replay the recordings marked "named":true, a subtest each (repro_manifest_test.go): the
# shards TestReproManifest0..7. MANIFEST_TEST is their common prefix, so an unanchored -run element
# (TestReproManifest/<recording>) selects the recording in whichever shard holds it; MANIFEST_RX is every
# shard as one element of an anchored ^(...)$ list.
MANIFEST_TEST = "TestReproManifest"
MANIFEST_RX = MANIFEST_TEST + "[0-9]+"
# The per-pool lockstep replays (a pool's recordings are replayed by one glob test, one subtest per
# recording, named by the recording's file name). TF-01 slices them by subtest like the manifest (a
# test-level slice cannot bound them: a pool of a few hundred recordings is minutes of subtests). Only in the
# full-coverage fallback string.
REPRO_GLOBS = "^TestStd[A-Za-z]*ReprosLockstep[A-Za-z]*$"
# The refusal-staleness check, kept in every replay arm. The whole test is ~190 s
# of serialized entry searches (one entry's 200-pair pool hunt is 42 s: Whim of
# Volrath, 11-27), which no cost deal can fit into a lane, so it is 8 shards
# (TestRefusedEntriesStillDiverge0..7, entry i in shard i % 8). The lanes name only the
# shards and deal them like any other test; the parent TestRefusedEntriesStillDiverge is
# the dirac gate's entry and checks only the shards its own -run does not select.
REFUSED = "^TestRefusedEntriesStillDiverge[0-9]+$"
# Every committed recording's replay: the manifest's ~595 named recordings, the oracle's other per-card
# replay tests (the MBC tables) and the refusal-staleness check. TF-01 (2026-11-20): the precheck no longer
# runs this blanket — with it 337 of 851 oracle legs ended at the 5m timeout (NOTES 05:55Z), the manifest's
# subtests alone overrunning the wall — it replays the changed recordings plus a 1/SLICE_MOD slice (plan's reprosel,
# slice_lane). The slice rotates with the merge base, so over successive PRs every recording still replays;
# every recording also replays in the merge-queue batch, whose coverage does not shrink: ci.yml's "Replay the
# committed repros" step runs `go test -run 'Repro|^TestRefusedEntriesStillDiverge[0-9]+$' ./dirac/oracle`.
# The batch runs no other part of ./dirac/oracle but the sentinels (ci_shard.sh drops the package and ci.yml's
# `oracle` job is dispatch-only); the whole package runs on main in oracle.yml.
REPLAYS = "^Test[A-Za-z0-9_]*Repro[A-Za-z0-9_]*$|" + REFUSED
REPRO_TESTS = "^TestReproCompactRoundTrip$|^TestReproGlobMatchesTable$|" + REPRO_GLOBS + "|" + REPLAYS
# The recorded games a precheck leaves to the batch unless their recording changed: the recordings (and the
# oracle's other per-card replay tests) whose -json Elapsed in a batch run is over the 10 s wall. A listed test
# with no Repro in its name is not in the batch's replay step: test-smart's census step (OnlyGrows, Census)
# plays it before the merge when the diff reaches the package, oracle.yml after. A card's own fix PR re-records its game shorter (kfuzz -shrink)
# and drops the entry. Read like the fast tier is;
# outside testdata so a refresh edits no fixture (a changed fixture runs the package whole).
SLOW_REPROS = "dirac/oracle/slowrepros.txt"
# Pinned$..OnlyGrows: the pin and registration ratchets whose names carry none of the words before them (11-17:
# #611 registered kw:Modular, left its pin in granted-keyword-gaps.txt, and TestGrantedKeywordHeadsAreRegisteredOrPinned
# failed only in the Mergify batch, three times). About 66 tests in engine/rules and engine/effects, ~5 s.
CENSUS = ("Census|Covers|IsCurrent|Ratchet|Padded|HasAClass|FileNames|OnlyShrinks|IotaBlock|NoVerbatim|"
          "PublicBoundary|Acceptance|EnumConst|^TestHeads$|^TestSetHeads$|"
          "Pinned$|AreRegistered|ParamsAreRead|FilesWellFormed|KnownKeys|OnlyGrows|"
          "NoNewFixedStateFields|StateLayoutPinned|HashPins")  # #627 11-17: dirac State layout guards failed lint
# Sub-second guards of the committed tables (repro compact encoding, repro glob table, kop ordinals) that
# fail the landing gate and the daemon precheck when a change breaks them. They run for every diff, as
# named tests, whatever the package's weight: forcing ./dirac/kop whole would be far slower.
GUARDS = (("./dirac/kop", "^TestKopOrdinalsUnchanged$"),
          ("./dirac/oracle", "^TestReproCompactRoundTrip$|^TestReproGlobMatchesTable$"))

# test-smart's narrow precheck (GORGE_TEST_SMART_GATES unset; coverage audit 2026-11-14, section 5 "affected
# plus groups"): test-targeted's selection with three changes, and the dirac and affected gates left to the
# prescreen and the lander.
#  - ./engine/rules runs whole, minus RULES_SKIP and one of KERNEL_PAIR, when a changed non-test file reaches
#    its test binary: its card-behaviour tests are what a unit's own change breaks. A diff that cannot reach
#    it (a dirac-only diff: nothing it imports is under dirac/) does not run it.
#  - ./dirac/oracle never runs whole (its lockstep games are the prescreen's): a forced whole becomes the
#    changed tests plus the repro, pin, census and guard families, and a reaching change adds the fast tier
#    (dirac/oracle/testdata/fasttier.txt, the tests of 0.5 s or less the dirac gate's step 8b runs).
#  - NARROW_HEAVY, the corpus-heavy packages a change usually reaches only through an import (kgen's 830
#    test-s, kfuzz's lockstep replays), run only their census tests unless the change is in them; the census
#    step runs their pins either way.
#  - the lint packages of NARROW_LINT always run whole beside ALWAYS; internal/archtest skips the 386
#    cross-build unless needs_386.
# Table data (11-20): the non-Go files tablesrc.go embeds in directories with no Go package (evidence, refusals,
# pools, set lists, the manifest's keys). A change there moves dirac's tables, and so kgen's supported set, but
# maps to no package, so it selected nothing that reads the tables: #2803/#2804 went red on kfuzz
# TestFuzzSynthWithFlag (a game count over the supported set) that no PR's fast had run. Such a change runs
# TABLE_READERS whole: the tables' reader and kfuzz, whose tests play the synth pools.
TABLE_DATA = ("dirac/support/", "dirac/cardset/pools/", "dirac/cardset/setlists/", "dirac/manifest/key/",
              "dirac/manifest/mech/")
TABLE_READERS = ("dirac/cardset", "dirac/oracle/cmd/kfuzz")
# The censuses that read another package's source with go/parser instead of importing it, so the
# reverse-dependency closure above (which follows imports only) never selects them. Each pair is
# (reader package, the repo-relative source directories its tests parse); a changed non-test .go file
# under a source dir selects the reader whole. Measured cost of the missing pair: #5642 changed
# dirac/kgen's lowering to lower the Fortify keyword head, `fast` stayed green and merge batch
# 46056406666 went red on dirac/crcoverage's TestLoweredKeywordRowsAreNotNA ("kgen lowers keyword head
# Fortify (expandedKeywords) but it is in neither loweredKeywordRule nor loweredNotKeywordRule").
# Whole, not a census -run: the test that catches it need not carry a census name (that one does not),
# and these readers are cheap (dirac/crcoverage: 2 s, scripts/ci-pkg-times.tsv). A reader too heavy to
# run whole needs its own named row, not an entry here. Every reader in this table is already a
# candidate of test-smart's census step (test-targets.sh's C regexp, coverage audit 2026-11-14,
# section 5); this table supplies the pairs that step's import graph cannot see. Adding a census is one
# row.
SOURCE_READERS = (("dirac/crcoverage", ("dirac/kgen",)),)
# The precheck's oracle replay slice ran as one go test process (~200 top-level tests: the census and refusal
# families plus the replay slice, then 25%) and outran the package's 5m cap on 34 of the last 40 targeted runs (11-27);
# its summed test wall was ~500 s. test-targets.sh runs the slice as this many concurrent, cost-dealt lanes
# (scripts/oracle_shard_deal.py, the same deal CI's matrix parts use): ORACLE_LANES per subtest -run and
# ORACLE_PLAIN_LANES for the plain half. The plain half is serial top-level tests, one core a process: at 4
# lanes its processes used 4 of a remote run's 12 cores (GORGE_REMOTE_CPU) and every plain lane reached the
# 5m cap on 3 of 3 runs at 25a4e7ea9d..d28a4a61ec (282-300 s, the sub lanes 73-214 s), so it has twice
# the lanes: 8 plain and 4 sub processes fill the run's 12 cores.
ORACLE_LANES = 4
ORACLE_PLAIN_LANES = 8
# The replay slice keeps one merge-base lane of SLICE_MOD (slice_lane). At 1/4 the idle-box targeted run at
# 6d2f131f6c (94 idle cores) played ~2,400 core-s in its 12 lanes (165-259 s each) against the run's 12-core
# quota, which fits ~1,800 core-s in 150 s; the manifest, the pool globs and the sliced replay tests were 66% of
# it. At 1/6 they are two thirds of that, and every recording still replays once over six merge bases.
SLICE_MOD = 6
# The rotated shard families. The cmd5 clean-pair ratchet's shards (TestCmd5SliceCleanOnlyGrows0..7, 1500 lockstep games on 4 workers each)
# match the slice's CENSUS arm on every run. In the oracle-slice targeted run at 6ea733296d (12 lanes, 1.9 idle
# cores) they took 106-161 s each, ~1,090 of the slice's ~3,800 lane-s, while the run's 12-core quota fits ~2,900
# lane-s in 240 s. The slice plays the shards of its merge base's rotation lane only (shard i when i % ROTATION
# is crc32(sha) % ROTATION: two of the eight, every pair once over four bases), as it plays a slice of the
# recordings; a diff that names a shard (a change to the ratchet's test or pin) plays it. test-smart's census
# step (OnlyGrows) plays all eight when the diff reaches the package, and oracle.yml after the merge; the batch
# plays none. The eight refusal shards (REFUSED, entry i in shard i % 8) rotate the same way: in the
# 6d2f131f6c idle run they were 20% of the slice's core-s (shards 4 and 5 weigh 44 and 53 here). A diff that
# changes a refusal entry (REFUSED_ENTRIES) keeps all eight; test-smart checks every entry (the narrow census
# step runs the shards, the gates mode the dirac gate's parent TestRefusedEntriesStillDiverge), and the batch
# and oracle.yml play all eight shards.
CMD5_SHARD = re.compile(r"^TestCmd5SliceCleanOnlyGrows([0-9]+)$")
REFUSED_SHARD = re.compile(r"^TestRefusedEntriesStillDiverge([0-9]+)$")
ROTATED_SHARDS = (CMD5_SHARD, REFUSED_SHARD)
ROTATION = 4
REFUSED_ENTRIES = "dirac/support/refused/"  # the refusal shards' input: a diff here rotates none of them


def rotated_out(name, lane, kept=()):
    """Whether a rotated family's shard (ROTATED_SHARDS, less the families in kept) sits outside the merge base's
    rotation lane: shard i plays when i % ROTATION is the lane."""
    for rx in ROTATED_SHARDS:
        m = rx.match(name)
        if m and rx not in kept:
            return int(m.group(1)) % ROTATION != lane
    return False


def lane_procs(run_procs, lanes):
    """Each oracle lane's GOMAXPROCS: twice its even share of the run's, at least 2, never above the run's.

    With the run's whole GOMAXPROCS every one of 12 lanes held ~140 runnable threads on 12 cores, so a one-thread
    lane (a lone recorded game) got ~1/12 core until the parallel lanes drained: at 6d2f131f6c the Orzhova census
    (19 s alone) held its lane for 198 s. A share near the even one makes a lane's wall follow its cost, which
    the deal balances. A running process cannot take the cores a finished lane frees; the changed tests, whose
    wall the 60 s budget judges, therefore run outside the lanes (oracle_lanes)."""
    run_procs = max(run_procs, 1)
    share = -(-2 * run_procs // max(lanes, 1))
    return min(max(share, 2), run_procs)



# The floor of a lane's GOMEMLIMIT share: a lone replay's live heap; below it the GC would run near-continuously.
LANE_MEM_FLOOR_MIB = 256


def lane_memlimit(run_limit, lanes):
    """Each oracle lane's GOMEMLIMIT: its even share of the run's (a "<N>GiB" or "<N>MiB" string), at least
    LANE_MEM_FLOOR_MIB; a limit it cannot read is returned as it is.

    The lanes run side by side in the run's one scope (MemoryMax 8G, GOMEMLIMIT 6GiB locally). Each lane took
    the run's whole limit, so none of 12 lanes' GC pushed back before the scope's sum reached MemoryMax: the
    local targeted run at 984093e993 (2026-11-27) was OOM-killed 44 s into its lanes at an 8G peak, twice."""
    m = re.fullmatch(r"([0-9]+)(GiB|MiB)", run_limit or "")
    if not m:
        return run_limit
    total = int(m.group(1)) * (1024 if m.group(2) == "GiB" else 1)
    return "%dMiB" % max(total // max(lanes, 1), LANE_MEM_FLOOR_MIB)


RULES = "./engine/rules"
ORACLE = "./dirac/oracle"
ARCHTEST = "./internal/archtest"
NARROW_LINT = ("./cmd/codeshape", "./cmd/pkgmap", "./cmd/testbudget")
NARROW_HEAVY = ("./dirac/kgen", "./dirac/oracle/cmd/kfuzz")
# The landing gate's global and post-merge lists (TestHeads runs in test-smart's census step).
RULES_SKIP = ("TestHeads", "TestLargeEliminationSweepDoesNotTripLivelockWatcher",
              "TestCloneFidelityShort[0-9A-Za-z]*", "TestCostStaticPlannedCastsNeverCostChange[0-9]*",
              "TestChainTargetOfferCensusAgreesWithCastFlow[0-9]*",
              "TestPaymentPlanOnePassMatchesReferenceOverAutoPayGameKernel[0-9A-Za-z]*")
# Two halves of one game's decision probes, ~16 s each with the long game's costliest cache verifiers off
# (kernel_world_longgame_test.go; they were 36 s each before that), each contained in the other's coverage and
# never red (audit item 8): a run plays one, chosen by the commit sha's parity, so consecutive commits cover both.
KERNEL_PAIR = ("TestKernelWorldsInFuzzGame5Shard0", "TestKernelWorldsInFuzzGame5Shard1")
PINS_TESTS = "^TestPinManifest$|^TestPinManifestCoversOldPins$"
ARCH32 = "TestEngineCompilesFor32Bit"
FAST_TIER = "dirac/oracle/testdata/fasttier.txt"
# One -timeout per package, used by test-one, test-targeted, test-smart and the dirac gate's step 8.
# -timeout is part of go's test-cache key: a package run at 5m in one mode and 10m in another runs twice.
# ./dirac/oracle stays 5m: the protected dirac gate (read from BASE; its step 0 refuses an edit to it) runs the
# same fast tier and repro families at 5m, so that is the one value both callers can use.
PACKAGE_TIMEOUTS = {"./dirac/oracle": "5m"}
DEFAULT_TIMEOUT = "5m"


def timeout_for(pkg):
    """The -timeout every mode uses for pkg, so its test-cache entry is the same one."""
    return PACKAGE_TIMEOUTS.get(pkg, DEFAULT_TIMEOUT)


def needs_386(changes):
    """Whether a change can move the GOARCH=386 build (TestEngineCompilesFor32Bit): a non-test Go file or
    go.mod/go.sum. Test files, testdata, docs and scripts are not in `go build ./...`."""
    return any(p in ("go.mod", "go.sum") or (p.endswith(".go") and not p.endswith("_test.go")) for p in changes)


def kernel_shard(sha):
    """The KERNEL_PAIR index a commit plays: its sha's last hex digit's parity (0 when unreadable)."""
    try:
        return int(sha.strip()[-1], 16) % 2
    except (ValueError, IndexError):
        return 0


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
        self.pkgs = 0  # packages that ended `ok`
        self.cached = 0  # of those, how many go answered from its test cache
        self.reasons = []
        self.detail = []


def is_changed(changed, pkg, test):
    """Whether (pkg import path, top-level test) is a test the diff adds or changes; changed is {(dir, test)}
    as changed_tests returns it, with the repo root keyed by its module import path (diff_changed_tests does
    that: `go test -json` reports the import path, never "."). A package matches its directory by import-path
    suffix (the module's own prefix is not needed, and a directory is never a bare suffix of another: the
    match is on a '/')."""
    return any(t == test and (pkg == d or (d != "." and pkg.endswith("/" + d))) for d, t in changed or ())


def classify(lines, rc=0, named=(), changed=None, fallback=""):
    """Classify the lines of a `go test -json` run. rc is go's exit status. named are the packages run
    with a -run filter that must match something. changed is {(dir, test)} the diff adds or changes: one of
    them over TEST_BUDGET_S makes the verdict BUDGET, naming it (below the 4-core budget, fallback is the CPU
    cap, it is only reported: the box was short of cores, not the test slow). Returns a Result."""
    res = Result()
    res.slow = []  # report lines for tests over the wall budget
    elapsed = {}  # (package, top-level test) -> longest Elapsed seen
    out = {}  # (package, test) -> output lines
    failed = []  # (package, test) in order
    pkg_fail = []
    raw = []
    budget = over_budget = env = empty = False
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
            if act == "output" and not test and text.startswith("ok "):
                res.pkgs += 1
                res.cached += "(cached)" in text
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
                    # The test's own skip message says which part of the environment: a remote run's note is
                    # all the local log gets (the box answered ENV with no reason, 11-14).
                    why = " ".join(l.strip() for l in out.get((pkg, test), [])
                                   if l.strip() and not l.lstrip().startswith(("=== ", "--- ")))
                    res.reasons.append("skipped for the environment: %s %s%s" % (pkg, test, " (%s)" % why[:160] if why else ""))
            else:
                res.ran += 1
            if act == "fail":
                failed.append((pkg, test))
            if act in ("pass", "fail") and "/" not in test and isinstance(ev.get("Elapsed"), (int, float)):
                elapsed[(pkg, test)] = max(elapsed.get((pkg, test), 0.0), float(ev["Elapsed"]))
        elif act in ("fail", "build-fail") and not test:
            pkg_fail.append(pkg)
    over = sorted(((p, t, el) for (p, t), el in elapsed.items() if el > TEST_BUDGET_S), key=lambda x: (-x[2], x[0], x[1]))
    for pkg, test, el in over:
        if is_changed(changed, pkg, test):
            if fallback:
                res.slow.append("changed test over the %.0fs budget, not enforced at the fallback cap (CPUQuota %s): %s %s %.1fs"
                                % (TEST_BUDGET_S, fallback, pkg, test, el))
            else:
                over_budget = True
                res.slow.append("BUDGET: changed test over the %.0fs budget: %s %s %.1fs; split it into chunk tests, "
                                "shard it or tag it nightly" % (TEST_BUDGET_S, pkg, test, el))
        else:
            res.slow.append("slow test over the %.0fs budget (not changed by this diff, so not a failure; "
                            "docs/agents/known-slow.md): %s %s %.1fs" % (TEST_BUDGET_S, pkg, test, el))
    res.slow.sort(key=lambda l: not l.startswith("BUDGET"))  # the enforced ones first
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
        # A real failure wins over an environment skip and over a changed slow test: the code, not the box or
        # the wall clock, decided the run. A timeout or kill above still wins over both.
        res.status = "FAIL"
    elif over_budget:
        res.status = "BUDGET"
    elif env:
        res.status = "ENV"
    elif empty or res.ran == 0:
        res.status = "EMPTY"
        if not res.reasons:
            res.reasons.append("no test ran")
    return res


def finish(mode, jsonfile, rc, sha, pkgs, seconds, named, out=sys.stdout, fallback="", changed=None):
    """fallback is the CPU cap of a run that fell back from the gate box to below the budget's 4 cores ("" when
    not): a timeout there says the machine was short of cores, not that the change is slow, so it is BUSY."""
    with open(jsonfile, errors="replace") as f:
        res = classify(f, rc, named, changed, fallback)
    if res.status == "BUDGET" and fallback:
        res.status = "BUSY"
        res.reasons.insert(0, "timed out at the fallback cap (CPUQuota %s; the gate box could not take the run): the "
                              "time budget is set for 4 cores, so this is BUSY, not a BUDGET of the change" % fallback)
    detail = res.detail[:MAX_DETAIL]
    for l in detail:
        print(l, file=out)
    if len(res.detail) > MAX_DETAIL:
        print("... %d more lines in %s" % (len(res.detail) - MAX_DETAIL, jsonfile), file=out)
    for r in res.reasons[:5]:
        print("  note: " + r, file=out)
    for r in res.slow[:MAX_SLOW_LINES]:
        print("  note: " + r, file=out)
    if len(res.slow) > MAX_SLOW_LINES:
        print("  note: ... %d more tests over the budget in %s" % (len(res.slow) - MAX_SLOW_LINES, jsonfile), file=out)
    print("  tests ran=%d skipped=%d packages=%d cached=%d" % (res.ran, res.skipped, res.pkgs, res.cached), file=out)
    if res.cached:
        # The verdict line alone made a one-second PASS look like nothing ran, and agents re-ran it (test-cache-audit 11-14).
        print("  note: cached=%d: go already ran these exact tests on these inputs and they passed; "
              "a cached ok IS a pass, do not re-run it" % res.cached, file=out)
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


_PKGHOOK = re.compile(r"^func (TestMain|init)\(")


def named_tests(lines, ranges, new=False):
    """The Test functions a set of changed line ranges lies in. Returns (names, whole): whole is True when
    a range touches anything that is not a Test function (a helper, an import, a type), so the package
    must run whole. ranges None means a new file: every line changed. A new file (ranges None, or new
    True for one the diff adds) cannot change what an existing test does, so its package clause, imports
    and helpers force nothing: its own Test functions run, and only a TestMain or init forces whole."""
    segs = segments(lines)
    if new or ranges is None:
        return sorted({t for _, _, t in segs if t}), any(_PKGHOOK.match(ln) for ln in lines)
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


def changed_tests(changes, added, lines_of):
    """{(directory, TestName)} the diff adds or changes: the Test functions of each changed _test.go file that
    a changed line lies in (every Test of a file the diff adds). A changed helper, import or fixture names no
    test here: it does not change a test's own body. changes and added are as plan takes them."""
    out = set()
    for path, ranges in changes.items():
        if not path.endswith("_test.go"):
            continue
        d = path.rsplit("/", 1)[0] if "/" in path else "."
        names, _ = named_tests(lines_of(path), ranges, path in added)
        out.update((d, t) for t in names)
    return out


def pkg_of(path, dirmap):
    """The package import path a changed file belongs to, or None. testdata maps to its package."""
    d = path.rsplit("/", 1)[0] if "/" in path else "."
    if "/testdata/" in "/" + path:
        d = ("/" + path).split("/testdata/")[0].lstrip("/") or "."
    return dirmap.get(d)


def repro_named(changes, dirmap, lines_of, cwd=None):
    """Test functions of the _test.go files that name a changed repro recording, as {import path: names}.

    A re-recorded custom-pool recording usually leaves its reader's _test.go unchanged, so the recording's
    basename is the only link: one `git grep -l -F` per changed recording finds the test files that name it,
    and every test function of those files runs. Recordings no test file names (standard-pool replays) get
    nothing here; the REPRO_TESTS families cover them.

    A recording marked `"named":true` is replayed by TestReproManifest as the subtest of its own name, so it
    maps to `TestReproManifest/<name>`. plan keeps that path when the run slices (the manifest's -run matches
    the manifest test alone, so no other test's subtests are filtered) and folds it to the parent test in the
    full-coverage fallback, where the whole manifest runs anyway (see replay_part)."""
    out, files = {}, {}
    for path in sorted(changes):
        if not path.startswith(REPRO_DIR) or not path.endswith(".repro.json"):
            continue
        base = path.rsplit("/", 1)[1][:-len(".repro.json")]
        p = dirmap.get(path.split("/testdata/")[0])
        if p is None:
            continue
        if '"named":true' in "".join(lines_of(path)):
            out.setdefault(p, set()).add(MANIFEST_TEST + "/" + base)
        if base not in files:
            # git grep exits 1 on no match, which is an answer, not an error
            files[base] = sh("git", "grep", "-l", "-F", base, "--", path.split("/testdata/")[0] + "/*_test.go",
                             check=False, cwd=cwd).split()
        for f in files[base]:
            out.setdefault(p, set()).update(m.group(1) for m in (re.match(r"func (Test[A-Za-z0-9_]+)\(", ln) for ln in lines_of(f)) if m)
    return out


def plan(changes, dirmap, deps, lines_of, repro_names=None, added=(), narrow=None, reprosel=None):
    """What test-targeted runs.

    changes   {path: ranges}: the changed files; ranges is a list of (start, end) new-file line ranges,
              or None for a file that is new or whose changes are not line-addressable
    dirmap    {repo-relative dir: import path} of every package
    deps      {import path: set of import paths} for every package that has a test binary
    lines_of  path -> lines of the file's current text
    repro_names  {import path: test names} the changed repro recordings' readers (repro_named), optional
    added     paths the diff adds (status A or untracked), as opposed to modifies (added_files), optional
    narrow    None for test-targeted; for test-smart's narrow precheck {"fast": oracle fast-tier names,
              "sha": the commit}, which adds the kind wholeskip (regex is the -skip) -- see RULES above
    reprosel  the replay-slice inputs (replay_sel), None for full coverage: with them, a replay arm
              replays the changed recordings plus a 1/SLICE_MOD slice (slice_lane) and leaves the rest to the
              batch, and the manifest's subtests run on their own -run appended to the result

    Returns a list of (kind, package, regex): whole, named (regex must match), census (regex may match
    nothing). Package names are ./relative, as `go test` takes them. A package whose test helper, import or
    testdata fixture changed runs whole even when it is heavy: the tests it feeds are otherwise unprovable."""
    rel = {v: k for k, v in dirmap.items()}
    src, touched, named, forced_whole, repro = set(), set(), {}, set(), set()
    # The precheck's replay slice (TF-01): the manifest's subtests (msub), the pool globs' subtests
    # (gsl, {glob test: recording names}) and the sliced replay tests (tsl) a run with slice inputs
    # replays, and the packages a replay arm has already covered. All empty keeps the blanket
    # REPLAYS: reprosel None is the batch's shape, never silently less.
    sel = {"msub": set(), "tsl": set(), "gsl": {}, "replayed": set()}
    # every changed recording's basename: its JSON changed, so it replays whatever its replay home
    changed_recs = {p.rsplit("/", 1)[1][:-len(".repro.json")] for p in changes
                    if p.startswith(REPRO_DIR) and p.endswith(".repro.json")}
    if reprosel:
        slow = set(reprosel.get("slow") or ())
        named_recs = reprosel.get("named") or ()
        # a changed recording always replays; an unchanged one only in its lane, and never over the 10 s wall
        sel["msub"].update(r for r in changed_recs if r in named_recs)
        sel["msub"].update(n.split("/", 1)[1] for ns in (repro_names or {}).values()
                           for n in ns if "/" in n)  # repro_names' subtest paths, as repro_named mapped them
        sel["msub"].update(n for n in named_recs
                           if n not in changed_recs and n not in slow and slice_lane(n, reprosel.get("sha")))
        exclude = {t for ns in (repro_names or {}).values() for t in ns if "/" not in t}
        sel["tsl"] = {t for t in (reprosel.get("tests") or ())
                      if t not in exclude and t not in slow and slice_lane(t, reprosel.get("sha"))}
        pools = reprosel.get("pools") or {}
        rec2pool = {r: pool for pool, rs in pools.items() for r in rs}
        for g, ps in sorted((reprosel.get("globs") or {}).items()):
            if g in slow:
                continue  # over the 10 s wall: the batch replays it; a changed recording of its pools
                          # still joins another glob of the same pool (the aliases share theirs)
            ps = set(ps)
            lane = {r for pool in ps for r in pools.get(pool, ())
                    if r not in slow and slice_lane(r, reprosel.get("sha"))}
            lane.update(r for r in changed_recs if rec2pool.get(r) in ps)
            if lane:
                sel["gsl"][g] = lane
    for p, names in (repro_names or {}).items():
        if names:
            if reprosel is None:
                # full coverage: a subtest path (TestReproManifest/<name>) runs as every manifest
                # shard (MANIFEST_RX), as the one parent test ran before (a '/' in -run would filter the subtests of every other selected test, but
                # the manifest is the only test here whose subtests matter)
                named.setdefault(p, set()).update(
                    MANIFEST_RX if n.split("/", 1)[0] == MANIFEST_TEST else n.split("/", 1)[0] for n in names)
            else:
                # the slice's -run matches the manifest test alone, so the subtest keeps its path and
                # no other selected test's subtests are filtered to it (see replay_part)
                named.setdefault(p, set()).update(n for n in names if "/" not in n)
    for path, ranges in sorted(changes.items()):
        if path.endswith(".md"):
            continue
        if path.startswith(TABLE_DATA) and not path.endswith(".go"):
            for d in TABLE_READERS:
                if d in dirmap:
                    touched.add(dirmap[d])
                    forced_whole.add(dirmap[d])
            continue
        p = pkg_of(path, dirmap)
        if p is None:
            continue
        touched.add(p)
        if not path.endswith(".go"):
            if path.startswith(REPRO_DIR):
                repro.add(p)  # read only by the repro table and lockstep tests
            elif "/testdata/" in "/" + path:
                # a modified fixture can move any test of the package; a new one is read only by new tests,
                # which a heavy package runs by name (a light package runs whole anyway)
                if not (path in added and ("./" + rel[p] if rel[p] != "." else ".") in HEAVY):
                    forced_whole.add(p)
            else:
                src.add(p)
        elif not path.endswith("_test.go"):
            src.add(p)
        else:
            names, whole = named_tests(lines_of(path), ranges, path in added)
            named.setdefault(p, set()).update(names)
            if whole:
                forced_whole.add(p)

    def local(p):
        d = rel[p]
        return "./" + d if d != "." else "."

    def replay_part(deps=False):
        """The replay part a precheck arm appends to the oracle's regex. Without slice inputs, full
        coverage as before (REPRO_TESTS in the recording arm, REPLAYS alone in the reverse-dependency
        arm); with them, the refusal check and the 1/SLICE_MOD test slice — the pool globs are sliced like
        the rest (they are in reprosel's tests). The manifest's subtests never join this regex: a '/'
        in a -run filters the subtests of every other selected test to the pattern after it, which
        would pass the subtest-bearing tests that stay — the guard TestReproCompactRoundTrip, the
        census tests, the TestRefusedEntriesStillDiverge shards — empty, so they run on their own -run, appended
        to the result after the plain entries."""
        if reprosel is None:
            return REPRO_TESTS if not deps else REPLAYS
        parts = [REFUSED]
        if sel["tsl"]:
            parts.append("|".join(sorted(sel["tsl"])))
        return "|".join(parts)

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
            if names and p in repro:
                out[lp] = ("named", lp, "^(%s)$|%s|%s" % ("|".join(names), replay_part(), CENSUS))
                sel["replayed"].add(lp)
            elif names:
                out[lp] = ("named", lp, "^(%s)$|%s" % ("|".join(names), CENSUS))
            elif p in repro:
                out[lp] = ("named", lp, "%s|%s" % (replay_part(), CENSUS))
                sel["replayed"].add(lp)
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
            elif lp == ORACLE:
                # a source change the oracle imports can move any recording's replay (REPLAYS above);
                # TF-01: the changed recordings plus a 1/SLICE_MOD slice, not every recording (replay_part)
                if lp not in sel["replayed"]:
                    rx = out[lp][2] if lp in out else CENSUS
                    out[lp] = ("named", lp, rx + "|" + replay_part(deps=True))
                    sel["replayed"].add(lp)
            elif lp not in out:
                out[lp] = ("census", lp, CENSUS)
    # A source-reading census (SOURCE_READERS) parses another package's files with go/parser, so the
    # import graph followed by the two loops above cannot reach it: the changed source file itself
    # belongs to the parsed package, never to the reader. Select the reader when the diff changes a
    # non-test .go file it parses (its own tests skip _test.go), never as a blanket always-run.
    for reader, dirs in SOURCE_READERS:
        if not any(p.endswith(".go") and not p.endswith("_test.go") and p.startswith(d + "/")
                   for p in changes for d in dirs):
            continue
        ip = dirmap.get(reader)
        if ip is None or ip not in deps:
            continue  # no such package (or no test binary): nothing to select
        lp = local(ip)
        if lp not in out:
            out[lp] = ("whole", lp, "")
    for lp, rx in GUARDS:
        if lp not in out:
            out[lp] = ("named", lp, rx)
        elif out[lp][0] != "whole":
            out[lp] = (out[lp][0], lp, out[lp][2] + "|" + rx)
    for lp in ALWAYS:
        out[lp] = ("whole", lp, "")
    if narrow is not None:
        _narrow(out, changes, src, forced_whole, named, deps, local, narrow, sel, replay_part())
    res = [out[k] for k in sorted(out)]
    # TF-01: the replay slice runs on its own -run (replay_part), appended after the plain entries:
    # the manifest's named recordings and the pool globs' recordings replay as subtests there — the
    # subtest names are recording file names, unique in the directory, so a pool glob's subtests
    # matching the pattern are exactly its pools' slice — and no other selected test's subtests are
    # filtered to them
    if ORACLE in sel["replayed"]:
        top = sorted(sel["gsl"])
        if sel["msub"]:
            top = [MANIFEST_TEST] + top
        sub = sel["msub"] | (set().union(*sel["gsl"].values()) if sel["gsl"] else set())
        if top and sub:
            res.append(("named", ORACLE, "%s/(%s)" % ("|".join(top), "|".join(sorted(sub)))))
    return res, skipped


def _narrow(out, changes, src, forced_whole, named, deps, local, narrow, sel, repro_part):
    """test-smart's narrow precheck on top of plan's selection (see RULES above); edits out in place. src is
    the packages a changed non-test file is in, forced_whole those whose test helpers or fixtures changed,
    sel plan's replay-slice state (the manifest entry fires when the oracle runs a replay arm), repro_part
    the replay part of the oracle's regex (plan's replay_part)."""
    ip = {local(p): p for p in deps}
    reached = {local(p) for p, ds in deps.items() if ds & src}
    for lp in NARROW_HEAVY:
        if lp in out and out[lp][0] == "whole" and ip.get(lp) not in src | forced_whole:
            out[lp] = ("census", lp, CENSUS)
    for lp in NARROW_LINT:
        if lp in ip:
            out[lp] = ("whole", lp, "")
    if RULES in reached or out.get(RULES, ("",))[0] == "whole":
        other = KERNEL_PAIR[1 - kernel_shard(narrow.get("sha", ""))]
        out[RULES] = ("wholeskip", RULES, "^(%s)$" % "|".join(RULES_SKIP + (other,)))
    if ARCHTEST in out and not needs_386(changes):
        out[ARCHTEST] = ("wholeskip", ARCHTEST, "^%s$" % ARCH32)
    if ORACLE in out:
        kind, _, rx = out[ORACLE]
        fast = narrow.get("fast") or ()
        fx = "^(%s)$" % "|".join(fast) if fast else ""
        if kind == "whole":
            names = sorted(named.get(ip.get(ORACLE), ()))
            parts = (["^(%s)$" % "|".join(names)] if names else []) + [repro_part, PINS_TESTS, CENSUS]
            parts += [g for lp, g in GUARDS if lp == ORACLE] + ([fx] if fx else [])
            out[ORACLE] = ("named", ORACLE, "|".join(parts))
            if sel["msub"] or sel["tsl"] or sel["gsl"]:  # the sliced replay: plan appends its own -run
                sel["replayed"].add(ORACLE)
        elif ORACLE in reached and fx:
            out[ORACLE] = (kind, ORACLE, rx + "|" + fx)


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


def diff_changed_tests(base):
    """changed_tests for the working tree against base; empty when the diff cannot be read (a base that does
    not resolve): the budget is then only reported, never enforced. A root-package test is keyed by the
    module import path `go test -json` reports for it (the dir is "." in the diff)."""
    try:
        changed = changed_tests(diff_changes(base), added_files(base), read_lines)
        if any(d == "." for d, _ in changed):
            root = sh("go", "list", "-m").strip()
            changed = {(root if d == "." else d, t) for d, t in changed}
        return changed
    except SystemExit:
        return set()


def cmd_census(a):
    dirmap, deps = go_graph()
    cands = [ln.strip() for ln in sys.stdin if ln.strip()]
    for p in census_pkgs(diff_changes(a.base), dirmap, deps, cands):
        print(p)
    return 0


def sh(*args, check=True, cwd=None):
    r = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, errors="replace", cwd=cwd)
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


def added_files(base):
    """The paths the working tree adds against the merge base with base: status A, plus untracked files."""
    mb = sh("git", "merge-base", base, "HEAD").strip()
    out = set(sh("git", "diff", "--name-only", "--diff-filter=A", mb).splitlines())
    out.update(sh("git", "ls-files", "-o", "--exclude-standard").splitlines())
    return out


def read_lines(path):
    try:
        with open(path, errors="replace") as f:
            return f.read().splitlines()
    except OSError:
        return []


def fast_tier(path=FAST_TIER):
    """The oracle fast tier's test names (comments and blank lines dropped); empty when the list is absent."""
    return [ln.strip() for ln in read_lines(path) if ln.strip() and not ln.lstrip().startswith("#")]


def slow_repros(path=SLOW_REPROS):
    """The slow list's names (comments and blank lines dropped); empty when the list is absent. The
    names are recordings the manifest test replays and the oracle's other replay tests, one namespace
    each (recordings are snake_case, tests CamelCase), so one file holds both."""
    return [ln.strip() for ln in read_lines(path) if ln.strip() and not ln.lstrip().startswith("#")]


def slice_lane(name, sha):
    """Whether a name (a recording or a replay test) lands in a merge base's 1/SLICE_MOD replay slice: crc32
    % SLICE_MOD equal, so a PR's slice is stable for its base and successive PRs rotate through the names."""
    return zlib.crc32(name.encode()) % SLICE_MOD == zlib.crc32((sha or "").encode()) % SLICE_MOD


_GLOBPOOL = re.compile(r"stdReprosGlob(?:Pools)?\(t, ((?:\"[A-Za-z0-9-]+\"(?:,\s*)?)+)\)")
_POOLFIELD = re.compile(r'"pool":"([A-Za-z0-9-]+)"')


def replay_sel(dirmap, sha):
    """plan's reprosel: the recordings the manifest test replays, each pool's recordings and the pool
    glob tests that replay them (the pool's recordings run as that test's subtests, named by the
    recording's file name), the oracle's other replay tests, the slow list and the merge base (the
    slice lane, slice_lane). None when the oracle is not a package of this module, which leaves the
    replay arms at full coverage (REPLAYS)."""
    pkg = REPRO_DIR.split("/testdata/")[0]
    if pkg not in dirmap or not os.path.isdir(REPRO_DIR):
        return None
    named, pools = set(), {}
    for f in sorted(os.listdir(REPRO_DIR)):
        if not f.endswith(".repro.json"):
            continue
        text = "".join(read_lines(REPRO_DIR + f))
        base = f[:-len(".repro.json")]
        if '"named":true' in text:
            named.add(base)
        m = _POOLFIELD.search(text)
        if m:
            pools.setdefault(m.group(1), set()).add(base)
    globs = {}
    for f in sorted(os.listdir(pkg)):
        if not f.endswith("_test.go"):
            continue
        lines = read_lines(pkg + "/" + f)
        for s, e, t in segments(lines):
            if not t or not re.match(REPRO_GLOBS, t):
                continue
            m = _GLOBPOOL.search("\n".join(lines[s - 1:e]))
            if m:
                globs[t] = re.findall(r'"([A-Za-z0-9-]+)"', m.group(1))
    # the test-level slice population: every Test*Repro* name but the manifest itself, the two
    # sub-second table checks (GUARDS keep them whole), the refusal check, and the pool globs (they
    # are sliced by subtest above)
    whole = re.compile("^TestReproCompactRoundTrip$|^TestReproGlobMatchesTable$|" + REFUSED)
    tests = set()
    for f in sorted(os.listdir(pkg)):
        if not f.endswith("_test.go"):
            continue
        for ln in read_lines(pkg + "/" + f):
            m = _TESTFN.match(ln)
            if m:
                n = m.group(1)
                if re.match(REPLAYS, n) and not whole.match(n) and not n.startswith(MANIFEST_TEST) and n not in globs:
                    tests.add(n)
    return {"named": named, "pools": pools, "globs": globs, "tests": tests,
            "slow": set(slow_repros()), "sha": sha}


def oracle_test_names(pkg=ORACLE):
    """The package's top-level test names, as ci-oracle-shard.sh lists them; [] when it cannot be listed."""
    return [ln.strip() for ln in sh("go", "test", "-list", ".*", pkg, check=False).splitlines()
            if ln.strip().startswith("Test")]


def oracle_pools_present():
    """Whether both deck pools are on disk, the shell's test (ci-oracle-shard.sh): without them the
    pool-only tests skip and weigh nothing, so they must not unbalance a lane."""
    pools = os.environ.get("GORGE_POOLS") or ".pools"
    return all(os.path.isdir(os.path.join(pools, p, "decks")) for p in ("fdn", "blb"))


# The share of a pool glob's (and the manifest's) cost the replay slice plays: slice_lane keeps crc32 % SLICE_MOD of
# the recordings, so a sub-run top weighs this fraction of its table cost.
SLICE_SHARE = 1.0 / SLICE_MOD


def sub_run_tops(rx, names):
    """The listed test names a subtest -run's top-level part matches ('A|B/(subs)': go's unanchored match)."""
    tops = rx.split("/", 1)[0].split("|")
    return [n for n in names if any(re.search(t, n) for t in tops)]


def oracle_lanes(res, names=None, lanes=ORACLE_LANES, weights=None, zero=None, plain_lanes=ORACLE_PLAIN_LANES, sha=None,
                 slow=None, changed=()):
    """Deal the ./dirac/oracle replay slice into cost-balanced bins of "oraclelane" entries: `plain_lanes`
    for the plain tests and `lanes` for each subtest -run. The tests the plain entry names (the diff's changed
    tests and the readers of a changed recording, its ^(a|b)$ prefix) stay a "named" entry before the lanes:
    test-targets.sh runs it first, alone, at the run's GOMAXPROCS, so the 60 s changed-test budget judges a wall
    no lane stretched.

    The replay slice is the census and refusal families plus the 1/SLICE_MOD slice (slice_lane) in one -run, and
    the manifest/pool-glob subtest -run plan appends after the plain entries. In one go test process they
    outran the package's 5m cap on 34 of the last 40 targeted runs (11-27, ~200 top-level tests and a
    ~500 s summed test wall); test-targets.sh starts every oraclelane entry as its own concurrent process.
    Each half is dealt by measured cost (oracle_shard_deal.deal, the deal ci-oracle-shard.sh performs) into
    its own lane budget: the plain tests as one deal into `plain_lanes`, and each subtest -run's tops as a deal of
    their own. Measured alone on 11-27, the subtest half was 178 s of the slice's ~380 s and, left as one
    undealt process, was the lane that hit the cap. One deal over both halves balances each bin's sum, but
    every bin becomes two processes -- one -run for its plain tests (^(a|b)$) and one for its subtest tops
    (^(A|B)$/(subs): a '/' in -run filters every test the pattern matches, so the subtest tops never share a
    process with the plain ones) -- and the process walls, each set by its own half, spanned 2x on 11-27
    while the bins looked balanced. Dealing a half at a time makes the balanced cost the cost of each
    process: the run starts at most `lanes` plain processes and `lanes` per subtest -run. Every selected
    test still runs exactly once: the union of the bins is the names the entries matched, and no name the
    package does not list can run.

    names is the package's test list (oracle_test_names); None lists it lazily. weights is the
    "<test> <seconds>" table (dirac/oracle/testdata/ci-shard-seconds.txt); a test it does not list
    weighs oracle_shard_deal.UNLISTED_SECONDS, the table's own default (it lists only tests of >= 1 s;
    the old table median, ~14 s, made every sub-second test outweigh the real ones). A subtest top weighs
    SLICE_SHARE of its table cost. zero is the names that weigh nothing (the pool-only tests when no deck
    pools are on disk). sha is the merge base (replay_sel's): with it, the cmd5 and refusal shards outside its
    rotation lane leave the slice unless the entry names them (rotated_out; changed, the diff's paths, keeps
    every refusal shard when one is under REFUSED_ENTRIES), and so do the slow list's
    replay tests (slow; None reads SLOW_REPROS) that the CENSUS arm matches by name alone: the slice
    population already leaves them to the batch (replay_sel), but a recording replay whose name carries
    "Census" (TestMBCTail08CensusSearchLockstep, 82.2 s alone at 76.6 idle cores on 11-27) joined every
    run through CENSUS. None keeps them all. When the slice cannot be dealt (no test list) every entry is left as it was, so a
    broken listing never runs less. Every other package and entry is untouched."""
    picks = [i for i, (kind, pkg, rx) in enumerate(res)
             if pkg == ORACLE and kind == "named" and "/" not in rx and REFUSED in rx]
    if not picks:
        return res
    if names is None:
        names = oracle_test_names()
    if not names:
        return res
    rx = res[picks[0]][2]
    selected = [n for n in names if re.search(rx, n)]  # go's -run: an unanchored match on the top-level name
    named = set(rx[2:rx.index(")$")].split("|")) if rx.startswith("^(") and ")$" in rx else set()
    if sha is not None:
        lane = zlib.crc32(sha.encode()) % ROTATION
        slow = set(slow_repros()) if slow is None else set(slow)
        kept = (REFUSED_SHARD,) if any(p.startswith(REFUSED_ENTRIES) for p in changed) else ()
        selected = [n for n in selected if n in named or (not rotated_out(n, lane, kept) and n not in slow)]
    if not selected:
        return res
    alone = [n for n in selected if n in named]
    selected = [n for n in selected if n not in named]
    subs = [i for i, (kind, pkg, srx) in enumerate(res) if pkg == ORACLE and kind == "named" and "/" in srx]
    sub_tops = {i: sub_run_tops(res[i][2], names) for i in subs}
    if weights is None:
        weights = oracle_shard_deal.read_weights()
    if zero is None:
        zero = set() if oracle_pools_present() else oracle_shard_deal.read_zero()
    dflt = oracle_shard_deal.UNLISTED_SECONDS
    # one item per plain test and per (sub entry, top): the sub items are keyed so a test in both halves
    # (none today) is dealt once as each
    cost = {}
    for n in selected:
        cost[("p", n)] = 0.0 if n in zero else weights.get(n, dflt)
    for i, tops in sub_tops.items():
        for n in tops:
            cost[("s%d" % i, n)] = 0.0 if n in zero else weights.get(n, dflt) * SLICE_SHARE
    bins = {}
    # one deal per half, each into its own lane budget: the plain tests as one deal (`plain_lanes`) and each
    # subtest -run's tops as a deal of their own, so a bin sum is a process's cost (a subtest top
    # never shares a process with a plain test: a '/' in -run filters every other test's subtests).
    halves = [("p", selected)] + [("s%d" % i, tops) for i, tops in sorted(sub_tops.items())]
    for kind, members in halves:
        flat = {n: cost[(kind, n)] for n in members}
        bins_n = plain_lanes if kind == "p" else lanes
        for part, n, _ in oracle_shard_deal.deal(sorted(flat), flat, bins_n, default=dflt, zero=frozenset()):
            bins.setdefault(part, {}).setdefault(kind, []).append(n)
    out = []
    for i, entry in enumerate(res):
        if i == picks[0]:
            if alone:
                out.append(("named", ORACLE, "^(%s)$" % "|".join(sorted(alone))))
            out.extend(("oraclelane", ORACLE, "^(%s)$" % "|".join(sorted(bins[p]["p"])))
                       for p in sorted(bins) if "p" in bins[p])
        elif i in sub_tops and not sub_tops[i]:
            out.append(("oraclelane", ORACLE, entry[2]))  # its tops are not listed: left whole, never dropped
        elif i in sub_tops:
            sub = entry[2].split("/", 1)[1]
            out.extend(("oraclelane", ORACLE, "^(%s)$/%s" % ("|".join(sorted(bins[p]["s%d" % i])), sub))
                       for p in sorted(bins) if "s%d" % i in bins[p])
        else:
            out.append(entry)
    return out


def merge_lane_logs(out, lanes):
    """Append the lane processes' `go test -json` streams to out, in order, and return the worst exit
    status. `finish` then classifies the one stream a single run produced: every lane's lines are kept, so
    a failure, a build error or a timeout panic (BUDGET) in any lane wins over a passing one."""
    rc = 0
    for path, lane_rc in lanes:
        rc = max(rc, lane_rc)
        with open(path, errors="replace") as f:
            data = f.read()
        if data and not data.endswith("\n"):
            data += "\n"  # a stream's last line is still a line: never glue two lanes' lines into one
        with open(out, "a") as f:
            f.write(data)
    return rc


def cmd_merge(a):
    if len(a.lane) != len(a.rc):
        sys.exit("test_targets: merge: each --lane needs its --rc")
    try:
        print(merge_lane_logs(a.out, list(zip(a.lane, a.rc))))
    except OSError as e:
        sys.exit("test_targets: merge: %s" % e)
    return 0


def cmd_select(a):
    dirmap, deps = go_graph()
    changes = diff_changes(a.base)
    mb = sh("git", "merge-base", a.base, "HEAD").strip()
    narrow = None
    if a.narrow:
        narrow = {"fast": fast_tier(), "sha": sh("git", "rev-parse", "HEAD", check=False).strip()}
    reprosel = replay_sel(dirmap, mb)
    res, skipped = plan(changes, dirmap, deps, read_lines, repro_named(changes, dirmap, read_lines),
                       added_files(a.base), narrow, reprosel)
    # The targeted run deals the oracle's replay slice into concurrent lanes (test-smart keeps its own
    # fixed chunker, oracle_chunked); nothing else about the selection moves.
    if reprosel is not None and not a.narrow:
        res = oracle_lanes(res, sha=reprosel.get("sha"), changed=changes)
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
    f.add_argument("--base", default="", help="the diff's base: a test it adds or changes over the 60 s budget is BUDGET")
    f.add_argument("--fallback", default="", help="the CPU cap of a fallback run below 4 cores: a timeout is BUSY")
    s = sub.add_parser("select")
    s.add_argument("--base", default="origin/main")
    s.add_argument("--narrow", action="store_true", help="test-smart's narrow precheck selection")
    c = sub.add_parser("census")
    c.add_argument("--base", default="origin/main")
    m = sub.add_parser("merge")
    m.add_argument("--out", required=True, help="the run's one go test -json stream, appended to")
    m.add_argument("--lane", action="append", default=[], help="a lane process's -json log")
    m.add_argument("--rc", action="append", default=[], type=int, help="that lane's exit status")
    t = sub.add_parser("timeout")
    t.add_argument("--pkg", required=True)
    n = sub.add_parser("laneprocs")
    n.add_argument("--procs", required=True, type=int, help="the run's GOMAXPROCS")
    n.add_argument("--lanes", required=True, type=int, help="the oracle lanes that run side by side")
    o = sub.add_parser("lanemem")
    o.add_argument("--limit", required=True, help="the run's GOMEMLIMIT")
    o.add_argument("--lanes", required=True, type=int, help="the oracle lanes that run side by side")
    a = ap.parse_args(argv)
    pk = [p for p in a.pkgs.split(",") if p] if hasattr(a, "pkgs") else []
    if a.cmd == "verdict":
        if a.msg:
            print("test-targets: %s: %s" % (a.mode, a.msg), file=sys.stderr)
        print(verdict_line(a.mode, a.status, a.sha, pk, a.seconds))
        return EXIT[a.status]
    if a.cmd == "finish":
        return finish(a.mode, a.json, a.rc, a.sha, pk, a.seconds, [p for p in a.named.split(",") if p], fallback=a.fallback,
                      changed=diff_changed_tests(a.base) if a.base else None)
    if a.cmd == "census":
        return cmd_census(a)
    if a.cmd == "merge":
        return cmd_merge(a)
    if a.cmd == "timeout":
        print(timeout_for(a.pkg))
        return 0
    if a.cmd == "laneprocs":
        print(lane_procs(a.procs, a.lanes))
        return 0
    if a.cmd == "lanemem":
        print(lane_memlimit(a.limit, a.lanes))
        return 0
    return cmd_select(a)


if __name__ == "__main__":
    sys.exit(main())
