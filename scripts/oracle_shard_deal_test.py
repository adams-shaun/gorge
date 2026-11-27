"""scripts/oracle_shard_deal.py: the one cost deal CI's oracle shards and the precheck's lanes share.
Run: python3 -m pytest scripts/oracle_shard_deal_test.py"""
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import oracle_shard_deal as deal  # noqa: E402


def test_every_test_is_dealt_once_to_the_lightest_part():
    tests = ["TestA", "TestB", "TestC", "TestD"]
    weights = {"TestA": 10.0, "TestB": 6.0, "TestC": 2.0, "TestD": 2.0}
    got = deal.deal(tests, weights, 2)
    assert sorted(n for _, n, _ in got) == sorted(tests)  # every test exactly once
    part = {n: p for p, n, _ in got}
    # heaviest first: 10 -> part 0, 6 -> part 1, 2 -> part 1 (10, 6), 2 -> part 1's 8 or 0's 10? least is 1
    assert part == {"TestA": 0, "TestB": 1, "TestC": 1, "TestD": 1}


def test_ties_break_by_name_and_the_first_light_part_wins():
    got = deal.deal(["TestB", "TestA"], {"TestA": 4.0, "TestB": 4.0}, 2)
    assert [(p, n) for p, n, _ in got] == [(0, "TestA"), (1, "TestB")]


def test_missing_test_weighs_the_table_median_not_zero():
    weights = {"TestA": 1.0, "TestB": 5.0, "TestC": 9.0}
    assert deal.median_weight(weights) == 5.0
    (_, _, secs), = deal.deal(["TestNew"], weights, 2)
    assert secs == 5.0  # the conservative stand-in: an unlisted test may be slow
    (_, _, secs), = deal.deal(["TestNew"], weights, 2, default=0.08)
    assert secs == 0.08  # an explicit default (ci-oracle-shard.sh's pre-existing 0.08) is honoured


def test_zero_names_weigh_nothing():
    got = deal.deal(["TestPool", "TestReal"], {"TestReal": 3.0}, 1, default=0.0, zero={"TestPool"})
    assert dict((n, s) for _, n, s in got) == {"TestPool": 0.0, "TestReal": 3.0}


def test_empty_table_still_deals_every_test():
    got = deal.deal(["TestA", "TestB", "TestC"], {}, 3)
    assert sorted(n for _, n, _ in got) == ["TestA", "TestB", "TestC"]
    assert len({p for p, _, _ in got}) == 3


def test_unlisted_default_never_outranks_a_listed_one_second_test():
    # 11-27: the lanes dealt with the table's median (~14 s) for an unlisted test, so an untested-at-all
    # test outweighed a listed 1 s one. The callers' UNLISTED_SECONDS default keeps the order honest.
    weights = {"TestSlow": 60.0, "TestOne": 1.0}
    got = deal.deal(["TestUnknown", "TestOne", "TestSlow"], weights, 2, default=deal.UNLISTED_SECONDS)
    assert [n for _, n, _ in got] == ["TestSlow", "TestOne", "TestUnknown"]  # 0.08 < 1.0 < 60
    part = {n: p for p, n, _ in got}
    # the 0.08 s test joins a lighter part rather than balancing one: it is not a lane's worth of work
    assert part["TestUnknown"] == part["TestOne"] != part["TestSlow"]


REPO = HERE.parent
# the planner's fixture graph for a source change the oracle imports (test_targets_slice_test.py's SRC):
# the precheck arm that replays the slice
ORACLE_DIRMAP = {"dirac": "m/dirac", "dirac/oracle": "m/dirac/oracle"}
ORACLE_DEPS = {"m/dirac": {"m/dirac"}, "m/dirac/oracle": {"m/dirac/oracle", "m/dirac"}}


