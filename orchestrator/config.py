"""Shared configuration for the gorge autonomous orchestrator.

Every path, model name and threshold the daemon uses lives here, in one
place, so a seat swap or policy change is a one-line edit rather than a
grep-and-replace across the package. Nothing in this module talks to a
network or a model -- it is pure constants, read once at import time.
"""

from __future__ import annotations

import os
from pathlib import Path

# --- repo layout -------------------------------------------------------

REPO = Path("/home/sadams/projects/gorge")
FEEDBACK_DIR = REPO / "feedback"
ISSUES_DIR = REPO / ".ds4" / "issues"
INBOX_DIR = ISSUES_DIR / "inbox"  # hand-authored tickets, dropped as .md files
WORKTREES_DIR = REPO / ".worktrees"
CONTEXT_FILE = REPO / ".superpowers" / "ds4" / "gorge-context.md"
DISPATCH_DIR = REPO / ".superpowers" / "ds4"
LEDGER_JSON = REPO / ".ds4" / "ledger.json"
LANE_RULES = REPO / ".ds4" / "lane-rules.txt"

ORCH_STATE_DIR = REPO / ".ds4" / "orchestrator"
LOG_FILE = ORCH_STATE_DIR / "daemon.log"
PAUSE_FILE = ORCH_STATE_DIR / "pause"
# Present while the paid provider refuses work (usage limit reached). Every
# paid slot reads as full, so paid rounds and reviews wait instead of failing.
# The daemon creates it on detection; delete it by hand once the plan resets.
PAID_OFF_FILE = ORCH_STATE_DIR / "paid-off"
PID_FILE = ORCH_STATE_DIR / "daemon.pid"

PI_AGENT_BIN = Path.home() / "projects" / "ds4-harness" / "bin" / "pi-agent"

# --- seat roster ---------------------------------------------------------
# Mirrors this session's agent-offload usage: a free local seat first, the
# opt-in ChatGPT-plan seats only on escalation. Swapping the local engine
# means editing ~/.ds4/local-seat.env, not this file -- LOCAL_PROVIDER here
# is the *pi-agent* provider name that env resolves to (see
# `python3 -m ds4.seat show` / references/pi-seat.md), which stays a fixed
# string on the pi-agent side even when the engine behind it changes.

LOCAL_PROVIDER = "bm-llms-glm"
LOCAL_MODEL = "glm-5.3-flash"
LOCAL_THINKING = "medium"

# Paid escalation seats use Terra; Astra requires an explicit user request and
# is never selected by this configuration.
IMPLEMENTER_ESCALATED_PROVIDER = "openai-codex"
IMPLEMENTER_ESCALATED_MODEL = "gpt-5.6-terra"

# User-set 2026-09-15: reviewer switched from Sol to Terra.
REVIEWER_PROVIDER = "openai-codex"
REVIEWER_MODEL = "gpt-5.6-terra"
REVIEWER_THINKING = "medium"

# User-set 2026-09-15: when the paid plan is spent (PAID_OFF_FILE) a finished
# round is reviewed on the local seat rather than parking in `dispatched`
# until the plan resets. The hard gates (gates.py) are unchanged either way --
# only the model reading the diff differs -- so a local review still cannot
# merge anything that fails a literal test/conformance/sim exit code.
REVIEW_LOCAL_FALLBACK = True
REVIEW_LOCAL_THINKING = "high"

# User-set 2026-09-16: the ChatGPT plan (Terra/Sol) is blocked for about a
# week. Same shape as REVIEW_LOCAL_FALLBACK: an escalated implementer round
# with no paid slot runs on the local seat instead of parking in `waiting`
# for the week. It still counts against MAX_ESCALATED_ROUNDS -- only the
# backend differs -- and the round is still reviewed (locally, per
# REVIEW_LOCAL_FALLBACK) and gated before it can merge.
ESCALATION_LOCAL_FALLBACK = True

MAX_MINUTES = 75

# --- loop policy -----------------------------------------------------------

