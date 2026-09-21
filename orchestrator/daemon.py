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

import fcntl
import json
import logging
import re
import shutil
import signal
import sys
import time
from pathlib import Path

from . import config, gates, git_ops, issues, pi, seats

log = logging.getLogger("orchestrator")

_running = True

# Held for the daemon's whole life. See acquire_single_instance_lock.
LOCK_FILE = config.ORCH_STATE_DIR / "daemon.lock"
_lock_handle = None


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


def _worktree_cleanup_due() -> bool:
    """Once per calendar day is plenty -- a `git status`/`rev-list` per
    worktree every 60s tick would spam the log with the same 'left for
    manual review' line on every orphan forever, for zero new information."""
    marker = config.ORCH_STATE_DIR / "worktree-cleanup-last-run"
    try:
        age = time.time() - marker.stat().st_mtime
    except OSError:
        age = float("inf")
    if age < 86400:
        return False
    marker.parent.mkdir(parents=True, exist_ok=True)
    marker.write_text(issues._now())
    return True


def cleanup_orphaned_worktrees() -> None:
    """A `.worktrees/<name>` directory the ledger no longer references (its
    issue merged/superseded and its worktree field is now stale, or someone
    made a one-off investigation worktree by hand and forgot it) never gets
    cleaned up on its own -- git and the orchestrator both leave it in place
    forever.

    Removal is deliberately conservative: a directory is only ever force-
    removed when ALL of these hold --
      - no OPEN issue's `worktree:` field names it (an issue still using it
        is obviously not orphaned), AND
      - its directory name does not appear anywhere in any open issue's
        brief/report/history text either -- this is what protects an
        archived salvage copy like `<id>-v1` that a brief points back to by
        name for cherry-picking (see inbox-rv2d-static-pt-variable-and-
        derived-types' own v1/v2 split, 2026-09-17): the field is empty (the
        LIVE worktree took over that field) but the text reference is real
        and must not be swept, AND
      - `git status --porcelain` in it is completely clean (an uncommitted
        change is exactly the kind of thing 'seats finish uncommitted' warns
        never to discard silently), AND
      - it carries zero commits `main` does not already have (a clean tree
        with unique commits nobody has looked at yet is still real, unmerged
        work -- just not linked from the ledger; a human decides its fate,
        not a nightly sweep).
    Anything failing any of these is logged and left exactly where it is."""
    if not _worktree_cleanup_due():
        return
    git_ops.prune_worktrees()

    referenced_names = set()
    text_blob_parts = []
    for issue in issues.open_issues():
        if issue.worktree:
            referenced_names.add(Path(issue.worktree).name)
        text_blob_parts.extend([issue.report, issue.brief, issue.history])
    text_blob = "\n".join(text_blob_parts)

    for path in git_ops.list_worktree_dirs():
        name = path.name
        if name in referenced_names or name in text_blob:
            continue
        if git_ops.worktree_dirty(path):
            log.warning("orphaned worktree %s has uncommitted changes; left for manual review", name)
            continue
        ahead = git_ops.commits_ahead_of_main(path)
        if ahead != 0:
            log.warning("orphaned worktree %s: %s commit(s) vs main (0=none unique, -1=undetermined); left for manual review",
                        name, ahead)
            continue
        git_ops.force_remove_worktree_and_branch(path)
        log.info("removed orphaned worktree %s (unreferenced, clean, no unique commits)", name)


def discover_agent_filed_tickets() -> None:
    """A seat is jailed to its own worktree and cannot write
    `.ds4/issues/inbox/` directly, so it files a new ticket, a story split, or
    a follow-up by dropping a file at `.ds4/new-tickets/*.md` inside ITS OWN
    worktree instead. Scan every open issue with a worktree on disk for such
    files each tick, turn each into a real issue, and set the source file
    aside (never delete -- it is the audit trail of what a seat asked for)
    so it is never re-ingested.

    Runs over `issues.open_issues()`, not just dispatched ones: a seat's own
    report/status write and this ticket file can land in the same turn, and
    the parent issue may already have moved to review/gate by the next tick.
    """
    for parent in issues.open_issues():
        if not parent.worktree:
            continue
        new_dir = config.REPO / parent.worktree / ".ds4" / "new-tickets"
        if not new_dir.is_dir():
            continue
        for path in sorted(new_dir.glob("*.md")):
            issue = issues.new_from_agent_ticket(parent.id, path)
            issue.save()
            parent.log(f"filed follow-up ticket {issue.id}: {issue.title}")
            parent.save()
            # ".filed" appended, not replacing ".md" -- the glob above is
            # "*.md" and must never re-match its own already-ingested output.
            path.rename(path.with_name(path.name + ".filed"))
            log.info("new issue %s filed by %s: %s", issue.id, parent.id, issue.title)


