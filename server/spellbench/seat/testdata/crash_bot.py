#!/usr/bin/env python3
"""A v2 agent that completes the handshake then exits on the first choose."""
import json
import sys

for line in sys.stdin.buffer:
    request = json.loads(line)
    if request["request_type"] == "choose":
        sys.exit(1)
    reply = {"protocol": "spellbench/v2", "request_id": request["request_id"]}
    if request["request_type"] == "hello":
        reply.update(response_type="hello_ok", bot={"name": "crash", "version": "1.0.0"})
    else:
        reply.update(response_type="ack")
    sys.stdout.write(json.dumps(reply) + "\n")
    sys.stdout.flush()
