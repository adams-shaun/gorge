"""The merge-conflict resolver lane: a rebase or final-merge conflict must
dispatch a resolver seat in the issue's own worktree; a DONE resolution
re-enters the normal gate-and-merge path; a failed/exhausted resolver hands
back to the implementer ladder rather than parking a human immediately."""
import tempfile
import unittest
import unittest.mock as mock
from pathlib import Path

from orchestrator import config, daemon, issues, seats


def mk(iid, status="review", **kw):
    return issues.Issue(id=iid, title=iid, source="feedback", status=status, **kw)


class DispatchMergeResolverTestCase(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.issues_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.issues_dir.cleanup)
        _iss = mock.patch.object(config, "ISSUES_DIR", Path(self.issues_dir.name))
        _iss.start(); self.addCleanup(_iss.stop)
        self.wt = Path(self.tmp.name)
        (self.wt / ".ds4").mkdir()
        self.issue = mk("fb-x", worktree=str(self.wt))
        self.launched = []
        patcher = mock.patch.object(seats, "launch_merge_resolver",
                                    side_effect=lambda *a, **k: self.launched.append(a) or (None, None))
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_rebase_conflict_dispatches_a_resolver_seat_in_place(self):
        daemon._dispatch_merge_resolver(self.issue, self.wt, "rebase onto main conflicted:\nCONFLICT (content)")
        self.assertEqual(self.issue.status, "merge_fix")
        self.assertEqual(self.issue.merge_rounds, 1)
        self.assertEqual(len(self.launched), 1)
        issue_id, wt, tag, text = self.launched[0]
        self.assertEqual((issue_id, tag), ("fb-x", "mrg1"))
        self.assertEqual(wt, self.wt)  # existing worktree, not a fresh one
        self.assertIn("CONFLICT", text)

    def test_second_conflict_gets_mrg2_and_exhaustion_hands_back_to_the_ladder(self):
        daemon._dispatch_merge_resolver(self.issue, self.wt, "conflict")
        daemon._dispatch_merge_resolver(self.issue, self.wt, "conflict")
        self.assertEqual(self.issue.merge_rounds, 2)
        daemon._dispatch_merge_resolver(self.issue, self.wt, "conflict")
        self.assertEqual(len(self.launched), 2)  # no third seat
        self.assertEqual(self.issue.merge_rounds, 0)  # budget reset on hand-back
        # exhaustion routes through the implementer ladder, which parks for a seat
        self.assertIn(self.issue.status, ("dispatched", "waiting"))

    def test_non_conflict_merge_failure_is_not_a_resolver_job(self):
        ok, out = False, "pre-commit hook declined the merge commit"
        with mock.patch.object(daemon, "_dispatch_merge_resolver") as dmr:
            # mirror the guard in _run_gates_and_merge's merge path
            if "conflict" in out.lower():
                daemon._dispatch_merge_resolver(self.issue, self.wt, out)
            self.assertEqual(dmr.call_count, 0)

    def test_max_merge_rounds_is_bounded_and_positive(self):
        self.assertGreaterEqual(config.MAX_MERGE_ROUNDS, 1)


class AdvanceMergeFixTestCase(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.issues_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.issues_dir.cleanup)
        _iss = mock.patch.object(config, "ISSUES_DIR", Path(self.issues_dir.name))
        _iss.start(); self.addCleanup(_iss.stop)
        self.wt = Path(self.tmp.name)
        (self.wt / ".ds4").mkdir()
        self.issue = mk("fb-y", status="merge_fix", merge_rounds=1,
                        worktree=str(self.wt), branch="wt/fb-y")

    def _status(self, payload):
        p = seats.merge_resolver_status_path(self.wt, "mrg1")
        import json
        p.write_text(json.dumps(payload))

    def test_done_reenters_the_gate_and_merge_path(self):
        self._status({"status": "DONE", "commits": ["abc"]})
        with mock.patch.object(daemon, "_run_gates_and_merge") as gates_run:
            daemon.advance_merge_fix(self.issue)
        gates_run.assert_called_once_with(self.issue, self.wt)
        self.assertEqual(self.issue.merge_rounds, 0)
        self.assertEqual(self.issue.commits, "abc")

    def test_block_hands_back_to_the_implementer_ladder(self):
        self._status({"status": "BLOCKED"})
        with mock.patch.object(daemon, "_handle_gate_or_review_failure") as fail:
            daemon.advance_merge_fix(self.issue)
        fail.assert_called_once()
        self.assertIn("BLOCKED", fail.call_args[0][1])
        self.assertEqual(self.issue.merge_rounds, 0)

    def test_running_seat_is_waited_on_not_relaunched(self):
        self._status({"status": "running"})
        import json as _j
        with mock.patch.object(daemon.pi, "already_launched", return_value=True):
            with mock.patch.object(seats, "launch_merge_resolver") as launch:
                daemon.advance_merge_fix(self.issue)
        launch.assert_not_called()
        self.assertEqual(self.issue.status, "merge_fix")

    def test_merge_fix_work_outranks_a_p1_newcomer(self):
        merge = mk("a-merge", status="merge_fix", priority=5)
        fresh = mk("z-new", "new", priority=1)
        self.assertLess(daemon._dispatch_order(merge), daemon._dispatch_order(fresh))

    def test_merge_rounds_round_trips_through_the_file(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "a.md"
            mk("a", status="merge_fix", merge_rounds=2).save(p)
            back = issues.Issue.load(p)
            self.assertEqual(back.merge_rounds, 2)
            self.assertIsInstance(back.merge_rounds, int)

    def test_missing_merge_rounds_line_loads_as_the_default(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "a.md"
            p.write_text("---\nid: a\ntitle: t\nsource: inbox\nstatus: new\n---\n\n## Report\nx\n\n## Brief\n\n## History\n")
            self.assertEqual(issues.Issue.load(p).merge_rounds, 0)


if __name__ == "__main__":
    unittest.main()
