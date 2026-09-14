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
import shutil
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


_DEPENDS_RE = None


def _unmet_dependency(text: str) -> str | None:
    """Inbox tickets may carry `Depends-On: <issue-id>[, <issue-id>]` lines.
    The issue is created (so it shows in the ledger) at once, but is not
    triaged -- triage reads main -- until every named issue is `merged`. This is how tickets that share hot files
    are serialized: parallel seats editing the same module make every
    rebase and review after the first one worthless."""
    import re
    global _DEPENDS_RE
    if _DEPENDS_RE is None:
        _DEPENDS_RE = re.compile(r"^Depends-On:\s*(.+)$", re.MULTILINE)
    for m in _DEPENDS_RE.finditer(text):
        for dep in (x.strip() for x in m.group(1).split(",")):
            if not dep:
                continue
            found = issues.find(dep)
            if found is None or found.status not in ("merged", "superseded"):
                return dep
    return None


def _slot_free(issue: issues.Issue, paid: bool, what: str) -> bool:
    if paid:
        running = pi.running_names(config.IMPLEMENTER_ESCALATED_MODEL, config.REVIEWER_MODEL)
        cap = config.MAX_PAID_SEATS
    else:
        running = pi.running_names(config.LOCAL_MODEL)
        cap = config.MAX_LOCAL_SEATS
    if len(running) < cap:
        return True
    log.debug("issue %s: %s held, %d %s seats running (cap %d)",
              issue.id, what, len(running), "paid" if paid else "local", cap)
    return False


def _local_slot_free(issue: issues.Issue, what: str) -> bool:
    return _slot_free(issue, False, what)


def _local_tier_seat(issue: issues.Issue, what: str) -> str | None:
    """'local' if a glm slot is free, else 'overflow' (terra) if a paid slot
    is free, else None -- the work waits."""
    if _slot_free(issue, False, what):
        return "local"
    if _slot_free(issue, True, what + " (terra overflow)"):
        return "overflow"
    return None


# --- per-status advancement --------------------------------------------------

def advance_new(issue: issues.Issue) -> None:
    if issue.source == "inbox" and "## Done means" in issue.report:
        # A hand-authored ticket that is already a full brief (it carries a
        # "Done means" checklist) goes straight to implementation: a triage
        # rewrite by the local seat can only lose fidelity.
        issue.brief = issue.report
        issue.status = "briefed"
        issue.log("inbox ticket is already a brief; triage skipped")
        issue.save()
        return
    name = seats.triage_name(issue.id)
    status_path = config.ORCH_STATE_DIR / "triage" / issue.id / "status.json"
    if not pi.already_launched(name):
        seat = _local_tier_seat(issue, "triage")
        if seat is None:
            return
        seats.launch_triage(issue.id, config.ISSUES_DIR / f"{issue.id}.md", overflow=(seat == "overflow"))
        issue.log("triage dispatched (" + ("terra, local cap full" if seat == "overflow" else "local") + ")")
        issue.save()
        return
    st = pi.read_status(status_path)
    if not pi.is_terminal(st):
        return
    state_dir = config.ORCH_STATE_DIR / "triage" / issue.id
    twt = git_ops.triage_worktree_path(issue.id)
    if twt.exists():
        # Keep the triage record (brief, report, system) beside its status,
        # then drop the worktree -- the brief lives on in the issue file.
        for f in seats.triage_out_dir(twt).glob("*"):
            if f.is_file():
                shutil.copy2(f, state_dir / f.name)
        git_ops.remove_triage_worktree(issue.id)
    if st.get("status") == "DONE":
        brief_path = state_dir / "brief.md"
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
    seat = _local_tier_seat(issue, "implementer round 1")
    if seat is None:
        return
    wt = git_ops.create_worktree(issue.id)
    issue.worktree = str(wt.relative_to(config.REPO))
    issue.branch = f"wt/{issue.id}"
    issue.seat_kind = seat
    issue.local_rounds = 1
    tag = seats.round_tag(issue)
    name = seats.implementer_name(issue.id, tag)
    if pi.already_launched(name):
        return  # a prior tick crashed after launch but before save; don't double-launch
    seats.launch_implementer(issue.id, wt, tag, issue.brief, escalated=False, findings_text=None,
                             overflow=(seat == "overflow"))
    issue.status = "dispatched"
    issue.log(f"implementer dispatched ({'terra, local cap full' if seat == 'overflow' else 'local'}, {tag})")
    issue.save()


