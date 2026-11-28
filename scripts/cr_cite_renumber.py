#!/usr/bin/env python3
"""Renumber keyword citations that name another keyword's rule. Stdlib only.

    python3 scripts/cr_cite_renumber.py [--write] <index.tsv> <file>...

A citation "CR 701.N" or "CR 702.N" (N at least 2) is flagged when its line
does not contain the cited rule's title but does contain, as a whole word, the
title (five or more letters) of a different 701 or 702 rule. This is the same
test as internal/archtest TestCRKeywordCitationsNameTheirKeyword.

A flagged citation whose line names exactly one other keyword is rewritten to
that keyword's rule number. The subrule letter is kept when the new rule number
followed by it is an id of the index, and dropped otherwise. Lines that name
two or more other keywords are printed with the prefix "SKIP " and left alone.

Only text after "//" is edited, and string literals in _test.go files. A string
literal in a non-test file is never edited: its text can reach the event log.

Without --write the edits are printed as a unified diff.
"""

import difflib
import re
import sys

CITATION_RE = re.compile(r"CR (\d{3}(?:\.\d+[a-z]?)?)")
KEYWORD_RE = re.compile(r"^(70[12]\.(\d+))([a-z]?)$")
MIN_LETTERS = 5


def read_index(path):
    """Return (ids, titles): every id of the index, and rule id -> title for
    rows of kind "rule" with a non-empty title."""
    ids, titles = set(), {}
    with open(path, encoding="utf-8") as f:
        for i, line in enumerate(f):
            row = line.rstrip("\n").split("\t")
            if i == 0 or not row[0]:
                continue
            ids.add(row[0])
            if len(row) >= 3 and row[1] == "rule" and row[2]:
                titles[row[0]] = row[2]
    return ids, titles


def keyword_rule(cited):
    """Split a cited id into (rule, letter) for 701.2+ and 702.2+, else None."""
    m = KEYWORD_RE.match(cited)
    if not m or int(m.group(2)) < 2:
        return None
    return m.group(1), m.group(3)


def rule_key(rule_id):
    sec, _, rest = rule_id.partition(".")
    digits = rest.rstrip("abcdefghijklmnopqrstuvwxyz")
    return int(sec), int(digits or 0), rest[len(digits):]


def keyword_titles(titles):
    """The 701 and 702 keyword rules that can be named, in rule order, with a
    compiled whole-word pattern for each title."""
    out = []
    for rule_id in sorted(titles, key=rule_key):
        if keyword_rule(rule_id) is None:
            continue
        title = titles[rule_id]
        if sum(c.isalpha() for c in title) < MIN_LETTERS:
            continue
        out.append((rule_id, re.compile(r"\b" + re.escape(title.lower()) + r"\b")))
    return out


def named_others(line, rule, titles, keywords):
    """The ids of other keyword rules the line names, in rule order, or [] when
    the line contains the cited rule's own title (or it has none)."""
    own = titles.get(rule)
    lower = line.lower()
    if own is None or own.lower() in lower:
        return []
    return [rid for rid, pat in keywords if rid != rule and pat.search(lower)]


def editable_spans(line, is_test):
    """Half-open column spans of a line that may be edited: the text after a
    "//" that is outside a literal, and (in a test file) the inside of string
    literals. Literals are scanned one line at a time."""
    spans = []
    i, n = 0, len(line)
    while i < n:
        c = line[i]
        if c == "/" and line.startswith("//", i):
            spans.append((i + 2, n))
            break
        if c in "\"`'":
            j = i + 1
            while j < n and line[j] != c:
                j += 2 if c != "`" and line[j] == "\\" else 1
            if is_test and c != "'":
                spans.append((i + 1, min(j, n)))
            i = j + 1
            continue
        i += 1
    return spans


def renumber_line(line, is_test, ids, titles, keywords):
    """Return (new line, skipped): the line with its flagged citations
    rewritten, and whether a flagged citation named two or more keywords."""
    spans = editable_spans(line, is_test)
    out, last, skipped = [], 0, False
    for m in CITATION_RE.finditer(line):
        kw = keyword_rule(m.group(1))
        if kw is None:
            continue
        rule, letter = kw
        others = named_others(line, rule, titles, keywords)
        if not others:
            continue
        if len(others) > 1:
            skipped = True
            continue
        if not any(a <= m.start(1) and m.end(1) <= b for a, b in spans):
            continue
        new = others[0] + letter if others[0] + letter in ids else others[0]
        out.append(line[last:m.start(1)])
        out.append(new)
        last = m.end(1)
    out.append(line[last:])
    return "".join(out), skipped


def renumber_file(path, ids, titles, keywords, log):
    """Return (old text, new text) for one file; SKIP lines go to log."""
    with open(path, encoding="utf-8") as f:
        old = f.read()
    is_test = path.endswith("_test.go")
    lines = old.split("\n")
    for i, line in enumerate(lines):
        new, skipped = renumber_line(line, is_test, ids, titles, keywords)
        if skipped:
            log.write("SKIP %s:%d: %s\n" % (path, i + 1, line.strip()))
        lines[i] = new
    return old, "\n".join(lines)


def main(argv, stdout=sys.stdout):
    write = False
    if argv and argv[0] == "--write":
        write, argv = True, argv[1:]
    if len(argv) < 2:
        sys.stderr.write("usage: cr_cite_renumber.py [--write] <index.tsv> <file>...\n")
        return 2
    ids, titles = read_index(argv[0])
    keywords = keyword_titles(titles)
    for path in argv[1:]:
        old, new = renumber_file(path, ids, titles, keywords, stdout)
        if old == new:
            continue
        if write:
            with open(path, "w", encoding="utf-8") as f:
                f.write(new)
            continue
        stdout.writelines(difflib.unified_diff(
            old.splitlines(keepends=True), new.splitlines(keepends=True),
            fromfile="a/" + path, tofile="b/" + path))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
