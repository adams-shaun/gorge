# Writing a native Go bot

This page walks a Go developer through a complete bot policy for gorge: the
interface it implements, registering it with options, playing it against the
default bot in-process, testing it with the harness gorge runs on its own
policies, and serving it from a `gorged` binary. The code is in
[`examples/bots/go`](https://github.com/adams-shaun/gorge/blob/main/examples/bots/go),
and every command below was run on 2026-10-31.

A native seat is the fastest way to play and the only one that can search, because it lives in the
same process as the engine.

## The interface

A bot is a `seat.Seat`
([`engine/seat/seat.go`](https://github.com/adams-shaun/gorge/blob/main/engine/seat/seat.go)):

```go
type Seat interface {
    Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error)
}
```

`v` is the seat's redacted view: its own hand and library contents (never
the order), the public board, counts for every hidden zone. `d` lists every
legal option; the answer is an `Intent{Seq, Player, Choices}` naming option
indices, and `d.Validate(in)` tells you before you return whether the engine
will accept it. A bot never errors in normal play: when it has nothing better
to say it should delegate.

Optional interfaces a seat can add, all detected by type assertion:

| Interface | Package | What it buys |
|---|---|---|
| `seat.PaymentPlanConsumer` (`WantsPaymentActions() bool`) | `engine/seat` | On a table with automatic mana payment, the decision carries the engine's payment plans, so the bot casts "this spell, paid this way" instead of tapping lands one decision at a time. Every registered bot answers true or forwards what it wraps. |
| `heuristic.BoardSeat` (`DecideBoard`) | `bots/heuristic` | The cheaper `botpolicy.Board` instead of a projected view. |
| `bots.EnvSeat` (`WantsEnv`, `DecideEnv`) | `bots` | For a search seat: the host builds an honest root, a redeal of the hidden cards from the seat's own observations, at the decisions the seat asks for. The policy must declare `Info.Env`. |
| `bots.MatchSeat` / `bots.MatchEnd` | `bots` | Lifecycle hooks: match context before the first decision, the result after the last. |
| `bots.RefusalAnswerer` | `bots` | A second attempt when the engine refuses an answer, before the host's own fallbacks. |
| `bots.BudgetedSeat` | `bots` | Declares a per-decision wall-clock budget; the host arms it as the `ctx` deadline. |

!!! warning "What a native seat is trusted with"
    A native seat runs inside the server, so it is trusted code. The plain
    `Decide` path hands it only a View. But `bots.MatchSeat.AttachMatch`
    receives `MatchContext.Engine`, the live engine, to be *read, never
    written*; nothing in the type system stops a seat that holds it from
    reading hidden cards. Use only the View (or an honest root through
    `EnvSeat`) in a policy that will play people.

## A complete policy

The example policy `allin` is the default heuristic bot with one change: it
attacks with every creature of at least `min-power` power, every combat. It
shows the three things every native policy does: answer from the View,
delegate what it does not specialise in, and register itself with its
options.

```go title="examples/bots/go/allin/allin.go (abridged)"
const Name = "allin"

func init() {
    bots.Register(bots.Entry{
        Info: bots.Info{
            Name:        Name,
            Label:       "All-in (example)",
            Description: "The heuristic bot, but it attacks with every creature of at least min-power power, every combat.",
            Tier:        bots.Experimental,
            Formats:     []string{"constructed", "commander"},
            Options: []bots.OptionSpec{{
                Key: "min-power", Type: "int", Default: "1",
                Help: "attack only with creatures of at least this power",
            }},
        },
        New: New,
    })
}

func New(o bots.Options) (seat.Seat, error) {
    b := heuristic.NewBot(o.Seed)
    if o.AutoPayMana {
        b.EnableAutoPayMana()
    }
    return &Seat{fallback: b, minPower: int32(o.Int("min-power"))}, nil
}

func (s *Seat) WantsPaymentActions() bool { return s.fallback.WantsPaymentActions() }

func (s *Seat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
    if d.Kind != decision.KAttackers {
        return s.fallback.Decide(ctx, v, d)
    }
    power := map[state.ObjID]int32{}
    for _, p := range v.Players {
        for _, c := range p.Battlefield {
            power[c.ID] = c.Power
        }
    }
    in := decision.Intent{Seq: d.Seq, Player: d.Player}
    attacking := map[state.ObjID]bool{} // one defender per creature
    groups := map[string]bool{}         // at most one option per Group
    for _, o := range d.Options {
        if attacking[o.Obj] || (o.Group != "" && groups[o.Group]) {
            continue
        }
        if !o.Required && power[o.Obj] < s.minPower {
            continue
        }
        in.Choices = append(in.Choices, o.Index)
        attacking[o.Obj] = true
        if o.Group != "" {
            groups[o.Group] = true
        }
    }
    if d.Validate(in) != nil { // a whole-declaration rule we did not model
        return s.fallback.Decide(ctx, v, d)
    }
    return in, nil
}
```

Points worth copying:

- **An attackers decision has one option per (creature, defender) pair.**
  With a planeswalker or battle on the other side, one creature has several
  options; declare at most one per creature. `Option.Required` marks a
  creature that must attack (goad, "attacks each combat").
- **`Validate` before returning.** It runs the same wire rules the host
  applies, so a seat finds out it was wrong here rather than in a refusal.
- **Deterministic.** Everything a policy does must be a pure function of its
  seed and its inputs: seeded `math/rand/v2` only, no wall clock, no `map`
  iteration order reaching a choice. (The maps above are only looked up.)

### The registry entry

`bots.Register` panics at init on a malformed entry, so a typo fails the
build's first test rather than a server start. The fields that matter:

| Field | Meaning |
|---|---|
| `Name` | the registry key: lower case, digits and dashes |
| `Tier` | `bots.Production` or `bots.Experimental` |
| `Formats`, `MaxSeats` | tables the policy accepts (`MaxSeats` 0 = no bound) |
| `Env`, `ClosedLists`, `Search` | an Env policy builds search over every seat's real decklist and is never seated against a human; `ClosedLists` marks one that closes opponents' lists and may be; `Search` lets the host bound concurrent searches |
| `NeedsAutoPay` | the policy has no manual-tap mode; a table without payment plans refuses it |
| `Caretaker` | a registered, non-`Env` policy named as this one's stand-in (`bots.Validate` checks it); a human seat's timeout caretaker is the table's own policy, not this field |
| `Cost` | the per-decision cost `GET /api/bot-policies` publishes |
| `Options` | the keys the factory reads (next section) |

### Options

Each `bots.OptionSpec` declares a key, a type (`string`, `int`, `float`,
`bool`, `path`), a default, help text and whether it is required. `bots.New`
fills every default into `Options.Config` before calling the factory, so
`o.Int("min-power")` is always set; reading an undeclared key panics, because
it is a programming error. A host supplies values from, highest first:

1. `-bot-opt <policy>.<key>=<value>` (repeatable)
2. `GORGED_BOT_<POLICY>_<KEY>` (upper case, `-` becomes `_`)
3. `-bot-config <policy>=<file.json>` (a flat JSON object; repeatable)
4. the declared default

`gorged` validates every value before it opens a listener, and its `-help`
lists every linked policy's options:

```text
$ ./gorged-allin -help
...
Bot policy options (set with -bot-opt <policy>.<key>=<value>, -bot-config <policy>=<file.json>, or GORGED_BOT_<POLICY>_<KEY>):
  allin (All-in (example)):
    allin.min-power (int; default "1")  attack only with creatures of at least this power

$ ./gorged-allin -bot-opt allin.min-power=two ...
gorged: -bot-opt "allin.min-power=two": option "min-power": want int, got "two"
$ ./gorged-allin -bot-opt allin.speed=2 ...
gorged: -bot-opt: bot policy "allin" has no option "speed" (known: min-power)
```

Option values never appear in `/api/bot-policies`.

## Its own module

The example is a Go module of its own, as a bot author's would be:

```text title="examples/bots/go/go.mod"
module github.com/adams-shaun/gorge/examples/bots/go

go 1.25.8

require github.com/adams-shaun/gorge v0.0.0

replace github.com/adams-shaun/gorge => ../../..
```

Inside this repository the `replace` points at the checkout. Outside it, drop
the `replace` and require a gorge commit (`go get
github.com/adams-shaun/gorge@<commit>`). gorge has no third-party
dependencies, so that is the whole dependency graph.

## Play it against the default bot

`cmd/duel` plays seeded games between two registered policies on real decks, seats traded each game. It is the
in-process path: the engine, both seats and the loop in one process, no network, no clock in any decision. It needs the
card corpus (`make fetch-cards` in the repository root).

```console
$ cd examples/bots/go
$ go run ./cmd/duel -cards ../../../.cards -a bot -b bot -games 40
bot (mono-green-stompy) 28 - 12 bot (mono-red-goblins), 0 draws, 0 stalls
40 games, 14045 decisions in 0.44s: 31.1 µs per decision, both seats, engine included
```

That is the control: `bot` against itself on these seeds and decks, showing what the deck pairing alone does. Run your
policy as `-a allin -b bot` over the same seeds and compare. Forty games is a smoke test, about ±15 percentage points;
[Bot-vs-bot games](../guide/bot-games.md#reading-the-result) covers how many games a real comparison needs.

`duel` is a thin loop over
[`testkit/bench.PlayGame`](https://github.com/adams-shaun/gorge/blob/main/testkit/bench/bench.go),
which takes a `rules.Config` and one `seat.Seat` per seat and returns the
outcome; `bench.RunPairs` runs a deck-pair matrix with seats traded across
worker goroutines.

## Test it

[`allin_test.go`](https://github.com/adams-shaun/gorge/blob/main/examples/bots/go/allin/allin_test.go)
has three tests a policy should copy. None needs the card corpus:
`testutil.SampleDecks` builds two synthetic 40-card decks.

- **`TestRegistered`**: the init ran and `bots.New` builds the policy with its
  declared defaults.
- **`TestPlaysAgainstTheBot`**: ten seeded games against `heuristic.NewBot`
  through `bench.PlayGame`, failing on any stall.
- **`TestHostContract`**: seats the policy at a real host table with
  `host.RunContractTable`, plays the match to the end and replays its log to
  the recorded head. This is the harness gorge runs on its own hosted
  policies
  ([`server/host/contract.go`](https://github.com/adams-shaun/gorge/blob/main/server/host/contract.go));
  its siblings `RunEnvLeakContract` and `RunClosedPrivacyContract` check that
  an Env seat's answers never depend on hidden cards.

```console
$ go test -v ./allin/
=== RUN   TestRegistered
--- PASS: TestRegistered (0.00s)
=== RUN   TestPlaysAgainstTheBot
    allin_test.go:58: allin won 0 of 10 against bot
--- PASS: TestPlaysAgainstTheBot (1.01s)
=== RUN   TestHostContract
    allin_test.go:67: allin: finished turn 20 intents 629 events 3283 rootRefusals 0 feeds false reason ""
--- PASS: TestHostContract (0.05s)
PASS
ok  	github.com/adams-shaun/gorge/examples/bots/go/allin	1.062s
```

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

```console
$ go build -o gorged-allin ./cmd/gorged-allin
$ ./gorged-allin -addr 127.0.0.1:8080 -cards ../../../.cards -decks ../../../testkit/testutil/decks \
    -dir /tmp/gorge-allin -feedback /tmp/gorge-allin/feedback -tables 0 -vsbot -pace 0 -prewarm=false \
    -bot-policy bot -bot-opt allin.min-power=2
$ curl -s http://127.0.0.1:8080/api/bot-policies | jq -r '.policies[].name'
bot
allin
forge-heuristic
forge-heuristic-hold
lethal-pressure
```

`allin` is now offered to `POST /api/games` (`"bot_policy": "allin"`) and to
the browser's play-vs-bot picker. The Python example agent played it:

```console
$ python3 gorge_agent.py --server http://127.0.0.1:8080 --bot-policy allin --policy simple
created table g1 match 1, seat 0 vs allin
...
match finished after 13 turns in 0.7 s: winner us (seat 0, mono-red-prowess)
```

`gorged.Extensions` also carries extra flags (`Flags`), a draft-event runtime (`Events`) and a match-review loader
(`Review`); `gorged.Serve` runs the same server on a listener you supply, which is how a test drives it. The full Go embedding surface is on
the [Go API reference](../api/go.md).

## Bot code that will ship

A bot that will ship follows the [rules for bot code](training-and-adoption.md#rules-for-bot-code),
and that page covers the gates a policy passes on its way from a bench to the default bot.
