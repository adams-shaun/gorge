#!/usr/bin/env python3
"""The autonomous defect pipeline for gorge.

Watches `feedback/` (player bug reports) and `.ds4/issues/inbox/` (hand-
authored tickets), turns each into a ledger entry, and drives it through
triage -> local implementation -> review (terra) -> deterministic gates ->
merge/push/deploy, escalating to a paid implementer (sol) after repeated
local failures, and parking anything it truly cannot resolve for a human.

Nothing in this file is itself an LLM agent. It is a plain state-machine
loop that calls out to pi-agent (orchestrator/seats.py) for the steps that
need a model's judgement, and to git/go/make directly (git_ops.py,
gates.py) for everything that must be a fact, not an opinion.

Run it directly (`python3 -m orchestrator.daemon`) or through
scripts/start-autonomous.sh, which also supervises the demo server and
restarts either process if it dies.
"""

from __future__ import annotations

import json
import logging
import signal
import sys
import time
from pathlib import Path

from . import config, gates, git_ops, issues, pi, seats

log = logging.getLogger("orchestrator")

_running = True


def _handle_stop(signum, frame):
    global _running
    log.info("received signal %s, finishing this tick then stopping", signum)
    _running = False


def setup_logging() -> None:
    config.ORCH_STATE_DIR.mkdir(parents=True, exist_ok=True)
    config.LOG_FILE.parent.mkdir(parents=True, exist_ok=True)
    handler = logging.FileHandler(config.LOG_FILE)
    handler.setFormatter(logging.Formatter("%(asctime)s %(levelname)s %(message)s"))
    root = logging.getLogger()
    root.setLevel(logging.INFO)
    root.addHandler(handler)
    root.addHandler(logging.StreamHandler(sys.stdout))


# --- discovery: turn raw defects into issue files ---------------------------

def discover_new_issues() -> None:
    config.FEEDBACK_DIR.mkdir(parents=True, exist_ok=True)
    config.INBOX_DIR.mkdir(parents=True, exist_ok=True)
    for report_dir in sorted(config.FEEDBACK_DIR.glob("*/")):
        if not (report_dir / "report.json").exists():
            continue
        rel = str(report_dir.relative_to(config.REPO))
        if issues.exists_for_feedback_dir(rel):
            continue
        issue = issues.new_from_feedback(report_dir)
        issue.save()
        log.info("new issue %s from feedback: %s", issue.id, issue.title)
    for md in sorted(config.INBOX_DIR.glob("*.md")):
        issue_id = f"inbox-{md.stem}"
        if issues.find(issue_id):
            continue
        issue = issues.new_from_inbox(md)
        issue.save()
        log.info("new issue %s from inbox: %s", issue.id, issue.title)


# --- per-status advancement --------------------------------------------------

def advance_new(issue: issues.Issue) -> None:
    name = seats.triage_name(issue.id)
    status_path = config.ORCH_STATE_DIR / "triage" / issue.id / "status.json"
    if not pi.already_launched(name):
        seats.launch_triage(issue.id, config.ISSUES_DIR / f"{issue.id}.md")
        issue.log("triage dispatched (local)")
        issue.save()
        return
    st = pi.read_status(status_path)
    if not pi.is_terminal(st):
        return
    if st.get("status") == "DONE":
        brief_path = config.ORCH_STATE_DIR / "triage" / issue.id / "brief.md"
        if brief_path.exists():
            issue.brief = brief_path.read_text()
            issue.status = "briefed"
            issue.log("triage complete, brief written")
        else:
            issue.status = "human_needed"
            issue.log("triage claimed DONE but wrote no brief.md")
    else:
        issue.status = "human_needed"
        issue.log(f"triage ended {st.get('status')}: {st.get('final_text', '')[:300]}")
    issue.save()


def advance_briefed(issue: issues.Issue) -> None:
    wt = git_ops.create_worktree(issue.id)
    issue.worktree = str(wt.relative_to(config.REPO))
    issue.branch = f"wt/{issue.id}"
    issue.seat_kind = "local"
    issue.local_rounds = 1
    tag = seats.round_tag(issue)
    name = seats.implementer_name(issue.id, tag)
    if pi.already_launched(name):
        return  # a prior tick crashed after launch but before save; don't double-launch
    seats.launch_implementer(issue.id, wt, tag, issue.brief, escalated=False, findings_text=None)
    issue.status = "dispatched"
    issue.log(f"implementer dispatched (local, {tag})")
    issue.save()


