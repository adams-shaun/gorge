#!/usr/bin/env python3
"""Render the engines' command-line flags into docs/site/engines/flags.md.

Each tool declares its flags with Go's standard flag package. This script reads
those declarations from the tool's source (name, default expression, usage
string) and writes one table per tool, so the page can't drift from the code.
docs_flags_test.py fails when the committed page is not what this script
writes. Regenerate it with:

    python3 scripts/docs_flags.py            # writes docs/site/engines/flags.md
    python3 scripts/docs_flags.py --check    # exit 1 when the page is stale

Stdlib only.
"""

import os
import re
import sys

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
OUT = os.path.join(ROOT, "docs", "site", "engines", "flags.md")

# (command, the package it is built from, the files that declare its flags, one line on what it is)
TOOLS = [
    ("mtgsim", "cmd/mtgsim", ["cmd/mtgsim/main.go"],
     "Plays seeded games on gorge between bots and checks each by replay."),
    ("forgec", "cmd/forgec", ["cmd/forgec/main.go"],
     "Fetches the Forge card corpus, compiles it into gorge's IR and reports coverage. The flags are shared by its four subcommands: `fetch`, `compile`, `report` and `coverage`."),
    ("cardfuzz", "cmd/cardfuzz", ["cmd/cardfuzz/main.go"],
     "Plays random and exploring games over the whole corpus on gorge, records failures and tracks which cards and abilities it has reached."),
    ("kgen", "dirac/cmd/kgen", ["dirac/kgen/main.go"],
     "Compiles card pools from gorge's IR into dirac's binary card tables, and answers which cards dirac supports. `dirac/cmd/kgen` is a thin main over package `dirac/kgen`."),
    ("dplay", "dirac/cmd/dplay", ["dirac/cmd/dplay/main.go"],
     "Plays seeded games on dirac alone and prints a line per game and a summary."),
    ("sbv2dirac", "dirac/cmd/sbv2dirac", ["dirac/cmd/sbv2dirac/main.go"],
     "Serves dirac as a SpellBench v2 engine over stdio."),
    ("kfuzz", "dirac/oracle/cmd/kfuzz", ["dirac/oracle/cmd/kfuzz/main.go"],
     "Plays seeded games on gorge and dirac in lockstep and reports the first decision where they differ."),
    ("khbot", "dirac/oracle/cmd/khbot", ["dirac/oracle/cmd/khbot/main.go"],
     "Compares gorge's heuristic bot with its port on dirac, decision by decision."),
    ("ksbt", "dirac/oracle/cmd/ksbt", ["dirac/oracle/cmd/ksbt/main.go"],
     "Compares gorge's sb-tactical bot answering on gorge with the same bot answering through a view built from the dirac position."),
]

DECL = re.compile(r"\b(?:flag|fs|flags)\.(String|Int|Int64|Uint64|Float64|Bool|Duration|Func)(Var)?\(")


def split_args(src, i):
    """The top-level arguments of the call whose '(' is at src[i-1]; returns (args, end)."""
    args, depth, cur = [], 1, []
    while i < len(src):
        c = src[i]
        if c in "\"`":
            j = i + 1
            while j < len(src) and src[j] != c:
                j += 2 if (c == '"' and src[j] == "\\") else 1
            cur.append(src[i:j + 1])
            i = j + 1
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
            if depth == 0:
                args.append("".join(cur).strip())
                return args, i + 1
        elif c == "," and depth == 1:
            args.append("".join(cur).strip())
            cur = []
            i += 1
            continue
        cur.append(c)
        i += 1
    raise ValueError("unterminated call")


def consts(dirpath):
    """String constants declared in the package directory, for usage strings that name one."""
    out = {}
    for fn in sorted(os.listdir(dirpath)):
        if fn.endswith(".go") and not fn.endswith("_test.go"):
            with open(os.path.join(dirpath, fn), encoding="utf-8") as f:
                for m in re.finditer(r'^\s*(\w+)\s*=\s*"([^"\n]*)"\s*$', f.read(), re.M):
                    out[m.group(1)] = m.group(2)
    return out


def literal(expr, known):
    """A Go string expression of literals and known constants, as text."""
    parts = [p.strip() for p in re.split(r"\+(?=(?:[^\"]*\"[^\"]*\")*[^\"]*$)", expr)]
    out = []
    for p in parts:
        if p.startswith('"') and p.endswith('"'):
            out.append(re.sub(r"\\(.)", r"\1", p[1:-1]))
        elif p.startswith("`") and p.endswith("`"):
            out.append(p[1:-1])
        elif p in known:
            out.append(known[p])
        else:
            raise ValueError("usage string uses %r, which is neither a literal nor a constant of its package" % p)
    return "".join(out)


def flags_of(path):
    full = os.path.join(ROOT, path)
    with open(full, encoding="utf-8") as f:
        lines = f.read().split("\n")
    known = consts(os.path.dirname(full))
    src = "\n".join("" if l.lstrip().startswith("//") else l for l in lines)
    found = []
    for m in DECL.finditer(src):
        args, _ = split_args(src, m.end())
        kind, var = m.group(1), m.group(2)
        if kind == "Func":
            name, usage, default = args[0], args[1], ""
        else:
            if var:
                args = args[1:]
            name, default, usage = args[0], args[1], args[2]
        if not name.startswith('"'):
            raise ValueError("%s: flag name %s is not a literal" % (path, name))
        found.append((name.strip('"'), kind.lower(), default, literal(usage, known)))
    return found


def cell(s):
    return s.replace("|", "\\|").replace("\n", " ")


def default_text(kind, default):
    if kind == "func":
        return "repeatable"
    d = default.strip()
    if d in ('""', ""):
        return "none"
    if d.startswith('"'):
        return "`%s`" % d.strip('"')
    return "`%s`" % d


def render():
    out = ["<!-- Generated by scripts/docs_flags.py from the tools' flag declarations; run it to refresh. -->",
           "",
           "# Command flags",
           "",
           "Every flag of the engines' command-line tools, read from the Go source by",
           "`scripts/docs_flags.py`. A test fails when this page and the source disagree.",
           "The tools use Go's `flag` package, so `-name value`, `-name=value` and `--name` all work.",
           "[Using the engines](using.md) says what each tool is for and shows examples.",
           ""]
    for name, pkg, files, blurb in TOOLS:
        flags = []
        for f in files:
            flags += flags_of(f)
        flags.sort(key=lambda x: x[0])
        out += ["## %s" % name, "", "`go run -trimpath ./%s`. %s" % (pkg, blurb), "",
                "| Flag | Type | Default | Meaning |", "|---|---|---|---|"]
        for n, kind, default, usage in flags:
            out.append("| `-%s` | %s | %s | %s |" % (n, kind if kind != "func" else "string", default_text(kind, default), cell(usage)))
        out.append("")
    return "\n".join(out)


def main(argv):
    text = render()
    if "--check" in argv:
        try:
            with open(OUT, encoding="utf-8") as f:
                same = f.read() == text
        except OSError:
            same = False
        if not same:
            print("docs_flags: %s is stale; run scripts/docs_flags.py" % os.path.relpath(OUT, ROOT), file=sys.stderr)
            return 1
        return 0
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with open(OUT, "w", encoding="utf-8") as f:
        f.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
