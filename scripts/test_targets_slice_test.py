"""TF-01: the precheck's oracle replay arm (scripts/test_targets.py plan) replays the changed
recordings plus a 25% slice (scripts/test_targets_slice_test.py), not every recording; full coverage
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
# the four 25% lanes to partition them. One of each is over the slow wall.
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
        if zlib.crc32(sha.encode()) % 4 == lane:
            return sha
        k += 1


# ---- the 25% slice ----------------------------------------------------------------------------------

def test_slice_is_stable_for_a_sha_and_rotates_across_shas():
    one = planned(SRC, reprosel=sel(sha=sha_in_lane(1)))
    again = planned(SRC, reprosel=sel(sha=sha_in_lane(1)))
    assert manifest_names(one) == manifest_names(again)  # stable for a merge base
    lanes = [manifest_names(planned(SRC, reprosel=sel(sha=sha_in_lane(l)))) for l in range(4)]
    named = set(RECORDINGS) - {SLOW_REC}
    assert set().union(*lanes) == named  # every unchanged recording lands in exactly one lane
    assert sum(len(s) for s in lanes) == len(named)
    for i, a in enumerate(lanes):
        for b in lanes[i + 1:]:
            assert not (a & b), (a, b)
    # the same rotation on the oracle's other replay tests
    tl = [set(re.findall(r"Test[A-Za-z0-9_]+", oracle(planned(SRC, reprosel=sel(sha=sha_in_lane(l))))[1]))
          for l in range(4)]
    tl = [s & set(REPRO_TESTS) for s in tl]
    assert set().union(*tl) == set(REPRO_TESTS) - {SLOW_TEST}


def test_a_changed_recording_is_always_included():
    # rec outside the sha's lane and on the slow list: only the change itself puts it in the -run
    lane = (zlib.crc32(SLOW_REC.encode()) + 1) % 4
    res = planned(SRC, repro_names={"m/dirac/oracle": {MANIFEST + SLOW_REC}},
                  reprosel=sel(sha=sha_in_lane(lane)))
    assert SLOW_REC in manifest_names(res)
    # and a recording whose reader test changed too runs it as a test name, not the whole manifest
    changes = dict(SRC, **{"dirac/oracle/testdata/repro/%s.repro.json" % RECORDINGS[0]: None})
    res = planned(changes,
                  repro_names={"m/dirac/oracle": {MANIFEST + RECORDINGS[0], "TestRecordingReader"}},
                  reprosel=sel(sha=sha_in_lane((zlib.crc32(RECORDINGS[0].encode()) + 1) % 4)))
    kind, rx = oracle(res)
    assert re.search(rx, "TestRecordingReader")
    assert RECORDINGS[0] in manifest_names(res)
    assert "TestReproManifest" not in rx.split("|")[0]  # the parent test never runs whole for a subtest


def test_the_slow_list_is_excluded_unless_the_recording_changed():
    lane = zlib.crc32(SLOW_REC.encode()) % 4
    res = planned(SRC, reprosel=sel(sha=sha_in_lane(lane)))
    assert SLOW_REC not in manifest_names(res)  # over the 10 s wall: the batch replays it
    # the same for the oracle's other replay tests
    lane = zlib.crc32(SLOW_TEST.encode()) % 4
    kind, rx = oracle(planned(SRC, reprosel=sel(sha=sha_in_lane(lane))))
    assert not re.search(rx, SLOW_TEST)


def test_the_replay_arm_keeps_the_refusal_check_and_the_pool_globs_are_sliced():
    lane = zlib.crc32("TestStdFdnReprosLockstep".encode()) % 4
    kind, rx = oracle(planned(SRC, reprosel=sel(sha=sha_in_lane(lane))))
    assert re.search(rx, "TestRefusedEntriesStillDiverge")
    assert re.search(rx, "TestStdFdnReprosLockstep")  # the pool globs are in the slice population
    assert not re.search(rx, "TestShieldReplacementCollision")  # whole-game lockstep stays in the batch
    # out of the lane the glob is left to the batch with the rest
    kind, rx = oracle(planned(SRC, reprosel=sel(sha=sha_in_lane((lane + 1) % 4))))
    assert not re.search(rx, "TestStdFdnReprosLockstep")
    res = planned(SRC, reprosel=sel(sha=sha_in_lane(zlib.crc32(RECORDINGS[0].encode()) % 4)))
    mrx = manifest_rx(res)
    assert mrx.count("/") == 1 and mrx.startswith(MANIFEST)  # its -run matches the manifest test only
    assert not re.search(mrx, "TestRefusedEntriesStillDiverge")
    assert not re.search(mrx, "TestStdFdnReprosLockstep")


def test_a_pool_recording_replays_as_its_glob_tests_subtest():
    lane0 = zlib.crc32("fdn_in_lane".encode()) % 4
    out = next(n for n in ("fdn_second", "fdn_third", "fdn_fourth")
               if zlib.crc32(n.encode()) % 4 != lane0)
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
                                       if n not in slow and zlib.crc32(n.encode()) % 4 == lane0}
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
    for name in ("TestMBCAmberTombReprosLockstep", "TestRefusedEntriesStillDiverge"):
        assert re.search(rx, name), name
    assert manifest_rx(planned(SRC)) is None


def test_a_changed_recording_alone_replays_itself_and_the_slice_not_the_manifest():
    rec = RECORDINGS[0]
    res = planned({"dirac/oracle/testdata/repro/%s.repro.json" % rec: None},
                  repro_names={"m/dirac/oracle": {MANIFEST + rec}},
                  reprosel=sel(sha=sha_in_lane((zlib.crc32(rec.encode()) + 1) % 4)))
    kind, rx = oracle(res)
    assert rec in manifest_names(res)
    assert not re.search(rx, "TestReproManifest3")  # the manifest does not fold into the shared -run
    # a recording change replays the other replay tests' slice too (as the blanket did)
    assert any(re.search(rx, t) for t in REPRO_TESTS if t != SLOW_TEST)


def test_narrow_applies_the_slice_too():
    # test-smart's narrow step: an oracle fixture change runs the package whole in the plain plan, and
    # the narrow precheck turns that into the named parts -- sliced like test-targeted's
    res = planned({"dirac/oracle/testdata/x.txt": None}, narrow={"fast": ("TestFastA",), "sha": "0"},
                  reprosel=sel(sha=sha_in_lane(zlib.crc32(RECORDINGS[0].encode()) % 4)))
    kind, rx = oracle(res)
    assert kind == "named"
    assert tt.REPLAYS not in rx
    for part in ("^TestPinManifest$", "^(TestFastA)$", "Census"):
        assert part in rx, part
    assert manifest_names(res), "the narrow precheck replays the changed recordings and the slice"
