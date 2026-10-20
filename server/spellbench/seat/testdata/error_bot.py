#!/usr/bin/env python3
"""A v2 agent that answers hello, then errors every choose, for the forfeit test."""
import json
import sys

for line in sys.stdin.buffer:
    request = json.loads(line)
    reply = {"protocol": "spellbench/v2", "request_id": request["request_id"]}
    if request["request_type"] == "hello":
        reply.update(response_type="hello_ok", bot={"name": "error", "version": "1.0.0"})
    elif request["request_type"] == "choose":
        reply.update(response_type="error", error={"code": "broken", "message": "no answer"})
    else:
        reply.update(response_type="ack")
    sys.stdout.write(json.dumps(reply) + "\n")
    sys.stdout.flush()
