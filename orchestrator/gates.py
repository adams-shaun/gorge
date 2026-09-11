"""The deterministic gate runner. Every function here is a literal
subprocess call whose exit code is ground truth -- no model's opinion, local
or paid, ever substitutes for one of these. This is the one module in the
package that must never call a seat.
"""

from __future__ import annotations

import subprocess
from dataclasses import dataclass
from pathlib import Path

from . import config


@dataclass
class GateResult:
    name: str
    ok: bool
    output: str


def _run(name: str, cmd: list[str], cwd: Path, env: dict | None = None, timeout: int = 900) -> GateResult:
    try:
        p = subprocess.run(
            cmd, cwd=str(cwd), env=env, capture_output=True, text=True, timeout=timeout,
        )
        out = (p.stdout or "") + (p.stderr or "")
        return GateResult(name, p.returncode == 0, out[-20000:])  # keep the tail; a runaway log must not blow memory
    except subprocess.TimeoutExpired as e:
        return GateResult(name, False, f"TIMEOUT after {timeout}s\n{e.stdout or ''}{e.stderr or ''}")


def go_build(worktree: Path) -> GateResult:
    return _run("go build", ["go", "build", "./..."], worktree)


def go_test_core(worktree: Path) -> GateResult:
    return _run("go test (core)", ["go", "test", "-count=1", *config.GO_TEST_PACKAGES], worktree, timeout=300)


def cr_conformance(worktree: Path) -> GateResult:
    import os
    env = dict(os.environ)
    env.update(config.CR_CONFORMANCE_ENV)
    r = _run(
        "CR conformance",
        ["go", "test", "-p=2", "-count=1", "./rules", "-run", "TestCR", "-v"],
        worktree, env=env, timeout=300,
    )
    r.ok = r.ok and "FAIL" not in _fail_lines(r.output)
    return r


def _fail_lines(output: str) -> str:
    return "\n".join(l for l in output.splitlines() if l.startswith("--- FAIL") or l == "FAIL")


def test_heads(worktree: Path) -> GateResult:
    return _run("TestHeads", ["go", "test", "./rules", "-run", "TestHeads", "-v"], worktree, timeout=120)


def make_sim(worktree: Path) -> GateResult:
    r = _run("make sim", ["make", "sim"], worktree, timeout=600)
    total = r.output.count("seed ")
    ok_count = r.output.count("replay OK")
    r.ok = r.ok and total > 0 and total == ok_count
    return r


def web_check(worktree: Path) -> GateResult:
    return _run("npm run check", ["npm", "run", "check"], worktree / "web", timeout=300)


def web_test(worktree: Path) -> GateResult:
    return _run("npm test", ["npm", "test"], worktree / "web", timeout=300)


def web_build(worktree: Path) -> GateResult:
    return _run("npm run build", ["npm", "run", "build"], worktree / "web", timeout=300)


def touches_web(worktree: Path, base: str = "main") -> bool:
    r = subprocess.run(
        ["git", "diff", "--name-only", f"{base}...HEAD"],
        cwd=str(worktree), capture_output=True, text=True,
    )
    return any(f.startswith("web/") for f in r.stdout.splitlines())


def run_all(worktree: Path) -> tuple[bool, list[GateResult]]:
    """The full gate sequence a task must clear before it can merge. Stops
    at the first failure -- there is no point burning a `make sim` run on a
    worktree that doesn't even build."""
    results: list[GateResult] = []

    def step(fn) -> bool:
        r = fn(worktree)
        results.append(r)
        return r.ok

    if not step(go_build):
        return False, results
    if not step(go_test_core):
        return False, results
    if not step(cr_conformance):
        return False, results
    heads = test_heads(worktree)
    results.append(heads)
    if not heads.ok:
        # Not an immediate failure: the caller (daemon.py) applies the
        # auto-accept-on-clean-CR-and-sim policy before deciding.
        pass
    if not step(make_sim):
        return False, results
    if touches_web(worktree):
        if not step(web_check):
            return False, results
        if not step(web_test):
            return False, results
        if not step(web_build):
            return False, results
    return True, results
