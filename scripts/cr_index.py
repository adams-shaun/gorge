#!/usr/bin/env python3
"""Derive the Comprehensive Rules index for engine/rules/crcoverage.

usage: cr_index.py <rules-text> [out.tsv]

Writes one row per section, rule and subrule id of the rules text:
id, kind, title, in_scope, examples. The index carries no rules text: a
title is a section title or a keyword name (rules of sections 701 and 702),
and examples is a count. in_scope is 0 when the id, its parent rule or its
section is listed in engine/rules/crcoverage/cr-scope-exclusions.tsv.
Stdlib only.
"""

import os
import re
import sys

SECTION = re.compile(r"^(\d{3})\. (\S.*)$")
RULE = re.compile(r"^\s*(\d{3})\.(\d+)\. (\S.*)$")
SUBRULE = re.compile(r"^\s*(\d{3})\.(\d+)([a-z]) \S")

BODY_START = "1. Game Concepts"
BODY_END = "Glossary"
KEYWORD_SECTIONS = ("701", "702")
KEYWORD_INTROS = ("701.1", "702.1")

HEADER = "id\tkind\ttitle\tin_scope\texamples"

DEFAULT_SCOPE = os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    "..", "engine", "rules", "crcoverage", "cr-scope-exclusions.tsv",
)


class CRIndexError(Exception):
    pass


def body_lines(text):
    """Return the body lines between the second BODY_START and the second BODY_END."""
    lines = text.replace("\r", "").replace("\f", "").split("\n")
    starts = [i for i, l in enumerate(lines) if l.strip() == BODY_START]
    ends = [i for i, l in enumerate(lines) if l.strip() == BODY_END]
    if len(starts) != 2:
        raise CRIndexError("want %r exactly twice, got %d" % (BODY_START, len(starts)))
    if len(ends) != 2:
        raise CRIndexError("want %r exactly twice, got %d" % (BODY_END, len(ends)))
    if ends[1] <= starts[1]:
        raise CRIndexError("%r precedes %r" % (BODY_END, BODY_START))
    return lines[starts[1]:ends[1]]


def parse(text):
    """Return rows [id, kind, title, examples] in rules-text order."""
    rows = []
    by_id = {}
    last = None
    for line in body_lines(text):
        if line.strip().startswith("Example:"):
            if last is not None:
                last[3] += 1
            continue
        m = SECTION.match(line)
        if m:
            rid, kind, title = m.group(1), "section", m.group(2).strip()
        else:
            m = RULE.match(line)
            if m:
                rid = "%s.%s" % (m.group(1), m.group(2))
                kind = "rule"
                title = ""
                if m.group(1) in KEYWORD_SECTIONS and rid not in KEYWORD_INTROS:
                    title = m.group(3).strip()
            else:
                m = SUBRULE.match(line)
                if not m:
                    continue
                rid = "%s.%s%s" % (m.group(1), m.group(2), m.group(3))
                kind, title = "subrule", ""
        if rid in by_id:
            continue
        row = [rid, kind, title, 0]
        by_id[rid] = row
        rows.append(row)
        last = row
    return rows


def read_scope(path):
    """Return the set of excluded ids from a scope file (header `id\treason`)."""
    with open(path, encoding="utf-8") as f:
        lines = f.read().split("\n")
    if not lines or lines[0] != "id\treason":
        raise CRIndexError("%s: header must be 'id\\treason'" % path)
    out = set()
    for n, l in enumerate(lines[1:], 2):
        if not l.strip():
            continue
        cells = l.split("\t")
        if len(cells) != 2 or not cells[0]:
            raise CRIndexError("%s:%d: want 'id<TAB>reason'" % (path, n))
        out.add(cells[0])
    return out


def ancestors(rid):
    """The id, its parent rule and its section."""
    out = [rid]
    sec = rid[:3]
    m = re.match(r"^(\d{3}\.\d+)[a-z]$", rid)
    if m:
        out.append(m.group(1))
    if rid != sec:
        out.append(sec)
    return out


def render(rows, excluded):
    out = [HEADER]
    for rid, kind, title, examples in rows:
        in_scope = "0" if any(a in excluded for a in ancestors(rid)) else "1"
        out.append("\t".join([rid, kind, title, in_scope, str(examples)]))
    return "\n".join(out) + "\n"


def summary(rows, excluded):
    kinds = {"section": 0, "rule": 0, "subrule": 0}
    ins = outs = examples = 0
    for rid, kind, _, ex in rows:
        kinds[kind] += 1
        examples += ex
        if kind == "section":
            continue
        if any(a in excluded for a in ancestors(rid)):
            outs += 1
        else:
            ins += 1
    return ("%d sections, %d rules, %d subrules, %d in-scope and %d out-of-scope "
            "rule and subrule ids, %d examples" % (
                kinds["section"], kinds["rule"], kinds["subrule"], ins, outs, examples))


def main(argv):
    if len(argv) not in (2, 3):
        sys.stderr.write("usage: cr_index.py <rules-text> [out.tsv]\n")
        return 2
    scope = os.environ.get("CR_SCOPE_EXCLUSIONS", DEFAULT_SCOPE)
    try:
        with open(argv[1], encoding="utf-8") as f:
            text = f.read()
        rows = parse(text)
        excluded = read_scope(scope)
    except (CRIndexError, OSError, UnicodeDecodeError) as e:
        sys.stderr.write("cr_index: %s\n" % e)
        return 1
    known = {r[0] for r in rows}
    missing = sorted(excluded - known)
    if missing:
        sys.stderr.write("cr_index: scope file names ids the text lacks: %s\n" % " ".join(missing))
        return 1
    data = render(rows, excluded)
    if len(argv) == 3:
        with open(argv[2], "w", encoding="utf-8", newline="\n") as f:
            f.write(data)
    else:
        sys.stdout.write(data)
    sys.stderr.write(summary(rows, excluded) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
