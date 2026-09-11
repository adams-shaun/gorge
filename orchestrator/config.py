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

# The two GPT-class codex seats this workflow names explicitly: SOL
# implements on escalation, TERRA reviews every round (local or escalated).
IMPLEMENTER_ESCALATED_PROVIDER = "openai-codex"
IMPLEMENTER_ESCALATED_MODEL = "gpt-5.6-sol"

REVIEWER_PROVIDER = "openai-codex"
REVIEWER_MODEL = "gpt-5.6-terra"
REVIEWER_THINKING = "high"

MAX_MINUTES = 75

# --- loop policy -----------------------------------------------------------

POLL_SECONDS = 60
MAX_LOCAL_ROUNDS = 2  # after this many failed local rounds, escalate to SOL
MAX_ESCALATED_ROUNDS = 2  # after this many failed SOL rounds, stop and flag a human

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