def _unmet_dependency(issue: issues.Issue) -> str | None:
    """The first dependency of `issue` that has not closed yet, or None.

    Inbox tickets may carry `Depends-On: <issue-id>[, <issue-id>]` lines. The
    issue is created (so it shows in the ledger) at once, but is not triaged --
    triage reads main -- until every named issue is `merged` or `superseded`.
    This is how tickets that share hot files are serialized: parallel seats
    editing the same module make every rebase and review after the first one
    worthless.

    The ids come from Issue.depends_on, the same parse cmd/ledger uses for the
    ledger's dependency paths, so a blocked item and the reason shown for it
    can never disagree. An id no issue file declares counts as unmet: a
    dependency nobody can find is not a satisfied one.
    """
    for dep in issue.depends_on:
        found = issues.find(dep)
        if found is None or found.status not in ("merged", "superseded"):
            return dep
    return None


def _slot_free(issue: issues.Issue, paid: bool, what: str) -> bool:
    if paid and config.PAID_OFF_FILE.exists():
        log.debug("issue %s: %s held, paid provider marked off (%s)", issue.id, what, config.PAID_OFF_FILE)
        return False
    if not paid and not _local_endpoint_up():
        return False
    if paid:
        running = pi.running_names(config.IMPLEMENTER_ESCALATED_MODEL, config.REVIEWER_MODEL, config.OVERFLOW_MODEL)
        cap = config.MAX_PAID_SEATS
    else:
        # Box-wide: 2026-09-15 lockup traced to per-repo scoping letting other
        # repos' fleets stack unbounded (repeated cgroup OOM kills of glm
        # "main" processes, 13:03-14:10, eventually took out redis too).
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

ENDPOINT_DOWN_RE = None  # compiled lazily below


def _endpoint_failed(st: dict | None) -> bool:
    """True when a seat ended because its model endpoint was unreachable (a
    pod still loading after a reboot, a gateway 503) rather than because the
    model failed the task. Only the agent harness's own errorMessage fields
    count, so a seat's test output that happens to say "Connection refused"
    does not."""
    global ENDPOINT_DOWN_RE
    import re
    if ENDPOINT_DOWN_RE is None:
        ENDPOINT_DOWN_RE = re.compile(
            r'"errorMessage":"[^"]*(upstream connect error|Connection refused|no healthy upstream|503 Service Unavailable)')
    if not st:
        return False
    for key in ("transcript", "events"):
        p = st.get(key)
        if not p:
            continue
        try:
            text = Path(p).read_text(errors="replace")
        except OSError:
            continue
        if ENDPOINT_DOWN_RE.search(text):
            return True
    return False


_local_probe: tuple[float, bool] = (0.0, True)


def _local_endpoint_up() -> bool:
    """Probe the local model endpoint (cached 30s). Any HTTP answer below 500
    with the seat credential means the upstream is serving; a connection error
    or a 5xx means it is down or still loading, so no local seat is launched
    into it."""
    global _local_probe
    now = time.time()
    if now - _local_probe[0] < 30:
        return _local_probe[1]
    import os
    import urllib.error
    import urllib.request
    up = True
    try:
        models = json.loads(Path.home().joinpath(".pi/agent/models.json").read_text())
        base = models.get("providers", models)[config.LOCAL_PROVIDER]["baseUrl"].rstrip("/")
        req = urllib.request.Request(base + "/models")
        key = os.environ.get("BM_LLMS_API_KEY")
        if key:
            req.add_header("Authorization", "Bearer " + key)
        try:
            with urllib.request.urlopen(req, timeout=5) as r:
                up = r.status < 500
        except urllib.error.HTTPError as e:
            up = e.code < 500
    except (urllib.error.URLError, OSError, KeyError, ValueError):
        up = False
    if up != _local_probe[1]:
        log.warning("local model endpoint %s", "back up" if up else "DOWN; local seats held")
    _local_probe = (now, up)
    return up