def _redispatch_implementer(issue: issues.Issue, findings: str) -> None:
    escalated = issue.seat_kind == "escalated"
    if escalated:
        issue.escalated_rounds += 1
    else:
        issue.local_rounds += 1
        if issue.local_rounds > config.MAX_LOCAL_ROUNDS:
            issue.seat_kind = "escalated"
            issue.escalated_rounds = 1
            escalated = True
    tag = seats.round_tag(issue)
    name = seats.implementer_name(issue.id, tag)
    if pi.already_launched(name):
        issue.status = "dispatched"
        issue.save()
        return
    wt = config.REPO / issue.worktree
    seats.launch_implementer(issue.id, wt, tag, issue.brief, escalated=escalated, findings_text=findings)
    issue.status = "dispatched"
    issue.log(f"implementer redispatched ({'escalated/sol' if escalated else 'local'}, {tag})")
    issue.save()


def advance_dispatched(issue: issues.Issue) -> None:
    wt = config.REPO / issue.worktree
    tag = seats.round_tag(issue)
    status_path = seats.implementer_status_path(wt, tag)
    st = pi.read_status(status_path)
    if not pi.is_terminal(st):
        return
    outcome = st.get("status")
    if outcome in ("DONE", "DONE_WITH_CONCERNS"):
        commits = " ".join(st.get("commits", []))
        if commits:
            issue.commits = commits
        review_name = seats.review_name(issue.id, tag)
        if pi.already_launched(review_name):
            issue.status = "review"
            issue.save()
            return
        seats.launch_review(issue.id, wt, tag)
        issue.status = "review"
        issue.log(f"implementer {outcome} ({tag}), review dispatched (terra)")
        issue.save()
    elif outcome == "NEEDS_CONTEXT":
        issue.status = "human_needed"
        issue.log(f"implementer needs context: {st.get('final_text', '')[:400]}")
        issue.save()
    else:  # BLOCKED, CAPPED, UNKNOWN -- a failed round, not a question
        if _escalation_exhausted(issue):
            issue.status = "human_needed"
            issue.log(f"implementer {outcome} ({tag}), escalation ladder exhausted")
            issue.save()
        else:
            _redispatch_implementer(issue, f"Previous round ({tag}) ended {outcome} with no usable result. Try a different approach.")


def _escalation_exhausted(issue: issues.Issue) -> bool:
    if issue.seat_kind == "escalated":
        return issue.escalated_rounds >= config.MAX_ESCALATED_ROUNDS
    return False  # a local exhaustion always has the SOL escalation left to try


def advance_review(issue: issues.Issue) -> None:
    wt = config.REPO / issue.worktree
    tag = seats.round_tag(issue)
    status_path = seats.review_status_path(wt, tag)
    st = pi.read_status(status_path)
    if not pi.is_terminal(st):
        return
    if st.get("status") != "DONE":
        # A reviewer that itself failed is treated as REQUEST_CHANGES, never
        # as a silent approval -- an unreviewed diff must never merge.
        _handle_gate_or_review_failure(issue, f"reviewer ended {st.get('status')} without a verdict")
        return
    approved, verdict_text = seats.read_verdict(wt, tag)
    if not approved:
        _handle_gate_or_review_failure(issue, verdict_text)
        return
    issue.log(f"review APPROVE ({tag}), running gates")
    _run_gates_and_merge(issue, wt)


def _handle_gate_or_review_failure(issue: issues.Issue, findings: str) -> None:
    if _escalation_exhausted(issue):
        issue.status = "human_needed"
        issue.log(f"review/gate failed and escalation ladder exhausted:\n{findings[:2000]}")
        issue.save()
        return
    _redispatch_implementer(issue, findings)


