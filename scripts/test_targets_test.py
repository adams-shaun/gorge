"""Smoke test of scripts/test_targets.py (the verdict parser, exit codes and test selection) and of
scripts/test-targets.sh's preflight. Run: python3 -m pytest scripts/test_targets_test.py"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

import pytest

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import test_targets as tt  # noqa: E402

PY = [sys.executable, "-I", str(HERE / "test_targets.py")]


def ev(action, pkg="m/p", test="", out=""):
    d = {"Action": action, "Package": pkg}
    if test:
        d["Test"] = test
    if out:
        d["Output"] = out
    return json.dumps(d)


PASS = [ev("run", test="TestA"), ev("pass", test="TestA"), ev("pass")]


def run_cli(*args):
    r = subprocess.run(PY + list(args), capture_output=True, text=True)
    return r.returncode, r.stdout, r.stderr


@pytest.mark.parametrize("status,code", [("PASS", 0), ("FAIL", 1), ("ENV", 3), ("EMPTY", 4), ("BUDGET", 5), ("BUSY", 75)])
def test_verdict_line_and_exit_code(status, code):
    rc, out, _ = run_cli("verdict", "--mode", "one", "--status", status, "--sha", "abc1234", "--pkgs", "./p", "--seconds", "7.9")
    assert rc == code
    assert out == "test-one: %s abc1234 ./p 7\n" % status


def test_verdict_names_many_packages_briefly():
    assert tt.verdict_line("targeted", "PASS", "s", ["a", "b", "c", "d", "e"], 1) == "test-targeted: PASS s a,b,c,+2more 1"


def test_pass():
    r = tt.classify(PASS)
    assert (r.status, r.ran) == ("PASS", 1)


def test_cached_packages_are_counted_and_called_a_pass(tmp_path):
    cached = [ev("output", pkg="m/a", out="ok  \tm/a\t(cached)\n"), ev("pass", pkg="m/a"),
              ev("output", pkg="m/b", out="ok  \tm/b\t1.20s\n"), ev("pass", pkg="m/b"),
              ev("output", pkg="m/b", test="TestX", out="ok  \tnot a package line\t(cached)\n")]
    r = tt.classify(PASS + cached)
    assert (r.status, r.pkgs, r.cached) == ("PASS", 2, 1)
    f = tmp_path / "t.json"
    f.write_text("\n".join(PASS + cached))
    rc, out, _ = run_cli("finish", "--mode", "smart", "--json", str(f), "--sha", "s", "--pkgs", "./a,./b", "--seconds", "1")
    assert rc == 0 and "  tests ran=1 skipped=0 packages=2 cached=1" in out.splitlines()
    assert any(l.startswith("  note: cached=1:") and "IS a pass" in l for l in out.splitlines())
    assert out.rstrip().splitlines()[-1] == "test-smart: PASS s ./a,./b 1"
    f.write_text("\n".join(PASS))
    _, out, _ = run_cli("finish", "--mode", "one", "--json", str(f), "--sha", "s", "--pkgs", "./p", "--seconds", "1")
    assert "note: cached" not in out


def test_fail_prints_the_tests_own_output():
    r = tt.classify([ev("run", test="TestA"), ev("output", test="TestA", out="    a_test.go:9: boom\n"), ev("fail", test="TestA"), ev("fail")], rc=1)
    assert r.status == "FAIL"
    assert "    a_test.go:9: boom" in r.detail


def test_build_failure_is_fail():
    r = tt.classify([ev("output", out="FAIL\tm/p [build failed]\n"), ev("fail")], rc=1)
    assert r.status == "FAIL"


def test_skip_for_a_missing_corpus_is_env_not_pass():
    ev_skip = [ev("run", test="TestA"), ev("output", test="TestA", out="    a_test.go:5: no .cards corpus\n"), ev("skip", test="TestA"), ev("pass")]
    assert tt.classify(ev_skip).status == "ENV"
    # a skip that is about something else stays a skip
    other = [ev("run", test="TestA"), ev("output", test="TestA", out="    a_test.go:5: needs linux\n"), ev("skip", test="TestA"), ev("pass")]
    assert tt.classify(other + PASS).status == "PASS"


def test_a_failure_beats_an_environment_skip():
    # The environment skip is printed, but the code decided the run: FAIL, not ENV.
    lines = [ev("run", test="TestSkipped"), ev("output", test="TestSkipped", out="    x_test.go:1: corpus missing card \"Foo\"\n"),
             ev("skip", test="TestSkipped"), ev("run", test="TestBad"), ev("output", test="TestBad", out="    x_test.go:9: boom\n"),
             ev("fail", test="TestBad"), ev("fail")]
    r = tt.classify(lines, rc=1)
    assert r.status == "FAIL"
    assert any("TestBad" in l for l in r.detail)
    assert any("skipped for the environment" in reason for reason in r.reasons)


@pytest.mark.parametrize("msg,env", [
    ("gorge corpus: open .cards/cardsfolder: no such file", True),
    ("no gorge corpus: boom", True),
    ("no FDN decks in \"x\" (set GORGE_POOLS or KERNEL_FDN_DECKS)", True),
    ("no fdn table (make dirac-tables): x", True),
    ("synthetic pools: none", True),
    ("FDN pool absent: x", True),
    ("Grizzly Bears not in the FDN table", False),
    ("no Bird subtype in the table", False),
    ("every pool card is supported: no inert fallback to play", False),
    ("every tail bucket fits one table: nothing to split", False),
    ("table too small for the scenario library", False),
    ("corpus missing card \"Gitaxian Probe\"", True),
    ("corpus lacks Grizzly Bears, so the test cannot play it", True),
    ("role_cursed token script not in the corpus", True),
    ("corpus fixture: decks/std/foo.txt missing", True),
    ("no deck directory: NETSIMD_DECKS is unset", False),
])
def test_which_skips_are_the_environments(msg, env):
    assert bool(tt.ENV_SKIP.search(msg)) is env


def test_run_that_matched_nothing_is_empty():
    lines = [ev("output", out="ok  \tm/p\t0.002s [no tests to run]\n"), ev("pass")]
    assert tt.classify(lines, named=["m/p"]).status == "EMPTY"
    assert tt.classify(lines).status == "EMPTY"  # nothing ran at all
    assert tt.classify(lines + PASS).status == "PASS"  # a package run whole may have no tests of its own
    assert tt.classify(lines + PASS, named=["m/p"]).status == "EMPTY"


def test_timeout_and_kill_are_budget():
    assert tt.classify([ev("output", out="panic: test timed out after 2m0s\n"), ev("fail")], rc=1).status == "BUDGET"
    assert tt.classify(PASS, rc=137).status == "BUDGET"
    assert tt.classify([ev("output", out="signal: killed\n"), ev("fail")], rc=1).status == "BUDGET"


def test_finish_cli_exit_codes(tmp_path):
    f = tmp_path / "t.json"
    base = ["finish", "--mode", "one", "--json", str(f), "--sha", "s", "--pkgs", "./p", "--seconds", "1"]
    f.write_text("\n".join(PASS))
    rc, out, _ = run_cli(*base)
    assert rc == 0 and out.rstrip().splitlines()[-1] == "test-one: PASS s ./p 1"
    f.write_text("\n".join(PASS + [ev("fail", test="TestB"), ev("fail")]))
    rc, out, _ = run_cli(*base, "--rc", "1")
    assert rc == 1 and out.rstrip().splitlines()[-1].startswith("test-one: FAIL")
    assert sum(1 for l in out.splitlines() if l.startswith("test-")) == 1


# ---- selection -------------------------------------------------------------------------------------

DIRMAP = {"engine/rules": "m/engine/rules", "engine/game/gorgegame": "m/engine/game/gorgegame", "server/host": "m/server/host",
          "dirac/oracle": "m/dirac/oracle",
          "leaf": "m/leaf", "internal/archtest": "m/internal/archtest", "internal/codeshape": "m/internal/codeshape"}
DEPS = {
    "m/engine/rules": {"m/engine/rules"},
    "m/engine/game/gorgegame": {"m/engine/game/gorgegame", "m/engine/rules"},
    "m/server/host": {"m/server/host", "m/engine/rules"},
    "m/dirac/oracle": {"m/dirac/oracle"},
    "m/leaf": {"m/leaf"},
    "m/internal/archtest": {"m/internal/archtest"},
    "m/internal/codeshape": {"m/internal/codeshape"},
}
LEAF_TEST = ["package leaf", "", "import \"testing\"", "", "func helper() {}", "", "func TestOne(t *testing.T) {", "\ta()", "}", "",
             "// TestTwo does it.", "func TestTwo(t *testing.T) {", "\tb()", "}"]


def planned(changes, files=None, repro_names=None):
    res, skipped = tt.plan(changes, DIRMAP, DEPS, lambda p: (files or {}).get(p, []), repro_names)
    return {p: (k, rx) for k, p, rx in res}, skipped


def test_engine_rules_change_selects_gorgegame_and_not_the_slow_packages():
    got, skipped = planned({"engine/rules/engine.go": None})
    assert got["./engine/game/gorgegame"][0] == "whole"  # the 2026-11-11 gate miss
    assert got["./engine/rules"][0] == "census"  # too big to run whole in the inner loop
    assert "./server/host" not in got and skipped == ["./server/host"]
    assert "./internal/archtest" in got and "./internal/codeshape" in got
    assert "./leaf" not in got


def test_test_only_diff_runs_named_tests():
    got, _ = planned({"leaf/leaf_test.go": [(8, 8)]}, {"leaf/leaf_test.go": LEAF_TEST})
    assert got["./leaf"] == ("named", "^(TestOne)$")
    got, _ = planned({"leaf/leaf_test.go": [(8, 8), (11, 11)]}, {"leaf/leaf_test.go": LEAF_TEST})
    assert got["./leaf"] == ("named", "^(TestOne|TestTwo)$")  # a doc comment belongs to its function


def test_helper_or_fixture_change_runs_the_package_whole():
    assert planned({"leaf/leaf_test.go": [(5, 5)]}, {"leaf/leaf_test.go": LEAF_TEST})[0]["./leaf"][0] == "whole"
    assert planned({"leaf/testdata/x.txt": None})[0]["./leaf"][0] == "whole"
    assert planned({"leaf/leaf.go": None})[0]["./leaf"][0] == "whole"


def test_heavy_package_helper_or_fixture_change_runs_whole_not_census():
    # A helper or fixture feeds tests the census regex never names: running only CENSUS would print PASS
    # on an unrun test.
    assert planned({"engine/rules/rules_test.go": [(5, 5)]}, {"engine/rules/rules_test.go": LEAF_TEST})[0]["./engine/rules"][0] == "whole"
    assert planned({"engine/rules/testdata/foo.txt": None})[0]["./engine/rules"][0] == "whole"
    assert planned({"dirac/oracle/testdata/x.txt": None})[0]["./dirac/oracle"][0] == "whole"


def test_touched_excluded_package_is_excluded_not_census():
    got, skipped = planned({"server/host/host.go": None})
    assert "./server/host" not in got
    assert "./server/host" in skipped


def test_heavy_package_with_named_tests_adds_the_census_tests():
    kind, rx = planned({"engine/rules/x_test.go": [(7, 7)]}, {"engine/rules/x_test.go": LEAF_TEST})[0]["./engine/rules"]
    assert kind == "named" and rx.startswith("^(TestOne)$|") and "Census" in rx


def test_docs_only_diff_selects_just_the_structural_packages():
    got, _ = planned({"docs/agents/invariants.md": None, "AGENTS.md": None})
    assert sorted(got) == ["./dirac/kop", "./dirac/oracle", "./internal/archtest", "./internal/codeshape"]
    assert got["./internal/archtest"][0] == got["./internal/codeshape"][0] == "whole"


def test_docs_only_diff_still_selects_the_three_guard_tests_as_named():
    got, _ = planned({"docs/agents/invariants.md": None, "AGENTS.md": None})
    assert got["./dirac/kop"] == ("named", "^TestKopOrdinalsUnchanged$")
    kind, rx = got["./dirac/oracle"]
    assert kind == "named" and rx.split("|") == ["^TestReproCompactRoundTrip$", "^TestReproGlobMatchesTable$"]


def test_guards_join_a_heavy_packages_regex_and_yield_to_a_whole_run():
    kind, rx = planned({"dirac/oracle/x_test.go": [(7, 7)]}, {"dirac/oracle/x_test.go": LEAF_TEST})[0]["./dirac/oracle"]
    assert kind == "named" and rx.startswith("^(TestOne)$|") and rx.endswith("|^TestReproCompactRoundTrip$|^TestReproGlobMatchesTable$")
    assert planned({"dirac/oracle/testdata/x.txt": None})[0]["./dirac/oracle"] == ("whole", "")


CENSUS_CANDS = ["./engine/rules", "./engine/game/gorgegame", "./dirac/oracle", "./leaf", "./internal/archtest",
                "./internal/codeshape"]


def census(changes):
    return tt.census_pkgs(changes, dict(DIRMAP, **{".": "m"}), DEPS, CENSUS_CANDS)


def test_census_runs_the_importers_and_the_touched_packages_only():
    assert census({"engine/rules/engine.go": None}) == [
        "./engine/game/gorgegame", "./engine/rules", "./internal/archtest", "./internal/codeshape"]
    assert census({"dirac/oracle/x_test.go": None}) == ["./dirac/oracle", "./internal/archtest", "./internal/codeshape"]
    assert census({"dirac/oracle/testdata/repro/a.json": None}) == [
        "./dirac/oracle", "./internal/archtest", "./internal/codeshape"]


def test_census_maps_a_package_less_directory_to_the_nearest_package_above():
    assert census({"dirac/oracle/fixtures/deep/cards.txt": None}) == [
        "./dirac/oracle", "./internal/archtest", "./internal/codeshape"]


def test_census_runs_everything_for_a_root_level_or_unmapped_change():
    assert census({"Makefile": None}) == sorted(CENSUS_CANDS)
    assert census({"decks/x.dck": None}) == sorted(CENSUS_CANDS)
    assert census({"docs/agents/invariants.md": None}) == ["./internal/archtest", "./internal/codeshape"]


# ---- the narrow precheck (test-smart's default step 4) --------------------------------------------------

NDIRMAP = dict(DIRMAP, **{"dirac": "m/dirac", "dirac/kop": "m/dirac/kop", "cmd/codeshape": "m/cmd/codeshape"})
NDEPS = dict(DEPS, **{
    "m/dirac": {"m/dirac"},
    "m/dirac/kop": {"m/dirac/kop", "m/dirac"},
    "m/dirac/oracle": {"m/dirac/oracle", "m/dirac", "m/engine/rules"},
    "m/cmd/codeshape": {"m/cmd/codeshape"},
})
FAST = ["TestFastA", "TestFastB"]


def narrowed(changes, files=None, sha="0", added=()):
    res, _ = tt.plan(changes, NDIRMAP, NDEPS, lambda p: (files or {}).get(p, []), None, added,
                     {"fast": FAST, "sha": sha})
    return {p: (k, rx) for k, p, rx in res}


def test_narrow_runs_engine_rules_whole_when_a_source_change_reaches_it():
    got = narrowed({"engine/rules/engine.go": None}, sha="abc4")
    kind, rx = got["./engine/rules"]
    assert kind == "wholeskip"
    assert "TestLargeEliminationSweepDoesNotTripLivelockWatcher" in rx and "TestHeads" in rx
    # The plain selection runs only its census.
    res, _ = tt.plan({"engine/rules/engine.go": None}, NDIRMAP, NDEPS, lambda p: [])
    assert dict((p, k) for k, p, _ in res)["./engine/rules"] == "census"


def test_narrow_plays_one_kernel_half_by_the_sha_parity():
    even = narrowed({"engine/rules/engine.go": None}, sha="abc4")["./engine/rules"][1]
    odd = narrowed({"engine/rules/engine.go": None}, sha="abc5")["./engine/rules"][1]
    assert "TestKernelWorldsInFuzzGame5Shard1" in even and "TestKernelWorldsInFuzzGame5Shard0" not in even
    assert "TestKernelWorldsInFuzzGame5Shard0" in odd and "TestKernelWorldsInFuzzGame5Shard1" not in odd
    assert tt.kernel_shard("") == 0 and tt.kernel_shard("f") == 1


def test_narrow_dirac_only_diff_does_not_run_engine_rules():
    got = narrowed({"dirac/state.go": None})
    assert "./engine/rules" not in got
    assert got["./dirac"] == ("whole", "") and got["./dirac/kop"] == ("whole", "")


def test_narrow_engine_rules_test_only_diff_stays_named():
    got = narrowed({"engine/rules/x_test.go": [(7, 7)]}, {"engine/rules/x_test.go": LEAF_TEST})
    assert got["./engine/rules"][0] == "named"


def test_narrow_never_runs_the_oracle_whole():
    kind, rx = narrowed({"dirac/oracle/testdata/x.txt": None})["./dirac/oracle"]
    assert kind == "named"
    for part in ("^TestPinManifest$", "^(TestFastA|TestFastB)$", "ReprosLockstep", "Census", "^TestReproGlobMatchesTable$"):
        assert part in rx
    # A source change it imports adds every recording replay (cause 64) and the fast tier; the plain selection
    # adds the replays but not the fast tier.
    kind, rx = narrowed({"dirac/state.go": None})["./dirac/oracle"]
    assert kind == "named" and tt.REPLAYS in rx and rx.endswith("|^(TestFastA|TestFastB)$")
    res, _ = tt.plan({"dirac/state.go": None}, NDIRMAP, NDEPS, lambda p: [])
    kind, rx = dict((p, (k, rx)) for k, p, rx in res)["./dirac/oracle"]
    assert kind == "named" and tt.REPLAYS in rx and "TestFastA" not in rx


def test_a_source_change_reaching_the_oracle_replays_every_recording():
    # 11-18 cause 64: a dirac source change ran only the oracle's census in `fast`; the per-card MBC replays it
    # broke first ran in the Mergify batch. The selection's regex must match them and the staleness check.
    res, _ = tt.plan({"dirac/state.go": None}, NDIRMAP, NDEPS, lambda p: [])
    rx = dict((p, rx) for _, p, rx in res)["./dirac/oracle"]
    for name in ("TestMBCDominateReprosLockstep", "TestMBCSpellBurstReprosLockstep", "TestStdSetReprosLockstep",
                 "TestRefusedEntriesStillDiverge", "TestManaRiderDepletionReprosReach", "TestStaggerReproReaches",
                 "TestReproGreenSunsZenithReach"):
        assert re.search(rx, name), name
    assert not re.search(rx, "TestShieldReplacementCollision")  # whole-game lockstep stays in the batch
    # an engine change the oracle imports replays them too
    res, _ = tt.plan({"engine/rules/engine.go": None}, NDIRMAP, NDEPS, lambda p: [])
    assert re.search(dict((p, rx) for _, p, rx in res)["./dirac/oracle"], "TestMBCDominateReprosLockstep")


def test_narrow_runs_the_lint_packages_and_skips_the_386_build_unless_go_source_moved():
    got = narrowed({"docs/agents/invariants.md": None})
    assert got["./cmd/codeshape"] == ("whole", "")
    assert "./cmd/pkgmap" not in got  # not a package of this module
    assert got["./internal/archtest"] == ("wholeskip", "^TestEngineCompilesFor32Bit$")
    assert narrowed({"leaf/leaf_test.go": [(8, 8)]}, {"leaf/leaf_test.go": LEAF_TEST})["./internal/archtest"][0] == "wholeskip"
    assert narrowed({"leaf/leaf.go": None})["./internal/archtest"] == ("whole", "")
    assert narrowed({"go.mod": None})["./internal/archtest"] == ("whole", "")


def test_narrow_heavy_reverse_deps_run_their_census_unless_the_change_is_in_them():
    dm = dict(NDIRMAP, **{"dirac/kgen": "m/dirac/kgen"})
    dp = dict(NDEPS, **{"m/dirac/kgen": {"m/dirac/kgen", "m/dirac"}})
    def got(changes):
        res, _ = tt.plan(changes, dm, dp, lambda p: [], None, (), {"fast": [], "sha": "0"})
        return {p: k for k, p, _ in res}
    assert got({"dirac/state.go": None})["./dirac/kgen"] == "census"
    assert got({"dirac/kgen/gen.go": None})["./dirac/kgen"] == "whole"
    assert got({"dirac/kgen/testdata/x.txt": None})["./dirac/kgen"] == "whole"
    res, _ = tt.plan({"dirac/state.go": None}, dm, dp, lambda p: [])
    assert dict((p, k) for k, p, _ in res)["./dirac/kgen"] == "whole"  # test-targeted unchanged


def test_needs_386():
    assert tt.needs_386({"a/b.go": None}) and tt.needs_386({"go.sum": None})
    assert not tt.needs_386({"a/b_test.go": None, "a/testdata/x.go.txt": None, "scripts/x.sh": None, "x.md": None})


def test_go_graph_reads_the_real_test_binaries():
    # The real `go list -e -test` path (not the hand-built DEPS): `.test` entries are the reverse-dep graph
    # that made engine/rules/engine.go select gorgegame on 2026-11-11.
    dirmap, deps = tt.go_graph()
    rules = dirmap.get("engine/rules", "")
    assert rules.endswith("/engine/rules")
    assert rules in deps and rules in deps[rules]
    assert dirmap.get("engine/game/gorgegame", "").endswith("/engine/game/gorgegame")
    assert any(rules in ds for ds in deps.values())


def test_diff_changes_reads_test_line_ranges(tmp_path, monkeypatch):
    f = tmp_path / "x_test.go"
    run = lambda *a: subprocess.run(a, cwd=tmp_path, check=True, capture_output=True)
    run("git", "init", "-q", "-b", "land")
    f.write_text("package x\n\nfunc TestOne(t *testing.T) {\n\ta()\n}\n")
    run("git", "add", "x_test.go")
    run("git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "x")
    f.write_text("package x\n\nfunc TestOne(t *testing.T) {\n\tb()\n}\n")
    monkeypatch.chdir(tmp_path)
    changes = tt.diff_changes("HEAD")
    assert changes["x_test.go"] == [(4, 4)]


# ---- the wrapper's preflight ----------------------------------------------------------------------

@pytest.fixture
def repo():
    d = tempfile.mkdtemp()
    sh = lambda *a: subprocess.run(a, cwd=d, check=True, capture_output=True)
    sh("git", "init", "-q", "-b", "land")
    sh("git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x")
    (Path(d) / "scripts").mkdir()
    for n in ("test-targets.sh", "test_targets.py"):
        shutil.copy(HERE / n, Path(d) / "scripts" / n)
    # The remote runner reads the operator's box settings: a stub keeps every run here.
    (Path(d) / "scripts" / "test-remote.sh").write_text("remote_try() { :; }\n")
    (Path(d) / "go.mod").write_text("module m\n\ngo 1.22\n")
    yield Path(d)
    shutil.rmtree(d)


def wrapper_output(repo, *args, env=None):
    e = dict(os.environ, TEST_TARGETS_SCOPED="1", BASE="land")
    e.update(env or {})
    e.pop("GORGE_CARDS", None)
    return subprocess.run(["bash", "scripts/test-targets.sh", *args], cwd=repo, env=e, capture_output=True, text=True)


def wrapper(repo, *args, env=None):
    r = wrapper_output(repo, *args, env=env)
    lines = [l for l in r.stdout.splitlines() if l.startswith("test-")]
    return r.returncode, lines, r.stderr


def test_missing_cards_link_is_env_exit_3(repo):
    for mode in ("targeted", "one", "smart"):
        rc, lines, err = wrapper(repo, mode, env={"PKG": "./x", "RUN": "TestA"})
        assert rc == 3, (mode, err)
        assert len(lines) == 1 and lines[0].startswith("test-%s: ENV " % mode)
        assert "no corpus" in err


def test_empty_cards_folder_is_env(repo):
    (repo / ".cards" / "cardsfolder").mkdir(parents=True)
    rc, lines, _ = wrapper(repo, "targeted")
    assert rc == 3 and lines[0].startswith("test-targeted: ENV ")


STUB_GO = """#!/bin/sh
echo "$@" >>"$GO_CALLS"
case "$1" in
list) echo m ;;
env) echo /usr/local/go ;;
test) echo '{"Action":"run","Package":"m/p","Test":"TestA"}'; echo '{"Action":"pass","Package":"m/p","Test":"TestA"}'
      echo '{"Action":"pass","Package":"m/p"}' ;;
