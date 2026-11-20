"""Tests for scripts/bench_tests.py (the parser/measurement core of scripts/bench-tests.sh) and the
shard split the shell uses. Run: python3 -m pytest scripts/bench_tests_test.py"""
import json
import sys
from pathlib import Path

import pytest

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import bench_tests as bt  # noqa: E402


def ev(action, pkg="m/p", test="", out="", elapsed=None):
    d = {"Action": action, "Package": pkg}
    if test:
        d["Test"] = test
    if out:
        d["Output"] = out
    if elapsed is not None:
        d["Elapsed"] = elapsed
    return json.dumps(d)


def test_cpu_stat_reads_usage_usec_without_floating_point_drift():
    assert bt.parse_cpu_stat("usage_usec 1500000\nuser_usec 900000\nsystem_usec 600000\n") == 1.5
    assert bt.parse_cpu_stat("usage_usec 1\n") == 1e-6
    with pytest.raises(ValueError):
        bt.parse_cpu_stat("user_usec 1\n")


def test_memory_peak_is_bytes_and_megabytes():
    assert bt.parse_memory_peak("1048576\n") == 1048576
    assert bt.peak_mb(1048576) == 1.0
    assert bt.peak_mb(4294967296) == 4096.0
    with pytest.raises(ValueError):
        bt.parse_memory_peak("")
    with pytest.raises(ValueError):
        bt.parse_memory_peak("not a number\n")


def test_canned_json_yields_a_pkg_row_test_rows_and_the_test_budget_over_row():
    stream = [
        "not json: go's build noise",
        ev("run", test="TestSlow"),
        ev("pass", test="TestSlow", elapsed=75.25),
        ev("run", test="TestFast"),
        ev("pass", test="TestFast", elapsed=0.5),
        ev("run", test="TestSlow/sub"),
        ev("pass", test="TestSlow/sub", elapsed=74.0),
        ev("pass", elapsed=80.1),
    ]
    s = bt.summarize("m/p", stream, "usage_usec 2000000\n", "1048576\n", 80.1, 0)
    rows = [r.split("\t") for r in s.rows()]
    pkg = [r for r in rows if r[0] == "pkg"]
    assert len(pkg) == 1
    assert pkg[0][1:3] == ["m/p", "-"]
    assert pkg[0][4] == "80.1"       # wall_s
    assert pkg[0][5] == "2.00"       # cpu_s from cpu.stat
    assert pkg[0][6] == "1.0"        # peak_rss_mb
    assert pkg[0][7] == "2"          # top-level tests only, not the subtest
    assert pkg[0][8] == "0"          # -count=1, so never cached
    tests = {r[2]: r for r in rows if r[0] == "test"}
    assert set(tests) == {"TestSlow", "TestFast"}
    assert tests["TestSlow"][3] == "75.25"
    over = [r for r in rows if r[0] == "over"]
    assert any(r[2] == "TestSlow" and r[9] == "test_over_60s" for r in over)
    assert not any(r[2] == "TestSlow/sub" for r in over)
    assert s.exit_code() == 0  # a slow test is reported, not a failed run


def test_a_package_over_4gib_or_a_timeout_is_an_over_row_and_exit_5():
    stream = [ev("run", test="TestA"), ev("pass", test="TestA", elapsed=0.1), ev("pass", elapsed=0.2)]
    over = bt.summarize("m/big", stream, "usage_usec 1000\n", str(4 * 1024**3 + 1), 0.2, 0)
    assert any(r.split("\t")[0] == "over" and r.split("\t")[9] == "rss_over_4g" for r in over.rows())
    assert over.exit_code() == 5
    timed = bt.summarize("m/t", [ev("output", out="panic: test timed out after 15m0s\n")], "", "", 900.0, 1)
    assert any(r.split("\t")[0] == "over" and r.split("\t")[9] == "timeout" for r in timed.rows())
    assert timed.exit_code() == 5


def test_a_missing_corpus_skip_is_env_via_test_targets_rule_and_a_failure_wins():
    skip = [ev("run", test="TestCorpus"), ev("output", test="TestCorpus", out="    x_test.go:3: no .cards corpus\n"),
            ev("skip", test="TestCorpus")]
    s = bt.summarize("m/p", skip, "", "", 0.1, 0)
    assert s.env and s.exit_code() == 3
    other = [ev("run", test="TestSkip"), ev("output", test="TestSkip", out="    x_test.go:3: needs linux\n"),
             ev("skip", test="TestSkip")]
    assert not bt.summarize("m/p", other, "", "", 0.1, 0).env
    failed = skip + [ev("run", test="TestBad"), ev("output", test="TestBad", out="    x_test.go:9: boom\n"),
                     ev("fail", test="TestBad", elapsed=0.2), ev("fail")]
    assert bt.summarize("m/p", failed, "", "", 0.2, 1).exit_code() == 1