SEAT_START_GRACE_S = 180


def _all_seat_models() -> tuple[str, ...]:
    return (config.LOCAL_MODEL, config.OVERFLOW_MODEL, config.IMPLEMENTER_ESCALATED_MODEL, config.REVIEWER_MODEL)


def _seat_dead(name: str, st: dict | None) -> bool:
    """A seat that was launched, never wrote a terminal status, and has no
    live process -- killed by a reboot or an OOM. Without this the issue waits
    forever on a status that will never arrive. The launch log's age gives a
    starting seat time to appear in /proc."""
    if pi.is_terminal(st) or not pi.already_launched(name):
        return False
    if name in pi.running_names(*_all_seat_models()):
        return False
    try:
        age = time.time() - pi.launch_log_path(name).stat().st_mtime
    except OSError:
        return False
    return age > SEAT_START_GRACE_S


def _relaunch_implementer_round(issue: issues.Issue, wt: Path, tag: str, why: str) -> None:
    """Re-run the CURRENT round under the same tag, uncounted: the seat died
    (or never started) without a result. Its uncommitted work and the round's
    findings stay in the worktree."""
    kind = issue.seat_kind
    paid = kind in ("escalated", "overflow")
    local_escalated = False
    if paid and not _slot_free(issue, True, f"rerun {tag}"):
        if kind == "escalated" and config.ESCALATION_LOCAL_FALLBACK and _local_slot_free(issue, f"rerun {tag} (escalation, local fallback)"):
            local_escalated = True
        else:
            return
    elif not paid and not _slot_free(issue, False, f"rerun {tag}"):
        return
    findings_path = wt / ".ds4" / f"findings-{tag}.md"
    prior = findings_path.read_text() if findings_path.exists() else ""
    note = (f"NOTE: an earlier run of round {tag} was lost ({why}) before it reported. Any uncommitted work "
            "it left is still in the worktree: check `git status`, keep what is sound, and finish the brief.\n\n")
    seats.launch_implementer(issue.id, wt, tag, issue.brief, escalated=(kind == "escalated"),
                             findings_text=note + prior, overflow=(kind == "overflow"), local_escalated=local_escalated)
    issue.log(f"implementer {tag} relaunched ({why}); round not counted")
    issue.save()


def advance_new(issue: issues.Issue) -> None:
    if issue.source in ("inbox", "agent") and "## Done means" in issue.report:
        # A hand-authored ticket that is already a full brief (it carries a
        # "Done means" checklist) goes straight to implementation: a triage
        # rewrite by the local seat can only lose fidelity. This also covers
        # agent-filed tickets whose body IS a brief (a shard/enumerator seat
        # filing deck-census tickets through .ds4/new-tickets/): same
        # reasoning -- the seat authored the brief; triage can only lose it.
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
    if _seat_dead(name, st):
        _set_aside(pi.launch_log_path(name))
        _set_aside(status_path)
        git_ops.remove_triage_worktree(issue.id)
        issue.log("triage seat died without a result; relaunch queued")
        issue.save()
        return
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
    elif _endpoint_failed(st):
        _set_aside(pi.launch_log_path(name))
        _set_aside(status_path)
        issue.log("triage failed on an unreachable model endpoint; relaunch queued")
    else:
        issue.status = "human_needed"
        issue.log(f"triage ended {st.get('status')}: {st.get('final_text', '')[:300]}")
    issue.save()


