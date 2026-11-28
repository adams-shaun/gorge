"""TF-01: the precheck's oracle replay arm (scripts/test_targets.py plan) replays the changed
recordings plus a 1/SLICE_MOD slice (scripts/test_targets.py slice_lane), not every recording; full coverage
stays with the batch. Run: python3 -m pytest scripts/test_targets_slice_test.py"""
import re
import sys
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import test_targets as tt  # noqa: E402

DIRMAP = {"dirac": "m/dirac", "dirac/oracle": "m/dirac/oracle",
          "internal/archtest": "m/internal/archtest", "internal/codeshape": "m/internal/codeshape"}
DEPS = {"m/dirac": {"m/dirac"},
        "m/dirac/oracle": {"m/dirac/oracle", "m/dirac"},
        "m/internal/archtest": {"m/internal/archtest"},
        "m/internal/codeshape": {"m/internal/codeshape"}}
# A source change the oracle imports: the arm that replays recordings (cause 64's shape).
SRC = {"dirac/state.go": None}
# Eight recordings the manifest test replays and eight of the oracle's other replay tests: enough for
# the SLICE_MOD lanes to partition them. One of each is over the slow wall.
RECORDINGS = ["amber_tomb_keeps_its_target", "basalt_shield_cracks_at_blocks", "cinder_veil_refunds_a_cast",
              "dewdrop_duo_last", "ember_wake_pays_in_shields", "frost_gate_asks_before_taps",
              "gloom_and_doom_repros_reach", "hollow_bridge_untaps_a_wall"]
REPRO_TESTS = ["TestMBCAmberTombReprosLockstep", "TestMBCBasaltShieldReprosLockstep", "TestMCCCinderVeilReprosReach",
               "TestMBCDewdropReprosLockstep", "TestMBCEmberWakeReprosReach", "TestMBCFrostGateReprosLockstep",
               "TestMBCGloomDoomReprosLockstep", "TestMBCHollowBridgeReprosReach",
               "TestStdFdnReprosLockstep", "TestStdPoolReprosLockstepFull"]
SLOW_REC = "dewdrop_duo_last"
SLOW_TEST = "TestMBCDewdropReprosLockstep"
MANIFEST = "TestReproManifest/"


def sel(sha, named=RECORDINGS, tests=REPRO_TESTS, slow=(SLOW_REC, SLOW_TEST), pools=None, globs=None):
    """A reprosel like cmd_select builds: the manifest's recordings, each pool's recordings, the pool
    glob tests, the oracle's other replay tests, the slow list and the merge base."""
    return {"named": set(named), "pools": pools or {}, "globs": globs or {}, "tests": set(tests),
            "slow": set(slow), "sha": sha}


def planned(changes, repro_names=None, reprosel=None, narrow=None):
    res, skipped = tt.plan(changes, DIRMAP, DEPS, lambda p: [], repro_names, (), narrow, reprosel)
    return res


def oracle(res):
    """The oracle's plain selection entry (its rx and kind), not the replay slice's own -run."""
    got = [(k, rx) for k, p, rx in res if p == "./dirac/oracle" and "/" not in rx]
    assert len(got) == 1, got
    return got[0]


def manifest_rx(res):
    """The -run over the replay slice's subtests (the manifest's and the pool globs'; None when there
    is none). Its top-level part matches those tests alone."""
    got = [rx for k, p, rx in res if p == "./dirac/oracle" and "/" in rx]
    assert len(got) <= 1, got
    return got[0] if got else None


def manifest_names(res):
    mrx = manifest_rx(res)
    if mrx is None:
        return set()
    return set(mrx.split("/", 1)[1].strip("()").split("|"))


def sha_in_lane(lane):
    """A plausible merge base whose crc32 lands in the lane."""
    k = 0
    while True:
        sha = "%040x" % k
        if zlib.crc32(sha.encode()) % tt.SLICE_MOD == lane:
            return sha
        k += 1


# ---- the 1/SLICE_MOD slice -------------------------------------------------------------------------------

