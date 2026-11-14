"""test-targeted's selector: a new test file or fixture in a heavy package does not force the whole package."""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import test_targets as tt  # noqa: E402
from test_targets_test import DEPS, DIRMAP  # noqa: E402

NEW_TEST = ["package oracle", "", "import \"testing\"", "", "func mbcHelper() {}", "", "func TestMBCFooLowers(t *testing.T) {", "\tmbcHelper()", "}"]
PATH = "dirac/oracle/mbc_foo_test.go"


def planned(changes, files=None, added=()):
    res, _ = tt.plan(changes, DIRMAP, DEPS, lambda p: (files or {}).get(p, []), None, added)
    return {p: (k, rx) for k, p, rx in res}


def test_a_new_test_file_selects_its_tests_and_the_census_not_the_package():
    # untracked (ranges None) and committed-as-added (ranges cover every line) are the same
    for changes, added in (({PATH: None}, ()), ({PATH: [(1, len(NEW_TEST))]}, {PATH})):
        kind, rx = planned(changes, {PATH: NEW_TEST}, added)["./dirac/oracle"]
        assert kind == "named" and rx.startswith("^(TestMBCFooLowers)$|") and tt.CENSUS in rx


def test_a_new_test_file_with_testmain_or_init_runs_whole():
    for hook in ("func TestMain(m *testing.M) {", "func init() {"):
        lines = NEW_TEST + ["", hook, "}"]
        assert planned({PATH: None}, {PATH: lines})["./dirac/oracle"][0] == "whole"


def test_a_modified_helper_line_in_an_existing_test_file_still_runs_whole():
    got = planned({PATH: [(5, 5)]}, {PATH: NEW_TEST}, added=())
    assert got["./dirac/oracle"][0] == "whole"


def test_an_added_fixture_does_not_force_whole_but_a_modified_one_does():
    fx = "dirac/oracle/testdata/new.json"
    assert planned({fx: None}, added={fx})["./dirac/oracle"][0] == "census"
    assert planned({fx: None, PATH: None}, {PATH: NEW_TEST}, {fx, PATH})["./dirac/oracle"][0] == "named"
    assert planned({fx: None})["./dirac/oracle"][0] == "whole"
    # a light package keeps the old rule
    lf = "leaf/testdata/new.json"
    assert planned({lf: None}, added={lf})["./leaf"][0] == "whole"