def test_shards_partition_every_package_exactly_once():
    pkgs = ["m/f", "m/b", "m/a", "m/e", "m/c", "m/d", "m/g", "m/h", "m/i", "m/j"]
    got = [set(bt.shard(pkgs, i, 3)) for i in (1, 2, 3)]
    assert set().union(*got) == set(pkgs)
    assert sum(len(s) for s in got) == len(pkgs)
    assert max(map(len, got)) - min(map(len, got)) <= 1
    assert bt.shard(pkgs, 1, 3) == ["m/a", "m/d", "m/g", "m/j"]
    assert bt.parse_shard("2/3") == (2, 3)
    for bad in ("0/3", "4/3", "1/0", "1", "a/b", ""):
        with pytest.raises(ValueError):
            bt.parse_shard(bad)


def test_parse_go_list_keeps_only_packages_with_tests():
    text = "m/a\n\nm/b\nm/c\n"
    assert bt.parse_go_list(text) == ["m/a", "m/b", "m/c"]
    assert bt.parse_go_list("") == []


def test_summarize_cli_prints_header_rows_and_exits_with_the_package_status(tmp_path):
    import subprocess
    j = tmp_path / "t.json"
    j.write_text("\n".join([ev("run", test="TestA"), ev("pass", test="TestA", elapsed=0.1), ev("pass", elapsed=0.2)]))
    cpu = tmp_path / "cpu.stat"
    cpu.write_text("usage_usec 500000\n")
    mem = tmp_path / "memory.peak"
    mem.write_text("2097152\n")
    r = subprocess.run([sys.executable, "-I", str(HERE / "bench_tests.py"), "summarize", "--pkg", "m/p",
                        "--json", str(j), "--cpu-stat", str(cpu), "--memory-peak", str(mem),
                        "--wall-ns", "200000000", "--rc", "0"], capture_output=True, text=True)
    assert r.returncode == 0, r.stderr
    lines = r.stdout.splitlines()
    assert lines[0].split("\t") == bt.HEADER
    assert lines[1].split("\t")[:2] == ["pkg", "m/p"]
    assert lines[1].split("\t")[4:7] == ["0.2", "0.50", "2.0"]


def test_shell_runs_each_package_in_one_4g_scope_with_the_cost_flags(tmp_path):
    """The shell contract the Python parser cannot see: one systemd scope per package, MemoryMax=4G,
    GOMAXPROCS=4, GOFLAGS='-p=1 -trimpath', and go test -count=1 -json -timeout 15m. Fake systemd-run and
    go make it a sub-second test that still fails if any of those is dropped."""
    import os
    import subprocess

    bindir = tmp_path / "bin"
    bindir.mkdir()
    go = bindir / "go"
    go.write_text("""#!/usr/bin/env bash
[ "${1:-}" = test ] || { echo "unexpected go $*" >&2; exit 99; }
printf '%s\\n' "$@" >"$FAKE_GO_ARGS"
printf 'GOMAXPROCS=%s\\nGOFLAGS=%s\\n' "$GOMAXPROCS" "$GOFLAGS" >"$FAKE_GO_ENV"
printf '%s\\n' \\
  '{"Action":"run","Package":"m/fake","Test":"TestFake"}' \\
  '{"Action":"pass","Package":"m/fake","Test":"TestFake","Elapsed":0.01}' \\
  '{"Action":"pass","Package":"m/fake","Elapsed":0.02}'
""")
    go.chmod(0o755)
    sd = bindir / "systemd-run"
    sd.write_text("""#!/usr/bin/env python3
import os
import sys
args = sys.argv[1:]
with open(os.environ["FAKE_SD_ARGS"], "a") as f:
    f.write(" ".join(args) + "\\n")
i = 0
while i < len(args):
    a = args[i]
    if a in ("--user", "--scope", "-q"):
        i += 1
    elif a in ("-p", "--property"):
        i += 2
    elif a.startswith("-p") or a.startswith("--property="):
        i += 1
    else:
        break
os.execvp(args[i], args[i:])
""")
    sd.chmod(0o755)
    env = dict(os.environ, PATH="%s:/usr/local/go/bin:/usr/bin:/bin" % bindir, PKGS="./fake",
               BENCH_LOG_DIR=str(tmp_path / "logs"), FAKE_GO_ARGS=str(tmp_path / "go.args"),
               FAKE_GO_ENV=str(tmp_path / "go.env"), FAKE_SD_ARGS=str(tmp_path / "sd.args"))
    r = subprocess.run(["bash", str(HERE / "bench-tests.sh")], cwd=HERE.parent, env=env,
                       capture_output=True, text=True)
    assert r.returncode == 0, r.stderr
    lines = r.stdout.splitlines()
    assert lines[0].split("\t") == bt.HEADER
    assert any(line.startswith("pkg\t./fake\t") for line in lines)
    assert any(line.startswith("test\t./fake\tTestFake\t") for line in lines)
    args = (tmp_path / "go.args").read_text().splitlines()
    for want in ("-count=1", "-json", "-timeout", "15m"):
        assert want in args, (want, args)
    assert "GOMAXPROCS=4" in (tmp_path / "go.env").read_text().splitlines()
    assert "GOFLAGS=-p=1 -trimpath" in (tmp_path / "go.env").read_text().splitlines()
    assert any("MemoryMax=4G" in line for line in (tmp_path / "sd.args").read_text().splitlines())
    assert (tmp_path / "logs" / "fake.json").exists()
