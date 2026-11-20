"""The one-minute per-test budget in scripts/test_targets.py: a test the diff adds or changes over 60 s is
BUDGET, the same test unchanged is a PASS with a report line. Run: python3 -m pytest scripts/test_targets_budget_test.py"""
import json
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import test_targets as tt  # noqa: E402

REPO = HERE.parent
SLOW = "m/leaf"
FILE = ["package leaf", "", "import \"testing\"", "", "func helper() {}", "", "func TestSlow(t *testing.T) {", "\ta()", "}", "",
        "func TestFast(t *testing.T) {", "\tb()", "}"]


def stream(slow_s=70.0, rc_pass=True):
    evs = [{"Action": "run", "Package": SLOW, "Test": "TestSlow"},
           {"Action": "pass", "Package": SLOW, "Test": "TestSlow", "Elapsed": slow_s},
           {"Action": "pass", "Package": SLOW, "Test": "TestFast", "Elapsed": 0.01},
           {"Action": "pass", "Package": SLOW, "Elapsed": slow_s}]
    return [json.dumps(e) for e in evs]


def test_changed_tests_names_the_added_or_edited_test_functions():
    assert tt.changed_tests({"leaf/x_test.go": None}, {"leaf/x_test.go"}, lambda p: FILE) == {("leaf", "TestSlow"), ("leaf", "TestFast")}
    assert tt.changed_tests({"leaf/x_test.go": [(8, 8)]}, set(), lambda p: FILE) == {("leaf", "TestSlow")}
    assert tt.changed_tests({"leaf/x_test.go": [(5, 5)]}, set(), lambda p: FILE) == set()  # a helper line names no test
    assert tt.changed_tests({"leaf/x.go": None, "docs/a.md": None}, {"leaf/x.go"}, lambda p: FILE) == set()


def test_a_changed_test_over_60s_is_budget_and_named():
    changed = {("leaf", "TestSlow")}
    res = tt.classify(stream(70), changed=changed)
    assert res.status == "BUDGET"
    assert any("TestSlow" in l and l.startswith("BUDGET") and "70.0s" in l for l in res.slow)


def test_the_same_test_unchanged_is_a_pass_with_a_report_line():
    res = tt.classify(stream(70), changed=set())
    assert res.status == "PASS"
    assert any("TestSlow" in l and "not changed by this diff" in l for l in res.slow)
    assert tt.classify(stream(70), changed=None).status == "PASS"  # no diff known: report only


def test_a_changed_test_under_the_budget_is_clean_and_a_fast_changed_test_beside_a_slow_one_is_not_blamed():
    assert tt.classify(stream(59.9), changed={("leaf", "TestSlow")}).slow == []
    res = tt.classify(stream(70), changed={("leaf", "TestFast")})
    assert res.status == "PASS" and res.slow and not res.slow[0].startswith("BUDGET")


def test_a_directory_only_matches_on_a_path_boundary():
    assert tt.is_changed({("rules", "TestSlow")}, "m/engine/rules", "TestSlow")
    assert not tt.is_changed({("rules", "TestSlow")}, "m/engine/brules", "TestSlow")


def test_the_fallback_cap_keeps_the_budget_off_a_slow_changed_test():
    res = tt.classify(stream(70), changed={("leaf", "TestSlow")}, fallback="200%")
    assert res.status == "PASS" and "not enforced at the fallback cap" in res.slow[0]
    # a timeout at the fallback cap is still BUSY (finish's rule)
    assert tt.EXIT["BUSY"] == 75


def test_finish_prints_the_budget_verdict_and_exit_5(tmp_path):
    f = tmp_path / "t.json"
    f.write_text("\n".join(stream(70)))
    import io
    out = io.StringIO()
    assert tt.finish("one", str(f), 0, "s", ["./leaf"], 3, [], out=out, changed={("leaf", "TestSlow")}) == 5
    text = out.getvalue()
    assert "BUDGET: changed test over the 60s budget: m/leaf TestSlow 70.0s" in text
    assert text.rstrip().splitlines()[-1] == "test-one: BUDGET s ./leaf 3"
    out = io.StringIO()
    assert tt.finish("one", str(f), 0, "s", ["./leaf"], 3, [], out=out, changed=set()) == 0
    assert "slow test over the 60s budget" in out.getvalue()
    assert out.getvalue().rstrip().splitlines()[-1] == "test-one: PASS s ./leaf 3"
    out = io.StringIO()  # at the fallback cap a timeout stays BUSY
    f.write_text("\n".join(stream(70) + [json.dumps({"Action": "output", "Package": SLOW, "Output": "panic: test timed out after 5m0s\n"})]))
    assert tt.finish("one", str(f), 1, "s", ["./leaf"], 3, [], out=out, fallback="200%", changed={("leaf", "TestSlow")}) == 75


def test_a_changed_slow_test_does_not_hide_a_real_failure():
    evs = stream(70) + [json.dumps({"Action": "fail", "Package": SLOW, "Test": "TestOther", "Elapsed": 0.01})]
    res = tt.classify(evs, rc=1, changed={("leaf", "TestSlow")})
    assert res.status == "FAIL", res.status
    # a timeout or kill still outranks the failure: the run did not finish
    timed = evs + [json.dumps({"Action": "output", "Package": SLOW, "Output": "panic: test timed out after 5m0s\n"})]
    assert tt.classify(timed, rc=1, changed={("leaf", "TestSlow")}).status == "BUDGET"


