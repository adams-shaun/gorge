"""The orchestrator daemon must be single-instance, whoever launches it.

On 2026-09-14 five copies of scripts/start-autonomous.sh were running at once
(one from a login shell, four from Codex sandboxes in their own PID
namespaces). Each judged the daemon dead from a PID it could not see and
started another, until 49 daemons were dispatching and merging from the same
state -- two of them raced one worktree removal into a FileNotFoundError.
The PID file cannot prevent that; a kernel file lock can, because it is held on
the file itself and is visible across PID namespaces.
"""
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from orchestrator import daemon

PROBE = (
    "import sys; from orchestrator import daemon; "
    "sys.exit(0 if daemon.acquire_single_instance_lock(sys.argv[1]) else 3)"
)


def probe(lock_path: Path) -> int:
    """Try to take the lock from a separate process, as a second daemon would."""
    return subprocess.run([sys.executable, "-c", PROBE, str(lock_path)],
                          cwd=Path(__file__).resolve().parent.parent).returncode


class SingleInstanceLockTestCase(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.lock = Path(self.dir.name) / "daemon.lock"

    def tearDown(self):
        self.dir.cleanup()

    def test_the_first_daemon_gets_the_lock(self):
        self.assertIsNotNone(daemon.acquire_single_instance_lock(self.lock))

    def test_a_second_daemon_is_refused_while_the_first_holds_it(self):
        held = daemon.acquire_single_instance_lock(self.lock)
        self.assertIsNotNone(held)
        self.assertEqual(probe(self.lock), 3)

    def test_the_lock_is_free_again_once_its_holder_releases_it(self):
        held = daemon.acquire_single_instance_lock(self.lock)
        held.close()
        self.assertEqual(probe(self.lock), 0)


if __name__ == "__main__":
    unittest.main()