def test_slice_is_stable_for_a_sha_and_rotates_across_shas():
    one = planned(SRC, reprosel=sel(sha=sha_in_lane(1)))
    again = planned(SRC, reprosel=sel(sha=sha_in_lane(1)))
    assert manifest_names(one) == manifest_names(again)  # stable for a merge base
    lanes = [manifest_names(planned(SRC, reprosel=sel(sha=sha_in_lane(l)))) for l in range(tt.SLICE_MOD)]
    named = set(RECORDINGS) - {SLOW_REC}
    assert set().union(*lanes) == named  # every unchanged recording lands in exactly one lane
    assert sum(len(s) for s in lanes) == len(named)
    for i, a in enumerate(lanes):
        for b in lanes[i + 1:]:
            assert not (a & b), (a, b)
    # the same rotation on the oracle's other replay tests
    tl = [set(re.findall(r"Test[A-Za-z0-9_]+", oracle(planned(SRC, reprosel=sel(sha=sha_in_lane(l))))[1]))
          for l in range(tt.SLICE_MOD)]
    tl = [s & set(REPRO_TESTS) for s in tl]
    assert set().union(*tl) == set(REPRO_TESTS) - {SLOW_TEST}


def test_a_changed_recording_is_always_included():
    # rec outside the sha's lane and on the slow list: only the change itself puts it in the -run
    lane = (zlib.crc32(SLOW_REC.encode()) + 1) % tt.SLICE_MOD
    res = planned(SRC, repro_names={"m/dirac/oracle": {MANIFEST + SLOW_REC}},
                  reprosel=sel(sha=sha_in_lane(lane)))
    assert SLOW_REC in manifest_names(res)
    # and a recording whose reader test changed too runs it as a test name, not the whole manifest
    changes = dict(SRC, **{"dirac/oracle/testdata/repro/%s.repro.json" % RECORDINGS[0]: None})
    res = planned(changes,
                  repro_names={"m/dirac/oracle": {MANIFEST + RECORDINGS[0], "TestRecordingReader"}},
                  reprosel=sel(sha=sha_in_lane((zlib.crc32(RECORDINGS[0].encode()) + 1) % tt.SLICE_MOD)))
    kind, rx = oracle(res)
    assert re.search(rx, "TestRecordingReader")
    assert RECORDINGS[0] in manifest_names(res)
    assert "TestReproManifest" not in rx.split("|")[0]  # the parent test never runs whole for a subtest


def test_the_slow_list_is_excluded_unless_the_recording_changed():
    lane = zlib.crc32(SLOW_REC.encode()) % tt.SLICE_MOD
    res = planned(SRC, reprosel=sel(sha=sha_in_lane(lane)))
    assert SLOW_REC not in manifest_names(res)  # over the 10 s wall: the batch replays it
    # the same for the oracle's other replay tests
    lane = zlib.crc32(SLOW_TEST.encode()) % tt.SLICE_MOD
    kind, rx = oracle(planned(SRC, reprosel=sel(sha=sha_in_lane(lane))))
    assert not re.search(rx, SLOW_TEST)


def test_the_replay_arm_keeps_the_refusal_check_and_the_pool_globs_are_sliced():
    lane = zlib.crc32("TestStdFdnReprosLockstep".encode()) % tt.SLICE_MOD
    kind, rx = oracle(planned(SRC, reprosel=sel(sha=sha_in_lane(lane))))
    assert re.search(rx, "TestRefusedEntriesStillDiverge0")  # the shards, never the parent (11-27 review)
    assert not re.search(rx, "TestRefusedEntriesStillDiverge")  # the whole 422 s parent is never dealt
    assert re.search(rx, "TestStdFdnReprosLockstep")  # the pool globs are in the slice population
    assert not re.search(rx, "TestShieldReplacementCollision")  # whole-game lockstep stays in the batch
    # out of the lane the glob is left to the batch with the rest
    kind, rx = oracle(planned(SRC, reprosel=sel(sha=sha_in_lane((lane + 1) % tt.SLICE_MOD))))
    assert not re.search(rx, "TestStdFdnReprosLockstep")
    res = planned(SRC, reprosel=sel(sha=sha_in_lane(zlib.crc32(RECORDINGS[0].encode()) % tt.SLICE_MOD)))
    mrx = manifest_rx(res)
    assert mrx.count("/") == 1 and mrx.startswith(MANIFEST)  # its -run matches the manifest test only
    assert not re.search(mrx, "TestRefusedEntriesStillDiverge0")
    assert not re.search(mrx, "TestStdFdnReprosLockstep")


