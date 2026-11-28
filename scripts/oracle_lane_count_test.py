"""How many oracle lanes the targeted precheck deals into (test_targets.py lane_counts): the fixed 8 plain
+ 4 subtest lanes filled the remote run's 12 cores, but on the local 4-core quota (CPUQuota 400%) those 12
lanes ran 24 threads on 4 cores and the plain lanes stretched 10-30x: four of the eight took over 150 s
(216, 205, 176, 157 s), on a run that passed every lane (11-28). The deal therefore follows the run's
procs: one lane per core. Run: python3 -m pytest scripts/oracle_lane_count_test.py"""
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import test_targets as tt  # noqa: E402


def test_a_twelve_core_run_keeps_the_lane_sizing_measured_for_it():
    # the deal the comment above ORACLE_LANES sizes (8 plain + 4 subtest fill the remote run's 12 cores):
    # a quota-derived count must not move it
    assert tt.lane_counts(12) == (tt.ORACLE_PLAIN_LANES, tt.ORACLE_LANES) == (8, 4)
    assert tt.lane_counts(32) == (8, 4)  # a bigger box caps at the same deal


def test_the_local_four_core_quota_deals_one_lane_per_core():
    plain, sub = tt.lane_counts(4)
    assert (plain, sub) == (3, 1)
    # precondition: one lane per core, so a lane gets a core (the ticket's run gave each lane a third)
    assert plain + sub == 4


def test_the_deal_always_stays_inside_the_constants_and_never_collapses():
    for procs in range(0, 40):
        plain, sub = tt.lane_counts(procs)
        assert 2 <= plain <= tt.ORACLE_PLAIN_LANES  # never one bin for the plain half (a collapsed deal)
        assert 1 <= sub <= tt.ORACLE_LANES


def test_without_the_runs_procs_the_select_keeps_the_full_sizing():
    # a caller that does not say (make test-targeted list, an older shell): the pre-change deal
    assert tt.lane_counts(None) == (tt.ORACLE_PLAIN_LANES, tt.ORACLE_LANES)


def test_select_deals_with_the_runs_procs(monkeypatch, capsys):
    seen = []

    def lanes(res, **kw):
        seen.append(kw)
        return res

    monkeypatch.setattr(tt, "go_graph", lambda: ({}, {}))
    monkeypatch.setattr(tt, "diff_changes", lambda base: {})
    monkeypatch.setattr(tt, "sh", lambda *a, **k: "abc\n")
    monkeypatch.setattr(tt, "replay_sel", lambda dirmap, sha: {"sha": sha})
    monkeypatch.setattr(tt, "repro_named", lambda *a, **k: {})
    monkeypatch.setattr(tt, "added_files", lambda base: ())
    monkeypatch.setattr(tt, "plan", lambda *a, **k: ([], []))
    monkeypatch.setattr(tt, "oracle_lanes", lanes)
    assert tt.main(["select", "--procs", "4"]) == 0
    assert seen[-1]["plain_lanes"] == 3 and seen[-1]["lanes"] == 1
    assert tt.main(["select", "--procs", "12"]) == 0
    assert seen[-1]["plain_lanes"] == 8 and seen[-1]["lanes"] == 4
    assert tt.main(["select"]) == 0  # no --procs: the full deal, as before
    assert seen[-1]["plain_lanes"] == tt.ORACLE_PLAIN_LANES and seen[-1]["lanes"] == tt.ORACLE_LANES


def test_the_shell_hands_select_the_runs_procs():
    sh = (HERE / "test-targets.sh").read_text()
    # both select calls (targeted and list) say how many cores the run has, as laneprocs/lanemem do
    assert 'select --base "$BASE" --procs "${GOMAXPROCS:-$(nproc)}"' in sh
    assert sh.count('select --base "$BASE" --procs') >= 2, "the list preview must preview the same deal"
