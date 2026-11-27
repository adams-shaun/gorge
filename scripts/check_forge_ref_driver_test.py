"""Tests for scripts/check-forge-ref-driver.sh's unreadable-base diagnostic.

The regression it guards (11-26 and 11-27): when the checkout cannot read <base>:Makefile, `old` comes
back empty, a FORGE_REF that has not moved reads as moved, and the step died on the `gh` check. Two PRs
(#4816, #5621) that do not touch FORGE_REF were reddened and blamed on a missing gh. An unreadable base
must now die with the true cause, still exit 2, and never print "gh is needed".

Run: python3 -m pytest scripts/check_forge_ref_driver_test.py
"""

import subprocess

from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "check-forge-ref-driver.sh"
PIN = "0123456789abcdef0123456789abcdef01234567"
MISSING = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"


def git(repo, *args):
    return subprocess.run(
        ["git", *args], cwd=repo, check=True, capture_output=True, text=True
    )


def make_repo(tmp_path):
    """A repo with a committed Makefile, plus an origin that does not carry MISSING."""
    origin = tmp_path / "origin.git"
    git(tmp_path, "init", "-q", "--bare", str(origin))
    repo = tmp_path / "repo"
    repo.mkdir()
    git(repo, "init", "-q")
    git(repo, "config", "user.email", "t@example.com")
    git(repo, "config", "user.name", "t")
    (repo / "Makefile").write_text(f"FORGE_REF ?= {PIN}\n")
    git(repo, "add", "Makefile")
    git(repo, "commit", "-q", "-m", "init")
    git(repo, "remote", "add", "origin", str(origin))
    git(repo, "push", "-q", "origin", "HEAD:refs/heads/main")
    return repo


def run(repo, base):
    return subprocess.run(
        ["bash", str(SCRIPT), base], cwd=repo, capture_output=True, text=True
    )


def test_unreadable_base_names_the_base_and_exits_2(tmp_path):
    repo = make_repo(tmp_path)
    r = run(repo, MISSING)
    out = r.stdout + r.stderr
    assert r.returncode == 2, r
    assert "cannot read Makefile at base" in r.stderr, r
    assert MISSING in r.stderr, r
    assert "refusing to guess whether FORGE_REF moved" in r.stderr, r
    assert "gh is needed" not in out, r


def test_unreadable_base_does_not_take_the_unchanged_shortcut(tmp_path):
    # A full-sha base that cannot resolve must not be confused with "FORGE_REF unchanged".
    repo = make_repo(tmp_path)
    r = run(repo, "0" * 40)
    assert r.returncode == 2, r
    assert "FORGE_REF unchanged" not in r.stdout, r
    assert "cannot read Makefile at base" in r.stderr, r


def test_readable_base_with_the_same_pin_still_exits_0(tmp_path):
    # Control: the pass path is untouched and still needs no gh.
    repo = make_repo(tmp_path)
    head = git(repo, "rev-parse", "HEAD").stdout.strip()
    r = run(repo, head)
    assert r.returncode == 0, r
    assert f"FORGE_REF unchanged ({PIN[:10]})" in r.stdout, r
    assert "cannot read Makefile at base" not in r.stderr, r
