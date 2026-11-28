"""The precheck's oracle replay slice plays the refusal shards of its merge base's rotation lane only
(test_targets.REFUSED_SHARD, rotated like the cmd5 shards): two of the eight per base, all eight over
the four lanes, every one a diff names. Run: python3 -m pytest scripts/oracle_refused_rotation_test.py"""
import sys
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import test_targets as tt  # noqa: E402

SHARDS = ["TestRefusedEntriesStillDiverge%d" % i for i in range(8)]
CMD5 = ["TestCmd5SliceCleanOnlyGrows%d" % i for i in range(8)]
OTHERS = ["TestReproCensusIsCurrent", "TestMBCAmberTombReprosLockstep"]
NAMES = SHARDS + CMD5 + OTHERS
WEIGHTS = {n: 10.0 for n in NAMES}


def sha_in(lane):
    return next("base%d" % i for i in range(1000) if zlib.crc32(("base%d" % i).encode()) % tt.ROTATION == lane)


def plain(res, sha):
    laned = tt.oracle_lanes(res, names=NAMES, weights=WEIGHTS, zero=set(), sha=sha)
    return sorted(n for k, _, rx in laned if k == "oraclelane" and "/" not in rx for n in rx[2:-2].split("|"))


def slice_entry(named=()):
    rx = "^(%s)$|" % "|".join(sorted(set(named) | {"TestMBCAmberTombReprosLockstep"})) + tt.REFUSED + "|" + tt.CENSUS
    return [("named", tt.ORACLE, rx)]


def test_a_base_plays_its_lanes_two_refusal_shards_and_the_lanes_cover_all_eight():
    res = slice_entry()
    # precondition: the slice's REFUSED arm alone selects every shard
    assert [n for n in plain(res, None) if n in SHARDS] == sorted(SHARDS)
    assert tt.ROTATION == 4
    seen = []
    for lane in range(tt.ROTATION):
        got = plain(res, sha_in(lane))
        shards = [n for n in got if n in SHARDS]
        assert shards == ["TestRefusedEntriesStillDiverge%d" % lane, "TestRefusedEntriesStillDiverge%d" % (lane + 4)], got
        # the cmd5 shards rotate on the same lane, and nothing else leaves the slice
        assert [n for n in got if n in CMD5] == ["TestCmd5SliceCleanOnlyGrows%d" % lane,
                                                 "TestCmd5SliceCleanOnlyGrows%d" % (lane + 4)], got
        assert all(n in got for n in OTHERS), got
        seen += shards
    assert sorted(seen) == sorted(SHARDS)


def test_a_refusal_shard_the_diff_names_plays_whatever_the_lane():
    got = plain(slice_entry(named=["TestRefusedEntriesStillDiverge1"]), sha_in(0))
    assert "TestRefusedEntriesStillDiverge1" in got
    assert "TestRefusedEntriesStillDiverge2" not in got  # still rotated: lane 0 plays 0 and 4


def test_the_parent_is_never_rotated_in():
    # rotated_out reads only the numbered shards: the dirac gate's parent is not a lane test
    assert not tt.rotated_out("TestRefusedEntriesStillDiverge", 1)
    assert tt.rotated_out("TestRefusedEntriesStillDiverge1", 0)
    assert not tt.rotated_out("TestRefusedEntriesStillDiverge5", 1)


def test_a_slow_listed_census_named_replay_leaves_the_lanes_unless_named():
    # TestMBCTail08CensusSearchLockstep replays one recording (82 s alone) and matches CENSUS by name only
    tail = "TestMBCTail08CensusSearchLockstep"
    names = NAMES + [tail]
    weights = dict(WEIGHTS, **{tail: 10.0})

    def got(named, slow):
        laned = tt.oracle_lanes(slice_entry(named), names=names, weights=weights, zero=set(), sha=sha_in(0),
                                slow=slow)
        return [n for k, _, rx in laned if k == "oraclelane" and "/" not in rx for n in rx[2:-2].split("|")]

    assert tail in got((), ())  # precondition: the CENSUS arm selects it
    assert tail not in got((), {tail})
    assert tail in got((tail,), {tail})  # a diff that names it plays it
    assert "TestReproCensusIsCurrent" in got((), {tail})
    # the real slow list carries it
    assert tail in tt.slow_repros(str(HERE.parent / tt.SLOW_REPROS))
