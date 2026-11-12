"""Smoke test of scripts/test_targets.py (the verdict parser, exit codes and test selection) and of
scripts/test-targets.sh's preflight. Run: python3 -m pytest scripts/test_targets_test.py"""
import json
import os
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


@pytest.mark.parametrize("msg,env", [
    ("gorge corpus: open .cards/cardsfolder: no such file", True),
    ("no gorge corpus: boom", True),
    ("no FDN decks in \"x\" (set GORGE_POOLS or KERNEL_FDN_DECKS)", True),
    ("no fdn table (make dirac-tables): x", True),
    ("synthetic pools: none", True),
    ("FDN pool absent: x", True),
    ("Grizzly Bears not in the FDN table", False),
    ("every pool card is supported: no inert fallback to play", False),
    ("every tail bucket fits one table: nothing to split", False),
    ("table too small for the scenario library", False),
    ("corpus missing card \"Gitaxian Probe\"", False),
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
          "leaf": "m/leaf", "internal/archtest": "m/internal/archtest", "internal/codeshape": "m/internal/codeshape"}
DEPS = {
    "m/engine/rules": {"m/engine/rules"},
    "m/engine/game/gorgegame": {"m/engine/game/gorgegame", "m/engine/rules"},
    "m/server/host": {"m/server/host", "m/engine/rules"},
    "m/leaf": {"m/leaf"},
    "m/internal/archtest": {"m/internal/archtest"},
    "m/internal/codeshape": {"m/internal/codeshape"},
}
LEAF_TEST = ["package leaf", "", "import \"testing\"", "", "func helper() {}", "", "func TestOne(t *testing.T) {", "\ta()", "}", "",
             "// TestTwo does it.", "func TestTwo(t *testing.T) {", "\tb()", "}"]


def planned(changes, files=None):
    res, skipped = tt.plan(changes, DIRMAP, DEPS, lambda p: (files or {}).get(p, []))
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


def test_heavy_package_with_named_tests_adds_the_census_tests():
    kind, rx = planned({"engine/rules/x_test.go": [(7, 7)]}, {"engine/rules/x_test.go": LEAF_TEST})[0]["./engine/rules"]
    assert kind == "named" and rx.startswith("^(TestOne)$|") and "Census" in rx


def test_docs_only_diff_selects_just_the_structural_packages():
    got, _ = planned({"docs/agents/invariants.md": None, "AGENTS.md": None})
    assert sorted(got) == ["./internal/archtest", "./internal/codeshape"]


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
    (Path(d) / "go.mod").write_text("module m\n\ngo 1.22\n")
    yield Path(d)
    shutil.rmtree(d)


def wrapper(repo, *args, env=None):
    e = dict(os.environ, TEST_TARGETS_SCOPED="1", BASE="land", **(env or {}))
    e.pop("GORGE_CARDS", None)
    r = subprocess.run(["bash", "scripts/test-targets.sh", *args], cwd=repo, env=e, capture_output=True, text=True)
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


def test_one_usage_errors_exit_2(repo):
    (repo / ".cards" / "cardsfolder").mkdir(parents=True)
    (repo / ".cards" / "cardsfolder" / "a.txt").write_text("x")
    for env in ({"PKG": "./...", "RUN": "T"}, {"PKG": "./p"}, {"PKG": "./p", "RUN": "T", "TESTENV": "PATH=/x"}, {"PKG": "./p", "RUN": "T", "TIMEOUT": "9m"}):
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