def _run_gates_and_merge(issue: issues.Issue, wt: Path) -> None:
    ok, rebase_out = git_ops.rebase_onto_main(wt)
    if not ok:
        _handle_gate_or_review_failure(issue, f"rebase onto main conflicted:\n{rebase_out[-3000:]}")
        return
    git_ops.link_node_modules(wt)
    passed, results = gates.run_all(wt)
    heads_result = next((r for r in results if r.name == "TestHeads"), None)
    if heads_result and not heads_result.ok:
        if config.AUTO_ACCEPT_HEAD_MOVES:
            cr_ok = next((r.ok for r in results if r.name == "CR conformance"), False)
            sim_ok = next((r.ok for r in results if r.name == "make sim"), False)
            if cr_ok and sim_ok:
                _apply_head_moves(issue, wt, heads_result.output)
                heads_recheck = gates.test_heads(wt)
                if heads_recheck.ok:
                    git_ops.commit_all(
                        wt, f"test(rules): pin the head(s) moved by {issue.id} (autonomous orchestrator)",
                    )
                else:
                    passed = False
            else:
                passed = False
        else:
            passed = False
    if not passed:
        fail_summary = "\n\n".join(f"### {r.name}\n{r.output[-3000:]}" for r in results if not r.ok)
        _handle_gate_or_review_failure(issue, f"gates failed:\n{fail_summary}")
        return
    _merge_push_deploy(issue, wt)


def _apply_head_moves(issue: issues.Issue, wt: Path, heads_output: str) -> None:
    import re
    pattern = re.compile(r"(\d+) seats: chain head ([0-9a-f]{16}), golden [0-9a-f]{16}")
    for seat_count, new_hash in pattern.findall(heads_output):
        git_ops.apply_head_move(
            wt, int(seat_count), new_hash,
            reason=f"resolving {issue.id} ({issue.title[:80]})",
        )
        issue.log(f"auto-accepted {seat_count}-seat head move to {new_hash}")


def _merge_push_deploy(issue: issues.Issue, wt: Path) -> None:
    sha = git_ops.head_sha(wt)
    message = (
        f"merge({issue.id}): {issue.title[:72]}\n\n"
        f"Autonomous orchestrator: triaged from {issue.source}, implemented by "
        f"{issue.seat_kind or 'local'} ({issue.local_rounds} local / "
        f"{issue.escalated_rounds} escalated round(s)), reviewed by terra, "
        f"gates clean.\n"
    )
    ok, out = git_ops.merge_to_main(f"wt/{issue.id}", message)
    if not ok:
        issue.status = "human_needed"
        issue.log(f"merge failed:\n{out[-2000:]}")
        issue.save()
        return
    push_ok, push_out = git_ops.push_main()
    deploy_ok, deploy_out = git_ops.deploy_demo()
    git_ops.refresh_ledger()
    git_ops.remove_worktree(issue.id)
    issue.status = "merged"
    issue.closed_by = git_ops.main_sha()
    issue.log(
        f"merged as {issue.closed_by}; push {'ok' if push_ok else 'FAILED: ' + push_out[-500:]}; "
        f"deploy {'ok' if deploy_ok else 'FAILED: ' + deploy_out[-500:]}"
    )
    issue.save()
    log.info("issue %s MERGED as %s", issue.id, issue.closed_by)


ADVANCERS = {
    "new": advance_new,
    "briefed": advance_briefed,
    "dispatched": advance_dispatched,
    "review": advance_review,
}


def tick() -> None:
    if git_ops.is_paused():
        log.debug("paused (touch %s to resume)", config.PAUSE_FILE)
        return
    discover_new_issues()
    for issue in issues.open_issues():
        advancer = ADVANCERS.get(issue.status)
        if advancer is None:
            continue
        try:
            advancer(issue)
        except Exception:
            log.exception("issue %s: advancer for status=%s raised", issue.id, issue.status)


def main() -> None:
    setup_logging()
    signal.signal(signal.SIGTERM, _handle_stop)
    signal.signal(signal.SIGINT, _handle_stop)
    log.info("orchestrator starting, poll=%ss", config.POLL_SECONDS)
    while _running:
        tick()
        time.sleep(config.POLL_SECONDS)
    log.info("orchestrator stopped")


if __name__ == "__main__":
    main()