def test_a_pool_recording_replays_as_its_glob_tests_subtest():
    lane0 = zlib.crc32("fdn_in_lane".encode()) % tt.SLICE_MOD
    out = next(n for n in ("fdn_second", "fdn_third", "fdn_fourth")
               if zlib.crc32(n.encode()) % tt.SLICE_MOD != lane0)
    pools = {"std-fdn": {"fdn_in_lane", out, "fdn_over_the_wall"}}
    globs = {"TestStdFdnReprosLockstep": ["std-fdn"],
             "TestStdPoolReprosLockstepFull": ["std-tla", "std-lci"]}  # an alias, over the wall
    slow = {"fdn_over_the_wall", SLOW_REC, SLOW_TEST}
    sha = sha_in_lane(lane0)
    res = planned(SRC, reprosel=sel(sha=sha, pools=pools, globs=globs, slow=slow))
    mrx = manifest_rx(res)
    top, sub = mrx.split("/")
    assert "TestStdFdnReprosLockstep" in top  # the manifest joins only when it has a slice too
    assert "TestStdPoolReprosLockstepFull" not in top  # over the 10 s wall: the batch replays it
    names = set(sub.strip("()").split("|"))
    assert names == {"fdn_in_lane"} | {n for n in RECORDINGS
                                       if n not in slow and zlib.crc32(n.encode()) % tt.SLICE_MOD == lane0}
    # the pool globs left the test-level slice: replaying one whole is the old 300 s stream
    kind, rx = oracle(res)
    assert not re.search(rx, "TestStdFdnReprosLockstep")
    # a changed recording replays whatever the lane says
    changes = dict(SRC, **{"dirac/oracle/testdata/repro/%s.repro.json" % out: None})
    res = planned(changes, reprosel=sel(sha=sha, pools=pools, globs=globs, slow=slow))
    assert out in manifest_names(res)
    # a pool glob whose lane is empty joins nothing
    res = planned(SRC, reprosel=sel(sha=sha, pools={"std-lci": {"lci_out"}}, globs={"TestStdLciReprosLockstep": ["std-lci"]},
                                     slow=slow))
    top, _ = manifest_rx(res).split("/")
    assert "TestStdLciReprosLockstep" not in top


def test_without_slice_input_the_replay_arm_still_covers_every_recording():
    # reprosel None: the batch's shape (ci.yml replays REPLAYS whole; a seat whose scan falls back
    # keeps full coverage rather than silently losing it)
    kind, rx = oracle(planned(SRC))
    assert tt.REPLAYS in rx
    for name in ("TestMBCAmberTombReprosLockstep", "TestRefusedEntriesStillDiverge0",
                 "TestRefusedEntriesStillDiverge7"):  # the shards are the batch's refusal coverage
        assert re.search(rx, name), name
    assert manifest_rx(planned(SRC)) is None


def test_a_changed_recording_alone_replays_itself_and_the_slice_not_the_manifest():
    rec = RECORDINGS[0]
    res = planned({"dirac/oracle/testdata/repro/%s.repro.json" % rec: None},
                  repro_names={"m/dirac/oracle": {MANIFEST + rec}},
                  reprosel=sel(sha=sha_in_lane((zlib.crc32(rec.encode()) + 1) % tt.SLICE_MOD)))
    kind, rx = oracle(res)
    assert rec in manifest_names(res)
    assert not re.search(rx, "TestReproManifest3")  # the manifest does not fold into the shared -run
    # a recording change replays the other replay tests' slice too (as the blanket did)
    assert any(re.search(rx, t) for t in REPRO_TESTS if t != SLOW_TEST)


def test_narrow_applies_the_slice_too():
    # test-smart's narrow step: an oracle fixture change runs the package whole in the plain plan, and
    # the narrow precheck turns that into the named parts -- sliced like test-targeted's
    res = planned({"dirac/oracle/testdata/x.txt": None}, narrow={"fast": ("TestFastA",), "sha": "0"},
                  reprosel=sel(sha=sha_in_lane(zlib.crc32(RECORDINGS[0].encode()) % tt.SLICE_MOD)))
    kind, rx = oracle(res)
    assert kind == "named"
    assert tt.REPLAYS not in rx
    for part in ("^TestPinManifest$", "^(TestFastA)$", "Census"):
        assert part in rx, part
    assert manifest_names(res), "the narrow precheck replays the changed recordings and the slice"


