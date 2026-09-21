"""The local-seat cap counts only seats working in THIS repo.

pi.running_names used to count every live pi-agent on the box, so another repo's
fleet sharing the same local model (bifrost's impl-m* seats) consumed gorge's
local cap. With repo= given, only processes whose --cwd or working directory is
inside that repo count.
"""
import os
import tempfile
import unittest
from pathlib import Path

from orchestrator import pi


def fake_proc(root: Path, pid: int, argv: list[str], cwd: Path) -> None:
    d = root / str(pid)
    d.mkdir(parents=True)
    (d / "cmdline").write_bytes(b"\0".join(a.encode() for a in argv) + b"\0")
    os.symlink(cwd, d / "cwd")


class RunningNamesRepoTestCase(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        base = Path(self.tmp.name)
        self.proc = base / "proc"
        self.gorge = base / "gorge"
        self.other = base / "bifrost"
        for p in (self.gorge / ".worktrees" / "a", self.gorge / ".worktrees" / "b", self.other / ".worktrees" / "m1"):
            p.mkdir(parents=True)
        m = "glm-5.3-flash"
        # a gorge seat identified by its --cwd argument (the pi-agent wrapper)
        fake_proc(self.proc, 101, ["bash", "pi-agent", "--cwd", str(self.gorge / ".worktrees" / "a"),
                                   "--model", m, "--name", "impl-a-r1"], Path("/"))
        # a gorge seat identified only by its working directory (the pi child)
        fake_proc(self.proc, 102, ["pi", "--model", m, "--name", "impl-b-r1"], self.gorge / ".worktrees" / "b")
        # another repo's seat on the same model
        fake_proc(self.proc, 201, ["bash", "pi-agent", "--cwd", str(self.other / ".worktrees" / "m1"),
                                   "--model", m, "--name", "impl-m1"], self.other / ".worktrees" / "m1")
        (self.proc / "self").mkdir()  # non-numeric entries are skipped

    def tearDown(self):
        self.tmp.cleanup()

    def test_without_repo_every_seat_on_the_box_counts(self):
        self.assertEqual(pi.running_names("glm-5.3-flash", proc=self.proc),
                         {"impl-a-r1", "impl-b-r1", "impl-m1"})

    def test_with_repo_only_that_repos_seats_count(self):
        self.assertEqual(pi.running_names("glm-5.3-flash", repo=self.gorge, proc=self.proc),
                         {"impl-a-r1", "impl-b-r1"})

    def test_a_sibling_directory_with_a_shared_prefix_is_not_the_repo(self):
        prefixed = Path(str(self.gorge) + "-old") / ".worktrees" / "z"
        prefixed.mkdir(parents=True)
        fake_proc(self.proc, 301, ["pi", "--model", "glm-5.3-flash", "--name", "impl-z"], prefixed)
        self.assertNotIn("impl-z", pi.running_names("glm-5.3-flash", repo=self.gorge, proc=self.proc))


if __name__ == "__main__":
    unittest.main()
