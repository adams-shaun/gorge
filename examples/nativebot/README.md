# nativebot

The smallest linked-in bot policy. It answers every decision with the first
legal option and declares one option, `greeting`, so it also demonstrates the
`-bot-opt` / `-bot-config` surface.

It is an example, not a competitive policy: it never attacks, blocks, or
searches. Use `bot`, `lethal-pressure` or `cast-profile` to play a real game.

## Build and run

The server needs the card corpus (`make fetch-cards compile-cards`).

```sh
go build ./examples/nativebot

./nativebot \
  -addr 127.0.0.1:8090 \
  -dir /tmp/gorge-nativebot \
  -cards .cards \
  -decks testkit/testutil/decks \
  -tables 1 -seats 2 \
  -bot-policy nativebot -bot-policies nativebot \
  -prewarm=false \
  -bot-opt nativebot.greeting=hello
```

The first decision of each `nativebot` seat logs `nativebot: hello`; the
example test seats one, so that game logs it once. Open
`http://127.0.0.1:8090/` to watch the table. Use the `8090-8099` port range on
the shared box, and a unique `-dir` (never the repo-root `gorged-data`).

## Options

A policy declares its options in `bots.Info.Options`. `gorged -help` lists
every registered policy's keys, types, defaults and help text:

```sh
./nativebot -help
```

Resolution order, highest first:

1. `-bot-opt <policy>.<key>=<value>` (repeatable)
2. `GORGED_BOT_<POLICY>_<KEY>` (upper case, `-` becomes `_`), e.g.
   `GORGED_BOT_NATIVEBOT_GREETING=hi`
3. `-bot-config <policy>=<file.json>` (repeatable; a flat JSON object)
4. the declared default

```sh
cat >/tmp/nativebot.json <<'JSON'
{"greeting": "configured from a file"}
JSON
./nativebot -bot-config nativebot=/tmp/nativebot.json ...
```

Startup fails before the listener opens if a policy or key is unknown, a value
does not parse as its declared type, or a required option is missing. The
error names the flag and key.

## Bench against `bot`

`mtgsim` builds the heuristic `bot` directly and cannot import a package-main
example, so it benchmarks the baseline policy, not `nativebot`:

```sh
go run ./cmd/mtgsim -seats 2 -games 20 -verify
```

To compare `nativebot` itself, run the table above and watch its result, or
move the registration in `bot.go` into an importable package and drive it
through a host harness. The policy is intentionally trivial, so the number
mostly measures the host loop, not a bot.
