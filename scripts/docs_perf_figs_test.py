#!/usr/bin/env python3
"""Tests for docs_perf_figs.py. Stdlib only:

    python3 -m unittest discover -s scripts -p 'docs_perf_figs_test.py'
"""

import os
import re
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import docs_perf_figs  # noqa: E402

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PAGES = os.path.join(ROOT, "docs", "site", "performance")


class PerfFigsTest(unittest.TestCase):
    def test_committed_figures_match_the_data(self):
        """Regenerating from data/perf.json reproduces every committed SVG byte for byte."""
        for name, body in docs_perf_figs.render().items():
            with self.subTest(name=name):
                with open(os.path.join(docs_perf_figs.OUT, name)) as f:
                    self.assertEqual(f.read(), body, f"{name} is stale: run python3 scripts/docs_perf_figs.py")

    def test_render_is_deterministic(self):
        self.assertEqual(docs_perf_figs.render(), docs_perf_figs.render())

    def test_every_referenced_figure_exists_and_every_figure_is_used(self):
        used = set()
        for page in ("index.md", "engine.md", "bots.md", "methodology.md"):
            with open(os.path.join(PAGES, page)) as f:
                used |= set(re.findall(r'--8<-- "performance/figs/([^"]+)"', f.read()))
        self.assertTrue(used)
        self.assertEqual(used, set(docs_perf_figs.FIGS))


if __name__ == "__main__":
    unittest.main()
