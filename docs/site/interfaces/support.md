# Support matrix

Each dot is a claim a test holds. Full means the wire plays whole games and a gate fails if it stops. Partial means it plays,
with the gaps listed below. None means the engine refuses it. The test behind each cell is on [Wires](../architecture/wires.md),
and `TestWireMatrixPageMatchesScript` keeps that list tied to the CI script.

<div class="gg-matrix" markdown>

| Interface | gorge | dirac |
|---|:---:|:---:|
| Native HTTP and stream | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> |
| ManaBrew | <span class="gg-dot gg-dot--part" title="partial: see the gaps below"></span><span class="gg-sr">partial</span> | <span class="gg-dot gg-dot--part" title="partial: see the gaps below"></span><span class="gg-sr">partial</span> |
| SpellBench v2 | <span class="gg-dot gg-dot--part" title="partial: two seats only"></span><span class="gg-sr">partial</span> | <span class="gg-dot gg-dot--part" title="partial: two seats only, some observation fields missing"></span><span class="gg-sr">partial</span> |
| gorge-agent/1 (drafts) | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> | <span class="gg-dot gg-dot--none" title="not supported: a draft event needs gorge"></span><span class="gg-sr">none</span> |
| Paired bot | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> | <span class="gg-dot gg-dot--none" title="not supported"></span><span class="gg-sr">none</span> |
| Go embedding | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> |

</div>

## ManaBrew gaps

=== "On both engines"

    - No spectators: ManaBrew has no such role, so spectating stays native.
    - A rule over a whole answer (attack together, share a controller) has no field. The prompt says it in text, and a broken answer comes back `invalidShape`.
    - The combat rule "can't attack or block alone" has no field either. It is in the prompt text.
    - A reveal from a zone the seat can't see resolves to a blank card carrying only an id.

=== "On dirac"

    - No reveal or dice prompts. Those acknowledge only, because dirac keeps no event log.

## SpellBench v2 gaps

=== "On both engines"

    - Two players only. `/api/sbv2` answers 409 for more seats or Commander, and the web client falls back to native.
    - No spectators.

=== "On dirac"

    - A v2 observation reads no trigger amount, no printed colour off the battlefield and no key for a planeswalker under attack.

## What dirac refuses

Two seats, Constructed, auto-pay only. It refuses `-events`, search bots, the paired bot and review decisions, naming the
reason at startup. See [Serving dirac](../guide/dirac.md).

## Where each cell is checked in code

| Claim | Code |
|---|---|
| Engines implement one seam | `engine/game/game.go:95` (`Engine`), `dirac/adapter/replay.go:14` |
| `-engine` picks the engine | `server/gorged/gorged.go:398` |
| dirac's refusals | `server/gorged/engine.go:28-49` |
| `-sbv2` plays the autopay surface on dirac | `server/gorged/engine.go:72` |
| Paired bot needs gorge | `server/host/pairedbot.go:171` |
| ManaBrew translator covers every decision kind | `server/manabrew/errors_test.go:161`, `server/manabrew/dispatch.go:17` |
| ManaBrew reveal gap | `server/manabrew/synthetic.go:42` |
| ManaBrew alone-restriction in text | `server/manabrew/prompt_combat.go:72` |
| SpellBench dirac env | `dirac/cmd/sbv2dirac/main.go` |
