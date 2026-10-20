#!/usr/bin/env python3
"""A v2 agent that completes the handshake then never answers a choose.

With SPELLBENCH_MARKER set it writes that file once the choose request has
been read, so a test can fire the decision deadline deterministically.
"""
import json
import os
import sys

marker = os.environ.get("SPELLBENCH_MARKER")

for line in sys.stdin.buffer:
    request = json.loads(line)
    if request["request_type"] != "choose":
        reply = {"protocol": "spellbench/v2", "request_id": request["request_id"]}
        if request["request_type"] == "hello":
            reply.update(response_type="hello_ok", bot={"name": "silent", "version": "1.0.0"})
        else:
            reply.update(response_type="ack")
        sys.stdout.write(json.dumps(reply) + "\n")
        sys.stdout.flush()
    elif marker:
        with open(marker, "wb") as f:
            f.write(b"choose\n")
