#!/usr/bin/env python3
"""Tests for scripts/dirac-instr-batch.sh, the merge-queue batch's EDH4 instruction check. Stdlib only:

    python3 scripts/dirac_instr_batch_test.py

The script runs in a scratch copy of scripts/ with stub `go`, `perf` and `taskset` on PATH: the stub
`go test -c` writes a stub test binary, and the stub perf counts FIXED + PER_OP * N instructions for
`-test.benchtime Nx`, so the script's per-op value is exactly PER_OP.
"""

import os
import shutil
import stat
import subprocess
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "dirac-instr-batch.sh")
BENCH = "BenchmarkTransitionEDH4"
BASE = 100000

STUB_GO = """#!/usr/bin/env bash
# go test -c -o <out> ./dirac/ : writes the stub bench binary.
out=
while [ $# -gt 0 ]; do [ "$1" = -o ] && out=$2; shift; done
[ -n "$out" ] || exit 1
[ "${STUB_BUILD_FAIL:-}" = 1 ] && { echo "build failed" >&2; exit 1; }
cat >"$out" <<'EOF'
#!/usr/bin/env bash
# Like the real binary: without -test.v a skipped bench and a bench that matches nothing both
# print only PASS.
v=0
for a in "$@"; do [ "$a" = -test.v ] && v=1; done
case ${STUB_BENCH:-ok} in
skip) if [ $v = 1 ]; then echo "BenchmarkTransitionEDH4"; echo "    benchedh_test.go:91: no edh table (make dirac-tables): no tables"; echo "--- SKIP: BenchmarkTransitionEDH4"; fi; echo PASS ;;
panic) echo "panic: autopay found no plan" >&2; exit 2 ;;
norun) echo PASS ;;
*) echo "BenchmarkTransitionEDH4 	   20000	     12345 ns/op"; echo PASS ;;
esac
EOF
chmod +x "$out"
"""

STUB_PERF = """#!/usr/bin/env bash
# perf stat -x, -e instructions:u <cmd...>: runs cmd, prints FIXED + PER_OP * N to stderr.
[ "${STUB_NO_PERF:-}" = 1 ] && { echo "perf: not supported" >&2; exit 1; }
shift 4
n=0 prev=
for a in "$@"; do [ "$prev" = -test.benchtime ] && n=${a%x}; prev=$a; done
"$@"; rc=$?
count=$((${STUB_FIXED:-5000000} + ${STUB_PER_OP:-0} * n))
[ "${STUB_NOT_COUNTED:-}" = 1 ] && [ "$n" != 0 ] && count="<not counted>"
echo "$count,,instructions:u,1000,100.00,," >&2
exit $rc
"""

STUB_TASKSET = """#!/usr/bin/env bash
shift 2
exec "$@"
"""


class Batch(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="instr-batch-")
        os.makedirs(os.path.join(self.tmp, "scripts"))
        os.makedirs(os.path.join(self.tmp, "dirac"))
        os.makedirs(os.path.join(self.tmp, "bin"))
        shutil.copy(SCRIPT, os.path.join(self.tmp, "scripts"))
        for name, body in (("go", STUB_GO), ("perf", STUB_PERF), ("taskset", STUB_TASKSET)):
            p = os.path.join(self.tmp, "bin", name)
            with open(p, "w") as f:
                f.write(body)
            os.chmod(p, os.stat(p).st_mode | stat.S_IXUSR)
        self.write_base("%s\t%d\tdeadbeef\n" % (BENCH, BASE))

    def tearDown(self):
        shutil.rmtree(self.tmp)

    def write_base(self, text):
        with open(os.path.join(self.tmp, "scripts", "dirac-instr-base.tsv"), "w") as f:
            f.write(text)

    def run_batch(self, per_op=BASE, enforce=None, **stub):
        env = {k: v for k, v in os.environ.items() if not k.startswith(("STUB_", "INSTR_"))}
        env["PATH"] = os.path.join(self.tmp, "bin") + os.pathsep + env["PATH"]
        env["STUB_PER_OP"] = str(per_op)
        env.update({"STUB_" + k.upper(): v for k, v in stub.items()})
        if enforce is not None:
            env["INSTR_EDH4_ENFORCE"] = enforce
        p = subprocess.run(
            ["bash", os.path.join(self.tmp, "scripts", "dirac-instr-batch.sh")],
            cwd=self.tmp, env=env, capture_output=True, text=True, check=False,
        )
        return p.returncode, p.stdout + p.stderr

    def line(self, out):
        lines = [ln for ln in out.splitlines() if ln.startswith(BENCH + ":")]
        self.assertEqual(len(lines), 1, out)
        return lines[0]