# ---- the targeted lanes ------------------------------------------------------------------------------

def test_lanes_partition_the_replay_selection_exactly():
    # test-targeted deals the oracle replay slice into concurrent lanes (11-27: 34 of 40 targeted runs
    # hit the 5m cap on one ~200-test process). Deal it and prove the union is the single-process
    # selection, name for name, and that the manifest/pool-glob subtest -run is still there, unchanged.
    res = planned(SRC, reprosel=sel(sha=sha_in_lane(1)))
    plain = oracle(res)[1]
    names = sorted(set(REPRO_TESTS) - {SLOW_TEST}) + [
        "TestRefusedEntriesStillDiverge0",          # a refusal shard the replay arm always adds
        "TestReproCensusIsCurrent",                 # selected by the census alternatives
        "TestPinManifest",                          # selected by nothing here
        "TestUnrelatedThing",
        tt.MANIFEST_TEST + "0", "TestStdFdnReprosLockstep"]  # the subtest entry's parents, not the plain entry
    before = [n for n in names if re.search(plain, n)]
    assert "TestReproCensusIsCurrent" in before and "TestUnrelatedThing" not in before
    laned = tt.oracle_lanes(res, names=names, weights={n: 10.0 for n in names}, zero=set())
    lanes = [(k, rx) for k, p, rx in laned if p == "./dirac/oracle" and k == "oraclelane" and "/" not in rx]
    assert 1 < len(lanes) <= tt.ORACLE_PLAIN_LANES
    got = []
    for _, rx in lanes:
        assert rx.startswith("^(") and rx.endswith(")$"), rx  # anchored: a name never matches as a substring
        got.extend(rx[2:-2].split("|"))
    assert sorted(got) == sorted(before), "the lanes must cover the single-process selection exactly once"
    # the subtest -run (manifest and pool globs, filtered to the slice) is dealt too: its tops are split across
    # the lanes (anchored, so the manifest's shards are named), every lane keeps the subtest filter as it was
    assert manifest_rx(res) is not None
    mtop, msub = manifest_rx(res).split("/", 1)
    subs = [(k, rx) for k, p, rx in laned if p == "./dirac/oracle" and "/" in rx]
    assert subs and all(k == "oraclelane" and rx.split("/", 1)[1] == msub for k, rx in subs)
    got_tops = []
    for _, rx in subs:
        top = rx.split("/", 1)[0]
        assert top.startswith("^(") and top.endswith(")$"), rx
        got_tops.extend(top[2:-2].split("|"))
    want_tops = [n for n in names if any(re.search(t, n) for t in mtop.split("|"))]
    assert want_tops and sorted(got_tops) == sorted(want_tops), "each subtest top runs in exactly one lane"
    assert len(laned) == len(res) - 2 + len(lanes) + len(subs)  # nothing else moved