def advance_briefed(issue: issues.Issue) -> None:
    # tag is always "r1" here (issue.local_rounds is about to be (re)set to 1
    # below), so a controller reset back to "briefed" for redispatch -- not
    # just the ordinary first-ever triage->briefed transition -- can find an
    # "r1" launch marker still sitting from days-old original attempt. Without
    # this check that marker makes every tick's already_launched() true
    # forever: no error, no history line, no dispatch -- a silent permanent
    # stall (seen live 2026-09-17 on inbox-rv2c/-rv2d after a manual reset).
    name = seats.implementer_name(issue.id, "r1")
    if pi.already_launched(name) and _seat_dead(name, pi.read_status(
        seats.implementer_status_path(config.WORKTREES_DIR / issue.id, "r1"))):
        _set_aside(pi.launch_log_path(name))
        _set_aside(seats.implementer_status_path(config.WORKTREES_DIR / issue.id, "r1"))
        issue.log("stale r1 launch marker from a prior attempt set aside; redispatching fresh")
        issue.save()
        return
    seat = _local_tier_seat(issue, "implementer round 1")
    if seat is None:
        return
    wt = git_ops.create_worktree(issue.id)
    issue.worktree = str(wt.relative_to(config.REPO))
    # Don't clobber a deliberately-set custom branch: a controller reset can
    # point `worktree` at a path that's actually checked out on a differently
    # -named branch (e.g. inbox-rv2d-...-v2, cut fresh from main after the
    # original wt/inbox-rv2d-... was abandoned 459 commits stale with an
    # unresolved conflict, 2026-09-17). Overwriting `branch` back to the
    # default `wt/{id}` here left the issue record pointing at the WRONG,
    # abandoned branch while the worktree itself was on the right one --
    # caught only because the eventual merge attempt would have merged
    # whichever branch that stale field named, not what was actually
    # reviewed and gated.
    if not issue.branch:
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
    local_escalated = False
    if will_escalate:
        if _slot_free(issue, True, "redispatch"):
            seat = "escalated"
        elif config.ESCALATION_LOCAL_FALLBACK and _local_slot_free(issue, "redispatch (escalation, local fallback)"):
            seat = "escalated"
            local_escalated = True
        else:
            seat = None
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
        stale_status = seats.implementer_status_path(config.REPO / issue.worktree, tag)
        if pi.is_terminal(pi.read_status(stale_status)):
            # A FINISHED round already owns this name, so it cannot be a launch
            # that is still starting up. That happens when an issue's round
            # counters roll back -- on 2026-09-14 duplicate daemons saved stale
            # copies of five issues -- and polling the old status would replay
            # an old round instead of ever running a new one. Set it aside,
            # exactly as a provider cut-off does, and launch this round fresh.
            _set_aside(stale_status)
            _set_aside(pi.launch_log_path(name))
            issue.log(f"round {tag} name was held by an already-finished run; set it aside, relaunching fresh")
        else:
            issue.status = "dispatched"
            issue.log(f"redispatch to {tag} skipped: a launch marker for {name} already exists (manual reset?) -- will poll its existing status file")
            issue.save()
            return
    wt = config.REPO / issue.worktree
    overflow = issue.seat_kind == "overflow"
    seats.launch_implementer(issue.id, wt, tag, issue.brief, escalated=escalated, findings_text=findings,
                             overflow=overflow, local_escalated=local_escalated)
    issue.status = "dispatched"
    issue.log(f"implementer redispatched ("
              f"{'escalated/local, paid seats unavailable' if local_escalated else 'escalated/sol' if escalated else 'terra overflow' if overflow else 'local'}, {tag})")
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
    name = seats.implementer_name(issue.id, tag)
    if _seat_dead(name, st):
        _set_aside(pi.launch_log_path(name))
        _set_aside(status_path)
        issue.log(f"implementer {tag} seat died without a result")
        issue.save()
    if not pi.is_terminal(st) and not pi.already_launched(name):
        _relaunch_implementer_round(issue, wt, tag, "seat died or never started")
        return
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
            stale_review_status = seats.review_status_path(wt, tag)
            if pi.is_terminal(pi.read_status(stale_review_status)):
                # Same class of bug as the implementer-side stale marker (see
                # _redispatch_implementer): a round-tag is reused across
                # cycles, so a FINISHED review from a previous cycle looks
                # like "already reviewing" and its stale verdict gets read as
                # this cycle's answer. Caught 2026-09-14 on
                # inbox-fbrepro2-repro-cli: the ladder logged
                # escalation-exhausted using a verdict.md from 8 hours
                # earlier while the current round's own report said DONE with
                # no concerns. Set the stale verdict aside and review fresh.
                _set_aside(stale_review_status)
                _set_aside(wt / ".ds4" / f"verdict-{tag}.md")
                _set_aside(pi.launch_log_path(review_name))
                issue.log(f"review {tag} name was held by an already-finished run; set it aside, reviewing fresh")
            else:
                issue.status = "review"
                issue.save()
                return
        local_review = False
        if not _slot_free(issue, True, "review"):
            # The paid plan is spent or busy. Rather than park a finished round
            # in `dispatched` until the plan resets, review it on the local
            # seat -- the hard gates are identical either way.
            if not (config.REVIEW_LOCAL_FALLBACK and _local_slot_free(issue, "review (local fallback)")):
                return  # stays dispatched; the terminal status is re-read next tick
            local_review = True
        seats.launch_review(issue.id, wt, tag, local=local_review)
        issue.status = "review"
        issue.log(f"implementer {outcome} ({tag}), review dispatched "
                  f"({'local, paid seats unavailable' if local_review else 'terra'})")
        issue.save()
    elif outcome == "NEEDS_CONTEXT":
        issue.status = "human_needed"
        issue.log(f"implementer needs context: {st.get('final_text', '')[:400]}")
        issue.save()
    elif _endpoint_failed(st):
        # The model endpoint was unreachable: not the seat's failure. Set the
        # round aside; the next tick relaunches the same round, uncounted, once
        # a slot (and, for local seats, a healthy endpoint) is available.
        _set_aside(status_path)
        _set_aside(pi.launch_log_path(name))
        issue.log(f"implementer {tag} failed on an unreachable model endpoint; round not counted, rerun queued")
        issue.save()
    elif _provider_cut_off(st):
        # The provider stopped the seat mid-round. Re-run the SAME round once
        # a paid slot opens: set the round's status and launch record aside so
        # the tag and name can be reused, and undo the round count the
        # redispatch is about to add back. The seat's uncommitted work stays in
        # the worktree for the rerun to continue.
        _mark_paid_off(issue, f"implementer {tag}")
        _set_aside(status_path)
        _set_aside(pi.launch_log_path(seats.implementer_name(issue.id, tag)))
        if issue.seat_kind == "escalated":
            issue.escalated_rounds -= 1
        else:
            issue.local_rounds -= 1
        issue.log(f"implementer {tag} cut off by the provider usage limit; round not counted, rerun queued")
        _redispatch_implementer(issue, f"Round {tag} was cut off by the provider's usage limit, not by a failure. "
                                "Your uncommitted work from that round is still in the worktree: review it, "
                                "continue from where it stopped, and finish the brief.")
    else:  # BLOCKED, CAPPED, UNKNOWN -- a failed round, not a question
        if _escalation_exhausted(issue):
            issue.status = "human_needed"
            issue.log(f"implementer {outcome} ({tag}), escalation ladder exhausted")
            issue.save()
        else:
            _redispatch_implementer(issue, f"Previous round ({tag}) ended {outcome} with no usable result. Try a different approach.")


