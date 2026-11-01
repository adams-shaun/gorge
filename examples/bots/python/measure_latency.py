#!/usr/bin/env python3
"""Measure the HTTP seat's latencies against a gorged started with -vsbot.

Plays GAMES games answering every decision with the minimal answer (pass,
keep, the first Min options), so the numbers are transport and engine, not
policy, and reports per-call round trips for GET pending, GET view and POST
intent, plus the gap between answering and the seat's next decision becoming
visible, first by polling GET pending flat out and then by waiting for the
SSE stream's decision frame.

    python3 measure_latency.py http://127.0.0.1:8080 10
"""
import http.client
import json
import queue
import statistics
import sys
import threading
import time

import gorge_agent as ga


def summary(xs):
    xs = sorted(xs)
    return "median %.2f ms, p95 %.2f ms (n=%d)" % (
        1000 * statistics.median(xs), 1000 * xs[min(len(xs) - 1, int(len(xs) * 0.95))], len(xs))


def sse_wakeups(server, client, q):
    """Open /api/stream, subscribe to the table, and put a wake-up on q for
    every decision frame addressed to our seat (and once at match end)."""
    conn = http.client.HTTPConnection(server.split("//", 1)[1])
    conn.request("GET", "/api/stream", headers={"Authorization": "Bearer " + client.token})
    resp = conn.getresponse()

    def reader():
        event = None
        try:
            for raw in resp:
                line = raw.decode().rstrip("\n")
                if line.startswith("event: "):
                    event = line[7:]
                elif line.startswith("data: "):
                    frame = json.loads(line[6:])
                    if event == "hello":
                        q.put(("hello", frame["body"]["session"]))
                    elif frame.get("table") != client.table:
                        continue
                    elif event == "decision" and frame["body"]["player"] == client.seat:
                        q.put(("decision", None))
                    elif event == "match_end":
                        q.put(("end", None))
        except (OSError, ValueError, AttributeError):
            pass  # the connection was closed under us

    threading.Thread(target=reader, daemon=True).start()
    kind, session = q.get(timeout=5)
    status, body = client._call("POST", "/api/subscribe",
                                {"session": session, "table": client.table, "mode": "focus"})
    if status != 204:
        sys.exit("subscribe: HTTP %d %s" % (status, body))
    return conn


def run(server, games, mode):
    get_p, get_v, post, gap = [], [], [], []
    for _ in range(games):
        g = ga.create_game(server)
        c = ga.SeatClient(server, g["table"], g["match"], g["seat"], g["token"])
        ga.wait_for_match(c)
        q = queue.Queue()
        conn = sse_wakeups(server, c, q) if mode == "sse" else None
        last_post, answered = None, -1
        while True:
            t0 = time.perf_counter()
            d = c.pending()
            dt = time.perf_counter() - t0
            if d is not None and d["seq"] <= answered:
                d = None
            if d is None:
                if mode == "sse":
                    kind, _ = q.get(timeout=30)
                    if kind == "end":
                        break
                else:
                    row = c.match_state()
                    if row and row["state"] != "live":
                        break
                continue
            get_p.append(dt)
            if last_post is not None:
                gap.append(time.perf_counter() - last_post)
            t0 = time.perf_counter()
            c.view()
            get_v.append(time.perf_counter() - t0)
            t0 = time.perf_counter()
            c.submit(d, ga.minimal(d))
            post.append(time.perf_counter() - t0)
            last_post, answered = time.perf_counter(), d["seq"]
        if conn:
            conn.close()
    print("%-4s GET pending  %s" % (mode, summary(get_p)))
    print("%-4s GET view     %s" % (mode, summary(get_v)))
    print("%-4s POST intent  %s" % (mode, summary(post)))
    print("%-4s answer -> next own decision visible  %s" % (mode, summary(gap)))


if __name__ == "__main__":
    server = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080"
    games = int(sys.argv[2]) if len(sys.argv) > 2 else 10
    run(server, games, "poll")
    run(server, games, "sse")
