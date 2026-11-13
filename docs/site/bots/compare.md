# Bot comparison

Every bot `gorged` serves, side by side. The registry is `bots/registry.go`, as `cmd/gorged` links it: the seven built-ins of
`bots/all`, the experimental search, SpellBench and cast-profile policies, and the three network policies, which register only when
a server names their network. A test keeps this table in step with it (`cmd/gorged/docs_table_test.go`). Costs are measured on gorge, 2026-10-31, from [Bot speed](../performance/bots.md). The code behind each verdict is on
[Each bot in detail](catalogue.md).

| Bot | What it does | Engines | Seats | Cost per decision | Fair or clairvoyant |
|---|---|---|---|---|---|
| `bot` | The default. Hand-written rules, a two-ply combat simulation on auto-pay tables, Forge's mulligan rule | gorge, dirac | any, human or bot | 29.7 µs | fair: sees only its redacted view |
| `forge-heuristic` | `bot` plus rules re-expressed from Forge's AI: targeting, removal timing, attachments, gang blocks | gorge, dirac | any, human or bot | not measured | fair |
| `forge-heuristic-hold` | `forge-heuristic` with `hold-instant` on: keeps instants back outside response windows | gorge, dirac | any, human or bot | not measured | fair |
| `lethal-pressure` | The older heuristic: an attack-into-lethal rule, no combat simulation, a coin-flip mulligan | gorge, dirac | any, human or bot | not measured | fair |
| `sb-tactical` | SpellBench's scored tactical heuristic, pricing plays through a redealt root | gorge, dirac | bot-only, 2 seats | not measured | fair on hidden cards, but reads every seat's decklist |
| `az-redeal` | AlphaZero-style tree search, 100 simulations per decision, no network | gorge only | bot-only, 2 seats | about 106 ms per searched decision | fair on hidden cards (determinised redeal), but reads every seat's decklist |
| `az-redeal-maxn` | `az-redeal` with the opponents' decision points in its tree, each opponent maximising its own value there (max^n), not all of them ganging up | gorge only | bot-only, up to 4 seats | about 106 ms per searched decision (2-seat; 4-seat pod cost unmeasured) | fair on hidden cards (determinised redeal), but reads every seat's decklist |
| `cast-profile` | `bot` playing from a cast profile; the embedded default is intent-identical to `bot` | gorge | any, human or bot | as `bot` (declared) | fair: sees only its redacted view |
| `sb-first` | SpellBench's first-option builtin | gorge | bot-only, 2 seats | negligible (declared) | fair on hidden cards, but reads every seat's decklist |
| `sb-uniform` | SpellBench's uniform-random builtin | gorge | bot-only, 2 seats | negligible (declared) | fair on hidden cards, but reads every seat's decklist |
| `sb-heuristic` | SpellBench's heuristic builtin | gorge | bot-only, 2 seats | negligible (declared) | fair on hidden cards, but reads every seat's decklist |
| `sb-search-lite-atk` | `sb-tactical` plus a determinised search of its top plays and its attack over four redealt worlds, two turns deep | gorge only | bot-only, 2 seats | 219 ms mean per searched decision (declared) | fair on hidden cards (determinised redeal), but reads every seat's decklist |
| `search` | The search teacher: replans the covered decisions over sampled worlds, `bot` elsewhere | gorge only | bot-only, 2 seats | 664 ms mean per searched decision (declared) | fair on hidden cards (determinised redeal), but reads every seat's decklist |

**Fair** means the bot uses only what its seat can see. **Clairvoyant** means it reads the opponent's hand, the library order or a
clone of the real engine. No registered bot is clairvoyant. The search bots redeal the hidden cards from their own observations;
they deal from each seat's real decklist, so a server never seats them against a person. Clairvoyant search exists only as a
bench ceiling.

The four heuristic bots answer on every wire: in process, over HTTP, and as a SpellBench v2 agent
(`spellbench-gorge-agent -policy <name>`). The search bots need an engine root, which `gorged` and `testkit/bench` supply and
`mtgsim` and the v2 agent (`server/spellbench/agent/policies.go:44`) do not. dirac serves the first five; it deals no search roots.
The rows from `cast-profile` down are linked by `cmd/gorged` beside `bots/all`, so only `gorged` and binaries that link the same
packages offer them.

## Formats and server offer

| Name | Formats, seats | Offered by `gorged` | Package |
|---|---|---|---|
| `bot` | constructed and Commander, any | yes | [`bots/bot`](https://github.com/adams-shaun/gorge/blob/main/bots/bot/bot.go) |
| `forge-heuristic` | constructed and Commander, any | yes | [`bots/forgeheuristic`](https://github.com/adams-shaun/gorge/blob/main/bots/forgeheuristic/forgeheuristic.go) |
| `forge-heuristic-hold` | constructed and Commander, any | yes | [`bots/forgeheuristic`](https://github.com/adams-shaun/gorge/blob/main/bots/forgeheuristic/forgeheuristic.go) |
| `lethal-pressure` | constructed and Commander, any | yes; `gorged`'s default `-bot-policy` | [`bots/lethalpressure`](https://github.com/adams-shaun/gorge/blob/main/bots/lethalpressure/lethalpressure.go) |
| `sb-tactical` | constructed, 2 | no: reads every seat's decklist | [`bots/sbtactical`](https://github.com/adams-shaun/gorge/blob/main/bots/sbtactical/sbtactical.go) |
| `az-redeal` | constructed, 2 | no: reads every seat's decklist | [`bots/azredeal`](https://github.com/adams-shaun/gorge/blob/main/bots/azredeal/azredeal.go) |
| `az-redeal-maxn` | constructed and Commander, up to 4 | no: reads every seat's decklist | [`bots/azredeal`](https://github.com/adams-shaun/gorge/blob/main/bots/azredeal/azredeal.go) |
| `cast-profile` | constructed and Commander, any | yes | [`bots/castprofile`](https://github.com/adams-shaun/gorge/blob/main/bots/castprofile/castprofile.go) |
| `sb-first`, `sb-uniform`, `sb-heuristic` | constructed, 2 | no: reads every seat's decklist | [`bots/sbbuiltin`](https://github.com/adams-shaun/gorge/blob/main/bots/sbbuiltin/sbbuiltin.go) |
| `sb-search-lite-atk` | constructed, 2 | no: reads every seat's decklist | [`bots/sbsearch`](https://github.com/adams-shaun/gorge/blob/main/bots/sbsearch/sbsearch.go) |
| `search` | constructed, 2 | no: reads every seat's decklist | [`bots/search`](https://github.com/adams-shaun/gorge/blob/main/bots/search/search.go) |
