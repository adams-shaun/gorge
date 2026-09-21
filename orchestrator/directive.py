"""Inject a controller directive at the top of one or more issues' briefs.

Every implementer dispatch (fresh, redispatch, escalation, dead-seat relaunch
-- see daemon.py's three `launch_implementer` call sites) writes `issue.brief`
verbatim to `<worktree>/.ds4/brief.md` at launch time. So updating
`issue.brief` here is sufficient for every FUTURE dispatch of that issue --
nothing else needs to change for that.

It is NOT sufficient for a seat already running right now: pi loads its
prompt file once at start and never rereads it. For issues that already have
a worktree on disk, this also patches the live `.ds4/brief.md` there directly,
so `cat .ds4/brief.md` shows the directive immediately and any already-running
seat's OWN next dispatch (redispatch/relaunch, which regenerates from
issue.brief anyway) is consistent with what a human sees now. It cannot make
an in-flight turn notice new text -- only a kill+relaunch does that (the
controller's call, same as any mid-round steer).

Usage:
    python3 -m orchestrator.directive "rebase before you do anything else" --all
    python3 -m orchestrator.directive "look at .ds4/reviews2/C-mana-costs.md for related evidence" \\
        --issue inbox-rv2c-autopass-uses-engine-offers inbox-rv2d-static-pt-variable-and-derived-types
    python3 -m orchestrator.directive "..." --status briefed,dispatched,waiting
    python3 -m orchestrator.directive "..." --file path/to/longer-directive.md --all
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import config, issues, pi


def _block(text: str) -> str:
    ts = issues._now()
    return f"## CONTROLLER DIRECTIVE ({ts})\n\n{text.strip()}\n"


def _prepend_after_title(brief: str, block: str) -> str:
    """Insert right after the brief's own `# Title` line (if present) so the
    directive reads first, before the title disappears into the body -- else
    at the very top."""
    lines = brief.splitlines()
    if lines and lines[0].startswith("# "):
        return "\n".join([lines[0], "", block, ""] + lines[1:])
    return block + "\n" + brief


def _live_seat_names(issue_id: str) -> list[str]:
    prefix = f"impl-{issue_id}-"
    return sorted(n for n in pi.running_names(
        config.LOCAL_MODEL, config.IMPLEMENTER_ESCALATED_MODEL, config.OVERFLOW_MODEL,
    ) if n.startswith(prefix))


def inject(issue_id: str, text: str) -> dict:
    issue = issues.find(issue_id)
    if issue is None:
        return {"issue": issue_id, "error": "no such issue"}
    block = _block(text)
    issue.brief = _prepend_after_title(issue.brief, block)
    issue.log(f"controller: injected directive at top of brief: {text.strip()[:80]!r}")
    issue.save()

    result = {"issue": issue_id, "brief_updated": True, "worktree_touched": False, "live_seat": None}

    wt = config.WORKTREES_DIR / issue_id
    live_brief = wt / ".ds4" / "brief.md"
    if live_brief.exists():
        current = live_brief.read_text()
        live_brief.write_text(_prepend_after_title(current, block))
        result["worktree_touched"] = True

    live = _live_seat_names(issue_id)
    if live:
        result["live_seat"] = live[0]
    return result


def _resolve_targets(args) -> list[str]:
    if args.issue:
        return list(args.issue)
    pool = issues.open_issues()
    if args.status:
        wanted = {s.strip() for s in args.status.split(",") if s.strip()}
        pool = [i for i in pool if i.status in wanted]
    elif not args.all:
        return []
    return [i.id for i in pool]


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("text", nargs="?", help="Directive text (omit if using --file)")
    ap.add_argument("--file", type=Path, help="Read directive text from this file instead of the positional arg")
    ap.add_argument("--issue", nargs="+", metavar="ID", help="Target specific issue id(s)")
    ap.add_argument("--status", help="Target open issues whose status is in this comma-separated list")
    ap.add_argument("--all", action="store_true", help="Target every open issue (not merged/superseded/human_needed)")
    args = ap.parse_args(argv)

    if args.file:
        text = args.file.read_text()
    elif args.text:
        text = args.text
    else:
        ap.error("provide directive text as an argument or via --file")
        return 2

    targets = _resolve_targets(args)
    if not targets:
        if not (args.issue or args.status or args.all):
            ap.error("scope required: --issue ID [ID...], --status a,b,c, or --all")
        print("no matching open issues", file=sys.stderr)
        return 1

    print(f"injecting into {len(targets)} issue(s):")
    for issue_id in targets:
        r = inject(issue_id, text)
        if r.get("error"):
            print(f"  {issue_id}: ERROR {r['error']}")
            continue
        tags = []
        if r["worktree_touched"]:
            tags.append("worktree patched")
        if r["live_seat"]:
            tags.append(f"LIVE SEAT RUNNING ({r['live_seat']}) -- will not see this until killed+relaunched")
        print(f"  {issue_id}: brief updated" + (f" [{', '.join(tags)}]" if tags else ""))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
