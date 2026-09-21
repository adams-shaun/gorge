"""A redispatch must not poll a FINISHED round just because its name is reused.

On 2026-09-14 duplicate daemons raced and saved stale copies of five issues,
rolling their round counters back. The next escalation recomputed tag `sol1`,
found round sol1's launch marker from the real first round, and -- treating
that as a launch still starting up -- polled the old, finished status file
instead of running a new round, so the issues never advanced.

An unfinished marker must still block: that is the original guard against
relaunching a seat under the same name while its first launch is starting.
"""
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from orchestrator import config, daemon, issues, pi, seats


class StaleMarkerTestCase(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        root = Path(self.tmp.name)
        self.issue = issues.Issue(id="x", title="t", source="inbox", status="waiting",
                                  seat_kind="overflow", local_rounds=2, worktree="wt")
        patches = [
            mock.patch.object(config, "REPO", root),
            mock.patch.object(config, "ORCH_STATE_DIR", root / "state"),
            mock.patch.object(issues.Issue, "save"),
            mock.patch.object(daemon, "_slot_free", return_value=True),
        ]
        for p in patches:
            p.start()
            self.addCleanup(p.stop)
        self.launch = mock.patch.object(seats, "launch_implementer").start()
        self.addCleanup(mock.patch.stopall)
        self.name = seats.implementer_name("x", "sol1")
        self.marker = pi.launch_log_path(self.name)
        self.marker.parent.mkdir(parents=True)
        self.marker.write_text("launched\n")
        self.status = seats.implementer_status_path(root / "wt", "sol1")
        self.status.parent.mkdir(parents=True)

    def tearDown(self):
        self.tmp.cleanup()

    def test_a_finished_round_holding_the_name_is_set_aside_and_relaunched(self):
        self.status.write_text(json.dumps({"status": "DONE"}))
        daemon._redispatch_implementer(self.issue, "findings")
        self.launch.assert_called_once()
        self.assertEqual(self.launch.call_args.args[2], "sol1")
        self.assertFalse(self.status.exists())
        self.assertTrue(self.status.with_name("status-sol1.cutoff.json").exists())
        self.assertEqual(self.issue.status, "dispatched")

    def test_an_unfinished_marker_still_blocks_a_same_name_launch(self):
        daemon._redispatch_implementer(self.issue, "findings")  # no status file yet
        self.launch.assert_not_called()
        self.assertTrue(self.marker.exists())
        self.assertEqual(self.issue.status, "dispatched")


if __name__ == "__main__":
    unittest.main()
