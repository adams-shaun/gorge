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
    pk, kgen, rp, _ = qp.plan(["dirac/manifest/mech.go", "dirac/oracle/x_test.go", "engine/rules/a.go"], census)
    assert pk == ["./dirac/manifest"] and kgen and rp  # the oracle and rules censuses are the batch's (minutes long)
    pk, kgen, rp, _ = qp.plan(["server/gorged/main.go", "internal/codeshape/a.go"], census)
    assert pk == ["./internal/codeshape"] and not kgen and not rp


def test_plan_replays_for_dirac_code_outside_kgen_or_a_recording():
    # 11-20, the 7th red: dirac/mana*.go alone moved the stdmeta and MBC lockstep replays.
    assert qp.plan(["dirac/mana_pool.go"], [])[2]
    assert qp.plan(["dirac/oracle/testdata/repro/x.repro.json"], [])[2]
    assert not qp.plan(["dirac/kgen/lower.go"], [])[2]  # kgen's own tests cover it
    assert not qp.plan(["dirac/mana_test.go", "engine/rules/a.go"], [])[2]


def test_replays_are_the_dirac_repros_jobs_set():
    # 11-20 #2849: TestStdDsk/TestStdPool ReprosLockstep and TestReproManifest were outside a hand-written subset.
    # The manifest is the shards TestReproManifest0..7 (repro_manifest_test.go).
    rx = qp.replays_rx((HERE.parent / qp.CI_YML).read_text())
    import re
    for name in ("TestStdDskReprosLockstep", "TestStdPoolReprosLockstep", "TestReproManifest0", "TestReproManifest7",
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


def test_queue_skips_conflicting_prs():
    p = pr(1)
    p["mergeable"] = "CONFLICTING"
    assert [x["number"] for x in qp.queued([p, pr(2)], REPO)] == [2]


def test_mergifys_batches_set_the_order():
    # 11-20: batch #2869 held #2744, which the estimate had outside its head.
    pulls = [{"number": 2869, "head": {"ref": "mergify/merge-queue/a"}, "created_at": "2026-11-20T21:46:49Z",
              "body": "#2669 #2688 #2744"},
             {"number": 2870, "head": {"ref": "mergify/merge-queue/b"}, "created_at": "2026-11-20T21:47:05Z",
              "body": "#2669 #2688 #2744 #2745 and #2669 again"},
             {"number": 2862, "head": {"ref": "mergify/merge-queue/c"}, "created_at": "2026-11-20T21:30:00Z",
              "body": "#1 #2 #3 #4 #5 #6"},
             {"number": 2700, "head": {"ref": "pr/x"}, "created_at": "2026-11-20T21:50:00Z", "body": "#9"}]
    assert qp.batch_numbers(pulls, "2026-11-20T21:41:00Z") == [2669, 2688, 2744, 2745]
    est = [pr(n) for n in (2540, 2669, 2688, 2716, 2744)]
    assert [p["number"] for p in qp.head_order([2669, 2744, 9999], est, 4, fresh=2)] == [2669, 2744, 2540, 2688]


def test_head_order_always_reaches_prs_without_a_status():
    # 11-21: queue_conditions require a passing queue-precheck. The batched PRs already carry one; a run must still
    # reach the next PRs that do not, or the queue drains before any new PR may batch.
    passed = {"queue-precheck": ("SUCCESS", "2026-11-21T10:00:00Z")}
    est = [pr(n, extra=passed if n < 120 else None) for n in range(100, 130)]
    batch = list(range(100, 120))  # two full batches, all passed
    got = [p["number"] for p in qp.head_order(batch, est, 20)]
    assert got[:20] == batch and got[20:] == list(range(120, 130))
    # a passed PR not (yet) in a batch is not re-checked as a fresh one
    est2 = [pr(200, extra=passed), pr(201)]
    assert [p["number"] for p in qp.head_order([], est2, 20)] == [201]


def test_generated_files_run_dirac_freshness_pins():
    # 11-20 #2744: ask ordinal 157 given twice; TestAskKindsUnchanged and TestManifestGenIsCurrent live in ./dirac.
    census = ["./dirac", "./dirac/manifest", "./engine/effects"]
    assert "./dirac" in qp.plan(["dirac/manifest/key/raProtectionChoice"], census)[0]
    assert "./engine/effects" in qp.plan(["engine/effects/kinds_gen.go"], census)[0]
    assert "./dirac" not in qp.plan(["engine/effects/a.go"], census)[0]
    import re
    assert re.search(qp.CENSUS, "TestAskKindsUnchanged") and re.search(qp.CENSUS, "TestManifestGenIsCurrent")


def test_dirac_code_runs_the_state_bar():
    # 11-20 batch #2874: tail-18 at 45184 B against the 45056 bar, summed over the batch's PRs (register cause 96).
    assert qp.plan(["dirac/coldstore.go"], [])[3]
    assert qp.plan(["dirac/kgen/lower.go"], [])[3]  # kgen's lowering sizes the pools' cold arrays
    assert qp.plan(["dirac/support/refused"], [])[3]  # so does the supported set
    assert qp.plan(["engine/effects/a.go"], [])[3]
    assert not qp.plan(["server/gorged/main.go", "bots/x.go"], [])[3]
    import re
    for n in ("TestEveryPoolFitsTheStateBar", "TestStateSize", "TestColdArraysHoldNoPointers"):
        assert re.match(qp.STATE_BAR, n)


def test_attribute_bisects_and_shares_runs():
    # 11-21: a linear prefix scan built one merged tree per queue PR (24 min for a 9-PR head).
    order = [pr(n) for n in range(100, 116)]  # 16 PRs
    a, b = ("test", "./dirac/oracle", "TestRainOfRust"), ("test", "./dirac", "TestAskKindsUnchanged")
    first = {a: 6, b: 13}  # #105 breaks a, #112 breaks b
    calls = []

    def on_prefix(k, items):
        calls.append(k)
        return [i for i in items if k >= first[i]]
    blame = qp.attribute([a, b], order, lambda items: [], on_prefix)
    assert blame == {105: [a], 112: [b]}
    assert len(set(calls)) <= 2 * 4 and len(calls) == len(set(calls))  # ~log2(16) per item, each k run once


def test_conflicting_prs_do_not_use_a_slot():
    # 11-22: 18 conflicting PRs (GitHub's mergeable read UNKNOWN) filled the head of every run; none merged, no
    # status was posted, and the gated queue merged nothing for 5.5 h.
    order = [pr(n) for n in range(1, 31)]
    conflicting = set(range(1, 19))
    merged = qp.merge_until(order, 10, lambda p: p["number"] not in conflicting)
    assert [p["number"] for p in merged] == list(range(19, 29))
    assert qp.merge_until(order, 50, lambda p: p["number"] not in conflicting)[-1]["number"] == 30


def test_replay_chunks_cover_every_name_once():
    # 11-22: the whole replay set (448 s) was a run's longest command; it now runs in REPLAY_CHUNKS concurrent chunks.
    import re
    names = ["TestStd%dReprosLockstep" % i for i in range(10)] + ["TestRefusedEntriesStillDiverge"]
    chunks = qp.chunk_names(names, 3)
    assert len(chunks) == 3
    hit = [n for n in names for c in chunks if re.match(c, n)]
    assert sorted(hit) == sorted(names)  # each name in exactly one chunk
    assert qp.chunk_names(["TestA"], 3) == ["^(TestA)$"]
    assert qp.BATCH_WAIT == 0


def test_tree_wide_ratchets_run_for_any_go_change():
    # 11-21 batch #3304: #3171 grew dirac (*State).scriptOffers past internal/codeshape's complexity ceiling; the
    # ratchet lives in internal/codeshape, which the touched packages' census never selects.
    assert qp.tree_wide(["dirac/script_offers.go"]) and qp.tree_wide(["server/host/x_test.go"])
    assert not qp.tree_wide(["dirac/support/evidence-games.tsv", "docs/site/index.md"])
    pkgs = {g[0][0]: g for g in qp.TREE_WIDE}
    assert pkgs["./internal/codeshape"][1:] == ("", "")  # the whole package: every function, shape and strings ratchet
    assert pkgs["./internal/archtest"][2] == "^TestEngineCompilesFor32Bit$"  # all but the ~50 s 386 build


def test_the_gate_runs_on_main_pushes_only():
    # 11-23 00:26-00:40Z: run 45088771344, started by a mergify/merge-queue/* push, held the one concurrency slot and
    # was cancelled when its batch closed; 22 main-push runs waited behind it and none ran (23 min with no green run).
    # A batch forms only from PRs a run already passed, so a batch-branch run only blocks the gate.
    import re
    wf = (HERE.parent / ".github/workflows/queue-precheck.yml").read_text()
    on = wf[wf.index("\non:\n"):wf.index("\npermissions:")]
    assert re.search(r"^  push:\n    branches: \[main\]$", on, re.M), on
    assert "mergify" not in on and "pull_request" not in on
    assert "schedule:" in on  # the floor
    assert re.search(r"^concurrency:\n  group: queue-precheck\n  cancel-in-progress: false$", wf, re.M)
