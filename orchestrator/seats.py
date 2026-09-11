"""Builds the concrete brief/dispatch/report file set for one issue's
current step and launches the pi-agent seat for it. This is the only module
that knows the *shape* of a triage vs. implementer vs. review dispatch;
daemon.py just calls one of these three functions and polls the result.
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


def launch_triage(issue_id: str, issue_path: Path) -> tuple[subprocess.Popen, Path]:
    """Triage runs directly against the main checkout (read-only exploration,
    no worktree needed) since it only writes one brief file, never code."""
    out_dir = config.ORCH_STATE_DIR / "triage" / issue_id
    out_dir.mkdir(parents=True, exist_ok=True)
    brief_path = out_dir / "brief.md"
    system_path = out_dir / "system.md"
    system_path.write_text(_combined_system(TRIAGE_TAIL.read_text()))
    status_path = out_dir / "status.json"
    # Triage reads the issue file itself for the report text; point it there
    # and at the exact output path via a tiny task file rather than baking
    # the report text into the shared triage-tail.md (which must stay
    # generic across every issue).
    task_path = out_dir / "task.md"
    task_path.write_text(
        f"Issue file: {issue_path.relative_to(config.REPO)}\n"
        f"Write the brief to: {brief_path.relative_to(config.REPO)}\n"
    )
    proc = pi.launch(
        name=f"triage-{issue_id}",
        cwd=config.REPO,
        brief_rel=str(task_path.relative_to(config.REPO)),
        system_rel=str(system_path.relative_to(config.REPO)),
        report_rel=str((out_dir / "triage-report.md").relative_to(config.REPO)),
        out_path=status_path,
        provider=config.LOCAL_PROVIDER, model=config.LOCAL_MODEL, thinking=config.LOCAL_THINKING,
    )
    return proc, status_path


def launch_implementer(
    issue_id: str, wt: Path, brief_text: str, escalated: bool, findings_text: str | None,
) -> tuple[subprocess.Popen, Path]:
    ds4 = wt / ".ds4"
    ds4.mkdir(parents=True, exist_ok=True)
    (ds4 / "brief.md").write_text(brief_text)
    tail = IMPL_TAIL_TEMPLATE.read_text().replace("{ID}", issue_id)
    (ds4 / "system.md").write_text(_combined_system(tail))
    message_rel = None
    if findings_text:
        (ds4 / "findings.md").write_text(findings_text)
        message_rel = ".ds4/findings.md"
    status_path = ds4 / "status.json"
    provider = config.IMPLEMENTER_ESCALATED_PROVIDER if escalated else config.LOCAL_PROVIDER
    model = config.IMPLEMENTER_ESCALATED_MODEL if escalated else config.LOCAL_MODEL
    thinking = "high" if escalated else config.LOCAL_THINKING
    proc = pi.launch(
        name=f"impl-{issue_id}",
        cwd=wt,
        brief_rel=".ds4/brief.md", system_rel=".ds4/system.md", report_rel=".ds4/report.md",
        out_path=status_path, provider=provider, model=model, thinking=thinking,
        message_rel=message_rel,
    )
    return proc, status_path


def launch_review(issue_id: str, wt: Path) -> tuple[subprocess.Popen, Path]:
    ds4 = wt / ".ds4"
    diff = subprocess.run(
        ["git", "diff", "main...HEAD"], cwd=str(wt), capture_output=True, text=True,
    ).stdout
    (ds4 / "diff.txt").write_text(diff)
    tail = REVIEWER_TAIL.read_text().replace("{ID}", issue_id)
    system_path = ds4 / "review-system.md"
    system_path.write_text(_combined_system(tail))
    status_path = ds4 / "review-status.json"
    task_path = ds4 / "review-task.md"
    task_path.write_text(
        "Read .ds4/brief.md, .ds4/report.md and .ds4/diff.txt. "
        "Write your verdict to .ds4/verdict.md.\n"
    )
    proc = pi.launch(
        name=f"review-{issue_id}",
        cwd=wt,
        brief_rel=".ds4/review-task.md", system_rel=".ds4/review-system.md",
        report_rel=".ds4/review-report.md",
        out_path=status_path,
        provider=config.REVIEWER_PROVIDER, model=config.REVIEWER_MODEL,
        thinking=config.REVIEWER_THINKING,
    )
    return proc, status_path


def read_verdict(wt: Path) -> tuple[bool, str]:
    verdict_path = wt / ".ds4" / "verdict.md"
    if not verdict_path.exists():
        return False, "reviewer produced no verdict.md"
    text = verdict_path.read_text()
    approved = "VERDICT: APPROVE" in text.split("\n", 1)[0]
    return approved, text