esac
"""


def test_timeout_table_is_one_per_package():
    # One -timeout per package, whatever mode runs it: a package in two modes with two timeouts has two cache entries.
    assert tt.timeout_for("./dirac/oracle") == "5m"
    for pkg in (".", "./engine/rules", "./dirac/kgen", "./server/gorged", "./internal/archtest"):
        assert tt.timeout_for(pkg) == "5m", pkg


def test_dirac_gate_step8_timeouts_are_the_table():
    # The gate is protected (its step 0 refuses an edit to it) and read from BASE, so it cannot import the table at
    # run time. Pin its step-8 literals to the table instead: a table change that forgets the gate fails here.
    gate = (HERE / "dirac-gate.sh").read_text()
    assert set(re.findall(r"(?:-timeout|test\.timeout) ([0-9]+[sm])", gate)) == {
        tt.timeout_for("./dirac/oracle")}


def test_every_mode_prints_the_cached_count(tmp_path):
    # The verdict hides whether go answered from its cache unless every mode prints the count. BUDGET/ENV and the
    # other early exits have no go stream to count, so this is the finish path each of one/targeted/smart uses.
    f = tmp_path / "t.json"
    f.write_text("\n".join(PASS + [ev("output", pkg="m/a", out="ok  \tm/a\t(cached)\n"), ev("pass", pkg="m/a")]))
    for mode in ("one", "targeted", "smart"):
        rc, out, _ = run_cli("finish", "--mode", mode, "--json", str(f), "--sha", "s", "--pkgs", "./p", "--seconds", "1")
        assert rc == 0, mode
        assert "  tests ran=1 skipped=0 packages=1 cached=1" in out.splitlines(), (mode, out)
        assert out.rstrip().splitlines()[-1] == "test-%s: PASS s ./p 1" % mode


def test_one_uses_the_package_table_timeout_and_the_same_flags(repo, tmp_path):
    # -timeout, -json and -v are part of go's test-cache key: test-one must use the same package timeout and flags
    # as targeted and smart, or each re-runs what test-one just ran.
    (repo / ".cards" / "cardsfolder").mkdir(parents=True)
    (repo / ".cards" / "cardsfolder" / "a.txt").write_text("x")
    (tmp_path / "bin").mkdir()
    (tmp_path / "bin" / "go").write_text(STUB_GO)
    (tmp_path / "bin" / "go").chmod(0o755)
    (tmp_path / "bin" / "make").write_text("#!/bin/sh\nexit 0\n")
    (tmp_path / "bin" / "make").chmod(0o755)
    calls = tmp_path / "go-calls"
    base = {"PATH": "%s:%s" % (tmp_path / "bin", os.environ["PATH"]), "GO_CALLS": str(calls), "RUN": "^TestA$",
            "GORGE_TEST_TARGETS_ENV": str(tmp_path / "none.env")}
    rc, lines, err = wrapper(repo, "one", env=dict(base, PKG="./p"))
    assert rc == 0 and lines[0].startswith("test-one: PASS "), err
    one = [l for l in calls.read_text().splitlines() if l.startswith("test ")][0]
    assert one.split("-timeout ")[1].split()[0] == tt.timeout_for("./p"), one
    for flag in ("-json", "-v", "-failfast"):
        assert flag in one.split(), (flag, one)
    calls.write_text("")
    rc, lines, err = wrapper(repo, "one", env=dict(base, PKG="./dirac/oracle"))
    assert rc == 0 and lines[0].startswith("test-one: PASS "), err
    one = [l for l in calls.read_text().splitlines() if l.startswith("test ")][0]
    assert one.split("-timeout ")[1].split()[0] == tt.timeout_for("./dirac/oracle") == "5m", one
    # targeted, smart's named/census/whole steps and the gate read the table, never a literal of their own.
    sh = (HERE / "test-targets.sh").read_text()
    assert 'gojson "$(timeout_for "${r%%$\'\\t\'*}")" -run' in sh
    assert not re.search(r'gojson \d+[sm] ', sh)


def test_one_usage_errors_exit_2(repo):
    (repo / ".cards" / "cardsfolder").mkdir(parents=True)
    (repo / ".cards" / "cardsfolder" / "a.txt").write_text("x")
    for env in ({"PKG": "./...", "RUN": "T"}, {"PKG": "./p"}, {"PKG": "./p", "RUN": "T", "TESTENV": "PATH=/x"},
                {"PKG": "./p", "RUN": "T", "TIMEOUT": "0s"},
                {"PKG": "./p", "RUN": "T", "TIMEOUT": "11m"}, {"PKG": "./p", "RUN": "T", "TIMEOUT": "0m"},
                {"PKG": "./engine/rules", "RUN": "^Test"}, {"PKG": "./engine/rules", "RUN": "^Test.*$"},
                {"PKG": "./dirac/oracle", "RUN": "."}):
        rc, lines, err = wrapper(repo, "one", env=env)
        assert rc == 2 and not lines, env


def test_one_runs_go_and_maps_the_stream(repo, tmp_path):
    (repo / ".cards" / "cardsfolder").mkdir(parents=True)
    (repo / ".cards" / "cardsfolder" / "a.txt").write_text("x")
    fake = tmp_path / "bin"
    fake.mkdir()
    stream = tmp_path / "stream.json"
    go = fake / "go"
    go.write_text('#!/bin/sh\ncase "$1" in list) echo m;; env) echo /usr/local/go;; test) cat "$STREAM"; exit "${GORC:-0}";; esac\n')
    go.chmod(0o755)
    env = {"PATH": "%s:%s" % (fake, os.environ["PATH"]), "STREAM": str(stream), "PKG": "./p", "RUN": "TestA"}
    stream.write_text("\n".join([ev("run", "m/p", "TestA"), ev("pass", "m/p", "TestA"), ev("pass", "m/p")]))
    rc, lines, _ = wrapper(repo, "one", env=env)
    assert rc == 0 and len(lines) == 1 and lines[0].startswith("test-one: PASS ") and "./p" in lines[0]
    stream.write_text(ev("output", "m/p", out="ok  \tm/p\t0.002s [no tests to run]\n") + "\n" + ev("pass", "m/p"))
    rc, lines, _ = wrapper(repo, "one", env=env)
    assert rc == 4 and lines[0].startswith("test-one: EMPTY ")
    stream.write_text(ev("fail", "m/p", "TestA") + "\n" + ev("fail", "m/p"))
    rc, lines, _ = wrapper(repo, "one", env=dict(env, GORC="1"))
    assert rc == 1 and lines[0].startswith("test-one: FAIL ")


# ---- test-smart: the lander runs the gates, a seat does not -----------------------------------------

def _recording_gate(path, label):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text('#!/bin/sh\necho %s >> "$GATE_CALLS"\n' % label)


def _commit(repo, *paths):
    subprocess.run(["git", "add", *paths], cwd=repo, check=True)
    subprocess.run(["git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "fixture"], cwd=repo, check=True)


def smart_fixture(repo, tmp_path):
    """Everything a smart run needs to reach steps 4-5 without the box: a corpus, fake go/make, and a
    recorder for each gate. BASE is a branch at the fixture commit, so a later commit is the diff."""
    (repo / ".cards" / "cardsfolder").mkdir(parents=True)
    (repo / ".cards" / "cardsfolder" / "a.txt").write_text("x")
    (repo / "scripts" / "docs_sets_test.py").write_text(
        "import unittest\n\n\nclass T(unittest.TestCase):\n    def test_ok(self):\n        pass\n")
    fake = tmp_path / "bin"
    fake.mkdir()
    stream = tmp_path / "stream.json"
    stream.write_text("\n".join([ev("run", "m/p", "TestA"), ev("pass", "m/p", "TestA"), ev("pass", "m/p")]))
    go = fake / "go"
    go.write_text("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GO_CALLS\"\n"
                  "case \"$1\" in list) echo m;; env) echo /usr/local/go;; test) cat \"$STREAM\"; exit \"${GORC:-0}\";; esac\n")
    go.chmod(0o755)
    make = fake / "make"
    make.write_text("#!/bin/sh\nexit 0\n")
    make.chmod(0o755)
    (repo / "scripts" / "fresh-clone-check.sh").write_text("echo 'fresh-clone-check: ok'\n")
    # The dirac gate is BASE's own script, so it has to be a committed file. The rest is committed too, so
    # the diff against BASE is only what a test commits on top.
    _recording_gate(repo / "scripts" / "dirac-gate.sh", "dirac")
    _commit(repo, "scripts", "go.mod")
    subprocess.run(["git", "branch", "base"], cwd=repo, check=True)
    # The affected-package gate is fetched from another repository at GATE_REF.
    affected = tmp_path / "affected"
    affected.mkdir()
    subprocess.run(["git", "init", "-q", "-b", "main"], cwd=affected, check=True)
    _recording_gate(affected / "ops" / "scripts" / "gate_affected.sh", "affected")
    _commit(affected, "ops/scripts/gate_affected.sh")
    gate_calls = tmp_path / "gate-calls"
    go_calls = tmp_path / "go-calls"
    env = {
        "PATH": "%s:%s" % (fake, os.environ["PATH"]),
        "STREAM": str(stream),
        "BASE": "base",
        "GATE_AFFECTED_REPO": str(affected),
        "GATE_CALLS": str(gate_calls),
        "GO_CALLS": str(go_calls),
        "GORGE_TEST_TARGETS_ENV": str(tmp_path / "none.env"),
        "GORGE_REMOTE_OFF": "1",
        "GORGE_HEAVY_LOCK": "",
        "GORGE_TEST_SMART_GATES": "",
        # The operator's env file is not read, but the box's harness exports some of these.
        "PRECHECK_DIRAC_GATE": "",
        "GATE_AFFECTED_PATH": "",
        "GATE_REF": "",
    }
    return env, gate_calls, go_calls


def _verdict_line(r):
    lines = [l for l in r.stdout.splitlines() if l.startswith("test-")]
    assert len(lines) == 1, r.stdout
    return lines[0]


def test_smart_default_runs_the_narrow_selection_and_launches_no_gate(repo, tmp_path):
    env, gate_calls, go_calls = smart_fixture(repo, tmp_path)
    r = wrapper_output(repo, "smart", env=env)
    assert r.returncode == 0, r.stderr + r.stdout
    assert "== 4 affected packages and the lint packages" in r.stdout
    line = _verdict_line(r)
    assert line.startswith("test-smart: PASS ") and "affected=narrow,dirac-gate=skipped" in line
    assert not gate_calls.exists() or gate_calls.read_text() == ""
    calls = go_calls.read_text()
    # the census step is widened by the groups, and archtest runs whole bar the 386 build (no Go source moved)
    assert "|StateLayoutPinned|" in calls
    assert "-skip ^TestEngineCompilesFor32Bit$ ./internal/archtest" in calls
    assert "-run ^TestKopOrdinalsUnchanged$ ./dirac/kop" in calls
    # a Go source change brings the 386 build back
    (repo / "x.go").write_text("package m\n")
    _commit(repo, "x.go")
    r = wrapper_output(repo, "smart", env=env)
    assert r.returncode == 0, r.stderr + r.stdout
    assert "-p 4 ./internal/archtest ./internal/codeshape" in go_calls.read_text().splitlines()[-2]


def test_smart_gates_off_prints_why_step_4_is_skipped_and_launches_no_gate(repo, tmp_path):
    env, gate_calls, go_calls = smart_fixture(repo, tmp_path)
    r = wrapper_output(repo, "smart", env=dict(env, GORGE_TEST_SMART_GATES="0"))
    assert r.returncode == 0, r.stderr + r.stdout
    assert "== 4 skipped (GORGE_TEST_SMART_GATES=0" in r.stdout
    assert _verdict_line(r).startswith("test-smart: PASS ")
    assert not gate_calls.exists() or gate_calls.read_text() == ""
    assert "StateLayoutPinned" not in go_calls.read_text()


def test_smart_gates_opt_in_launches_both_gates(repo, tmp_path):
    env, gate_calls, _ = smart_fixture(repo, tmp_path)
    r = wrapper_output(repo, "smart", env=dict(env, GORGE_TEST_SMART_GATES="1"))
    assert r.returncode == 0, r.stderr + r.stdout
    assert "== 4 affected gate and dirac gate, side by side" in r.stdout
    assert _verdict_line(r).startswith("test-smart: PASS ")
    assert sorted(gate_calls.read_text().split()) == ["affected", "dirac"]


def test_smart_gates_run_the_386_build_on_the_affected_side_only_when_go_source_moved(repo, tmp_path):
    env, _, go_calls = smart_fixture(repo, tmp_path)
    env = dict(env, GORGE_TEST_SMART_GATES="1")
    want = "-run ^TestEngineCompilesFor32Bit$ ./internal/archtest"
    (repo / "x_test.go").write_text("package m\n")
    _commit(repo, "x_test.go")
    r = wrapper_output(repo, "smart", env=env)
    assert r.returncode == 0, r.stderr + r.stdout
    assert want not in go_calls.read_text()
    (repo / "x.go").write_text("package m\n")
    _commit(repo, "x.go")
    r = wrapper_output(repo, "smart", env=env)
    assert r.returncode == 0, r.stderr + r.stdout
    assert want in go_calls.read_text()


def test_smart_off_runs_the_repro_tables_only_when_the_diff_touches_them(repo, tmp_path):
    env, _, go_calls = smart_fixture(repo, tmp_path)
    env = dict(env, GORGE_TEST_SMART_GATES="0")
    r = wrapper_output(repo, "smart", env=env)
    assert r.returncode == 0, r.stderr + r.stdout
    assert "-run ^TestStd[A-Za-z]*ReprosLockstep" not in go_calls.read_text()
    repro = repo / "dirac" / "oracle" / "testdata" / "repro" / "one.txt"
    repro.parent.mkdir(parents=True)
    repro.write_text("x")
    _commit(repo, "dirac/oracle/testdata/repro/one.txt")
    r = wrapper_output(repo, "smart", env=env)
    assert r.returncode == 0, r.stderr + r.stdout
    assert "-run ^TestStd[A-Za-z]*ReprosLockstep" in go_calls.read_text()
    assert _verdict_line(r).startswith("test-smart: PASS ")


# ---- test-smart: the recorded verdict is reused, the lane wait is clamped ----------------------------

@pytest.fixture
def smart_repo(tmp_path):
    """A committed repo whose smart run reaches a verdict without running any gate: step 2 fails because
    scripts/docs_sets_test.py does not exist here, so the run is a FAIL before any go test or gate runs."""
    d = tmp_path / "repo"
    d.mkdir()
    sh = lambda *a: subprocess.run(a, cwd=d, check=True, capture_output=True)
    sh("git", "init", "-q", "-b", "land")
    (d / "scripts").mkdir()
    for n in ("test-targets.sh", "test_targets.py"):
        shutil.copy(HERE / n, d / "scripts" / n)
    (d / "scripts" / "dirac-gate.sh").write_text("#!/bin/sh\nexit 0\n")  # read, never run, on this path
    (d / "go.mod").write_text("module m\n\ngo 1.22\n")
    (d / ".cards" / "cardsfolder").mkdir(parents=True)
    (d / ".cards" / "cardsfolder" / "a.txt").write_text("card\n")
    # The repo the affected-gate blob is read from (GATE_REF=main, GATE_AFFECTED_PATH, the defaults).
    g = tmp_path / "gate"
    g.mkdir()
    gg = lambda *a: subprocess.run(a, cwd=g, check=True, capture_output=True)
    gg("git", "init", "-q", "-b", "main")
    (g / "ops" / "scripts").mkdir(parents=True)
    (g / "ops" / "scripts" / "gate_affected.sh").write_text("#!/bin/sh\nexit 0\n")
    gg("git", "add", "ops")
    gg("git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "gate")
    sh("git", "add", "scripts", "go.mod")
    sh("git", "add", "-f", ".cards")  # the fixture's gitignore (like the real repo's) excludes .cards
    sh("git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "x")
    return d, g


def smart_env(tmp_path, repo, gate, **extra):
    fake = tmp_path / "smartbin"
    fake.mkdir(exist_ok=True)
    go = fake / "go"
    # Every invocation is logged, so a test can see whether the gates' go test ran.
    go.write_text('#!/bin/sh\necho "$1" >>"$CALLS"\n'
                  'case "$1" in list) echo m;; env) echo /usr/local/go;; test) cat "$STREAM"; exit "${GORC:-0}";; esac\n')
    go.chmod(0o755)
    env = dict(os.environ,  # wrapper() adds TEST_TARGETS_SCOPED and BASE
               GATE_AFFECTED_REPO=str(gate), PATH="%s:%s" % (fake, os.environ["PATH"]),
               CALLS=str(tmp_path / "go-calls"), STREAM=str(tmp_path / "stream.json"),
               GORGE_TEST_TARGETS_ENV=str(tmp_path / "no-machine.env"),
               GORGE_TEST_SMART_GATES="1", **extra)
    env.pop("GORGE_CARDS", None)
    return env


def smart_verdict_file(repo):
    sha = subprocess.run(["git", "rev-parse", "--short=10", "HEAD"], cwd=repo, check=True,
                         capture_output=True, text=True).stdout.strip()
    return repo / ".ds4" / "scratch" / "test" / ("smart-%s.verdict" % sha), sha


def go_calls(tmp_path):
    p = tmp_path / "go-calls"
    return p.read_text().splitlines() if p.exists() else []


def test_smart_second_run_reuses_the_recorded_verdict_without_rerunning_the_gates(tmp_path, smart_repo):
    repo, gate = smart_repo
    env = smart_env(tmp_path, repo, gate)
    rc1, lines1, _ = wrapper(repo, "smart", env=env)
    assert rc1 == 1 and len(lines1) == 1 and lines1[0].startswith("test-smart: FAIL ")
    vfile, sha = smart_verdict_file(repo)
    assert vfile.exists() and vfile.read_text() == lines1[0] + "\n"
    assert "test" not in go_calls(tmp_path)  # the run failed before any go test, so nothing to reuse yet
    rc2, lines2, err2 = wrapper(repo, "smart", env=env)
    assert rc2 == 1
    assert "reused verdict for %s; make test-smart FORCE=1 to rerun" % sha in err2
    assert lines2 == lines1  # the recorded verdict line, not a new run's
    assert "test" not in go_calls(tmp_path)  # the fake go never ran a test again


def test_smart_force_reruns_instead_of_reusing(tmp_path, smart_repo):
    repo, gate = smart_repo
    env = smart_env(tmp_path, repo, gate)
    wrapper(repo, "smart", env=env)
    vfile, _ = smart_verdict_file(repo)
    # A stale record with a label no gate script could produce: FORCE must rerun and re-record over it.
    vfile.write_text("test-smart: FAIL 0000000000 stale-label 1\n")
    rc, lines, err = wrapper(repo, "smart", env=dict(env, FORCE="1"))
    assert rc == 1 and len(lines) == 1 and lines[0].startswith("test-smart: FAIL ")
    assert "reused verdict" not in err  # FORCE reran the gates, which failed again
    assert "affected@" in vfile.read_text()  # the rerun re-recorded with the real pkgs label
    assert "test" not in go_calls(tmp_path)


@pytest.mark.parametrize("stored", ["BUSY", "ENV"])
def test_smart_a_stored_busy_or_env_verdict_is_not_reused(tmp_path, smart_repo, stored):
    # BUSY, ENV, EMPTY and BUDGET describe the box, not the commit: recorded or not, they must rerun.
    repo, gate = smart_repo
    env = smart_env(tmp_path, repo, gate)
    _, lines, _ = wrapper(repo, "smart", env=env)
    vfile, _ = smart_verdict_file(repo)
    f = lines[0].split()
    vfile.write_text("test-smart: %s %s %s 9\n" % (stored, f[2], f[3]))
    rc, lines2, err = wrapper(repo, "smart", env=env)
    assert rc == 1
    assert "reused verdict" not in err
    assert lines2[0].startswith("test-smart: FAIL ")  # the gates reran and failed again
    assert vfile.read_text().startswith("test-smart: FAIL ")  # and its own verdict replaced the stored one


def test_smart_lane_wait_above_1500_is_clamped(tmp_path, smart_repo):
    repo, gate = smart_repo
    env = smart_env(tmp_path, repo, gate, GORGE_TEST_LANE_WAIT="14400")
    rc, lines, err = wrapper(repo, "smart", env=env)
    assert rc == 1 and lines[0].startswith("test-smart: FAIL ")
    assert "GORGE_TEST_LANE_WAIT=14400 clamped to 1500" in err
    # a value the environment cannot make sense of falls back to 1500 silently
    _, _, err2 = wrapper(repo, "smart", env=dict(env, GORGE_TEST_LANE_WAIT="soon", FORCE="1"))
    assert "clamped" not in err2


# ---- oracle repro recordings run their own tests, not the whole package -----------------------------

def test_oracle_repro_recording_change_runs_the_repro_tests_not_the_whole_package():
    path = "dirac/oracle/testdata/repro/x.repro.json"
    assert tt.pkg_of(path, DIRMAP) == "m/dirac/oracle"  # the precondition: the file maps to the heavy package
    got, _ = planned({path: None})
    kind, rx = got["./dirac/oracle"]
    assert kind == "named"
    for family in ("TestReproCompactRoundTrip", "TestReproGlobMatchesTable", "ReprosLockstep"):
        assert family in rx
    assert "Census" in rx


def test_oracle_repro_change_beside_a_named_test_keeps_both():
    got, _ = planned({"dirac/oracle/testdata/repro/x.repro.json": None, "dirac/oracle/x_test.go": [(7, 7)]},
                     {"dirac/oracle/x_test.go": LEAF_TEST})
    kind, rx = got["./dirac/oracle"]
    assert kind == "named" and rx.startswith("^(TestOne)$|") and "ReprosLockstep" in rx


def test_oracle_non_repro_fixture_still_runs_whole():
    path = "dirac/oracle/testdata/pins/x.json"
    assert tt.pkg_of(path, DIRMAP) == "m/dirac/oracle"
    assert planned({path: None})[0]["./dirac/oracle"][0] == "whole"
    # a repro change alongside such a fixture still needs the whole package
    both = planned({path: None, "dirac/oracle/testdata/repro/x.repro.json": None})[0]
    assert both["./dirac/oracle"][0] == "whole"


def test_engine_rules_testdata_change_still_runs_whole():
    path = "engine/rules/testdata/repro/x.repro.json"
    assert tt.pkg_of(path, DIRMAP) == "m/engine/rules"
    assert planned({path: None})[0]["./engine/rules"][0] == "whole"


def test_oracle_repro_change_runs_the_tests_that_name_the_recording():
    # A re-recorded custom-pool recording leaves its reader's _test.go unchanged, so plan() takes the
    # reader's tests from repro_named (repro_named itself is pinned in the repo test below).
    recording = "dirac/oracle/testdata/repro/a_ready_to_rumble_damages_a_shielded_creature.repro.json"
    reader = (HERE.parent / "dirac/oracle/cantprevent_test.go").read_text()
    assert "a_ready_to_rumble_damages_a_shielded_creature" in reader  # the precondition: a reader names it
    got, _ = planned({recording: None}, repro_names={"m/dirac/oracle": {"TestCantPreventRecord", "TestCantPreventReprosReach"}})
    kind, rx = got["./dirac/oracle"]
    assert kind == "named"
    for name in ("TestCantPreventRecord", "TestCantPreventReprosReach", "TestReproCompactRoundTrip", "TestReproGlobMatchesTable", "ReprosLockstep"):
        assert name in rx
    assert "Census" in rx


def test_repro_names_for_a_untouched_package_add_nothing():
    got, _ = planned({}, repro_names={"m/dirac/oracle": {"TestOne"}})
    # names merge only into a package the diff touched; the always-run guards still name the oracle package
    assert "TestOne" not in got.get("./dirac/oracle", ("", ""))[1]


def test_repro_named_maps_a_recording_to_its_reader(repo):
    (repo / "dirac/oracle/testdata/repro").mkdir(parents=True)
    (repo / "dirac/oracle/testdata/repro/foo.repro.json").write_text("{}")
    (repo / "dirac/oracle").mkdir(parents=True, exist_ok=True)
    (repo / "dirac/oracle/x_test.go").write_text(
        "package oracle\n\nvar cases = []struct{ name string }{{\"foo\"}}\n\nfunc TestFooReprosReach(t *testing.T) {}\n")
    (repo / "dirac/oracle/other_test.go").write_text("package oracle\n\nfunc TestBarReprosReach(t *testing.T) {}\n")
    sh = lambda *a: subprocess.run(a, cwd=repo, check=True, capture_output=True)
    sh("git", "add", "dirac/oracle")
    changes = {"dirac/oracle/testdata/repro/foo.repro.json": None}
    got = tt.repro_named(changes, {"dirac/oracle": "m/dirac/oracle"},
                         lambda p: (repo / p).read_text().splitlines(), cwd=str(repo))
    assert got == {"m/dirac/oracle": {"TestFooReprosReach"}}  # the reader file's tests, not the other file's
    # a recording no test file names maps to nothing: the fallback families cover it
    (repo / "dirac/oracle/testdata/repro/bar.repro.json").write_text("{}")
    got = tt.repro_named({"dirac/oracle/testdata/repro/bar.repro.json": None}, {"dirac/oracle": "m/dirac/oracle"},
                         lambda p: (repo / p).read_text().splitlines(), cwd=str(repo))
    assert got == {}


def test_repro_named_maps_a_named_recording_to_its_manifest_subtest(repo):
    d = repo / "dirac/oracle/testdata/repro"
    d.mkdir(parents=True)
    (d / "marked.repro.json").write_text('{"named":true,"pool":"tail-01"}\n')
    (d / "globbed.repro.json").write_text('{"pool":"custom"}\n')
    (d / "served.repro.json").write_text('{"named":true,"pool":"tail-02","served":true}\n')
    sh = lambda *a: subprocess.run(a, cwd=repo, check=True, capture_output=True)
    sh("git", "add", "dirac/oracle")
    dm = {"dirac/oracle": "m/dirac/oracle"}
    read = lambda p: (repo / p).read_text().splitlines() if (repo / p).exists() else []  # as read_lines

    def mapped(*names):
        return tt.repro_named({"dirac/oracle/testdata/repro/%s.repro.json" % n: None for n in names}, dm, read, cwd=str(repo))
    assert mapped("marked") == {"m/dirac/oracle": {"TestReproManifest/marked"}}
    assert mapped("served", "marked") == {"m/dirac/oracle": {"TestReproManifest/marked", "TestReproManifest/served"}}
    # a recording in a globbed pool is not marked: the glob test replays it, so nothing maps
    assert mapped("globbed") == {}
    # a deleted recording reads as empty and maps to nothing
    assert mapped("gone") == {}


def test_plan_runs_a_manifest_subtest_as_its_parent_test():
    # `-run` splits at a slash outside brackets, so a `TestReproManifest/x` element would filter the subtests of
    # the REPRO_TESTS tests too; the plan selects the parents, every manifest shard (TestReproManifest0..7), and the
    # whole manifest runs
    recording = "dirac/oracle/testdata/repro/marked.repro.json"
    got, _ = planned({recording: None}, repro_names={"m/dirac/oracle": {"TestReproManifest/marked"}})
    kind, rx = got["./dirac/oracle"]
    assert kind == "named" and rx.startswith("^(TestReproManifest[0-9]+)$|")
    assert "/" not in rx
    for k in range(8):
        assert re.match(rx, "TestReproManifest%d" % k)
    for name in ("TestReproCompactRoundTrip", "TestReproGlobMatchesTable", "ReprosLockstep"):
        assert name in rx


def test_census_selects_the_pin_and_registration_ratchets():
    # 11-17 #611: TestGrantedKeywordHeadsAreRegisteredOrPinned matched no census word, so fast never ran it on the
    # PR and it failed three Mergify batches. The pin ratchets must match, in the py selection and test-smart's C,
    # and the lockstep farm replays (TestPinnedFarm*, minutes in ./dirac/oracle) must not.
    sh = (HERE / "test-targets.sh").read_text()
    c = [l for l in sh.splitlines() if l.startswith("C='Census|")][0][3:-1]
    for rx in (tt.CENSUS, c):
        for name in ("TestGrantedKeywordHeadsAreRegisteredOrPinned", "TestStateLayoutPinned",
                     "TestEveryRepoDeckParamsAreRead", "TestNonAPIPrimitivesAreRegistered",
                     "TestKnownUnsupportedFilesWellFormed", "TestCmd5SliceCleanOnlyGrows"):
            assert re.search(rx, name), (rx, name)
        assert not re.search(rx, "TestPinnedFarmWarpSpeed")


def test_smart_step_3_names_the_guard_tests_and_finds_their_packages():
    sh = (HERE / "test-targets.sh").read_text()
    c = [l for l in sh.splitlines() if l.startswith("C='Census|")][0]
    for _, rx in tt.GUARDS:
        for name in rx.split("|"):
            assert name in c
    # the anchored names cannot match inside the Test...($C) grep, so cpk needs its own for them
    assert "TestKopOrdinalsUnchanged|TestReproCompactRoundTrip|TestReproGlobMatchesTable" in sh


def test_a_reused_fail_verdict_prints_the_failures_behind_it(repo, tmp_path):
    # 11-14: a reused FAIL printed only its verdict line; the seat saw no failing test, reported DONE twice on a
    # passing local run, and the ticket parked. The rerun must show what failed.
    env, _, _ = smart_fixture(repo, tmp_path)
    (tmp_path / "stream.json").write_text("\n".join([
        ev("run", "m/p", "TestFileNames"),
        ev("output", "m/p", "TestFileNames", "    names_test.go:88: x10_test.go ends in a number\n"),
        ev("fail", "m/p", "TestFileNames"), ev("fail", "m/p")]))
    env = dict(env, GORC="1")
    first = wrapper_output(repo, "smart", env=env)
    assert _verdict_line(first).startswith("test-smart: FAIL "), first.stdout
    again = wrapper_output(repo, "smart", env=env)
    assert "reused verdict" in again.stderr
    assert _verdict_line(again).startswith("test-smart: FAIL ")
    assert "== the failures behind the reused verdict" in again.stdout
    assert "x10_test.go ends in a number" in again.stdout


def test_env_skip_reason_carries_the_skip_message():
    lines = [ev("run", test="TestA"), ev("output", test="TestA", out="=== RUN   TestA\n"),
             ev("output", test="TestA", out="    a_test.go:31: corpus at 418ba, need 91402: refetch .cards\n"),
             ev("output", test="TestA", out="--- SKIP: TestA (0.00s)\n"), ev("skip", test="TestA"), ev("pass")]
    r = tt.classify(lines)
    assert r.status == "ENV"
    assert r.reasons == ["skipped for the environment: m/p TestA (a_test.go:31: corpus at 418ba, need 91402: refetch .cards)"]


def test_fallback_timeout_is_busy_not_budget(tmp_path):
    f = tmp_path / "t.json"
    f.write_text("\n".join([ev("output", out="panic: test timed out after 5m0s\n"), ev("fail")]))
    base = ["finish", "--mode", "smart", "--json", str(f), "--rc", "1", "--sha", "s", "--pkgs", "./p", "--seconds", "1"]
    rc, out, _ = run_cli(*base)
    assert rc == 5 and out.rstrip().splitlines()[-1].startswith("test-smart: BUDGET ")
    rc, out, _ = run_cli(*base, "--fallback", "100%")
    assert rc == 75 and out.rstrip().splitlines()[-1].startswith("test-smart: BUSY ")
    assert "timed out at the fallback cap (CPUQuota 100%" in out
    # A real failure in the same fallback run is still the change's.
    f.write_text("\n".join([ev("fail", test="TestB"), ev("fail")]))
    rc, out, _ = run_cli(*base, "--fallback", "100%")
    assert rc == 1


def test_table_data_change_runs_the_table_readers_whole():
    # 11-20 #2803/#2804: a change to table data in a directory with no Go package (dirac/support evidence, pools,
    # set lists) moved kgen's supported set and selected nothing; kfuzz's synth game count went red in the batch.
    dm = dict(NDIRMAP, **{"dirac/cardset": "m/dirac/cardset", "dirac/oracle/cmd/kfuzz": "m/dirac/oracle/cmd/kfuzz"})
    dp = dict(NDEPS, **{"m/dirac/cardset": {"m/dirac/cardset"},
                        "m/dirac/oracle/cmd/kfuzz": {"m/dirac/oracle/cmd/kfuzz", "m/dirac/cardset"}})
    for path in ("dirac/support/evidence-games.tsv", "dirac/cardset/pools/fdn/a.json",
                 "dirac/cardset/setlists/FDN.txt", "dirac/manifest/mech/x"):
        res, _ = tt.plan({path: None}, dm, dp, lambda p: [])
        got = {p: k for k, p, _ in res}
        assert got.get("./dirac/cardset") == "whole", (path, got)
        assert got.get("./dirac/oracle/cmd/kfuzz") == "whole", (path, got)
        res, _ = tt.plan({path: None}, dm, dp, lambda p: [], None, (), {"fast": [], "sha": "0"})
        assert {p: k for k, p, _ in res}.get("./dirac/oracle/cmd/kfuzz") == "whole", path  # not narrowed to census




def test_smart_runs_the_oracle_in_chunks():
    # 11-20: ./dirac/oracle's census set (~120 tests) and step 4's named set (fast tier + every repro replay) each
    # summed past the oracle's 5m cap in one call on a loaded box, and test-smart printed BUDGET on diffs that never
    # touched the oracle. Both run through oracle_chunked: fixed chunks, each its own call.
    sh = (HERE / "test-targets.sh").read_text()
    assert "cpk=$(printf '%s\\n' $cpk | grep -vx './dirac/oracle' || true)" in sh
    assert 'oracle_chunked "$C" \'^TestStd[A-Za-z]*ReprosLockstep\'' in sh
    assert 'if [ "$pkg" = ./dirac/oracle ]; then oracle_chunked "$rx"' in sh
    assert 'go test -list "$1" ./dirac/oracle' in sh
    assert "chunks+=('^TestCmd5SliceCleanOnlyGrows$')" in sh
