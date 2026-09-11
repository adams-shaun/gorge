"""A thin, direct wrapper around the pi-agent CLI -- the same binary and
flag shapes this session used by hand for every dispatch today. Nothing
here is an "agent"; it is a subprocess launcher and a status-file reader.
The intelligence lives entirely in the model pi-agent invokes, never in
this process.
"""

from __future__ import annotations

import json
import subprocess
import time
from pathlib import Path
from typing import Optional

from . import config

TERMINAL_STATUSES = {"DONE", "DONE_WITH_CONCERNS", "NEEDS_CONTEXT", "BLOCKED", "CAPPED", "UNKNOWN"}


def launch(
    name: str,
    cwd: Path,
    brief_rel: str,
    system_rel: str,
    report_rel: str,
    out_path: Path,
    provider: str,
    model: str,
    thinking: str,
    message_rel: Optional[str] = None,
) -> subprocess.Popen:
    """Start one pi-agent run, detached, and return immediately.

    cwd is the worktree root; brief/system/report paths are given RELATIVE
    TO cwd (pi-agent resolves them there -- passing an already-worktree-
    prefixed path here double-resolves and the run dies on turn one, a
    failure mode this session hit earlier). out_path is the one path NOT
    relative to cwd: the wrapper writes it itself, from outside the jail.
    """
    cmd = [
        str(config.PI_AGENT_BIN),
        "--cwd", str(cwd),
        "--brief", brief_rel,
        "--system", system_rel,
        "--report", report_rel,
        "--out", str(out_path),
        "--provider", provider,
        "--model", model,
        "--thinking", thinking,
        "--max-minutes", str(config.MAX_MINUTES),
        "--name", name,
    ]
    if message_rel:
        cmd += ["--message", message_rel]
    log_path = config.ORCH_STATE_DIR / "launches" / f"{name}.log"
    log_path.parent.mkdir(parents=True, exist_ok=True)
    log_f = open(log_path, "ab")
    return subprocess.Popen(
        cmd,
        cwd=str(config.REPO),
        stdout=log_f, stderr=subprocess.STDOUT,
        env=config.env_with_bm_key(),
        start_new_session=True,  # detach fully, survives the daemon's own restarts
    )


def read_status(out_path: Path) -> Optional[dict]:
    if not out_path.exists():
        return None
    try:
        return json.loads(out_path.read_text())
    except (json.JSONDecodeError, OSError):
        return None


def is_terminal(status: Optional[dict]) -> bool:
    return bool(status) and status.get("status") in TERMINAL_STATUSES


def events_age_seconds(status: dict) -> Optional[float]:
    """How long since the transcript last grew -- the one thing that proves
    a dispatch is genuinely alive rather than a process that died silently
    (see this session's own '[A dead dispatch reports SUCCESS]' lesson)."""
    events = status.get("events")
    if not events or not Path(events).exists():
        return None
    return time.time() - Path(events).stat().st_mtime
