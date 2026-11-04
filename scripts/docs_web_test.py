#!/usr/bin/env python3
"""Drift guard for the Web client pages (docs/site/web/layouts.md, gameplay.md).

Reads the layout vocabulary and the auto-pass presets from the Svelte client's
source and fails when a preset id, a layout option label, a play preset or a
cited source path is missing from the pages. Standard library only; no npm.

    python3 scripts/docs_web_test.py
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "web" / "src"
PAGES = ROOT / "docs" / "site" / "web"


def const_list(text, name):
    m = re.search(r"export const %s[^=]*=\s*\[([^\]]*)\]" % name, text)
    return re.findall(r"'([^']+)'", m.group(1)) if m else None


def const_labels(text, name):
    m = re.search(r"export const %s[^=]*=\s*\{(.*?)\};" % name, text, re.S)
    if not m:
        return None
    return re.findall(r"(?:'[^']+'|\w+):\s*'((?:[^'\\]|\\.)*)'", m.group(1))


def main():
    prof = (SRC / "lib" / "layoutprofile.ts").read_text()
    play = (SRC / "components" / "PlaySettingsPanel.svelte").read_text()
    layouts = (PAGES / "layouts.md").read_text()
    gameplay = (PAGES / "gameplay.md").read_text()
    lower = layouts.lower()
    errs = []

    presets = const_list(prof, "PRESET_IDS")
    if not presets:
        errs.append("could not read PRESET_IDS from layoutprofile.ts")
    for pid in presets or []:
        if "`%s`" % pid not in layouts:
            errs.append("layouts.md does not name preset id `%s`" % pid)
    frames = layouts.count('class="gg-wire__frame')
    if presets and frames < len(presets):
        errs.append("layouts.md has %d wireframes, fewer than %d presets" % (frames, len(presets)))

    for name in ("PRESET_LABELS", "ARRANGEMENT_LABELS", "ORIENTATION_LABELS", "OVERFLOW_LABELS",
                 "RAIL_LABELS", "LOG_LABELS", "PROMPT_LABELS", "ORDER_LABELS", "ANCHOR_LABELS", "REGION_LABELS"):
        labels = const_labels(prof, name)
        if not labels:
            errs.append("could not read %s from layoutprofile.ts" % name)
            continue
        for lab in labels:
            if lab.lower() not in lower:
                errs.append("layouts.md does not mention %s value %r" % (name, lab))

    for lab in re.findall(r"label: '([^']+)',\s*blurb:", play):
        if lab not in gameplay:
            errs.append("gameplay.md does not mention auto-pass preset %r" % lab)
    if not re.findall(r"label: '([^']+)',\s*blurb:", play):
        errs.append("could not read PRESET_LIST from PlaySettingsPanel.svelte")

    for page in (layouts, gameplay):
        for path in sorted(set(re.findall(r"`(web/src/[\w./-]+?)(?::[\d,\- :]+)?`", page))):
            if not (ROOT / path).is_file():
                errs.append("cited path does not exist: %s" % path)

    if errs:
        print("\n".join(errs))
        return 1
    print("ok: web client pages match the client source")
    return 0


if __name__ == "__main__":
    sys.exit(main())
