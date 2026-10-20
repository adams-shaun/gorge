#!/usr/bin/env python3
"""A v2 agent that reads hello but never answers it, for the startup timeout.

With SPELLBENCH_MARKER set it writes that file once hello has been read, so a
test can fire the startup deadline deterministically.
"""
import os
import sys
import time

marker = os.environ.get("SPELLBENCH_MARKER")

for _line in sys.stdin.buffer:
    if marker:
        with open(marker, "wb") as f:
            f.write(b"hello\n")
    time.sleep(60)