def test_each_half_is_dealt_into_its_own_lane_budget():
    # 11-27 review: one deal over both halves balanced each bin's sum, but every bin starts two
    # processes (the ^(a|b)$ and the ^(A|B)$/(subs)), so the process walls spanned 2x while the bins
    # looked balanced (69-131 s on the real table). Each half must be dealt into its own lanes
    # budget: a process's load is exactly what the deal of that half alone gives it. On this
    # fixture the single deal puts 40+20 and 40+20 in two plain processes (loads 40,40,60,60)
    # while the plain-only deal gives 40,50,50,60 -- the multiset equality below is the contract.
    res = planned(SRC, reprosel=sel(sha=sha_in_lane(1)))
    plain_rx = oracle(res)[1]
    # CENSUS names join the plain selection from any lane; the two lane-1 replay tests and the
    # refusal guard are the rest of the plain half. Four manifest shards make the sub half.
    names = (["TestHalfPlainACensus", "TestHalfPlainBCensus", "TestHalfPlainCCensus",
              "TestHalfPlainDCensus", "TestRefusedEntriesStillDiverge0",
              "TestMBCBasaltShieldReprosLockstep", "TestStdPoolReprosLockstepFull"]
             + [tt.MANIFEST_TEST + str(i) for i in range(4)])
    selected = [n for n in names if re.search(plain_rx, n)]
    assert len(selected) == 7, selected
    weights = dict(zip(sorted(selected), [40.0, 40.0, 40.0, 30.0, 20.0, 20.0, 10.0]))
    for i in range(4):
        weights[tt.MANIFEST_TEST + str(i)] = [120.0, 120.0, 80.0, 80.0][i]  # x SLICE_SHARE = 30,30,20,20
    laned = tt.oracle_lanes(res, names=names, weights=weights, zero=set(), lanes=4, plain_lanes=4)
    plain = [rx for k, p, rx in laned if k == "oraclelane" and "/" not in rx]
    subs = [rx for k, p, rx in laned if k == "oraclelane" and "/" in rx]
    assert plain and subs

    def members(rx):
        top = rx if "/" not in rx else rx.split("/", 1)[0]
        return top[2:-2].split("|")

    def loads(rxs, share):
        return sorted(sum(weights[n] * share for n in members(rx)) for rx in rxs)

    def dealt(names_, share):
        flat = {n: weights[n] * share for n in names_}
        got = [0.0] * 4
        for part, _, secs in tt.oracle_shard_deal.deal(sorted(flat), flat, 4,
                                                       default=tt.oracle_shard_deal.UNLISTED_SECONDS):
            got[part] += secs
        return sorted(got)

    tops = [tt.MANIFEST_TEST + str(i) for i in range(4)]
    assert sorted(n for rx in plain for n in members(rx)) == sorted(selected)
    assert sorted(n for rx in subs for n in members(rx)) == sorted(tops)
    assert loads(plain, 1.0) == dealt(selected, 1.0), plain
    # the deal reports seconds to the millisecond (a 1/6 share is not exact)
    assert [round(x, 3) for x in loads(subs, tt.SLICE_SHARE)] == [round(x, 3) for x in dealt(tops, tt.SLICE_SHARE)], subs
    # and so no process carries over 40% of its own half's summed wall
    assert loads(plain, 1.0)[-1] <= 0.40 * sum(weights[n] for n in selected)
    assert loads(subs, tt.SLICE_SHARE)[-1] <= 0.40 * sum(weights[n] for n in tops) * tt.SLICE_SHARE
    # an unlisted test weighs oracle_shard_deal.UNLISTED_SECONDS, not the table median: one listed 1 s
    # test against seven unlisted ones (0.08 each) must leave the listed one alone in its process; at
    # the median (1.0 for a one-entry table) the deal would spread the eight evenly, 4 to a lane.
    few = ["TestListedCensus"] + ["TestUnlistedCensus%d" % i for i in range(7)]  # match the CENSUS arm
    laned = tt.oracle_lanes(planned(SRC, reprosel=sel(sha=sha_in_lane(1))), names=few,
                            weights={"TestListedCensus": 1.0}, zero=set(), lanes=2)
    lanes2 = [rx[2:-2].split("|") for k, p, rx in laned if k == "oraclelane" and "/" not in rx]
    assert sorted(n for l in lanes2 for n in l) == sorted(few)
    assert any(l == ["TestListedCensus"] for l in lanes2), lanes2


def test_lanes_deal_by_cost_and_leave_an_undealtable_slice_alone():
    res = planned(SRC, reprosel=sel(sha=sha_in_lane(1)))
    # two lanes, one heavy test: it and nothing else is alone in its lane
    names = ["TestHeavyCensus", "TestRefusedEntriesStillDiverge0"]
    weights = {"TestHeavyCensus": 100.0, "TestRefusedEntriesStillDiverge0": 1.0}
    laned = tt.oracle_lanes(res, names=names, weights=weights, zero=set(), lanes=2)
    lanes = [rx for k, p, rx in laned if p == "./dirac/oracle" and "/" not in rx]
    assert "TestHeavyCensus" in lanes[0] and "TestHeavyCensus" not in lanes[1]
    # no listing: the entries are left whole, never split away the slice
    assert tt.oracle_lanes(res, names=[], weights={}, zero=set()) == res
    # no replay entry in the selection: nothing is dealt
    assert tt.oracle_lanes([("named", "./dirac/oracle", tt.CENSUS)], names=names, weights={}, zero=set()) == \
        [("named", "./dirac/oracle", tt.CENSUS)]