def _real_slice_process_shares(monkeypatch, weights):
    """Each lane process of the real oracle replay slice, for every one of the four merge-base slice
    lanes: (slice lane, the process's -run, its share of its half's summed cost). The names are the
    package's top-level test functions, the selection is the planner's (replay_sel over the real
    recordings and test files), the deal is test_targets.oracle_lanes with ORACLE_LANES."""
    import test_targets as tt
    monkeypatch.chdir(REPO)  # replay_sel reads dirac/oracle relative to the module root
    names = sorted({m.group(1) for f in sorted((REPO / "dirac" / "oracle").glob("*_test.go"))
                    for ln in f.read_text().splitlines() for m in [tt._TESTFN.match(ln)] if m})
    out = []
    for lane in range(4):
        sha = next("base%d" % i for i in range(1000) if tt.zlib.crc32(("base%d" % i).encode()) % 4 == lane)
        reprosel = tt.replay_sel(ORACLE_DIRMAP, sha)
        assert reprosel and reprosel["tests"], "the real replay selection must load"
        res, _ = tt.plan({"dirac/state.go": None}, ORACLE_DIRMAP, ORACLE_DEPS, lambda p: [], None, (), None, reprosel)
        laned = tt.oracle_lanes(res, names=names, weights=weights, zero=set())
        procs = [rx for k, _, rx in laned if k == "oraclelane"]
        assert len(procs) > tt.ORACLE_LANES, "the slice must be dealt into lanes: %s" % procs
        halves = {}
        for rx in procs:
            top, _, sub = rx.partition("/")
            share = tt.SLICE_SHARE if sub else 1.0
            members = top[2:-2].split("|") if top.startswith("^(") and top.endswith(")$") else []
            cost = sum(weights.get(n, deal.UNLISTED_SECONDS) * share for n in members)
            halves.setdefault(sub, []).append((rx, cost))
        for sub, procs_ in halves.items():
            total = sum(c for _, c in procs_)
            out.extend((lane, rx, c / total if total else 0.0) for rx, c in procs_)
    return out


def test_the_real_slice_deals_no_lane_process_over_40_percent_of_its_half(monkeypatch):
    # Done-means (11-27): no lane of the precheck's oracle replay slice may carry over 40% of the summed
    # wall. The slice is the planner's real selection over the package's real tests, dealt by
    # test_targets.oracle_lanes with the committed table; each half (the plain -run and each subtest
    # -run) is its own deal, so a process's share is of its own half's cost.
    table = REPO / "dirac" / "oracle" / "testdata" / "ci-shard-seconds.txt"
    weights = deal.read_weights(str(table))
    assert weights, "the cost table must exist and parse"
    shares = _real_slice_process_shares(monkeypatch, weights)
    worst = max(shares, key=lambda x: x[2])
    assert worst[2] <= 0.40, "slice lane %d's process %s carries %.0f%% of its half" % (
        worst[0], worst[1][:120], 100 * worst[2])
    # the check can fail: one test of the plain slice weighed as the whole slice is alone over 40%
    plain = [rx for _, rx, _ in shares if "/" not in rx]
    victim = plain[0][2:-2].split("|")[0]
    heavy = dict(weights, **{victim: sum(weights.values())})
    assert max(s for _, _, s in _real_slice_process_shares(monkeypatch, heavy)) > 0.40


def test_read_zero_takes_the_top_level_name_of_a_subtest_path(tmp_path):
    p = tmp_path / "poolskips.txt"
    p.write_text("# comment\nTestA\nTestB/sub/test\n")
    assert deal.read_zero(str(p)) == {"TestA", "TestB"}
    assert deal.read_zero(str(tmp_path / "absent.txt")) == set()


def test_read_weights_skips_comments_and_malformed_lines(tmp_path):
    p = tmp_path / "seconds.txt"
    p.write_text("# <test> <seconds>\nTestA 12.5\nbroken line here\nTestB nope\n\n")
    assert deal.read_weights(str(p)) == {"TestA": 12.5}
    assert deal.read_weights(str(tmp_path / "absent.txt")) == {}
