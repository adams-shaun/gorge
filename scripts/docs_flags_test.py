#!/usr/bin/env python3
"""Tests for docs_flags.py. Stdlib only:

    python3 -m unittest discover -s scripts -p 'docs_flags_test.py'
"""

import os
import re
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import docs_flags  # noqa: E402


class FlagsTest(unittest.TestCase):
    def test_committed_page_is_current(self):
        with open(docs_flags.OUT, encoding="utf-8") as f:
            self.assertEqual(f.read(), docs_flags.render(), "run scripts/docs_flags.py")

    def test_every_declaration_is_found(self):
        # A looser count than the parser's: every flag.X( / fs.X( / flags.X( call in each file.
        loose = re.compile(r"^\s*(?!//).*\b(?:flag|fs|flags)\.(?:String|Int|Int64|Uint64|Float64|Bool|Duration|Func)(?:Var)?\(", re.M)
        for name, _, files, _ in docs_flags.TOOLS:
            for path in files:
                with open(os.path.join(docs_flags.ROOT, path), encoding="utf-8") as f:
                    want = len(loose.findall(f.read()))
                got = len(docs_flags.flags_of(path))
                self.assertEqual(got, want, "%s: %s declares %d flags, the page has %d" % (name, path, want, got))
                self.assertGreater(got, 0, name)

    def test_known_flags(self):
        page = docs_flags.render()
        for flag in ("-seats", "-verify", "-spellbench", "-surface", "-sample-replay", "-refuse", "-source-revision", "-hold"):
            self.assertIn("| `%s` |" % flag, page)

    def test_a_usage_string_naming_a_constant_is_resolved(self):
        usage = {n: u for n, _, _, u in docs_flags.flags_of("dirac/cmd/dplay/main.go")}["config"]
        self.assertIn("lockstep-autopay", usage)

    def test_split_args_respects_strings_and_nesting(self):
        src = 'flag.String("a,b", f(1, 2), "x (y), z")'
        args, _ = docs_flags.split_args(src, src.index("(") + 1)
        self.assertEqual(args, ['"a,b"', "f(1, 2)", '"x (y), z"'])


if __name__ == "__main__":
    unittest.main()
