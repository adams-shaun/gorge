#!/usr/bin/env python3
"""Read JSON Lines record history in any of its forms: plain .jsonl, gzipped
.jsonl.gz, or the condensed .cjsonl.gz of gorge's testkit/recstream (delta
frames against the previous record, a keyframe every N records, gzip). The
format is specified in testkit/recstream/doc.go. Standard library only.

    import recstream
    for rec in recstream.records(path):   # dicts (fresh objects, safe to mutate)
        ...
    for line in recstream.lines(path):    # record text, no trailing newline
        ...
    recstream.open_text(path)             # file-like JSON Lines text, any form

A condensed record comes back semantically identical to the one written
(same values, same key order); Go's reader is byte-identical, Python
re-spells string escapes.

    python3 recstream.py cat FILE...      # JSON Lines to stdout
    python3 recstream.py check A B        # same records (parsed) in A and B
"""
import gzip
import io
import json
import sys

HEADER = "#recstream/1"
EXT = ".cjsonl.gz"


def _open_binary(path):
    f = open(path, "rb")
    if f.peek(2)[:2] == b"\x1f\x8b":
        g = gzip.GzipFile(fileobj=f)
        g.myfileobj = f  # GzipFile closes it
        return g
    return f


def is_condensed(path):
    with _open_binary(path) as f:
        return f.read(len(HEADER)) == HEADER.encode()


def _raw_lines(path):
    with _open_binary(path) as b:
        t = io.TextIOWrapper(b, encoding="utf-8", newline="")
        for line in t:
            yield line[:-1] if line.endswith("\n") else line


def _apply_obj(base, patch):
    if not isinstance(base, dict) or not isinstance(patch, dict):
        raise ValueError("recstream: malformed patch")
    out = dict(base)
    for k, op in patch.items():
        if type(op) is list and not op:
            del out[k]
        else:
            out[k] = _apply(out.get(k), op)
    return out


def _apply(old, op):
    if type(op) is not list:
        return op
    t = op[0]
    if t == 0:
        return op[1]
    if t == 1:
        return _apply_obj(old, op[1])
    if t == 2:
        p, s, items = op[1], op[2], op[3]
        return old[:p] + items + old[len(old) - s:]
    if t == 3:
        new = list(old)
        for i, e in op[1].items():
            i = int(i)
            new[i] = _apply(new[i], e)
        return new
    if t == 5:
        new = [_apply(None, op[1])]
        for e in op[2:]:
            new.append(new[-1] if type(e) is list and not e else _apply(new[-1], e))
        return new
    if t == 4:
        new, at = [], 0
        for h in range(1, len(op), 3):
            p, d, x = op[h], op[h + 1], op[h + 2]
            new.extend(old[at:p])
            new.extend(x)
            at = p + d
        new.extend(old[at:])
        return new
    raise ValueError("recstream: unknown op %r" % (t,))


def _frames(it):
    """Yield (record_text, record_or_None) from a condensed frame stream."""
    state = None
    for n, line in enumerate(it, 1):
        if not line:
            raise ValueError("recstream: frame %d: empty frame" % n)
        c = line[0]
        if c == "#":
            continue
        if c == "=":
            text = line[1:]
            try:
                state = json.loads(text)
            except ValueError:
                state = None
            if not isinstance(state, dict):
                state = None
            yield text, None
        elif c in "+~:;":
            if c in ":;":
                state = {}
            elif state is None:
                raise ValueError("recstream: frame %d: delta without a base" % n)
            state = _apply_obj(state, json.loads(line[1:]))
            if c in "+:":
                yield json.dumps(state, separators=(",", ":"), ensure_ascii=False), state
            else:
                yield json.dumps(state, ensure_ascii=False), state
        else:
            raise ValueError("recstream: frame %d: unknown frame %r" % (n, c))


def lines(path):
    """Yield each record's JSON text (no newline), whatever path's form."""
    it = _raw_lines(path)
    first = next(it, None)
    if first is None:
        return
    if first.startswith(HEADER):
        for text, _ in _frames(it):
            yield text
    else:
        yield first
        yield from it


def records(path):
    """Yield each record parsed; every yielded object is a fresh copy.
    Blank lines in plain JSON Lines are skipped."""
    for line in lines(path):
        if line.strip():
            yield json.loads(line)


class _Text(io.RawIOBase):
    def __init__(self, path):
        self._it = lines(path)
        self._buf = b""

    def readable(self):
        return True

    def readinto(self, b):
        while not self._buf:
            line = next(self._it, None)
            if line is None:
                return 0
            self._buf = (line + "\n").encode("utf-8")
        n = min(len(b), len(self._buf))
        b[:n] = self._buf[:n]
        self._buf = self._buf[n:]
        return n


def open_text(path):
    """A text file object of path's records as JSON Lines, whatever its form:
    a drop-in for open(path) / gzip.open(path, "rt") in a reader."""
    return io.TextIOWrapper(io.BufferedReader(_Text(path), 1 << 16), encoding="utf-8", newline="")


def _main(argv):
    if len(argv) >= 2 and argv[0] == "cat":
        out = sys.stdout
        for p in argv[1:]:
            for line in lines(p):
                out.write(line)
                out.write("\n")
        return 0
    if len(argv) == 3 and argv[0] == "check":
        a, b = records(argv[1]), records(argv[2])
        n = 0
        while True:
            x, y = next(a, None), next(b, None)
            if x is None and y is None:
                print("%d records equal" % n)
                return 0
            n += 1
            if x != y or (x is not None and list(x) != list(y)):
                print("record %d differs" % n, file=sys.stderr)
                return 1
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(_main(sys.argv[1:]))
