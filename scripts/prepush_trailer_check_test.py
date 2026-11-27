"""Tests for scripts/prepush-trailer-check.sh, the pre-push hook's trailer scan.

The regression it guards (11-27): the hook scanned $REMOTE_SHA..$LOCAL_SHA, so a REBASE push of an
existing branch also scanned every commit main had gained since the old head. Two already-merged
commits in main carry a Co-Authored-By line, so both rebased branches (#5602, #5609) were refused and
their tickets went human_needed. A push must be judged on the commits it ADDS.
"""

import os
import subprocess

HELPER = os.path.join(os.path.dirname(os.path.abspath(__file__)), "prepush-trailer-check.sh")


def git(repo, *args, **kwargs):
    return subprocess.run(
        ["git", *args], cwd=repo, check=True, capture_output=True, text=True, **kwargs
    )


def commit(repo, name, body=""):
    path = os.path.join(repo, name)
    with open(path, "w") as f:
        f.write(body or name)
    git(repo, "add", name)
    git(repo, "commit", "-q", "-m", f"add {name}\n\n{body}" if body else f"add {name}")
    return git(repo, "rev-parse", "HEAD").stdout.strip()


def run(repo, sha):
    return subprocess.run(
        ["sh", HELPER, sha], cwd=repo, capture_output=True, text=True
    )


def test_current_commit_is_clean(tmp_path):
    repo = str(tmp_path)
    git(repo, "init", "-q")
    git(repo, "config", "user.email", "t@example.com")
    git(repo, "config", "user.name", "t")
    commit(repo, "a.txt")
    assert run(repo, "HEAD").returncode == 0


def test_a_new_commit_carrying_a_trailer_is_refused(tmp_path):
    repo = str(tmp_path)
    git(repo, "init", "-q")
    git(repo, "config", "user.email", "t@example.com")
    git(repo, "config", "user.name", "t")
    commit(repo, "a.txt")
    commit(repo, "b.txt", "Co-Authored-By: someone <n@example.com>")
    res = run(repo, "HEAD")
    assert res.returncode == 1
    assert "b.txt" in res.stdout


def test_a_rebase_over_a_trailered_origin_commit_is_allowed(tmp_path):
    """The 11-27 regression: main itself holds a trailer; a rebased branch must still push."""
    origin = str(tmp_path / "origin.git")
    repo = str(tmp_path / "work")
    subprocess.run(["git", "init", "-q", "--bare", origin], check=True)
    subprocess.run(["git", "clone", "-q", origin, repo], check=True)
    git(repo, "config", "user.email", "t@example.com")
    git(repo, "config", "user.name", "t")
    git(repo, "checkout", "-q", "-b", "main")
    commit(repo, "a.txt")
    git(repo, "push", "-q", "origin", "main")
    # A branch parked at this head, later rebased onto main.
    git(repo, "checkout", "-q", "-b", "feature")
    old_head = commit(repo, "feature.txt")
    git(repo, "checkout", "-q", "main")
    # main gains a commit carrying the forbidden trailer and is pushed.
    commit(repo, "m.txt", "Co-Authored-By: someone <n@example.com>")
    git(repo, "push", "-q", "origin", "main")
    # Rebase the branch onto main: the push adds only feature.txt, cleanly.
    git(repo, "checkout", "-q", "feature")
    git(repo, "rebase", "-q", "main")
    new_head = git(repo, "rev-parse", "HEAD").stdout.strip()
    assert new_head != old_head
    # The old behaviour scanned old_head..new_head, which holds main's trailered commit.
    dirty_range = git(repo, "log", "--format=%h %s", f"{old_head}..{new_head}").stdout
    assert "m.txt" in dirty_range
    # What the push actually adds is clean.
    assert run(repo, new_head).returncode == 0
