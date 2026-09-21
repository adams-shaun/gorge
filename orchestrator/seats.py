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
MERGE_TAIL = config.DISPATCH_DIR / "merge-resolver-tail.md"


def _combined_system(tail_text: str) -> str:
    return config.CONTEXT_FILE.read_text() + tail_text


def round_tag(issue) -> str:
    """The current round's tag, used to namespace every file/name a
    dispatch for this round touches. Escalated rounds get a visually
    distinct prefix so a directory listing alone shows where a task's
    ladder currently stands."""
    if issue.seat_kind == "escalated":
        return f"sol{issue.escalated_rounds}"
    if issue.seat_kind == "overflow":
        return f"t{issue.local_rounds}"
    return f"r{issue.local_rounds}"


def triage_name(issue_id: str) -> str:
    return f"triage-{issue_id}"  # single-shot: triage is never redispatched


def triage_out_dir(wt: Path) -> Path:
    return wt / ".ds4" / "triage"


def launch_triage(issue_id: str, issue_path: Path, overflow: bool = False) -> tuple[subprocess.Popen, Path]:
    """Triage runs in its own detached worktree of main (git_ops.
    create_triage_worktree). It only writes one brief file, never code; the
    daemon copies that brief out and removes the worktree when triage ends.
    The status file stays under ORCH_STATE_DIR: pi-agent's wrapper writes it
    from outside the jail."""
    from . import git_ops
    wt = git_ops.create_triage_worktree(issue_id)
    out_dir = triage_out_dir(wt)
    out_dir.mkdir(parents=True, exist_ok=True)
    state_dir = config.ORCH_STATE_DIR / "triage" / issue_id
    state_dir.mkdir(parents=True, exist_ok=True)
    system_path = out_dir / "system.md"
    system_path.write_text(_combined_system(TRIAGE_TAIL.read_text()))
    task_path = out_dir / "task.md"
    task_path.write_text(
        f"Issue file: .ds4/issues/{issue_path.name}\n"
        f"Write the brief to: .ds4/triage/brief.md\n"
    )
    status_path = state_dir / "status.json"
    proc = pi.launch(
        name=triage_name(issue_id),
        cwd=wt,
        brief_rel=".ds4/triage/task.md",
        system_rel=".ds4/triage/system.md",
        report_rel=".ds4/triage/triage-report.md",
        out_path=status_path,
        provider=config.OVERFLOW_PROVIDER if overflow else config.LOCAL_PROVIDER,
        model=config.OVERFLOW_MODEL if overflow else config.LOCAL_MODEL,
        thinking=config.OVERFLOW_THINKING if overflow else config.LOCAL_THINKING,
    )
    return proc, status_path


def implementer_name(issue_id: str, tag: str) -> str:
    return f"impl-{issue_id}-{tag}"


def implementer_status_path(wt: Path, tag: str) -> Path:
    return wt / ".ds4" / f"status-{tag}.json"


def launch_implementer(
    issue_id: str, wt: Path, tag: str, brief_text: str, escalated: bool, findings_text: str | None,
    overflow: bool = False, local_escalated: bool = False,
) -> tuple[subprocess.Popen, Path]:
    ds4 = wt / ".ds4"
    ds4.mkdir(parents=True, exist_ok=True)
    (ds4 / "brief.md").write_text(brief_text)
    tail = IMPL_TAIL_TEMPLATE.read_text().replace("{ID}", issue_id).replace("{TAG}", tag)
    (ds4 / f"system-{tag}.md").write_text(_combined_system(tail))
    message_rel = None
    if findings_text:
        findings_path = ds4 / f"findings-{tag}.md"
        findings_path.write_text(findings_text)
        message_rel = str(findings_path.relative_to(wt))
    status_path = implementer_status_path(wt, tag)
    if escalated and local_escalated:
        # The paid plan is out (e.g. a week-long subscription block): run the
        # escalation round on the local seat instead of parking it in
        # `waiting` until the plan resets. Still counts against the
        # escalation ladder (MAX_ESCALATED_ROUNDS) -- only the backend differs.
        provider, model, thinking = config.LOCAL_PROVIDER, config.LOCAL_MODEL, config.LOCAL_THINKING
    elif escalated:
        provider, model, thinking = config.IMPLEMENTER_ESCALATED_PROVIDER, config.IMPLEMENTER_ESCALATED_MODEL, "high"
    elif overflow:
        provider, model, thinking = config.OVERFLOW_PROVIDER, config.OVERFLOW_MODEL, config.OVERFLOW_THINKING
    else:
        provider, model, thinking = config.LOCAL_PROVIDER, config.LOCAL_MODEL, config.LOCAL_THINKING
    proc = pi.launch(
        name=implementer_name(issue_id, tag),
        cwd=wt,
        brief_rel=".ds4/brief.md", system_rel=f".ds4/system-{tag}.md", report_rel=f".ds4/report-{tag}.md",
        out_path=status_path, provider=provider, model=model, thinking=thinking,
        message_rel=message_rel,
    )
    return proc, status_path


def merge_resolver_name(issue_id: str, tag: str) -> str:
    return f"mergefix-{issue_id}-{tag}"


def merge_resolver_status_path(wt: Path, tag: str) -> Path:
    return wt / ".ds4" / f"status-merge-{tag}.json"


