"""The precheck's oracle replay slice plays the cmd5 ratchet shards of its merge base's lane only
(test_targets.CMD5_SHARD): two of the eight per base, all eight over the four lanes, every one a diff
names. Run: python3 -m pytest scripts/oracle_cmd5_rotation_test.py"""
import sys
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import test_targets as tt  # noqa: E402

SHARDS = ["TestCmd5SliceCleanOnlyGrows%d" % i for i in range(8)]
OTHERS = ["TestReproCensusIsCurrent"]  # the refusal shards rotate too (oracle_refused_rotation_test.py)
NAMES = SHARDS + OTHERS
WEIGHTS = {n: 10.0 for n in NAMES}


def sha_in(lane):
    return next("base%d" % i for i in range(1000) if zlib.crc32(("base%d" % i).encode()) % 4 == lane)


def plain(res, sha):
    laned = tt.oracle_lanes(res, names=NAMES, weights=WEIGHTS, zero=set(), sha=sha)
    return sorted(n for _, _, rx in laned if "/" not in rx for n in rx[2:-2].split("|"))


def slice_entry(named=()):
    rx = ("^(%s)$|" % "|".join(named) if named else "") + tt.REFUSED + "|" + tt.CENSUS
    return [("named", tt.ORACLE, rx)]


def test_a_base_plays_its_lanes_two_shards_and_the_lanes_cover_all_eight():
    res = slice_entry()
    # precondition: the slice's CENSUS arm alone selects every shard
    assert plain(res, None) == sorted(NAMES)
    seen = []
    for lane in range(4):
        got = plain(res, sha_in(lane))
        shards = [n for n in got if n in SHARDS]
        assert shards == ["TestCmd5SliceCleanOnlyGrows%d" % lane, "TestCmd5SliceCleanOnlyGrows%d" % (lane + 4)], got
        assert all(n in got for n in OTHERS), got  # nothing else leaves the slice
        seen += shards
    assert sorted(seen) == sorted(SHARDS)


def test_a_shard_the_diff_names_plays_whatever_the_lane():
    lane = 0
    got = plain(slice_entry(named=["TestCmd5SliceCleanOnlyGrows1"]), sha_in(lane))
    assert "TestCmd5SliceCleanOnlyGrows1" in got
    assert "TestCmd5SliceCleanOnlyGrows2" not in got  # still rotated: lane 0 plays 0 and 4


def test_the_selector_passes_the_merge_base():
    # cmd_select hands oracle_lanes replay_sel's merge base, so the targeted run rotates
    src = (HERE / "test_targets.py").read_text()
    assert 'res = oracle_lanes(res, sha=reprosel.get("sha"), changed=changes)' in src