def test_a_changed_root_package_test_is_budget(tmp_path):
    """The root package's JSON Package is its module import path, not ".": a root test the diff changes is
    still recognised, and the same test unchanged is only reported."""
    import subprocess
    repo = tmp_path / "r"
    repo.mkdir()
    g = lambda *a: subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *a], cwd=repo, check=True, capture_output=True)
    g("init", "-q", "-b", "main")
    (repo / "go.mod").write_text("module example.com/gorge\n\ngo 1.21\n")
    (repo / "root_test.go").write_text("package gorge\n\nfunc TestRootSlow(t *testing.T) {\n}\n")
    g("add", "."), g("commit", "-q", "-m", "base")
    log = tmp_path / "t.json"
    log.write_text(json.dumps({"Action": "pass", "Package": "example.com/gorge", "Test": "TestRootSlow", "Elapsed": 70.0}))
    run = lambda: subprocess.run([sys.executable, "-I", str(HERE / "test_targets.py"), "finish", "--mode", "one", "--json", str(log),
                                  "--sha", "s", "--pkgs", ".", "--base", "main"], cwd=repo, capture_output=True, text=True)
    r = run()  # unchanged: reported, not failed
    assert r.returncode == 0 and "not changed by this diff" in r.stdout, r.stdout
    (repo / "root_test.go").write_text("package gorge\n\nfunc TestRootSlow(t *testing.T) {\n\t// changed\n}\n")
    r = run()
    assert r.returncode == 5, r.stdout
    assert "BUDGET: changed test over the 60s budget: example.com/gorge TestRootSlow 70.0s" in r.stdout


# The tests over the budget that the repo owns up to, with their owner action. Rows leave when the test is
# fixed; never add one. Keep it in step with docs/agents/known-slow.md (the 13-row seed from the precheck
# audit's slowest-50 table; see its Seeding).
KNOWN_SLOW_PINS = frozenset({
    ("dirac/oracle", "TestGraveReduceLockstep"),
    ("dirac/oracle", "TestGrantSearchPainlandFundingLockstep"),
    ("engine/rules", "TestKernelWorldsInFuzzGame2"),
    ("dirac/kgen", "TestTableBuildDeterministic"),
    ("dirac/oracle", "TestCountheadsLockstep"),
    ("dirac/oracle", "TestFiredmgLockstep"),
    ("dirac/oracle", "TestCascadeLockstep"),
    ("dirac/oracle", "TestMBCTheWorldTreeReprosLockstep"),
    ("dirac/oracle", "TestCopyTokenTappedLockstep"),
    ("dirac/oracle", "TestEclblightLockstep"),
    ("dirac/oracle", "TestDividedXTotalClassLockstep"),
    ("engine/rules", "TestKernelWorldsInFuzzGame5Shard1"),
    ("dirac/oracle", "TestLeftzoneMutation"),
})
ACTIONS = {"split", "shard", "nightly"}


def known_slow_rows(text):
    rows = []
    for ln in text.splitlines():
        cells = [c.strip() for c in ln.strip().strip("|").split("|")]
        if ln.startswith("|") and len(cells) == 4 and cells[0] not in ("Package", "") and not set(cells[0]) <= set("-"):
            rows.append(cells)
    return rows


def test_known_slow_list_only_shrinks():
    rows = known_slow_rows((REPO / "docs/agents/known-slow.md").read_text())
    got = {(r[0], r[1]) for r in rows}
    extra, stale = got - KNOWN_SLOW_PINS, KNOWN_SLOW_PINS - got
    assert not extra, "known-slow.md gained a row (never add one; split the test): %s" % sorted(extra)
    assert not stale, "KNOWN_SLOW_PINS has a pair the table no longer lists (delete both when fixed): %s" % sorted(stale)
    for r in rows:
        assert r[3].split()[0].strip("`") in ACTIONS, "row %s has no owner action (split, shard or nightly)" % r[:2]
        assert re.fullmatch(r"\d+(\.\d+)?", r[2]), "row %s: median seconds is not a number" % r[:2]


def test_known_slow_ratchet_has_teeth():
    row = "| m/leaf | TestSlow | 70 | split |"
    assert known_slow_rows("| Package | Test | Median s | Owner action |\n|---|---|---|---|\n" + row) == [["m/leaf", "TestSlow", "70", "split"]]


def test_fixture_diff_adding_a_70s_test_is_budget_and_unchanged_is_pass(tmp_path):
    """The acceptance, end to end through the CLI against a real git diff."""
    import subprocess
    repo = tmp_path / "r"
    (repo / "leaf").mkdir(parents=True)
    g = lambda *a: subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *a], cwd=repo, check=True, capture_output=True)
    g("init", "-q", "-b", "main")
    (repo / "leaf" / "a_test.go").write_text("package leaf\n\nfunc TestOld(t *testing.T) {\n}\n")
    g("add", "."), g("commit", "-q", "-m", "base")
    log = tmp_path / "t.json"
    evs = [{"Action": "pass", "Package": "m/leaf", "Test": t, "Elapsed": 70.0} for t in ("TestOld", "TestNew")]
    log.write_text("\n".join(json.dumps(e) for e in evs))
    run = lambda: subprocess.run([sys.executable, "-I", str(HERE / "test_targets.py"), "finish", "--mode", "one", "--json", str(log),
                                  "--sha", "s", "--pkgs", "./leaf", "--base", "main"], cwd=repo, capture_output=True, text=True)
    r = run()  # nothing changed: both tests are over the budget and neither is the diff's
    assert r.returncode == 0 and r.stdout.count("slow test over the 60s budget") == 2, r.stdout
    (repo / "leaf" / "b_test.go").write_text("package leaf\n\nfunc TestNew(t *testing.T) {\n}\n")
    r = run()
    assert r.returncode == 5, r.stdout
    assert "BUDGET: changed test over the 60s budget: m/leaf TestNew 70.0s" in r.stdout
    assert "slow test over the 60s budget" in r.stdout and "TestOld" in r.stdout  # the unchanged one is only reported
