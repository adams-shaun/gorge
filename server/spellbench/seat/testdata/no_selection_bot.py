#!/usr/bin/env python3
"""A v2 agent that returns a choice response with no selection object."""
import json
import sys

for line in sys.stdin.buffer:
    request = json.loads(line)
    reply = {"protocol": "spellbench/v2", "request_id": request["request_id"]}
    if request["request_type"] == "hello":
        reply.update(response_type="hello_ok", bot={"name": "no-selection", "version": "1.0.0"})
    elif request["request_type"] == "choose":
        reply.update(response_type="choice")
    else:
        reply.update(response_type="ack")
    sys.stdout.write(json.dumps(reply) + "\n")
    sys.stdout.flush()
