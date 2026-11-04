# Bot comparison

Every registered bot side by side. The registry is `bots/registry.go`, as linked by `bots/all`, and a test keeps this table in step
with it. Costs are measured on gorge, 2026-10-31, from [Bot speed](../performance/bots.md). The code behind each verdict is on
[Each bot in detail](catalogue.md).

| Bot | What it does | Engines | Seats | Cost per decision | Fair or clairvoyant |
|---|---|---|---|---|---|
| `bot` | The default. Hand-written rules, a two-ply combat simulation on auto-pay tables, Forge's mulligan rule | gorge, dirac | any, human or bot | 29.7 µs | fair: sees only its redacted view |
| `forge-heuristic` | `bot` plus rules re-expressed from Forge's AI: targeting, removal timing, attachments, gang blocks | gorge, dirac | any, human or bot | not measured | fair |
| `forge-heuristic-hold` | `forge-heuristic` with `hold-instant` on: keeps instants back outside response windows | gorge, dirac | any, human or bot | not measured | fair |
| `lethal-pressure` | The older heuristic: an attack-into-lethal rule, no combat simulation, a coin-flip mulligan | gorge, dirac | any, human or bot | not measured | fair |
| `sb-tactical` | SpellBench's scored tactical heuristic, pricing plays through a redealt root | gorge, dirac | bot-only, 2 seats | not measured | fair on hidden cards, but reads every seat's decklist |
| `az-redeal` | AlphaZero-style tree search, 100 simulations per decision, no network | gorge only | bot-only, 2 seats | about 106 ms per searched decision | fair on hidden cards (determinised redeal), but reads every seat's decklist |


**Fair** means the bot uses only what its seat can see. **Clairvoyant** means it reads the opponent's hand, the library order or a
clone of the real engine. No registered bot is clairvoyant. The two search bots redeal the hidden cards from their own
observations, but deal from each seat's real decklist, so a server never seats them against a person. Clairvoyant search exists
only as a bench ceiling.

The four heuristic bots answer on every wire: in process, over HTTP, and as a SpellBench v2 agent
(`spellbench-gorge-agent -policy <name>`). The two search bots need an engine root, which `gorged` and `testkit/bench` supply and
`mtgsim` and the v2 agent (`server/spellbench/agent/policies.go:44`) do not. dirac serves the first five; it deals no search roots.

## Formats and server offer

| Name | Formats, seats | Offered by `gorged` | Package |
|---|---|---|---|
| `bot` | constructed and Commander, any | yes | [`bots/bot`](https://github.com/adams-shaun/gorge/blob/main/bots/bot/bot.go) |
| `forge-heuristic` | constructed and Commander, any | yes | [`bots/forgeheuristic`](https://github.com/adams-shaun/gorge/blob/main/bots/forgeheuristic/forgeheuristic.go) |
| `forge-heuristic-hold` | constructed and Commander, any | yes | [`bots/forgeheuristic`](https://github.com/adams-shaun/gorge/blob/main/bots/forgeheuristic/forgeheuristic.go) |
| `lethal-pressure` | constructed and Commander, any | yes; `gorged`'s default `-bot-policy` | [`bots/lethalpressure`](https://github.com/adams-shaun/gorge/blob/main/bots/lethalpressure/lethalpressure.go) |
| `sb-tactical` | constructed, 2 | no: reads every seat's decklist | [`bots/sbtactical`](https://github.com/adams-shaun/gorge/blob/main/bots/sbtactical/sbtactical.go) |
| `az-redeal` | constructed, 2 | no: reads every seat's decklist | [`bots/azredeal`](https://github.com/adams-shaun/gorge/blob/main/bots/azredeal/azredeal.go) |