PROVIDER_LIMIT_MARKERS = ("usage limit has been reached",)


def _provider_cut_off(st: dict | None) -> bool:
    """True when a seat ended because the paid provider refused more work
    (plan usage limit), not because the model failed the task. Such a round
    must not count against the ticket's escalation ladder."""
    if not st:
        return False
    for key in ("transcript", "events"):
        p = st.get(key)
        if not p:
            continue
        try:
            text = Path(p).read_text(errors="replace")
        except OSError:
            continue
        if any(m in text for m in PROVIDER_LIMIT_MARKERS):
            return True
    return False


def _mark_paid_off(issue: issues.Issue, what: str) -> None:
    if not config.PAID_OFF_FILE.exists():
        config.PAID_OFF_FILE.write_text(f"{issues._now()} {issue.id} {what}: provider usage limit reached\n")
        log.warning("paid provider usage limit reached (%s %s); paid seats off until %s is removed",
                    issue.id, what, config.PAID_OFF_FILE)


def _set_aside(path: Path) -> None:
    if path.exists():
        path.rename(path.with_name(path.stem + ".cutoff" + path.suffix))


def _escalation_exhausted(issue: issues.Issue) -> bool:
    if issue.seat_kind == "escalated":
        return issue.escalated_rounds >= config.MAX_ESCALATED_ROUNDS
    return False  # a local exhaustion always has the SOL escalation left to try


