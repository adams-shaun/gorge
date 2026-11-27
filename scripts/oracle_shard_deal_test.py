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
