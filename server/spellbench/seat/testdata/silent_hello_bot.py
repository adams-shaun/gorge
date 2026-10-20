#!/usr/bin/env python3
"""A v2 agent that reads hello but never answers it, for the startup timeout."""
import sys
import time

for _line in sys.stdin.buffer:
    time.sleep(60)