def advance_review(issue: issues.Issue) -> None:
    wt = config.REPO / issue.worktree
    tag = seats.round_tag(issue)
    status_path = seats.review_status_path(wt, tag)
    st = pi.read_status(status_path)
    rname = seats.review_name(issue.id, tag)
    if _seat_dead(rname, st) or (not pi.is_terminal(st) and not pi.already_launched(rname)):
        _set_aside(pi.launch_log_path(rname))
        _set_aside(status_path)
        issue.status = "dispatched"  # advance_dispatched relaunches the review when a paid slot opens
        issue.log(f"review {tag} seat died without a verdict; review requeued")
        issue.save()
        return
    if not pi.is_terminal(st):
        return
    if st.get("status") != "DONE" and _provider_cut_off(st):
        _mark_paid_off(issue, f"review {tag}")
        _set_aside(status_path)
        _set_aside(pi.launch_log_path(seats.review_name(issue.id, tag)))
        issue.status = "dispatched"  # advance_dispatched relaunches the review when a paid slot opens
        issue.log(f"review {tag} cut off by the provider usage limit; review requeued")
        issue.save()
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
        if "conflict" not in rebase_out.lower():
            _handle_gate_or_review_failure(issue, f"rebase onto main failed:\n{rebase_out[-3000:]}")
        else:
            _dispatch_merge_resolver(issue, wt, f"rebase onto main conflicted:\n{rebase_out[-3000:]}")
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


def _dispatch_merge_resolver(issue: issues.Issue, wt: Path, conflict_text: str) -> None:
    """Hand a rebase/merge conflict to a resolver seat in the issue's own
    worktree. Bounded by MAX_MERGE_ROUNDS; when the rounds run out the issue
    hands back to the implementer escalation ladder (which may actually need
    code changes to unstick the semantic half of the conflict), resetting
    the merge counter so a later conflict after a fresh fix gets its own
    budget. Runs synchronously from the gate path, so it can briefly exceed
    MAX_LOCAL_SEATS -- acceptable: conflicts are rare and resolvers are short
    (the alternative is the whole merge lane blocking behind a parked
    human_needed)."""
    if issue.merge_rounds >= config.MAX_MERGE_ROUNDS:
        issue.merge_rounds = 0
        issue.log("merge resolver budget spent; handing back to the implementer ladder")
        _handle_gate_or_review_failure(issue, f"merge conflicts a resolver could not settle:\n{conflict_text[-2000:]}")
        return
    issue.merge_rounds += 1
    tag = f"mrg{issue.merge_rounds}"
    name = seats.merge_resolver_name(issue.id, tag)
    if pi.already_launched(name):
        stale = seats.merge_resolver_status_path(wt, tag)
        if pi.is_terminal(pi.read_status(stale)):
            _set_aside(stale)
            _set_aside(pi.launch_log_path(name))
            issue.log(f"merge resolver {tag} name was held by an already-finished run; set it aside, relaunching fresh")
        else:
            issue.status = "merge_fix"
            issue.save()
            return
    seats.launch_merge_resolver(issue.id, wt, tag, conflict_text)
    issue.status = "merge_fix"
    issue.log(f"merge resolver dispatched ({tag})")
    issue.save()


