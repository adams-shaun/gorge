# Payment surfaces by wire

A decision surface is the set of questions a wire puts to a seat while a spell is paid for. Pick a wire.

Every gorge wire sits on the same cast flow. The engine announces the choices of CR 601.2b first, then offers the mana window of CR 601.2g, then pays. What differs is how much of that a wire hands to the seat.

=== "gorge native"

    The seat sees three ways to pay. It can answer a plain `cast`, submit a payment plan, or announce the cast and tap in the window.

    ```mermaid
    flowchart LR
      A[Priority] --> B[Announce]
      B --> C[Plan or window]
      C --> D[Tap or Auto-fill]
      D --> E[Paid]
      C --> F[Cancel cast]
    ```

    - **Plans.** With `auto_mana` on, the host asks the engine for payment plans and attaches them to the priority decision as `payment_actions`. A table without it never sends plans to a human seat ([`server/host/match.go:624`](https://github.com/adams-shaun/gorge/blob/main/server/host/match.go#L624), [`server/host/table.go:151`](https://github.com/adams-shaun/gorge/blob/main/server/host/table.go#L151)). The `-auto-mana` and `-bot-auto-mana` flags set it ([`server/gorged/gorged.go:416`](https://github.com/adams-shaun/gorge/blob/main/server/gorged/gorged.go#L416)).
    - **Two intents.** `payment` carries a whole plan. `announce` names an action and starts the cast with no plan ([`engine/decision/decision.go:1316`](https://github.com/adams-shaun/gorge/blob/main/engine/decision/decision.go#L1316), [`announce.go:56`](https://github.com/adams-shaun/gorge/blob/main/engine/decision/announce.go#L56)). An action with no plan cannot be announced.
    - **The mana window.** It opens only when the pool cannot pay and an untapped source exists ([`cast_paymentplan.go:269`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/cast_paymentplan.go#L269)). After an announce it carries the cost, what is still owed and the pool, with per-colour `mana` options, `autofill`, `undo_tap` and `cancel_cast` ([`announce_pay.go:98`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/announce_pay.go#L98), [`decision/announce.go:26`](https://github.com/adams-shaun/gorge/blob/main/engine/decision/announce.go#L26)). Without one it offers `activate` and `done`.
    - **Which casts get a plan.** Only the printed-cost cast from hand, command zone, flashback and the plain may-play ([`payment_plan.go:602`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/payment_plan.go#L602)). The planner also declines a cost with X, snow, hybrid or life ([`pay/classify.go:128`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/pay/classify.go#L128)) and a convoke, delve, modal or optional-cost shape ([`payment_plan.go:312`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/payment_plan.go#L312)). Those casts go through announcements and the window.
    - **One plan per action.** The type holds a list ([`decision/payment_plan.go:232`](https://github.com/adams-shaun/gorge/blob/main/engine/decision/payment_plan.go#L232)). The planner builds one ([`payment_plan.go:667`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/payment_plan.go#L667)), ranked by the planner. A seat accepts it or pays another way.

    **Announcements, in order.** The cast asks for these before any mana moves:

    | Announcement | Ask | Where |
    |---|---|---|
    | Either-or additional cost | `altaddcost` choice | [`cast_asks.go:27`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/cast_asks.go#L27), called at [`cast_begin.go:1101`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/cast_begin.go#L1101) |
    | Convoke, improvise, harmonize, waterbend taps | one option per contribution | [`pay/castasks.go:55`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/pay/castasks.go#L55), [`cast_begin.go:1202`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/cast_begin.go#L1202) |
    | The value of X | `x` choice | [`cast_subcounter.go:96`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/cast_subcounter.go#L96) |
    | Delve | graveyard cards to exile | [`cast_subcounter.go:581`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/cast_subcounter.go#L581) |
    | Each hybrid and Phyrexian pip | `pay_<colour>`, `pay_generic`, `pay_life` | [`pay/castasks.go:209`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/pay/castasks.go#L209), [`cast_begin.go:1253`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/cast_begin.go#L1253) |
    | Sacrifice, discard, tap-another costs | chosen as the cost is paid | [`cast_begin.go:1289`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/cast_begin.go#L1289) |

    A kicked, flashback or other alternative-cost cast is a separate `cast` option, with its mode on the option.

=== "gorge-agent/1"

    The wire carries the native decision, so the surface is the native one. The agent's `hello` picks whether it sees plans.

    ```mermaid
    flowchart LR
      A[hello] --> B{payment_actions}
      B -->|true| C[Plan or announce]
      B -->|false| D[Taps and cast]
      C --> E[Paid]
      D --> E
    ```

    - `payment_actions: true` makes the driver attach plans before the decision is handed over, and the agent may answer with `intent.payment` or `intent.announce` ([`draft/draftlab/extern/player.go:57`](https://github.com/adams-shaun/gorge/blob/main/draft/draftlab/extern/player.go#L57), [`:174`](https://github.com/adams-shaun/gorge/blob/main/draft/draftlab/extern/player.go#L174); [`engine/seat/seat.go:42`](https://github.com/adams-shaun/gorge/blob/main/engine/seat/seat.go#L42)).
    - `false` leaves the ordinary options, so the agent taps lands and casts.
    - `skip_forced` never skips a decision that carries a payment choice ([`player.go:212`](https://github.com/adams-shaun/gorge/blob/main/draft/draftlab/extern/player.go#L212)).

    The wire is specified in [gorge-agent/1](../api/gorge-agent.md).

=== "SpellBench v2"

    The engine neutral vocabulary has no field for a plan. A gorged table declares `engine_autopay`, so the agent never taps.

    ```mermaid
    flowchart LR
      A[hello_ok] --> B[engine_autopay]
      B --> C[cast_spell]
      C --> D[Planner pays]
      D --> E[Paid]
      B -.-> F[asks=native]
    ```

    - **Autopay (the default).** `cast_spell` pays with the planner's first plan. The host itself answers a mid-resolution mana window, a mana ability's colour ask and a triggered cost's pay-or-decline, and counts those as `x_gorge.autopay_fallbacks` ([`env/identity.go:114`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/env/identity.go#L114), [`session/session.go:119`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/session/session.go#L119), [`agent/agent.go:368`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/agent/agent.go#L368)).
    - **A hybrid pip.** The host picks the first offered face. v2 has no kind for it ([`mapping/payment.go:18`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/mapping/payment.go#L18)).
    - **X.** The agent answers `choose_number` ([`mapping/simple.go:249`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/mapping/simple.go#L249)).
    - **Convoke-family taps.** One `select_object` per contribution object ([`mapping/tail.go:121`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/mapping/tail.go#L121)).
    - **Manual mana.** `spellbench-gorge-env -manual-mana` offers `activate_mana_ability` candidates instead ([`cmd/spellbench-gorge-env/main.go:26`](https://github.com/adams-shaun/gorge/blob/main/cmd/spellbench-gorge-env/main.go#L26), [`protocol/kinds.go:31`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/protocol/kinds.go#L31)). gorged's own tables do not.
    - **Taps on gorged.** A spell with no plan, such as an X spell or an equip, is offered only once mana floats. The client posts the tap on the native routes ([spellbench-v2.md](../api/spellbench-v2.md)).
    - **`asks=native`.** A seat attached this way answers the colour ask, the pay-or-decline, the mana window and each flexible pip itself.

    The wire is specified in [SpellBench v2](../api/spellbench-v2.md).

=== "ManaBrew"

    ManaBrew has a payment prompt of its own, `payManaCost`. gorge maps the announced window to it.

    ```mermaid
    flowchart LR
      A[chooseAction] --> B[pay-id action]
      B --> C[payManaCost]
      C --> D[act, auto or cancel]
      D --> E[Paid]
    ```

    - A plan-backed cast rides on `chooseAction` as a `pay-<id>` cast action. Choosing it sends `intent.announce`, never a plan ([`server/manabrew/prompt_priority.go:48`](https://github.com/adams-shaun/gorge/blob/main/server/manabrew/prompt_priority.go#L48), [`:240`](https://github.com/adams-shaun/gorge/blob/main/server/manabrew/prompt_priority.go#L240)).
    - `payManaCost` lists one `activateAbility` action per source, ability and colour, plus `undoMana` and `autofill`. `canConfirmFromPool` is true only when a `done` option exists ([`prompt_payment.go:40`](https://github.com/adams-shaun/gorge/blob/main/server/manabrew/prompt_payment.go#L40), [`:69`](https://github.com/adams-shaun/gorge/blob/main/server/manabrew/prompt_payment.go#L69)).
    - The answers are `act` for a tap or undo, `pay` with `auto` for Auto-fill, and `cancel` for the cancel option. A cancel is refused on a window that does not offer one ([`prompt_payment.go:154`](https://github.com/adams-shaun/gorge/blob/main/server/manabrew/prompt_payment.go#L154)).
    - The cost shown is the resolved amount owed, not the printed pips.
    - Convoke-family taps and the hybrid pip ask are plain selections over labels ([`prompt_choose.go:97`](https://github.com/adams-shaun/gorge/blob/main/server/manabrew/prompt_choose.go#L97)). An either-or additional cost arrives as a generic label selection.

    The wire is specified in [ManaBrew](../api/manabrew.md).

=== "dirac"

    dirac has no manual mana. Mana abilities are never priority options, and a cast is offered when the pool and untapped sources can pay.

    ```mermaid
    flowchart LR
      A[Priority] --> B[Cast offered]
      B --> C[X and pips]
      C --> D[Planner order]
      D --> E[Paid]
    ```

    - **Two surfaces.** Autopay offers a spell the planner never prices only when the floating pool pays it. Full also offers the plays gorge reaches by lowering a potential play ([`dirac/lockstepconfig.go:10`](https://github.com/adams-shaun/gorge/blob/main/dirac/lockstepconfig.go#L10), [`dirac/state.go:71`](https://github.com/adams-shaun/gorge/blob/main/dirac/state.go#L71)). The lockstep checks both, with `kfuzz -surface autopay|full` ([`kfuzz/main.go:93`](https://github.com/adams-shaun/gorge/blob/main/dirac/oracle/cmd/kfuzz/main.go#L93)).
    - **Serving.** `gorged -engine dirac` refuses `-auto-mana=false` and `-bot-auto-mana=false` ([`server/gorged/engine.go:28`](https://github.com/adams-shaun/gorge/blob/main/server/gorged/engine.go#L28), [`dirac/docs/serving.md:54`](https://github.com/adams-shaun/gorge/blob/main/dirac/docs/serving.md#L54)). It plays on the full surface, and on the autopay one when `-sbv2` is on ([`engine.go:70`](https://github.com/adams-shaun/gorge/blob/main/server/gorged/engine.go#L70)). `dplay` defaults to autopay ([`dirac/cmd/dplay/game.go:39`](https://github.com/adams-shaun/gorge/blob/main/dirac/cmd/dplay/game.go#L39)).
    - **X and pips.** X spells are offered with X from 0 up to the most affordable, and a hybrid pip's colour is asked after modes and before targets ([`dirac/doc.go:196`](https://github.com/adams-shaun/gorge/blob/main/dirac/doc.go#L196), [`:334`](https://github.com/adams-shaun/gorge/blob/main/dirac/doc.go#L334)).
    - **Order.** The kernel spends sources in gorge's planner order, so both engines tap the same lands ([`dirac/doc.go:116`](https://github.com/adams-shaun/gorge/blob/main/dirac/doc.go#L116)).
    - **Deviation on record.** The autopay priority vocabulary has no mana-ability options and no concede ([`dirac/docs/DEVIATIONS.md:15`](https://github.com/adams-shaun/gorge/blob/main/dirac/docs/DEVIATIONS.md#L15)).

    Read more in [dirac serving](https://github.com/adams-shaun/gorge/blob/main/dirac/docs/serving.md) and the [dirac guide](../guide/dirac.md).
