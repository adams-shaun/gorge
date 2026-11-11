# Quickstart: a follower in ten minutes

This page takes a runner that plays nothing through every stage: setup, registration, the card check,
a single-card run, a whole-game run and a sweep. Every command and every output below is from a real
run on 2026-11-11. By the end you have the loop you'll use while building the real thing.

You need Go (the version in gorge-compliance's `go.mod`, 1.25.8 today), git, make and Python 3, and
about 200 MB for gorge's card corpus. No JVM is needed: XMage and Forge only run when you ask for them.

## 1. Set up gorge and gorge-compliance

The comparison harness lives in [gorge-compliance](https://github.com/adams-shaun/gorge-compliance).
It pins one gorge commit, and gorge's card corpus must be the one that commit expects.

```sh
git clone https://github.com/adams-shaun/gorge.git
git clone https://github.com/adams-shaun/gorge-compliance.git
git -C gorge checkout "$(cat gorge-compliance/GORGE_PIN)"
make -C gorge fetch-cards compile-cards    # Forge's card scripts at gorge's FORGE_REF, into gorge/.cards
cd gorge-compliance
make cards                                 # links ../gorge/.cards here as .cards
make boundary oracle-audit ORACLE_RUN=Fatal_Push
```

The last line should end with `ok` for both packages. If it prints `ok` in a few milliseconds, the
corpus isn't linked and the tests skipped; `make cards` says what's wrong.

Everything the harness writes goes outside the repository, or under its gitignored `.oracle-build/`.
The requests carry Forge script text, which is GPL-3.0, so the commands refuse an output path anywhere
else in the tree. The examples use `$OUT`:

```sh
export OUT=/tmp/xe-quickstart
```

## 2. Register your engine

A follower is one entry in the registry, `oracle/xengine/follower/follower.go`. Add yours at the end
of `registry`:

```go
{
    Name:         "toy",
    Display:      "Toy",
    Runner:       "scripts/toy-oracle-run.sh",
    Prep:         PrepFollowerExport,
    StampsEngine: true,
    LacksCardMsg: "toy lacks card:",
},
```

- `Name` is the id the commands take (`-engine toy`) and the value your rows put in `"engine"`.
- `Runner` is the program the harness calls. Its path is relative to the repository root.
- `Prep: PrepFollowerExport` sends your engine the engine-neutral requests described in
  [the protocol](protocol.md#requests).
- `StampsEngine` makes the harness refuse a row that doesn't say `"engine":"toy"`, so a file from
  another engine can't be compared as yours by mistake.

Every other field records a quirk of a driver and starts at its zero value. The
[registry fields](protocol.md#registry-fields) table lists them. Check that the entry is there:

```console
$ go run ./cmd/xengine followers
#name	runner	prep	jvm	runner_env	mem_env	wait_env
xmage	scripts/xmage-oracle-run.sh	xprep	true	XENGINE_XMAGE_RUNNER	XENGINE_XMAGE_MEM	XENGINE_XMAGE_WAIT
forge	scripts/forge-oracle-run.sh	forge-export	true	XENGINE_FORGE_RUNNER	XENGINE_FORGE_MEM	XENGINE_FORGE_WAIT
toy	scripts/toy-oracle-run.sh	follower-export	false	XENGINE_TOY_RUNNER	XENGINE_TOY_MEM	XENGINE_TOY_WAIT
```

## 3. Write a runner that plays nothing

This runner answers the card check, refusing one card so you can see a refusal, and turns every
request into a `harness` row: "I couldn't do this". That is always a valid answer. It is never
counted as a disagreement.

```python title="scripts/toy-oracle-run.sh"
#!/usr/bin/env python3
"""A follower that plays nothing: it answers --preflight and writes one harness row per request."""
import json, sys, time

if sys.argv[1] == "--preflight":
    for name in open(sys.argv[2], encoding="utf-8"):
        name = name.rstrip("\n")
        if name == "Burst Lightning":
            print(f"{name}\tno card data")      # NAME<TAB>REASON for each card you lack
    sys.exit(0)                                 # 0: the answer is complete

src, dst = sys.argv[1], sys.argv[2]
with open(src, encoding="utf-8") as fin, open(dst, "w", encoding="utf-8") as fout:
    for line in fin:
        t0 = time.monotonic()
        try:
            req = json.loads(line)
            item = req.get("item", req)
            row = {"id": req.get("id") or item["id"], "name": item.get("name", ""),
                   "engine": "toy", "strict": True, "snapshots": [],
                   "harness": "harness: toy follower places nothing yet"}
        except Exception as e:                  # one bad line never stops the batch
            row = {"id": "", "name": "", "engine": "toy", "strict": True, "snapshots": [],
                   "harness": f"harness: bad request line: {e}"}
        row["ms"] = int((time.monotonic() - t0) * 1000)
        fout.write(json.dumps(row) + "\n")
        fout.flush()                            # one row per request, in input order
```

```sh
chmod +x scripts/toy-oracle-run.sh
```

## 4. One scenario per card

The smallest requests are generated one per card of a set: the card in hand, a simple board, cast it
and let it resolve. Generate them for Foundations, export them for your engine, replay, and diff:

```console
$ mkdir -p $OUT/lvA
$ go run ./cmd/oraclediff gen -level A -manifest oracle/compliance/manifests/FDN.json -out $OUT/lvA/scen.jsonl
gen FDN: 517 scenarios, 0 skipped (/tmp/xe-quickstart/lvA/scen.jsonl.skips.jsonl)

$ go run ./cmd/oraclediff follower-export -engine toy -scenarios $OUT/lvA/scen.jsonl -out $OUT/lvA/toy-req.jsonl
wrote 516 requests to /tmp/xe-quickstart/lvA/toy-req.jsonl; 1 of 517 items refused (1 of 525 cards lacking) to /tmp/xe-quickstart/lvA/toy-prep.jsonl
...

$ cat $OUT/lvA/toy-prep.jsonl
{"id":"Burst Lightning/cast-resolve/v1","reason":"toy_lacks_card","detail":"Burst Lightning: no card data"}

$ scripts/toy-oracle-run.sh $OUT/lvA/toy-req.jsonl $OUT/lvA/toy.jsonl

$ go run ./cmd/oraclediff follower-diff -engine toy -req $OUT/lvA/toy-req.jsonl -forge $OUT/lvA/toy.jsonl \
    -cache $OUT/lvA/cache -out $OUT/lvA/diff.jsonl
HARNESS                      516
```

`follower-export` called your runner with `--preflight` once, with the 525 card names the 517
scenarios use. The scenario that needs Burst Lightning was held back and recorded as a refusal, and the
other 516 became requests. The diff then reports every one as `HARNESS`, which is right for a runner
that does nothing. (The flag is called `-forge` because this command started as Forge's; it takes any
follower's rows.)

## 5. A whole game

Now the real material: gorge plays a game between two decks drawn from the FDN commons, and the game
is cut into a whole-game tape plus one window per turn.

```console
$ go run ./cmd/xengine play -set FDN -pool oracle/xengine/pools/fdn-commons.txt -games 1 -windows -out $OUT/play
pool oracle/xengine/pools/fdn-commons.txt (sha256 a29fcd2f98bc): 196 names, 191 candidates, 5 basic
played 1 games into /tmp/xe-quickstart/play
  lower_gap      1
  lower_ok       19
  refused        2

$ go run ./cmd/oraclediff follower-export -engine toy -scenarios $OUT/play/items.jsonl -out $OUT/play/toy-req.jsonl
wrote 19 requests to /tmp/xe-quickstart/play/toy-req.jsonl; 0 of 19 items refused (0 of 47 cards lacking) to /tmp/xe-quickstart/play/toy-prep.jsonl
  ability.class                                    93
  ability.cost                                     93
  ability.ordinal                                  90
  ability.ordinal_omit:keyword_minted              3
  ...

$ scripts/toy-oracle-run.sh $OUT/play/toy-req.jsonl $OUT/play/toy.jsonl

$ go run ./cmd/xengine diff -play $OUT/play -engine toy -rows $OUT/play/toy.jsonl -out $OUT/diff-toy
compared 20 items against toy into /tmp/xe-quickstart/diff-toy
  harness         19
  lower_gap       1
```

- `lower_ok 19` items were cut and checked against gorge's own runner, so they were sent to you.
- `refused 2` windows held something setup can't place. Here both started on a turn where a card had
  already entered the battlefield, which is history setup can't recreate
  (`field:Object.EnteredThisTurn`). They never reach a follower. [What gets refused](protocol.md#what-you-are-never-sent) lists the reasons.
- `lower_gap 1` is an item gorge's runner couldn't reproduce. That is a harness bug on our side, and
  it is never sent to you either.
- The census after the export shows how many of the engine-neutral identities it could fill. The
  [protocol page](protocol.md#neutral) explains them.

The `-pool` decks are drawn against a coverage ledger that favours cards played least so far. With the
same seed and ledger you get the same games.

## 6. See what agreement looks like

`$OUT/play/gorge.jsonl` holds gorge's own snapshots for every item, which is exactly what your rows
should contain. A runner that copied them would agree everywhere. Try it, and change one life total to
see a disagreement:

```python
import json
OUT = "/tmp/xe-quickstart"
gorge = {json.loads(l)["id"]: json.loads(l)["result"] for l in open(f"{OUT}/play/gorge.jsonl")}
with open(f"{OUT}/play/toy.jsonl", "w") as out:
    tampered = False
    for line in open(f"{OUT}/play/toy-req.jsonl"):
        req = json.loads(line)
        snaps = [{k: v for k, v in s.items() if k != "offered"} for s in gorge[req["id"]]["snapshots"]]
        if not tampered and len(snaps) > 3:
            snaps[3]["players"][1]["life"] -= 1
            tampered = True
        out.write(json.dumps({"id": req["id"], "name": req["item"]["name"], "engine": "toy",
                              "strict": True, "ms": 0, "snapshots": snaps}) + "\n")
```

```console
$ go run ./cmd/xengine diff -play $OUT/play -engine toy -rows $OUT/play/toy.jsonl -out $OUT/diff-toy2
compared 20 items against toy into /tmp/xe-quickstart/diff-toy2
  agree           18
  diverge         1
  lower_gap       1

$ grep diverge $OUT/diff-toy2/verdicts.jsonl
{"id":"xe:FDN:0:0:tape","engine":"toy","class":"diverge","checkpoint":"step 2 (cast)","field":"p1.life","gorge":"20","other":"19","step":2}
```

The diff stops at the first field that differs and names the checkpoint, the field and both values.
`gorge.jsonl` is also your best debugging aid: when a row disagrees, gorge's snapshot at the same
checkpoint is the answer it expected.

## 7. Run a sweep

The sweep script runs play, export, your runner and diff for many games, in resumable chunks, and keeps
the coverage ledger. With only your engine as follower, no JVM starts:

```console
$ XENGINE_ROOT=$OUT/sweep XENGINE_FOLLOWERS=toy XENGINE_POOL=oracle/xengine/pools/fdn-commons.txt \
    XENGINE_TOY_MEM=2G scripts/xengine-sweep.sh --sets FDN --chunks 1 --games 2
== xengine sweep seed1: root /tmp/xe-quickstart/sweep, 1 chunk(s) to do
== FDN chunk 0: play
== FDN chunk 0: toy
FDN/0: done
```

`$OUT/sweep/SUMMARY.md` has the class histogram for each follower:

```text
### toy: class histogram
harness 19
lower_gap 1
not_sent 16
```

The 16 `not_sent` items are the ones that use Burst Lightning, which your runner refused. Add
`forge` or `xmage` to `XENGINE_FOLLOWERS` to compare against them too, once their builds are set up
(`make forge-oracle-setup`, `make xmage-oracle-setup`).

## Next

- Replace the harness rows with a real `"setup"` snapshot: [place a board](howto.md#place-a-board-without-entry-effects)
  and [write snapshots](howto.md#write-snapshots-gorge-can-compare).
- Read [the protocol](protocol.md) end to end before you write the step machine.
- When rows start disagreeing, [reading results](results.md) says which ones are leads.
