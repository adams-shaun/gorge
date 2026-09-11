"""Builds the concrete brief/dispatch/report file set for one issue's
current step and launches the pi-agent seat for it. This is the only module
that knows the *shape* of a triage vs. implementer vs. review dispatch;
daemon.py just calls one of these three functions and polls the result.

Every implementer/review dispatch is ROUND-TAGGED (`round_tag`, e.g. "r1",
"r2", "sol1") in both its pi-agent `--name` and its status/report file
names. This is not cosmetic: reusing one fixed name/path across rounds
caused two real bugs the first time this loop ran live --
  1. relaunching under the SAME --name kills the prior still-running
     instance's credential store (see pi.already_launched's docstring for
     the incident), and
  2. reading a fixed status.json path after redispatching a new round can
     read the PREVIOUS round's already-terminal status before the new
     round has written its own, making round 2 look instantly "done".
Round-tagging both the name and the path makes each round's state fully
independent of every other round's.
"""

from __future__ import annotations

import subprocess
from pathlib import Path

from . import config, pi

TRIAGE_TAIL = config.DISPATCH_DIR / "triage-tail.md"
IMPL_TAIL_TEMPLATE = config.DISPATCH_DIR / "implementer-tail-template.md"
REVIEWER_TAIL = config.DISPATCH_DIR / "reviewer-tail.md"


def _combined_system(tail_text: str) -> str:
    return config.CONTEXT_FILE.read_text() + tail_text


def round_tag(issue) -> str:
    """The current round's tag, used to namespace every file/name a
    dispatch for this round touches. Escalated rounds get a visually
    distinct prefix so a directory listing alone shows where a task's
    ladder currently stands."""
    if issue.seat_kind == "escalated":
        return f"sol{issue.escalated_rounds}"
    return f"r{issue.local_rounds}"


def triage_name(issue_id: str) -> str:
    return f"triage-{issue_id}"  # single-shot: triage is never redispatched


def launch_triage(issue_id: str, issue_path: Path) -> tuple[subprocess.Popen, Path]:
    """Triage runs directly against the main checkout (read-only exploration,
    no worktree needed) since it only writes one brief file, never code."""
    out_dir = config.ORCH_STATE_DIR / "triage" / issue_id
    out_dir.mkdir(parents=True, exist_ok=True)
    brief_path = out_dir / "brief.md"
    system_path = out_dir / "system.md"
    system_path.write_text(_combined_system(TRIAGE_TAIL.read_text()))
    status_path = out_dir / "status.json"
    task_path = out_dir / "task.md"
    task_path.write_text(
        f"Issue file: {issue_path.relative_to(config.REPO)}\n"
        f"Write the brief to: {brief_path.relative_to(config.REPO)}\n"
    )
    proc = pi.launch(
        name=triage_name(issue_id),
        cwd=config.REPO,
        brief_rel=str(task_path.relative_to(config.REPO)),
        system_rel=str(system_path.relative_to(config.REPO)),
        report_rel=str((out_dir / "triage-report.md").relative_to(config.REPO)),
        out_path=status_path,
        provider=config.LOCAL_PROVIDER, model=config.LOCAL_MODEL, thinking=config.LOCAL_THINKING,
    )
    return proc, status_path


def implementer_name(issue_id: str, tag: str) -> str:
    return f"impl-{issue_id}-{tag}"


def implementer_status_path(wt: Path, tag: str) -> Path:
    return wt / ".ds4" / f"status-{tag}.json"


def launch_implementer(
    issue_id: str, wt: Path, tag: str, brief_text: str, escalated: bool, findings_text: str | None,
) -> tuple[subprocess.Popen, Path]:
    ds4 = wt / ".ds4"
    ds4.mkdir(parents=True, exist_ok=True)
    (ds4 / "brief.md").write_text(brief_text)
    tail = IMPL_TAIL_TEMPLATE.read_text().replace("{ID}", issue_id)
    (ds4 / f"system-{tag}.md").write_text(_combined_system(tail))
    message_rel = None
    if findings_text:
        findings_path = ds4 / f"findings-{tag}.md"
        findings_path.write_text(findings_text)
        message_rel = str(findings_path.relative_to(wt))
    status_path = implementer_status_path(wt, tag)
    provider = config.IMPLEMENTER_ESCALATED_PROVIDER if escalated else config.LOCAL_PROVIDER
    model = config.IMPLEMENTER_ESCALATED_MODEL if escalated else config.LOCAL_MODEL
    thinking = "high" if escalated else config.LOCAL_THINKING
    proc = pi.launch(
        name=implementer_name(issue_id, tag),
        cwd=wt,
        brief_rel=".ds4/brief.md", system_rel=f".ds4/system-{tag}.md", report_rel=f".ds4/report-{tag}.md",
        out_path=status_path, provider=provider, model=model, thinking=thinking,
        message_rel=message_rel,
    )
    return proc, status_path


def review_name(issue_id: str, tag: str) -> str:
    return f"review-{issue_id}-{tag}"


def review_status_path(wt: Path, tag: str) -> Path:
    return wt / ".ds4" / f"review-status-{tag}.json"


def launch_review(issue_id: str, wt: Path, tag: str) -> tuple[subprocess.Popen, Path]:
    ds4 = wt / ".ds4"
    diff = subprocess.run(
        ["git", "diff", "main...HEAD"], cwd=str(wt), capture_output=True, text=True,
    ).stdout
    diff_path = ds4 / f"diff-{tag}.txt"
    diff_path.write_text(diff)
    tail = REVIEWER_TAIL.read_text().replace("{ID}", issue_id)
    system_path = ds4 / f"review-system-{tag}.md"
    system_path.write_text(_combined_system(tail))
    status_path = review_status_path(wt, tag)
    task_path = ds4 / f"review-task-{tag}.md"
    verdict_path = ds4 / f"verdict-{tag}.md"
    task_path.write_text(
        f"Read .ds4/brief.md, .ds4/report-{tag}.md and .ds4/diff-{tag}.txt. "
        f"Write your verdict to {verdict_path.relative_to(wt)}.\n"
    )
    proc = pi.launch(
        name=review_name(issue_id, tag),
        cwd=wt,
        brief_rel=str(task_path.relative_to(wt)), system_rel=str(system_path.relative_to(wt)),
        report_rel=f".ds4/review-report-{tag}.md",
        out_path=status_path,
        provider=config.REVIEWER_PROVIDER, model=config.REVIEWER_MODEL,
        thinking=config.REVIEWER_THINKING,
    )
    return proc, status_path


def read_verdict(wt: Path, tag: str) -> tuple[bool, str]:
    verdict_path = wt / ".ds4" / f"verdict-{tag}.md"
    if not verdict_path.exists():
        return False, "reviewer produced no verdict.md"
    text = verdict_path.read_text()
    approved = "VERDICT: APPROVE" in text.split("\n", 1)[0]
    return approved, text
