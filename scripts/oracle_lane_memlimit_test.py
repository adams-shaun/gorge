"""Each targeted oracle lane's GOMEMLIMIT (test_targets.py lane_memlimit): the lanes share the run's one scope
(MemoryMax 8G, GOMEMLIMIT 6GiB locally), so each takes its even share of the run's soft limit. With the run's whole
6GiB in each of 12 lanes no lane's GC pushed back and the scope was OOM-killed 44 s into the lanes (local run at
984093e993, 2026-11-27, `oom-kill` at an 8G memory peak). Run: python3 -m pytest scripts/oracle_lane_memlimit_test.py"""
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import test_targets as tt  # noqa: E402


def mib(s):
    m = re.fullmatch(r"([0-9]+)MiB", s)
    assert m, s
    return int(m.group(1))


def test_the_lanes_together_stay_within_the_run_limit():
    # precondition: the run's limit is what one lane alone would get, so the shares must sum to it, not 12x it
    assert mib(tt.lane_memlimit("6GiB", 1)) == 6 * 1024
    assert mib(tt.lane_memlimit("6GiB", 12)) * 12 <= 6 * 1024
    assert mib(tt.lane_memlimit("6GiB", 12)) == 512
    assert mib(tt.lane_memlimit("3072MiB", 4)) == 768


def test_a_lane_keeps_a_floor_and_an_unreadable_limit_passes_through():
    assert mib(tt.lane_memlimit("1GiB", 64)) == tt.LANE_MEM_FLOOR_MIB
    # a limit the helper cannot read (off, bytes) is the run's own: the lanes keep it as before
    assert tt.lane_memlimit("off", 12) == "off"
    assert tt.lane_memlimit("", 12) == ""


def test_the_shell_gives_each_lane_its_share(capsys):
    assert tt.main(["lanemem", "--limit", "6GiB", "--lanes", "12"]) == 0
    assert capsys.readouterr().out == "512MiB\n"
    sh = (HERE / "test-targets.sh").read_text()
    assert re.search(r'lane_mem=\$\("\$\{py\[@\]\}" lanemem --limit "\$\{GOMEMLIMIT:-\}" '
                     r'--lanes "\$\{#lanes\[@\]\}"\)', sh), "test-targets.sh computes the lane GOMEMLIMIT itself"
    assert 'GOMEMLIMIT=$lane_mem GOMAXPROCS=$lane_procs go test' in sh