def advance_merge_fix(issue: issues.Issue) -> None:
    wt = config.REPO / issue.worktree
    tag = f"mrg{issue.merge_rounds}"
    name = seats.merge_resolver_name(issue.id, tag)
    status_path = seats.merge_resolver_status_path(wt, tag)
    st = pi.read_status(status_path)
    if _seat_dead(name, st):
        _set_aside(pi.launch_log_path(name))
        _set_aside(status_path)
        issue.log(f"merge resolver {tag} seat died without a result")
        issue.save()
    if not pi.is_terminal(st) and not pi.already_launched(name):
        seats.launch_merge_resolver(issue.id, wt, tag,
                                    "the previous resolver run died without writing a status; "
                                    "re-run the resolution from the current tree state")
        issue.log(f"merge resolver {tag} relaunched (seat died or never started)")
        issue.save()
        return
    if not pi.is_terminal(st):
        return
    outcome = st.get("status")
    if outcome in ("DONE", "DONE_WITH_CONCERNS"):
        commits = " ".join(st.get("commits", []))
        if commits:
            issue.commits = commits
        issue.merge_rounds = 0
        issue.status = "review"
        issue.log(f"merge resolver {tag} DONE; re-running the gate-and-merge path")
        issue.save()
        _run_gates_and_merge(issue, wt)
        return
    report = wt / ".ds4" / f"report-merge-{tag}.md"
    detail = report.read_text()[-1500:] if report.exists() else "(no resolver report)"
    issue.merge_rounds = 0
    issue.log(f"merge resolver {tag} ended {outcome}\n{detail}")
    _handle_gate_or_review_failure(issue, f"merge resolver {tag} ended {outcome}:\n{detail}")


