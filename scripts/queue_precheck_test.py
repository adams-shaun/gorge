"""Tests of scripts/queue_precheck.py: the queue's order, the checks a diff gets, and who is blamed."""
import importlib.util
import pathlib

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("queue_precheck", HERE / "queue_precheck.py")
qp = importlib.util.module_from_spec(spec)
spec.loader.exec_module(qp)

REPO = "o/gorge"


def pr(n, head="pr/x", fast=("SUCCESS", "2026-11-20T10:00:00Z"), lint=("SUCCESS", "2026-11-20T10:00:00Z"),
       draft=False, repo=REPO, extra=None, labels=()):
    checks = {"fast": fast, "lint": lint}
    checks.update(extra or {})
    return {"number": n, "isDraft": draft, "baseRefName": "main", "headRefName": head, "sha": "s%d" % n,
            "headRepo": repo, "labels": list(labels), "checks": checks}


def test_queue_order_is_mergifys_priority_then_queue_time():
    prs = [pr(1, fast=("SUCCESS", "2026-11-20T10:05:00Z")),
           pr(2, head="evl/a", fast=("SUCCESS", "2026-11-20T10:09:00Z")),
           pr(3, head="hotfix/b", fast=("SUCCESS", "2026-11-20T10:10:00Z")),
           pr(4, fast=("SUCCESS", "2026-11-20T10:01:00Z")),
           pr(5, labels=("hotfix",), fast=("SUCCESS", "2026-11-20T10:20:00Z"))]
    assert [p["number"] for p in qp.queued(prs, REPO)] == [3, 5, 2, 4, 1]


def test_queue_skips_what_mergify_does_not_queue():
    prs = [pr(1, draft=True), pr(2, repo="fork/gorge"), pr(3, fast=("FAILURE", "x")), pr(4, lint=("", None)),
           pr(5, head="mergify/merge-queue/abc"), pr(6, extra={"queue-precheck": ("FAILURE", "x")}), pr(7)]
    assert [p["number"] for p in qp.queued(prs, REPO)] == [7]


def test_plan_runs_census_of_touched_packages_and_kgen_for_dirac_or_engine():
    census = ["./dirac/kgen", "./dirac/manifest", "./engine/rules", "./dirac/oracle", "./internal/codeshape"]
    pk, kgen, rp = qp.plan(["dirac/manifest/mech.go", "dirac/oracle/x_test.go", "engine/rules/a.go"], census)
    assert pk == ["./dirac/manifest"] and kgen and rp  # the oracle and rules censuses are the batch's (minutes long)
    pk, kgen, rp = qp.plan(["server/gorged/main.go", "internal/codeshape/a.go"], census)
    assert pk == ["./internal/codeshape"] and not kgen and not rp


def test_plan_replays_for_dirac_code_outside_kgen_or_a_recording():
    # 11-20, the 7th red: dirac/mana*.go alone moved the stdmeta and MBC lockstep replays.
    assert qp.plan(["dirac/mana_pool.go"], [])[2]
    assert qp.plan(["dirac/oracle/testdata/repro/x.repro.json"], [])[2]
    assert not qp.plan(["dirac/kgen/lower.go"], [])[2]  # kgen's own tests cover it
    assert not qp.plan(["dirac/mana_test.go", "engine/rules/a.go"], [])[2]


def test_replays_are_the_dirac_repros_jobs_set():
    # 11-20 #2849: TestStdDsk/TestStdPool ReprosLockstep and TestReproManifest were outside a hand-written subset.
    rx = qp.replays_rx((HERE.parent / qp.CI_YML).read_text())
    import re
    for name in ("TestStdDskReprosLockstep", "TestStdPoolReprosLockstep", "TestReproManifest",
                 "TestStdMetaReprosLockstep", "TestMBCStormTheSeedcoreReprosLockstep", "TestRefusedEntriesStillDiverge"):
        assert re.search(rx, name), (rx, name)
    assert qp.replays_rx("jobs:\n  dirac-repros:\n    steps:\n      - run: go test -list 'Repro|^X$' ./dirac/oracle | grep\n  other:\n") == "Repro|^X$"


def test_attribute_blames_the_first_prefix_that_fails_and_drops_main_reds():
    order = [pr(10), pr(11), pr(12)]
    a, b, m = ("test", "./dirac/kgen", "TestRefused"), ("vet", "./engine/cards", ""), ("test", "./x", "TestMainRed")
    fails_at = {1: set(), 2: {a}, 3: {a, b}}  # #11 breaks a; #12 (with #10, #11 ahead) breaks b
    blame = qp.attribute([a, b, m], order, lambda items: [i for i in items if i == m],
                         lambda k, items: [i for i in items if i in fails_at[k]])
    assert blame == {11: [a], 12: [b]}


def test_attribute_blames_nobody_when_only_main_is_red():
    m = ("test", "./x", "TestMainRed")
    assert qp.attribute([m], [pr(1)], lambda items: items, lambda k, items: items) == {}
