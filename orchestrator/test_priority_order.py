"""Issue priority: carried on the file, and honoured when the daemon dispatches.

Priority orders WHAT to work on; it does not override the existing rule that
in-flight work claims a seat before new work. A P1 that jumped ahead of reviews
and merges would starve the pipeline it is trying to speed up, so status stays
the primary key and priority orders within it.
"""
import tempfile
import unittest
import unittest.mock
from pathlib import Path

from orchestrator import daemon, issues


def mk(iid, status, priority=None):
    kw = {} if priority is None else {"priority": priority}
    return issues.Issue(id=iid, title=iid, source="inbox", status=status, **kw)


class PriorityFieldTestCase(unittest.TestCase):
    def test_priority_defaults_to_the_middle_of_the_band(self):
        self.assertEqual(mk("a", "new").priority, 3)

    def test_priority_round_trips_through_the_file_as_an_int(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "a.md"
            mk("a", "new", 1).save(p)
            self.assertIn("priority: 1", p.read_text())
            back = issues.Issue.load(p)
            self.assertEqual(back.priority, 1)
            self.assertIsInstance(back.priority, int)

    def test_a_missing_priority_line_loads_as_the_default(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "a.md"
            p.write_text("---\nid: a\ntitle: t\nsource: inbox\nstatus: new\n---\n\n## Report\nx\n\n## Brief\n\n## History\n")
            self.assertEqual(issues.Issue.load(p).priority, 3)


class DispatchOrderTestCase(unittest.TestCase):
    def test_in_flight_work_still_outranks_a_p1_newcomer(self):
        urgent_new = mk("z-new", "new", 1)
        a_review = mk("a-review", "review", 5)
        self.assertLess(daemon._dispatch_order(a_review), daemon._dispatch_order(urgent_new))

    def test_priority_orders_within_the_same_status(self):
        items = [mk("c", "waiting", 3), mk("a", "waiting", 1), mk("b", "waiting", 2)]
        self.assertEqual([i.id for i in sorted(items, key=daemon._dispatch_order)], ["a", "b", "c"])

    def test_equal_priority_falls_back_to_a_stable_id_order(self):
        items = [mk("b", "new"), mk("a", "new")]
        self.assertEqual([i.id for i in sorted(items, key=daemon._dispatch_order)], ["a", "b"])

    def test_an_unknown_status_sorts_last(self):
        self.assertGreater(daemon._dispatch_order(mk("x", "weird", 1)), daemon._dispatch_order(mk("y", "new", 5)))


if __name__ == "__main__":
    unittest.main()


class UnmetDependencyTestCase(unittest.TestCase):
    """The daemon's blocking check reads the same Depends-On parse the ledger does."""

    def setUp(self):
        self.known = {}
        patcher = unittest.mock.patch.object(issues, "find", side_effect=lambda i: self.known.get(i))
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_no_depends_on_line_blocks_nothing(self):
        self.assertIsNone(daemon._unmet_dependency(mk("a", "new")))

    def test_an_open_dependency_blocks_and_is_named(self):
        self.known["dep"] = mk("dep", "dispatched")
        x = mk("a", "new"); x.report = "Depends-On: dep"
        self.assertEqual(daemon._unmet_dependency(x), "dep")

    def test_a_merged_or_superseded_dependency_does_not_block(self):
        self.known["dep"] = mk("dep", "merged")
        x = mk("a", "new"); x.report = "Depends-On: dep"
        self.assertIsNone(daemon._unmet_dependency(x))
        self.known["dep"] = mk("dep", "superseded")
        self.assertIsNone(daemon._unmet_dependency(x))

    def test_an_unknown_dependency_blocks(self):
        x = mk("a", "new"); x.brief = "Depends-On: nosuch"
        self.assertEqual(daemon._unmet_dependency(x), "nosuch")

    def test_the_line_is_read_from_report_and_brief_and_tolerates_indent_and_case(self):
        self.known["d1"] = mk("d1", "merged")
        x = mk("a", "new"); x.report = "  depends-on: d1, d2"
        self.assertEqual(daemon._unmet_dependency(x), "d2")