def _pending_findings_path(issue: issues.Issue) -> Path:
    return config.ORCH_STATE_DIR / "pending" / f"{issue.id}.md"


def _redispatch_implementer(issue: issues.Issue, findings: str) -> None:
    escalated = issue.seat_kind == "escalated"
    will_escalate = escalated or issue.local_rounds + 1 > config.MAX_LOCAL_ROUNDS
    if will_escalate:
        seat = "escalated" if _slot_free(issue, True, "redispatch") else None
    else:
        seat = _local_tier_seat(issue, "redispatch")
    if seat is None:
        # Park with the findings; advance_waiting retries each tick. Nothing
        # above (a gate run, a verdict read) is repeated while parked.
        pending = _pending_findings_path(issue)
        pending.parent.mkdir(parents=True, exist_ok=True)
        pending.write_text(findings)
        if issue.status != "waiting":
            issue.status = "waiting"
            issue.log(f"fix round due ({'sol' if will_escalate else 'local'}); waiting for a seat slot")
            issue.save()
        return
    _pending_findings_path(issue).unlink(missing_ok=True)
    if escalated:
        issue.escalated_rounds += 1
    else:
        issue.local_rounds += 1
        if issue.local_rounds > config.MAX_LOCAL_ROUNDS:
            issue.seat_kind = "escalated"
            issue.escalated_rounds = 1
            escalated = True
        else:
            issue.seat_kind = seat  # local, or terra overflow when glm is full
    tag = seats.round_tag(issue)
    name = seats.implementer_name(issue.id, tag)
    if pi.already_launched(name):
        issue.status = "dispatched"
        issue.log(f"redispatch to {tag} skipped: a launch marker for {name} already exists (manual reset?) -- will poll its existing status file")
        issue.save()
        return
    wt = config.REPO / issue.worktree
    overflow = issue.seat_kind == "overflow"
    seats.launch_implementer(issue.id, wt, tag, issue.brief, escalated=escalated, findings_text=findings,
                             overflow=overflow)
    issue.status = "dispatched"
    issue.log(f"implementer redispatched ({'escalated/sol' if escalated else 'terra overflow' if overflow else 'local'}, {tag})")
    issue.save()