POLL_SECONDS = 60
# Concurrency caps, counted from live pi-agent processes across every repo on
# the box. Local Vision seats use the ds4-r8-vision endpoint. Paid Terra
# implementation and review seats share the ChatGPT plan. User-set 2026-09-14:
# local 4, paid 2; paid raised to 5 on 2026-09-15. A launch with no free slot waits: new work
# stays in new/briefed, an in-flight ticket parks in `waiting` with its
# findings, and a finished implementer waits in `dispatched` for a reviewer.
# User-set 2026-09-15: 3 local (glm) seats BOX-WIDE -- per-repo scoping let
# multiple repos' fleets stack unbounded and locked the machine via repeated
# cgroup OOM kills. Counted across every repo on the box (pi.running_names,
# no repo= filter).
MAX_LOCAL_SEATS = 2
MAX_PAID_SEATS = 5
# User-set 2026-09-14: work held only by the local cap goes to terra instead
# of waiting (triage, and every non-escalated implementer round). Those
# rounds are tagged t<n> and still count toward MAX_LOCAL_ROUNDS; escalation
# also uses Terra.
OVERFLOW_PROVIDER = "openai-codex"
OVERFLOW_MODEL = "gpt-5.6-terra"
OVERFLOW_THINKING = "high"
MAX_LOCAL_ROUNDS = 2  # after this many failed local rounds, escalate to Terra
MAX_ESCALATED_ROUNDS = 5  # User-set 2026-09-15: raised from 2 -- after this many Terra rounds, stop and flag a human
# Guard rail for any FUTURE session-resume capability, not one that exists
# today: every current dispatch (pi.py's launch(), called from seats.py's
# three launch_implementer sites) starts a brand-new `pi` session every round
# -- no --continue/--resume/--session flag is ever passed, confirmed by
# reading ds4-harness/bin/pi-agent's actual invocation 2026-09-17. If a
# resume/continue path is ever added (to save the model re-reading a huge
# brief on every retry, say), it MUST check
# `pi.session_too_large_to_reuse(status)` first and fall back to a fresh
# session above this threshold, rather than growing one session's context
# without bound across relaunches. pi-agent already records each round's
# final context size at `status["tokens"]["prompt"]` in STATUS.json, so the
# data this needs already exists -- see pi.session_too_large_to_reuse.
MAX_SESSION_TOKENS_FOR_REUSE = 200_000
# Merge-conflict resolver rounds per merge attempt: a pi seat resolves the
# conflict in the issue's own worktree, then the daemon re-runs the gates and
# merges itself. When both rounds fail, the issue hands back to the normal
# implementer escalation ladder (merge_rounds resets) -- only a ladder that
# ALSO exhausts parks the issue for a human.
MAX_MERGE_ROUNDS = 2

# TestHeads policy (confirmed with the user 2026-09-11): a moved chain head
# is auto-accepted -- the golden is rewritten and committed by the daemon --
# iff the CR conformance lane is 0 FAIL and `make sim` is 20/20 replay OK.
# Neither of those is sensitive to bot-choice *quality*, only to engine
# correctness, which is the same proxy this session used by hand for every
# head move today (anytgt1, ba1). If either is not clean, the task is
# escalated to a human rather than merged.
AUTO_ACCEPT_HEAD_MOVES = True

# --- gates -------------------------------------------------------------
# Every gate is a literal subprocess call whose exit code is ground truth.
# No model's opinion ever substitutes for one of these -- see gates.py.

GO_TEST_PACKAGES = ["./effects", "./rules", "./cards"]
CR_CONFORMANCE_ENV = {"GORGE_CR_CONFORMANCE": "1", "GOMEMLIMIT": "5GiB"}


def bm_llms_api_key() -> str:
    """The cluster secret every local/omitted-provider dispatch inherits.

    Read fresh per call (not cached at import time) so a long-running daemon
    survives a token rotation without a restart.
    """
    import subprocess

    out = subprocess.run(
        [
            "kubectl", "--context", "k3d-bm", "-n", "llm-system",
            "get", "secret", "bm-llms-secrets", "-o", "jsonpath={.data.apiToken}",
        ],
        capture_output=True, text=True, check=True,
    ).stdout
    import base64
    return base64.b64decode(out).decode()


def env_with_bm_key() -> dict:
    env = dict(os.environ)
    try:
        env["BM_LLMS_API_KEY"] = bm_llms_api_key()
    except Exception:
        pass
    return env
