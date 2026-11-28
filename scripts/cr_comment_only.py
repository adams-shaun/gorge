#!/usr/bin/env python3
"""Prove a diff changes only rule numbers. Stdlib only.

    python3 scripts/cr_comment_only.py <base-ref> <pathspec>...

Runs `git diff -U0 <base-ref> -- <pathspec>...` and checks, per file:

 1. the file ends in .go (else "not a Go file");
 2. every hunk removes as many lines as it adds (else "hunk changes line
    count"); removed and added lines are paired in order;
 3. each pair is equal once every rule number (7xx.N with an optional letter)
    is replaced by "#" (else "changed more than a rule number");
 4. in a file not ending in _test.go, the pair first differs after the start
    of the line's comment: the first "//" outside a "..." string, a backquoted
    string and a 'x' rune literal (else "edit outside a comment in a non-test
    file"). A line with no such "//" fails.

Exits 0 and prints "comment-only: files=N lines=M" when every check passes,
else exits 1 and prints one "path:line: reason" per offending line.
"""

import re
import subprocess
import sys

RULE_RE = re.compile(r"\b7\d\d\.\d+[a-z]?\b")
HUNK_RE = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@")


def comment_start(line):
    """Index of the first "//" outside a string or rune literal, or -1."""
    i, n = 0, len(line)
    while i < n:
        c = line[i]
        if c == "/" and line.startswith("//", i):
            return i
        if c in "\"`'":
            j = i + 1
            while j < n and line[j] != c:
                j += 2 if c != "`" and line[j] == "\\" else 1
            i = j + 1
            continue
        i += 1
    return -1


def first_difference(a, b):
    for i, (x, y) in enumerate(zip(a, b)):
        if x != y:
            return i
    return min(len(a), len(b))


def check_pair(path, lineno, old, new, problems):
    if RULE_RE.sub("#", old) != RULE_RE.sub("#", new):
        problems.append("%s:%d: changed more than a rule number" % (path, lineno))
        return
    if path.endswith("_test.go"):
        return
    start = comment_start(new)
    if start < 0 or first_difference(old, new) <= start:
        problems.append("%s:%d: edit outside a comment in a non-test file" % (path, lineno))


def check_diff(diff_text):
    """Return (problems, files, lines) for the text of `git diff -U0`."""
    problems, files, lines = [], 0, 0
    path = None
    removed, added, hunk_line = [], [], 0

    def flush():
        nonlocal lines
        if path is None or (not removed and not added):
            return
        if not path.endswith(".go"):
            return
        if len(removed) != len(added):
            problems.append("%s:%d: hunk changes line count" % (path, hunk_line))
            return
        for k, (old, new) in enumerate(zip(removed, added)):
            check_pair(path, hunk_line + k, old, new, problems)
            lines += 1

    for raw in diff_text.split("\n"):
        if raw.startswith("diff --git "):
            flush()
            removed, added = [], []
            m = re.match(r"diff --git a/(.*) b/(.*)$", raw)
            path = m.group(2) if m else raw.split()[-1]
            files += 1
            if not path.endswith(".go"):
                problems.append("%s:0: not a Go file" % path)
            continue
        if raw.startswith("+++ ") or raw.startswith("--- "):
            if raw.startswith("+++ b/"):
                path = raw[len("+++ b/"):]
            continue
        m = HUNK_RE.match(raw)
        if m:
            flush()
            removed, added = [], []
            hunk_line = int(m.group(3))
            continue
        if raw.startswith("-"):
            removed.append(raw[1:])
        elif raw.startswith("+"):
            added.append(raw[1:])
    flush()
    return problems, files, lines


def main(argv, stdout=sys.stdout):
    if len(argv) < 2:
        sys.stderr.write("usage: cr_comment_only.py <base-ref> <pathspec>...\n")
        return 2
    diff = subprocess.run(
        ["git", "diff", "-U0", argv[0], "--"] + argv[1:],
        check=True, capture_output=True, text=True).stdout
    problems, files, lines = check_diff(diff)
    for p in problems:
        stdout.write(p + "\n")
    if problems:
        return 1
    stdout.write("comment-only: files=%d lines=%d\n" % (files, lines))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
