"""The corpus link a box slot holds (.cards, a symlink) is never a changed path."""

import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import test_targets as tt  # noqa: E402


def git(repo, *args):
    subprocess.run(["git", *args], cwd=repo, check=True, capture_output=True)


def test_an_untracked_cards_symlink_is_not_a_change(tmp_path, monkeypatch):
    repo = tmp_path / "repo"
    repo.mkdir()
    corpus = tmp_path / "corpus"
    corpus.mkdir()
    (corpus / "x.txt").write_text("card\n")
    git(repo, "init", "-q", "-b", "land")
    (repo / ".gitignore").write_text((HERE.parent / ".gitignore").read_text())
    git(repo, "add", ".gitignore")
    git(repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "x")
    (repo / ".cards").symlink_to(corpus, target_is_directory=True)
    (repo / "new.go").write_text("package m\n")
    # Precondition: the link is there, and untracked files are seen at all.
    assert (repo / ".cards").is_symlink() and (repo / ".cards").is_dir()
    monkeypatch.chdir(repo)
    changes = tt.diff_changes("land")
    added = tt.added_files("land")
    assert "new.go" in changes and "new.go" in added
    assert [p for p in changes if p.startswith(".cards")] == []
    assert [p for p in added if p.startswith(".cards")] == []