def launch_merge_resolver(
    issue_id: str, wt: Path, tag: str, conflict_text: str,
) -> tuple[subprocess.Popen, Path]:
    """Resolve a rebase/merge conflict in the issue's EXISTING worktree: no
    new worktree, no new branch — the approved branch itself is completed
    in place so the daemon's normal gate-and-merge path can re-run on it.
    The conflict output plus the ground rules go in as the task brief."""
    ds4 = wt / ".ds4"
    ds4.mkdir(parents=True, exist_ok=True)
    task_path = ds4 / f"merge-conflict-{tag}.md"
    task_path.write_text(
        f"# Merge conflict on branch wt/{issue_id}\n\n"
        f"The daemon's attempt to integrate main failed with:\n\n"
        f"```\n{conflict_text[-6000:]}\n```\n\n"
        f"Resolve it in THIS worktree per your instructions, complete the git "
        f"operation, and commit.\n"
    )
    tail = MERGE_TAIL.read_text().replace("{ID}", issue_id).replace("{TAG}", tag)
    system_path = ds4 / f"system-merge-{tag}.md"
    system_path.write_text(_combined_system(tail))
    status_path = merge_resolver_status_path(wt, tag)
    proc = pi.launch(
        name=merge_resolver_name(issue_id, tag),
        cwd=wt,
        brief_rel=str(task_path.relative_to(wt)), system_rel=str(system_path.relative_to(wt)),
        report_rel=f".ds4/report-merge-{tag}.md",
        out_path=status_path,
        provider=config.LOCAL_PROVIDER, model=config.LOCAL_MODEL,
        thinking=config.LOCAL_THINKING,
    )
    return proc, status_path


def review_name(issue_id: str, tag: str) -> str:
    return f"review-{issue_id}-{tag}"


def review_status_path(wt: Path, tag: str) -> Path:
    return wt / ".ds4" / f"review-status-{tag}.json"


def launch_review(issue_id: str, wt: Path, tag: str, local: bool = False) -> tuple[subprocess.Popen, Path]:
    ds4 = wt / ".ds4"
    diff = subprocess.run(
        ["git", "diff", "main...HEAD"], cwd=str(wt), capture_output=True, text=True,
    ).stdout
    diff_path = ds4 / f"diff-{tag}.txt"
    diff_path.write_text(diff)
    touched = subprocess.run(
        ["git", "diff", "--stat", "main...HEAD"], cwd=str(wt), capture_output=True, text=True,
    ).stdout.strip()
    tail = REVIEWER_TAIL.read_text().replace("{ID}", issue_id)
    system_path = ds4 / f"review-system-{tag}.md"
    system_path.write_text(_combined_system(tail))
    status_path = review_status_path(wt, tag)
    _salvage_report(wt, tag)
    task_path = ds4 / f"review-task-{tag}.md"
    verdict_path = ds4 / f"verdict-{tag}.md"
    findings = ds4 / f"findings-{tag}.md"
    ruling = (
        f" Also read .ds4/{findings.name}: it holds the previous review and any "
        f"CONTROLLER RULING, which overrides the brief where they conflict -- "
        f"judge the diff against the ruling, not the superseded brief text."
        if findings.exists() else ""
    )
    task_path.write_text(
        f"Read .ds4/brief.md, .ds4/report-{tag}.md and .ds4/diff-{tag}.txt.{ruling}\n\n"
        f"Touched files (git diff --stat main...HEAD) -- this is the whole scope of the diff; "
        f"treat any file not listed here as untouched, and use this list directly for the "
        f"attack plan's scope and regression-surface steps instead of re-deriving it:\n\n"
        f"{touched}\n\n"
        f"Write your verdict to {verdict_path.relative_to(wt)}.\n"
    )
    proc = pi.launch(
        name=review_name(issue_id, tag),
        cwd=wt,
        brief_rel=str(task_path.relative_to(wt)), system_rel=str(system_path.relative_to(wt)),
        report_rel=f".ds4/review-report-{tag}.md",
        out_path=status_path,
        provider=config.LOCAL_PROVIDER if local else config.REVIEWER_PROVIDER,
        model=config.LOCAL_MODEL if local else config.REVIEWER_MODEL,
        thinking=config.REVIEW_LOCAL_THINKING if local else config.REVIEWER_THINKING,
    )
    return proc, status_path


def _salvage_report(wt: Path, tag: str) -> None:
    """An implementer that wrote the legacy `.ds4/report.md` instead of the
    round-tagged path must not be auto-rejected for a filename: two sol
    rounds with real, green fixes were parked on 2026-09-11 purely because
    the reviewer could not find `report-<tag>.md`. Copy it across when it is
    newer than the round's system prompt (i.e. written during this round)."""
    ds4 = wt / ".ds4"
    tagged, legacy, system = ds4 / f"report-{tag}.md", ds4 / "report.md", ds4 / f"system-{tag}.md"
    if tagged.exists() or not legacy.exists():
        return
    if system.exists() and legacy.stat().st_mtime < system.stat().st_mtime:
        return
    tagged.write_text(legacy.read_text())


def read_verdict(wt: Path, tag: str) -> tuple[bool, str]:
    verdict_path = wt / ".ds4" / f"verdict-{tag}.md"
    if not verdict_path.exists():
        return False, "reviewer produced no verdict.md"
    text = verdict_path.read_text()
    approved = "VERDICT: APPROVE" in text.split("\n", 1)[0]
    return approved, text
