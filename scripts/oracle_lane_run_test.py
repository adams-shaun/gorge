"""How the targeted precheck's oracle lanes run (test_targets.py oracle_lanes and lane_procs): each lane's
GOMAXPROCS, the tests the diff changes running outside the lanes, and the refusal shards that stop rotating
when the diff changes a refusal entry. Run: python3 -m pytest scripts/oracle_lane_run_test.py"""
import re
import sys
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import test_targets as tt  # noqa: E402

SHARDS = ["TestRefusedEntriesStillDiverge%d" % i for i in range(8)]
CMD5 = ["TestCmd5SliceCleanOnlyGrows%d" % i for i in range(8)]
NAMES = SHARDS + CMD5 + ["TestReproCensusIsCurrent", "TestMBCAmberTombReprosLockstep"]
WEIGHTS = {n: 10.0 for n in NAMES}


def sha_in(lane):
    return next("base%d" % i for i in range(1000) if zlib.crc32(("base%d" % i).encode()) % tt.ROTATION == lane)


def entry(named):
    rx = "^(%s)$|" % "|".join(sorted(named)) + tt.REFUSED + "|" + tt.CENSUS
    return [("named", tt.ORACLE, rx)]


def split(res, **kw):
    """(the names that run outside the lanes, the names the plain lanes hold)."""
    laned = tt.oracle_lanes(res, names=NAMES, weights=WEIGHTS, zero=set(), sha=sha_in(0), slow=(), **kw)

    def names(kind):
        return sorted(n for k, _, rx in laned if k == kind and "/" not in rx
                      for n in NAMES if re.search(rx, n))

    return names("named"), names("oraclelane")


def test_a_lane_gets_twice_its_even_share_and_never_more_than_the_run():
    # twelve lanes on the runs this ticket measured: twice the even share, with the floor of 2
    assert tt.lane_procs(12, 12) == 2
    assert tt.lane_procs(5, 12) == 2
    assert tt.lane_procs(32, 12) == 6
    # one or two lanes: twice the even share would pass the run's own GOMAXPROCS
    assert tt.lane_procs(12, 1) == 12
    assert tt.lane_procs(12, 2) == 12
    # the floor never lifts a lane above the run either
    assert tt.lane_procs(1, 12) == 1


def test_the_shell_asks_for_the_lane_gomaxprocs(capsys):
    assert tt.main(["laneprocs", "--procs", "12", "--lanes", "1"]) == 0
    assert capsys.readouterr().out == "12\n"
    sh = (HERE / "test-targets.sh").read_text()
    assert re.search(r'lane_procs=\$\("\$\{py\[@\]\}" laneprocs --procs "\$\{GOMAXPROCS:-\$\(nproc\)\}" '
                     r'--lanes "\$\{#lanes\[@\]\}"\)', sh), "test-targets.sh computes the lane GOMAXPROCS itself"
    assert 'GOMAXPROCS=$lane_procs go test' in sh


def test_the_tests_the_diff_changes_run_outside_the_lanes():
    changed = ["TestCmd5SliceCleanOnlyGrows1", "TestMBCAmberTombReprosLockstep"]
    alone, laned = split(entry(changed))
    # the 60 s budget is a changed test's own wall: it runs at the run's GOMAXPROCS, beside no lane
    assert alone == sorted(changed)
    assert not set(changed) & set(laned)
    # precondition and the rest: the lanes still hold the unchanged selection, and nothing runs twice
    assert "TestReproCensusIsCurrent" in laned and "TestCmd5SliceCleanOnlyGrows0" in laned
    assert len(alone + laned) == len(set(alone + laned))


def test_the_changed_tests_run_before_the_lanes_and_alone_when_nothing_else_is_selected():
    res = [("named", tt.ORACLE, "^(TestReproCensusIsCurrent)$|" + tt.REFUSED)]
    laned = tt.oracle_lanes(res, names=["TestReproCensusIsCurrent"], weights=WEIGHTS, zero=set(), sha=sha_in(0),
                            slow=())
    assert laned == [("named", tt.ORACLE, "^(TestReproCensusIsCurrent)$")]
    both = tt.oracle_lanes(entry(["TestReproCensusIsCurrent"]), names=NAMES, weights=WEIGHTS, zero=set(),
                           sha=sha_in(0), slow=())
    assert [k for k, _, _ in both][0] == "named" and {k for k, _, _ in both[1:]} == {"oraclelane"}


def test_a_changed_refusal_entry_keeps_every_refusal_shard():
    res = entry(["TestMBCAmberTombReprosLockstep"])
    _, rotated = split(res, changed={"dirac/state.go": None})
    # precondition: lane 0 rotates six of the eight refusal shards out of a diff that changes no entry
    assert [n for n in rotated if n in SHARDS] == ["TestRefusedEntriesStillDiverge0", "TestRefusedEntriesStillDiverge4"]
    _, kept = split(res, changed={"dirac/state.go": None, "dirac/support/refused/Maze_of_Ith.json": None})
    assert [n for n in kept if n in SHARDS] == sorted(SHARDS)
    # only the refusal shards stop rotating: an entry is no input of the cmd5 ratchet
    assert [n for n in kept if n in CMD5] == ["TestCmd5SliceCleanOnlyGrows0", "TestCmd5SliceCleanOnlyGrows4"]


def test_select_hands_the_diff_to_the_lane_deal(monkeypatch, capsys):
    changes = {"dirac/support/refused/Maze_of_Ith.json": None}
    seen = {}
    monkeypatch.setattr(tt, "go_graph", lambda: ({}, {}))
    monkeypatch.setattr(tt, "diff_changes", lambda base: changes)
    monkeypatch.setattr(tt, "sh", lambda *a, **k: "abc\n")
    monkeypatch.setattr(tt, "replay_sel", lambda dirmap, sha: {"sha": sha})
    monkeypatch.setattr(tt, "repro_named", lambda *a, **k: {})
    monkeypatch.setattr(tt, "added_files", lambda base: ())
    monkeypatch.setattr(tt, "plan", lambda *a, **k: ([], []))

    def lanes(res, **kw):
        seen.update(kw)
        return res

    monkeypatch.setattr(tt, "oracle_lanes", lanes)
    assert tt.main(["select"]) == 0
    assert seen == {"sha": "abc", "changed": changes}