def advance_waiting(issue: issues.Issue) -> None:
    pending = _pending_findings_path(issue)
    findings = pending.read_text() if pending.exists() else "Previous round failed; see the last findings file."
    _redispatch_implementer(issue, findings)


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
        if git_ops.commits_ahead(issue.id) == 0:
            dirty = git_ops.uncommitted_paths(wt)
            if dirty:
                # A seat that reports DONE with its fix still in the working
                # tree would otherwise be reviewed as an empty diff, approved,
                # and closed as superseded -- deleting the worktree and the
                # work with it (fb-20260914T114737Z-5aaad6ad lost a full fix
                # that way). Send it back to commit instead.
                listing = "\n".join(f"- {p}" for p in dirty[:40])
                _redispatch_implementer(issue, f"Round {tag} reported {outcome} but committed nothing; "
                                        f"these changes are still uncommitted in the worktree:\n{listing}\n\n"
                                        "Verify them against the brief, then commit (stage specific paths). "
                                        "Nothing is reviewed until it is committed.")
                return
        review_name = seats.review_name(issue.id, tag)
        if pi.already_launched(review_name):
            issue.status = "review"
            issue.save()
            return
        if not _slot_free(issue, True, "review"):
            return  # stays dispatched; the terminal status is re-read next tick
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
    if git_ops.commits_ahead(issue.id) == 0 and git_ops.uncommitted_paths(wt):
        issue.status = "human_needed"
        issue.log("approved with no commits but uncommitted changes in the worktree; parked, worktree kept")
        issue.save()
        return
    if git_ops.commits_ahead(issue.id) == 0:
        # An approved round with no commits is a "nothing to do here" outcome
        # (superseded / already covered). Never record it as a merge: the old
        # path logged "merged as <current main>", which named someone else's
        # commit as this issue's fix.
        _archive_round_files(issue, wt)
        git_ops.remove_worktree(issue.id)
        issue.status = "superseded"
        issue.closed_by = ""
        issue.log("approved with no commits; closed as superseded (see last report)")
        issue.save()
        log.info("issue %s SUPERSEDED (no commits)", issue.id)
        return
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
        # Always keep the failure in the issue's own history: the findings
        # file is only written when a redispatch actually launches, and a
        # skipped launch (already-launched marker) used to lose the output.
        issue.log("gates failed: " + ", ".join(r.name for r in results if not r.ok)
                  + "\n" + fail_summary[-1500:])
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
    # The commit-msg hook requires a Test-Budget-Approved trailer on any
    # commit whose diff raises a budget_s -- including this merge commit, which
    # carries the branch's raise. Re-state every such trailer the branch's own
    # commits already carry (never invent one).
    import subprocess as _sp
    trailers = sorted(set(l.strip() for l in _sp.run(
        ["git", "log", "--format=%B", f"main..wt/{issue.id}"], cwd=str(config.REPO),
        capture_output=True, text=True).stdout.splitlines() if l.startswith("Test-Budget-Approved:")))
    message = (
        f"merge({issue.id}): {issue.title[:72]}\n\n"
        f"Autonomous orchestrator: triaged from {issue.source}, implemented by "
        f"{issue.seat_kind or 'local'} ({issue.local_rounds} local / "
        f"{issue.escalated_rounds} escalated round(s)), reviewed by terra, "
        f"gates clean.\n"
    )
    if trailers:
        message += "\n" + "\n".join(trailers) + "\n"
    ok, out = git_ops.merge_to_main(f"wt/{issue.id}", message)
    if not ok:
        issue.status = "human_needed"
        issue.log(f"merge failed:\n{out[-2000:]}")
        issue.save()
        return
    push_ok, push_out = git_ops.push_main()
    deploy_ok, deploy_out = git_ops.deploy_demo()
    git_ops.refresh_ledger()
    _archive_round_files(issue, wt)
    git_ops.remove_worktree(issue.id)
    issue.status = "merged"
    issue.closed_by = git_ops.main_sha()
    issue.log(
        f"merged as {issue.closed_by}; push {'ok' if push_ok else 'FAILED: ' + push_out[-500:]}; "
        f"deploy {'ok' if deploy_ok else 'FAILED: ' + deploy_out[-500:]}"
    )
    issue.save()
    log.info("issue %s MERGED as %s", issue.id, issue.closed_by)


def _archive_round_files(issue: issues.Issue, wt: Path) -> None:
    """Keep every brief/report/findings/verdict/status file after the
    worktree is removed: the review verdicts are the audit trail for an
    unattended merge, and removing the worktree used to delete them."""
    import shutil
    dest = config.ORCH_STATE_DIR / "archive" / issue.id
    dest.mkdir(parents=True, exist_ok=True)
    for pattern in ("brief.md", "report*.md", "findings-*.md", "verdict-*.md", "*status*.json", "diff-*.txt"):
        for f in (wt / ".ds4").glob(pattern):
            shutil.copy2(f, dest / f.name)


ADVANCERS = {
    "new": advance_new,
    "briefed": advance_briefed,
    "dispatched": advance_dispatched,
    "waiting": advance_waiting,
    "review": advance_review,
}


def tick() -> None:
    if git_ops.is_paused():
        log.debug("paused (touch %s to resume)", config.PAUSE_FILE)
        return
    discover_new_issues()
    # In-flight tickets claim free seat slots before new work does.
    order = {"review": 0, "dispatched": 1, "waiting": 2, "briefed": 3, "new": 4}
    for issue in sorted(issues.open_issues(), key=lambda i: order.get(i.status, 5)):
        advancer = ADVANCERS.get(issue.status)
        if advancer is None:
            continue
        # Inbox tickets carry Depends-On in the report; triage writes it into
        # the brief. Either one holds the issue until the dependency closes.
        if issue.status in ("new", "briefed"):
            blocker = _unmet_dependency(issue.report + "\n" + (issue.brief or ""))
            if blocker:
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