class TestBatchReportsOnly(Batch):
    def test_three_percent_over_reports_without_the_variable(self):
        rc, out = self.run_batch(per_op=103000)
        self.assertEqual(rc, 0, out)
        self.assertEqual(self.line(out), "%s: head 103000 instr/op, base %d (max 1.02x), enforce=0" % (BENCH, BASE))

    def test_any_value_but_one_does_not_enforce(self):
        rc, out = self.run_batch(per_op=103000, enforce="0")
        self.assertEqual(rc, 0, out)
        self.assertIn("enforce=0", self.line(out))

    def test_infrastructure_reports_without_the_variable(self):
        rc, out = self.run_batch(no_perf="1")
        self.assertEqual(rc, 0, out)
        self.assertIn("perf", out)


class TestBatchEnforces(Batch):
    def test_three_percent_over_fails(self):
        rc, out = self.run_batch(per_op=103000, enforce="1")
        self.assertEqual(rc, 1, out)
        self.assertEqual(self.line(out), "%s: head 103000 instr/op, base %d (max 1.02x), enforce=1" % (BENCH, BASE))

    def test_within_the_band_passes(self):
        rc, out = self.run_batch(per_op=101900, enforce="1")
        self.assertEqual(rc, 0, out)
        self.assertIn("head 101900 ", self.line(out))

    def test_a_lower_count_passes(self):
        rc, out = self.run_batch(per_op=90000, enforce="1")
        self.assertEqual(rc, 0, out)
        self.assertIn("head 90000 ", self.line(out))


class TestBatchInfrastructure(Batch):
    def test_missing_base_row(self):
        self.write_base("BenchmarkTransitionFDN\t64068\tdeadbeef\n")
        rc, out = self.run_batch(enforce="1")
        self.assertEqual(rc, 2, out)
        self.assertIn("no %s row" % BENCH, out)

    def test_no_perf(self):
        rc, out = self.run_batch(enforce="1", no_perf="1")
        self.assertEqual(rc, 2, out)
        self.assertIn("perf", out)

    def test_no_edh_table(self):
        rc, out = self.run_batch(enforce="1", bench="skip")
        self.assertEqual(rc, 2, out)
        self.assertIn("no edh table", out)

    def test_bench_crashed(self):
        rc, out = self.run_batch(enforce="1", bench="panic")
        self.assertEqual(rc, 2, out)
        self.assertIn("crashed", out)

    def test_bench_not_run(self):
        # A renamed or dropped bench: the binary matches nothing and exits 0, and perf still
        # counts the process start-up.
        rc, out = self.run_batch(per_op=0, enforce="1", bench="norun")
        self.assertEqual(rc, 2, out)
        self.assertIn("not measured: no result line at 20000x: bench renamed or dropped? (exit 0)", out)
        self.assertNotIn("instr/op", out)

    def test_perf_gave_no_count(self):
        # The bench completed (its result line is there) but perf did not count it.
        rc, out = self.run_batch(enforce="1", not_counted="1")
        self.assertEqual(rc, 2, out)
        self.assertIn("not measured: perf gave no instructions:u count at 20000x (exit 0)", out)
        self.assertNotIn("renamed", out)
        self.assertNotIn("instr/op", out)

    def test_build_failed(self):
        rc, out = self.run_batch(enforce="1", build_fail="1")
        self.assertEqual(rc, 2, out)
        self.assertIn("does not build", out)


if __name__ == "__main__":
    unittest.main()
