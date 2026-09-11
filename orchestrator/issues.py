"""The defect ledger's live-state layer: one Markdown file per issue.

`.ds4/issues/<id>.md` is the single source of truth for a defect from the
moment it's noticed to the moment it merges. Each file is `---`-delimited
frontmatter (a flat key: value map -- no nested YAML, so a hand-rolled
parser is honest about what it supports and never silently mis-parses a
shape it doesn't) followed by free-form Markdown sections (the report text,
the brief, a chronological history log).

This mirrors the third-source ledger design from the backlog research: a
tracked issue tree keyed so a fix can close it, with status derived rather
than hand-maintained -- except here "derived" means "written by the daemon
that did the work", not "read by a human". `cmd/ledger` is meant to grow a
reader for this directory as its own follow-up task; this module is only
the writer/mutator side the daemon needs to run its loop.

No YAML library, no JSON: everything is stdlib, matching the rest of this
repo's own no-dependencies discipline.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

from . import config

# Every state an issue passes through. NEW -> ... -> MERGED is the happy
# path; HUMAN_NEEDED is the one state the daemon can enter but never leaves
# on its own -- see daemon.py's escalation ladder.
STATUSES = (
    "new",             # just noticed, no brief yet
    "briefed",         # a brief exists, not yet dispatched
    "dispatched",      # an implementer seat is running or just finished
    "review",          # a reviewer seat is running or just finished
    "gate",            # local deterministic gates are running
    "merged",          # landed on main, pushed, deployed
    "human_needed",    # the ladder ran out; a person must look
)


def _now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


@dataclass
class Issue:
    id: str
    title: str
    source: str  # "feedback" | "inbox" | "corpus"
    status: str = "new"
    created: str = field(default_factory=_now)
    updated: str = field(default_factory=_now)
    seat_kind: str = ""       # "" | "local" | "escalated"
    local_rounds: int = 0
    escalated_rounds: int = 0
    worktree: str = ""
    branch: str = ""
    commits: str = ""         # space-joined shas, kept flat (no lists in frontmatter)
    closed_by: str = ""       # merge sha once merged
    feedback_dir: str = ""
    report: str = ""          # the original defect text
    brief: str = ""           # filled in once triaged
    history: str = ""         # newline-joined log lines, newest last

    # --- (de)serialisation -------------------------------------------------

    _FRONT_RE = re.compile(r"^---\n(.*?)\n---\n(.*)$", re.DOTALL)
    _LINE_RE = re.compile(r"^([a-z_]+):\s?(.*)$")

    @classmethod
    def load(cls, path: Path) -> "Issue":
        text = path.read_text()
        m = cls._FRONT_RE.match(text)
        if not m:
            raise ValueError(f"{path}: no frontmatter block")
        front, body = m.group(1), m.group(2)
        fields = {}
        for line in front.splitlines():
            lm = cls._LINE_RE.match(line)
            if lm:
                fields[lm.group(1)] = lm.group(2)
        # Body sections are "## Name\n<text>\n\n## Next\n..." -- split on the
        # heading and keep report/brief/history as the three the daemon reads
        # back; anything else a human added by hand is preserved verbatim by
        # round-tripping the whole body through report/brief/history's own
        # "everything after my heading, up to the next one" extraction.
        sections = {"report": "", "brief": "", "history": ""}
        current = None
        buf: list[str] = []
        for line in body.splitlines():
            hm = re.match(r"^## (Report|Brief|History)\s*$", line)
            if hm:
                if current:
                    sections[current.lower()] = "\n".join(buf).strip()
                current = hm.group(1)
                buf = []
            elif current:
                buf.append(line)
        if current:
            sections[current.lower()] = "\n".join(buf).strip()
        known = {f.name for f in cls.__dataclass_fields__.values()}
        fields = {k: v for k, v in fields.items() if k in known}
        for k in ("local_rounds", "escalated_rounds"):
            if k in fields:
                fields[k] = int(fields[k])
        return cls(**fields, **sections)

    def save(self, path: Optional[Path] = None) -> Path:
        path = path or (config.ISSUES_DIR / f"{self.id}.md")
        self.updated = _now()
        d = asdict(self)
        report, brief, history = d.pop("report"), d.pop("brief"), d.pop("history")
        front_lines = [f"{k}: {v}" for k, v in d.items()]
        text = (
            "---\n" + "\n".join(front_lines) + "\n---\n\n"
            f"## Report\n{report}\n\n"
            f"## Brief\n{brief}\n\n"
            f"## History\n{history}\n"
        )
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
        return path

    def log(self, message: str) -> None:
        line = f"- {_now()} {message}"
        self.history = (self.history + "\n" + line).strip() if self.history else line


def all_issues() -> list[Issue]:
    config.ISSUES_DIR.mkdir(parents=True, exist_ok=True)
    out = []
    for p in sorted(config.ISSUES_DIR.glob("*.md")):
        try:
            out.append(Issue.load(p))
        except Exception:
            continue  # a malformed hand-edited file must not wedge the loop
    return out


def open_issues() -> list[Issue]:
    return [i for i in all_issues() if i.status not in ("merged", "human_needed")]


def find(issue_id: str) -> Optional[Issue]:
    p = config.ISSUES_DIR / f"{issue_id}.md"
    return Issue.load(p) if p.exists() else None


def exists_for_feedback_dir(feedback_dir: str) -> bool:
    return any(i.feedback_dir == feedback_dir for i in all_issues())


def new_from_feedback(report_dir: Path) -> Issue:
    """Build a new NEW-status issue from one feedback/<ts>/report.json.

    The id is derived from the feedback directory's own name (already a
    sortable, collision-resistant timestamp+random string minted by
    cmd/gorged/feedback.go), so re-running this against the same directory
    is idempotent by construction rather than by a separate dedup table.
    """
    import json

    data = json.loads((report_dir / "report.json").read_text())
    text = data.get("text", "").strip()
    title = (text[:77] + "...") if len(text) > 80 else text
    issue = Issue(
        id=f"fb-{report_dir.name}",
        title=title or "(no text)",
        source="feedback",
        feedback_dir=str(report_dir.relative_to(config.REPO)),
        report=text + (f"\n\nURL: {data['url']}" if data.get("url") else ""),
    )
    issue.log(f"noticed in {issue.feedback_dir}")
    return issue


def new_from_inbox(md_path: Path) -> Issue:
    """A hand-authored ticket: the whole file body becomes the report, and
    the id is the filename stem so re-dropping the same file is a no-op."""
    text = md_path.read_text().strip()
    title_line = next((l for l in text.splitlines() if l.strip()), md_path.stem)
    issue = Issue(
        id=f"inbox-{md_path.stem}",
        title=title_line.lstrip("#").strip()[:80],
        source="inbox",
        report=text,
    )
    issue.log(f"noticed in inbox/{md_path.name}")
    return issue
