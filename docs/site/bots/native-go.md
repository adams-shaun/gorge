# Writing a native Go bot

A native bot is a Go `seat.Seat` linked into the engine's process: the fastest way to play, and the only kind that can
search. This page builds one, registers it with options, plays it against the default bot, tests it and serves it.

<div class="gg-needs" markdown>

| You need | For |
|---|---|
| Go and a gorge checkout | building the example module |
| The card corpus (`make fetch-cards`) | the duel and the server; the tests run without it |
| The code | [`examples/bots/go`](https://github.com/adams-shaun/gorge/blob/main/examples/bots/go), checked by the `examples` CI job |

</div>

Commands were run on 2026-10-31, the duel again on 2026-11-08.

## What is a bot?

A bot is a `seat.Seat`
([`engine/seat/seat.go`](https://github.com/adams-shaun/gorge/blob/main/engine/seat/seat.go)):

```go title="engine/seat/seat.go"
type Seat interface {
    Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error)
}
```

| Argument | What it holds |
|---|---|
| `v` | The seat's redacted view: its own hand and library contents (never the order), the public board, counts for every hidden zone. |
| `d` | Every legal option. The answer is an `Intent{Seq, Player, Choices}` naming option indices. |
| `d.Validate(in)` | Tells you, before you return, whether the engine will accept the answer. |

A bot never errors in normal play: when it has nothing better to say, it delegates.

### What else can a seat ask for?

Each optional interface is detected by type assertion.

| Interface | Package | What it buys |
|---|---|---|
| `seat.PaymentPlanConsumer` (`WantsPaymentActions() bool`) | `engine/seat` | On a table with automatic mana payment, the decision carries the engine's payment plans, so the bot casts "this spell, paid this way" instead of tapping lands one decision at a time. Every registered bot answers true or forwards what it wraps. |
| `heuristic.BoardSeat` (`DecideBoard`) | `bots/heuristic` | The cheaper `botpolicy.Board` instead of a projected view. |
| `bots.EnvSeat` (`WantsEnv`, `DecideEnv`) | `bots` | For a search seat: the host builds an honest root, a redeal of the hidden cards from the seat's own observations, at the decisions the seat asks for. The policy must declare `Info.Env`. |
| `bots.MatchSeat` / `bots.MatchEnd` | `bots` | Lifecycle hooks: match context before the first decision, the result after the last. |
| `bots.RefusalAnswerer` | `bots` | A second attempt when the engine refuses an answer, before the host's own fallbacks. |
| `bots.BudgetedSeat` | `bots` | Declares a per-decision wall-clock budget; the host arms it as the `ctx` deadline. |

!!! warning "A native seat is trusted code"
    It runs inside the server. The plain `Decide` path hands it only a View, but
    `bots.MatchSeat.AttachMatch` receives `MatchContext.Engine`, the live engine, to be *read, never
    written*; nothing in the type system stops a seat that holds it from reading hidden cards. A policy
    that will play people uses only the View (or an honest root through `EnvSeat`).

## A complete policy

The example policy `allin` is the default heuristic bot with one change: it attacks with every creature of at least
`min-power` power, every combat. It does the three things every native policy does: answers from the View, delegates
what it does not specialise in, and registers itself with its options.

```go title="examples/bots/go/allin/allin.go (abridged)"
func init() {
    bots.Register(bots.Entry{
        Info: bots.Info{
            Name: "allin", Label: "All-in (example)", Tier: bots.Experimental,
            Description: "The heuristic bot, but it attacks with every creature of at least min-power power, every combat.",
            Formats:     []string{"constructed", "commander"},
            Options: []bots.OptionSpec{{Key: "min-power", Type: "int", Default: "1",
                Help: "attack only with creatures of at least this power"}},
        },
        New: New,
    })
}

func New(o bots.Options) (seat.Seat, error) {
    b := heuristic.NewBot(o.Seed)
    if o.AutoPayMana { b.EnableAutoPayMana() }
    return &Seat{fallback: b, minPower: int32(o.Int("min-power"))}, nil
}

func (s *Seat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
    if d.Kind != decision.KAttackers {
        return s.fallback.Decide(ctx, v, d)
    }
    in := decision.Intent{Seq: d.Seq, Player: d.Player}
    // ... pick one option per creature whose power >= s.minPower (or Required) ...
    if d.Validate(in) != nil { // a whole-declaration rule we did not model
        return s.fallback.Decide(ctx, v, d)
    }
    return in, nil
}
// WantsPaymentActions forwards to s.fallback.WantsPaymentActions().
```

!!! tip "Three habits worth copying"
    - **One option per (creature, defender) pair** on `attackers`: declare at most one per creature.
      `Option.Required` marks a creature that must attack.
    - **`Validate` before returning.** It runs the host's wire rules, so you find out here rather than in a refusal.
    - **Deterministic.** Seeded `math/rand/v2` only, no wall clock, no `map` iteration order reaching a choice.

### What goes in the registry entry? { #the-registry-entry }

`bots.Register` panics at init on a malformed entry, so a typo fails the build's first test rather than a server start.

| Field | Meaning |
|---|---|
| `Name` | the registry key: lower case, digits and dashes |
| `Tier` | `bots.Production` or `bots.Experimental` |
| `Formats`, `MaxSeats` | tables the policy accepts (`MaxSeats` 0 = no bound) |
| `Env`, `ClosedLists`, `Search` | an Env policy builds search over every seat's real decklist and is never seated against a human; `ClosedLists` marks one that closes opponents' lists and may be; `Search` lets the host bound concurrent searches |
| `NeedsAutoPay` | the policy has no manual-tap mode; a table without payment plans refuses it |
| `Caretaker` | a registered, non-`Env` policy named as this one's stand-in (`bots.Validate` checks it); a human seat's timeout caretaker is the table's own policy, not this field |
| `Cost` | the per-decision cost `GET /api/bot-policies` publishes |
| `Options` | the keys the factory reads ([Options](#options)) |

### How are options set? { #options }

Each `bots.OptionSpec` declares a key, a type (`string`, `int`, `float`, `bool`, `path`), a default, help text and
whether it is required. `bots.New` fills every default into `Options.Config` before calling the factory, so
`o.Int("min-power")` is always set; reading an undeclared key panics, because it is a programming error.

A host takes each value from the first source that sets it:

| Precedence | Source |
|---|---|
| 1 | `-bot-opt <policy>.<key>=<value>` (repeatable) |
| 2 | `GORGED_BOT_<POLICY>_<KEY>` (upper case, `-` becomes `_`) |
| 3 | `-bot-config <policy>=<file.json>` (a flat JSON object; repeatable) |
| 4 | the declared default |

`gorged` validates every value before it opens a listener, and its `-help` lists every linked policy's options. A bad
value fails at startup:

```text
gorged: -bot-opt "allin.min-power=two": option "min-power": want int, got "two"
```

Option values never appear in `/api/bot-policies`.

### Using it from your own module { #its-own-module }

The example is a Go module of its own (`examples/bots/go/go.mod`, `replace github.com/adams-shaun/gorge => ../../..`).
Outside this repository, drop the `replace` and `go get github.com/adams-shaun/gorge@<commit>`. gorge has no
third-party dependencies.

## Play it against the default bot

`examples/bots/go/cmd/duel` plays seeded games between two registered policies on real decks, seats traded each game.
The engine, both seats and the loop share one process: no network, no clock in any decision.

```console title="The control: bot against bot (2026-11-08)"
$ cd examples/bots/go
$ go run ./cmd/duel -cards ../../../.cards -a bot -b bot -games 40
bot (mono-green-stompy) 30 - 10 bot (mono-red-goblins), 0 draws, 0 stalls
40 games, 14540 decisions in 0.39s: 26.8 µs per decision, both seats, engine included
```

The control shows what the deck pairing alone does. Run your policy as `-a allin -b bot` over the same seeds and compare.

!!! note "Forty games is a smoke test"
    About ±15 percentage points. [Bot-vs-bot games](../guide/bot-games.md#reading-the-result) covers how many games a
    real comparison needs.

`duel` is a thin loop over
[`testkit/bench.PlayGame`](https://github.com/adams-shaun/gorge/blob/main/testkit/bench/bench.go), which takes a
`rules.Config` and one `seat.Seat` per seat and returns the outcome; `bench.RunPairs` runs a deck-pair matrix with seats
traded across worker goroutines.

## Test it

[`allin_test.go`](https://github.com/adams-shaun/gorge/blob/main/examples/bots/go/allin/allin_test.go) has three tests a
policy should copy. None needs the card corpus: `testutil.SampleDecks` builds two synthetic 40-card decks.

| Test | What it proves |
|---|---|
| `TestRegistered` | The init ran, and `bots.New` builds the policy with its declared defaults. |
| `TestPlaysAgainstTheBot` | Ten seeded games against `heuristic.NewBot` through `bench.PlayGame`, failing on any stall. |
| `TestHostContract` | Seats the policy at a real host table with `host.RunContractTable`, plays the match to the end and replays its log to the recorded head. |

`TestHostContract` uses the harness gorge runs on its own hosted policies
([`server/host/contract.go`](https://github.com/adams-shaun/gorge/blob/main/server/host/contract.go)). Its siblings
`RunEnvLeakContract` and `RunClosedPrivacyContract` check that an Env seat's answers never depend on hidden cards.

## Serve it

A server is `gorged.Main` plus blank imports: gorge's own bots and yours.

```go title="examples/bots/go/cmd/gorged-allin/main.go"
package main

import (
    "os"

    _ "github.com/adams-shaun/gorge/bots/all"
    _ "github.com/adams-shaun/gorge/examples/bots/go/allin"
    "github.com/adams-shaun/gorge/server/gorged"
)

func main() { os.Exit(gorged.Main(os.Args[1:], gorged.Extensions{})) }
```

```console title="Build, run, and list the offered policies"
$ go build -o gorged-allin ./cmd/gorged-allin
$ ./gorged-allin -addr 127.0.0.1:8080 -cards ../../../.cards -decks ../../../testkit/testutil/decks \
    -dir /tmp/gorge-allin -feedback /tmp/gorge-allin/feedback -tables 0 -vsbot -pace 0 -prewarm=false \
    -bot-policy bot -bot-opt allin.min-power=2
$ curl -s http://127.0.0.1:8080/api/bot-policies | jq -r '.policies[].name'
```

`allin` is now offered to `POST /api/games` (`"bot_policy": "allin"`) and the browser's play-vs-bot picker, beside the
policies that do not read decklists. The [Python example agent](examples.md) can play it with `--bot-policy allin`.

`gorged.Extensions` also carries extra flags (`Flags`), a draft-event runtime (`Events`) and a match-review loader
(`Review`); `gorged.Serve` runs the same server on a listener you supply, which is how a test drives it. The full
surface is in the [Go API reference](../api/go.md).

## Shipping it { #bot-code-that-will-ship }

A bot that will ship follows the [rules for bot code](training-and-adoption.md#rules-for-bot-code); that page covers the
gates a policy passes on its way from a bench to the default bot.