def _merge_push_deploy(issue: issues.Issue, wt: Path) -> None:
    sha = git_ops.head_sha(wt)
    # The commit-msg hook requires a Test-Budget-Approved trailer on any
    # commit whose diff raises a budget_s -- including this merge commit, which
    # carries the branch's raise. Re-state every such trailer the branch's own
    # commits already carry (never invent one).
    # issue.branch may not be the default `wt/{id}` -- a controller reset can
    # point a worktree at a differently-named branch (see advance_briefed's
    # "don't clobber a deliberately-set custom branch" comment); merging the
    # default name here would silently merge whatever stale/wrong branch
    # happens to still exist under that name instead of what was actually
    # reviewed and gated.
    branch = issue.branch or f"wt/{issue.id}"
    import subprocess as _sp
    trailers = sorted(set(l.strip() for l in _sp.run(
        ["git", "log", "--format=%B", f"main..{branch}"], cwd=str(config.REPO),
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
    ok, out = git_ops.merge_to_main(branch, message)
    if not ok:
        # A conflict at the final merge (main moved between the gates and the
        # merge) used to park the issue on a human with the whole approved fix
        # hostage to a mechanical conflict. Offload the resolution to a seat;
        # the gates re-run afterwards either way. A non-conflict failure (a
        # commit hook refusing the merge commit, say) is not a resolver's job.
        if "conflict" in out.lower():
            _dispatch_merge_resolver(issue, wt, f"merge to main failed:\n{out[-3000:]}")
            return
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
    "merge_fix": advance_merge_fix,
}


# In-flight work claims a free seat before new work does; priority orders items
# WITHIN a status. Putting priority first would let a P1 newcomer overtake the
# reviews and merges that free the seats it needs.
_STATUS_ORDER = {"review": 0, "gate": 0, "merge_fix": 0, "dispatched": 1, "waiting": 2, "briefed": 3, "new": 4}


def _dispatch_order(issue: issues.Issue) -> tuple[int, int, str]:
    """Sort key for one tick's dispatch pass: status, then priority, then id."""
    return (_STATUS_ORDER.get(issue.status, 5), getattr(issue, "priority", 3), issue.id)


def tick() -> None:
    if git_ops.is_paused():
        log.debug("paused (touch %s to resume)", config.PAUSE_FILE)
        return
    discover_new_issues()
    discover_agent_filed_tickets()
    cleanup_orphaned_worktrees()
    for issue in sorted(issues.open_issues(), key=_dispatch_order):
        advancer = ADVANCERS.get(issue.status)
        if advancer is None:
            continue
        # Inbox tickets carry Depends-On in the report; triage writes it into
        # the brief. Either one holds the issue until the dependency closes.
        if issue.status in ("new", "briefed"):
            blocker = _unmet_dependency(issue)
            if blocker:
                continue
        try:
            advancer(issue)
        except Exception:
            log.exception("issue %s: advancer for status=%s raised", issue.id, issue.status)


def acquire_single_instance_lock(path):
    """Take the one-daemon lock, or return None if another daemon holds it.

    A kernel flock, not a PID file: the lock is held on the file itself, so it
    is honoured across PID namespaces. On 2026-09-14 supervisors running inside
    Codex sandboxes could not see each other's daemon PIDs, each concluded the
    daemon was dead, and 49 daemons ended up dispatching and merging from the
    same state. Returns the open handle; the lock lasts as long as it stays open.
    """
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    handle = open(path, "a+")
    try:
        fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        handle.close()
        return None
    return handle


def _last_reason(issue: issues.Issue) -> str:
    # History entries are logged with `- <ts> <message>` but the message
    # itself may contain embedded newlines (e.g. findings text), so the
    # last physical LINE is not the last ENTRY -- split on the "- "
    # prefix instead, or a truncated log's tail word looks like the
    # whole reason (e.g. "th", "confli").
    entries = re.split(r"\n(?=- \d{4}-\d\d-\d\dT)", issue.history.strip())
    if not entries or not entries[0]:
        return "(no history)"
    entry = entries[-1].split(" ", 2)
    text = entry[2] if len(entry) > 2 else entries[-1]
    text = text.replace("\n", " ")
    return text[:300]


def status_snapshot() -> dict:
    """The daemon's live state as plain data: `python3 -m orchestrator.daemon
    status` renders this as text, and `orchestrator.dashboard` serves it as
    JSON. One source, so the two views cannot drift. Read-only: takes no
    lock, safe to run alongside a live daemon.
    """
    from collections import Counter

    all_issues = issues.all_issues()
    counts = Counter(i.status for i in all_issues)

    def row(i: issues.Issue) -> dict:
        return {
            "id": i.id,
            "title": i.title,
            "status": i.status,
            "seat_kind": i.seat_kind,
            "local_rounds": i.local_rounds,
            "max_local_rounds": config.MAX_LOCAL_ROUNDS,
            "escalated_rounds": i.escalated_rounds,
            "max_escalated_rounds": config.MAX_ESCALATED_ROUNDS,
            "updated": i.updated,
            "reason": _last_reason(i),
        }

    human_needed = [row(i) for i in sorted(
        (i for i in all_issues if i.status == "human_needed"), key=lambda i: i.updated, reverse=True)]
    active = [row(i) for i in sorted(
        (i for i in all_issues if i.status in ("new", "briefed", "dispatched", "waiting", "review", "gate")),
        key=lambda i: i.updated, reverse=True)]

    return {
        "generated": issues._now(),
        "counts": {k: counts[k] for k in issues.STATUSES if counts[k]},
        "paid_off": config.PAID_OFF_FILE.exists(),
        "paid_off_reason": config.PAID_OFF_FILE.read_text().strip() if config.PAID_OFF_FILE.exists() else "",
        "human_needed": human_needed,
        "active": active,
    }


def print_status() -> None:
    """`python3 -m orchestrator.daemon status` -- a structured snapshot of the
    queue, meant to answer "why is this stuck" without grepping daemon.log.
    """
    snap = status_snapshot()
    print("queue: " + " ".join(f"{k}={v}" for k, v in snap["counts"].items()))
    if snap["paid_off"]:
        print(f"paid seats: OFF -- {snap['paid_off_reason']}")
    else:
        print("paid seats: on")

    if snap["human_needed"]:
        print(f"\nhuman_needed ({len(snap['human_needed'])}):")
        for i in snap["human_needed"]:
            rounds = f"{i['local_rounds']}/{i['max_local_rounds']} local, {i['escalated_rounds']}/{i['max_escalated_rounds']} esc"
            print(f"  {i['id']}  ({rounds})\n    {i['reason']}")

    if snap["active"]:
        print(f"\nactive ({len(snap['active'])}):")
        for i in snap["active"]:
            rounds = f"{i['local_rounds']}/{i['max_local_rounds']} local, {i['escalated_rounds']}/{i['max_escalated_rounds']} esc"
            print(f"  {i['id']}  status={i['status']} seat={i['seat_kind'] or '-'}  ({rounds})\n    {i['reason']}")


def main() -> None:
    if len(sys.argv) > 1 and sys.argv[1] == "status":
        print_status()
        return
    global _lock_handle
    _lock_handle = acquire_single_instance_lock(LOCK_FILE)
    if _lock_handle is None:
        # Not an error worth a traceback: something already runs the pipeline.
        print(f"orchestrator: another daemon holds {LOCK_FILE}; exiting", file=sys.stderr)
        sys.exit(3)
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
