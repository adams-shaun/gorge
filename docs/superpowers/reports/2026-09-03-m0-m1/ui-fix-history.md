# Was the client↔engine interface a primary cause of the play client's misses?

*Read-only study of `/home/sadams/projects/mtgbld` (repo A, production XMage bridge +
browser client) against `/home/sadams/projects/gorge` (repo B, the new Go engine).
Nothing in either repo was modified.*

---

## Executive summary

- **102 play-client commits** (`mtgserve/internal/views/templates/play*`, 2026-05-07 → 2026-07-15).
  Classification: **27 interface limitation (a)**, **49 client rendering/UX (b)**,
  **0 engine/XMage as primary (c)**, **21 new feature (d)**, **5 refactor/merge/revert (e)**.
- Excluding features and refactors, the bug-fix population is **76 commits: 27 (36%) were
  interface limitations, 49 (64%) were pure client bugs.** So the interface was a *major
  contributing* factor, **not the primary one** — the plurality of misses were CSS/flex/layout
  and render-order bugs that no engine interface could have prevented.
- But the interface misses were the **expensive** ones: they caused the two production hangs
  (matches 69/70), the vendored watchdog patch, the "game seems frozen" reports, the one real
  **information leak** (Chrome Mox showing opponents' hands), and every "the engine decided for
  me" complaint. Class (c) reads 0 only because pure engine fixes never touch the client
  templates; the adjacent engine-only population (patches 0002/0014/0015, commits `1a7f833`,
  `b9322cf`, `1d43874`) is where they live, and 3 of those 6 are themselves interface faults.
- **56 of 112 Playwright regression specs** are play-domain, issue-numbered — i.e. at least 56
  of these misses reached a real user, were reported through `match_issues`, and were locked in.
- **11 interface themes** identified. Verdict on "destined to repeat": **partly, and mostly no.**
  **7 of 11 do not recur** under repo B's interface (self-contained views, silent trigger
  answers, engine-side legality, offer/accept asymmetry, stale/misrouted answers, hidden-zone
  leaks, stack observability). **3 recur partly** (trivial-decision hinting, compound/ordering
  decision kinds, liveness). **1 recurs outright** (no "why can't I do X" channel) and one new
  gap — **no printing/art identity on the wire** — reopens repo A's entire artwork family.
- Repo B is a *rules* interface only. It has **no transport, reconnect, timers, concede, chat,
  spectator, persistence, undo or prefs channel** — every miss in those areas will recur for
  reasons unrelated to the rules interface.

---

## 1. Classification table

Every commit touching `mtgserve/internal/views/templates/play.html`,
`.../templates/play/`, or `.../templates/play_*.html`, first-parent from HEAD.

Classes: **a** interface limitation · **b** client rendering/UX · **c** engine/XMage ·
**d** new feature · **e** other (merge/refactor/revert).

| # | sha | date | subject | change | cls |
|---|---|---|---|---|---|
| 1 | `7b9f243` | 07-15 | fix(play): carry free_mulligans into rematch (#4) | Rematch POST rebuilt from match meta but dropped `free_mulligans` | b |
| 2 | `60e3995` | 07-13 | match-id recycling, MDFC land side, mulligan hand render | #125: *"the 'put N on bottom' target prompt carried no board snapshot, so #myHand kept showing the pre-mulligan hand… Non-priority prompts now carry a `me` snapshot"*; +#128 bridge land enumeration (c), +#130 SQLite rowid reuse (e) | **a** |
| 3 | `a28065d` | 07-10 | triage(#117) | Hover preview gains a P/T line | b |
| 4 | `fcc1ffe` | 07-10 | Merge triage/issue-110 | merge | e |
| 5 | `770e6a5` | 07-10 | triage(#116): mulligan prompt accounts for free mulligans | Prompt carried no free-mulligan count; `WebSocketPlayer`+`SeatPlayer` had to add `free_mulligans` before the client could render it | **a** |
| 6 | `6ee4cda` | 07-10 | triage(#110) | Drop a `.corner-piles .pile-label` CSS override | b |
| 7 | `506f103` | 06-26 | reveal opponent hand (#111), delve payment (#114) | #111: *"`lookAtCards`… PlayerImpl's default only records cards for a local Swing GUI — a headless WebSocket seat surfaced nothing"* → new `reveal` frame. #114: *"the WebSocket pay-mana prompt only listed land taps, so delve was never offered"* → also collect `getSpecialActions()` | **a** |
| 8 | `8b1993d` | 06-25 | 7 Legacy system decks | deck fixtures + picker slug map | d |
| 9 | `35df14f` | 06-15 | stash | SOUND toggle + `05-sound.js` WebAudio cues (issue #3) | d |
| 10 | `1f4a397` | 05-25 | two more archetype decks + ai-smoke-ci | AI deck picker | d |
| 11 | `2be9c0d` | 05-25 | human-vs-AI matches via the lobby | opponent-mode radio group | d |
| 12 | `0b8cc24` | 05-23 | replay log follows active frame | *"match_snapshots.seq and match_events.seq are INDEPENDENT monotonic counters"* → client must correlate by `created_at` | **a** |
| 13 | `ea1f201` | 05-23 | Full-board view for anonymous shared replays | new route + payload | d |
| 14 | `0a254ec` | 05-21 | Merge triage/issue-104 | merge | e |
| 15 | `9f60d11` | 05-21 | triage(#104): bfCells attachment map | Client built the attachment map from the row subset it was rendering, losing cross-row auras | b |
| 16 | `7467e96` | 05-21 | triage(#103): treat undo as trivial in auto-pass | Three separate client heuristics decide whether a frame "is really just a yield in disguise"; `undo` wasn't in the allowlist → auto-pass stopped | **a** |
| 17 | `b573200` | 05-21 | triage(#99): no-auto-tap on complex-cost sources | *"Phyrexian Tower's {T}, Sacrifice a creature… its sacrifice cost should NEVER be the engine's choice — that has to be the user's deliberate decision"* | **a** |
| 18 | `85fe61f` | 05-20 | match cancel/delete + replay share-link | lifecycle controls | d |
| 19 | `753e387` | 05-20 | target-prompt panel (#96 #97) | Engine already sent every option and `prompt_text`; the client enumerated only off-board options and demoted the effect text to a hint banner | b |
| 20 | `97c8633` | 05-19 | universal undo (Pass B) | engine bookmark stack + `kind:"undo"` option | d |
| 21 | `cfbad4a` | 05-19 | mandatory flag end-to-end (Pass A, #95) | *"The rules don't permit declining; the cancel was a UI bug that let players silently bypass forced effects."* Engine had to tag `mandatory:true` where `min > 0` across **8** prompt kinds | **a** |
| 22 | `46b537b` | 05-19 | commander pile draggable (#94) | new drag affordance | d |
| 23 | `b83693c` | 05-19 | token art `background-size: cover` (#93) | CSS shorthand reset clobbered background-size | b |
| 24 | `34840b4` | 05-18 | replay log panel (#89) | *"`snap.seq` was always undefined. The engine's buildGodViewSnapshot doesn't carry a seq field; seq is a DB-assigned counter living on the outer wrapper"* | **a** |
| 25 | `179c26d` | 05-18 | text filter on mode prompts >12 options (#83) | 100+ subtype buttons; *"Engine-side filtering… is deferred — needs source-of-truth data XMage doesn't surface in the prompt today"* → client filter box as workaround | **a** |
| 26 | `b3f5786` | 05-18 | action overlay resolves names from every zone (#85) | Options carry only `card_id`; *"all hit the fallback `opt.card_id \|\| '?'` and the action row rendered the raw UUID"* | **a** |
| 27 | `3220a0f` | 05-18 | triage(#88): `.blocker-pick` uses undefined `--bg-0` | white-on-white select | b |
| 28 | `ca2e7ee` | 05-18 | triage(#81) | STACK→STK tab label, side panel 340→227px | b |
| 29 | `9000b70` | 05-18 | graveyard targets clickable (#79), empty commander pile (#80) | Both fixes consume data the client already had (`TARGETING` options; engine's `in_zone` flag) | b |
| 30 | `d38f60f` | 05-18 | stack art + ordering, Choice prompts, combat re-render | #75: *"`so.getName()` returns rule text for triggered/activated abilities"* → engine adds `card_name`/`source_id`. #76: *"every other domain-specific Choice subclass that previously got silently auto-picked"* → surfaced as a mode prompt. #77 client (b) | **a** |
| 31 | `d86e168` | 05-17 | split play.html into 17 chunks | byte-equivalent refactor | e |
| 32 | `c367c3c` | 05-17 | cross-viewer combat indicator (#74) | *"`playerSummary` now emits `attacking` + `attacking_defender_id`… so BOTH the attacking and defending player see the red glow (combat declaration is global knowledge in MTG)"* | **a** |
| 33 | `f99239b` | 05-17 | blocker prompt uses a dropdown (#72) | per-blocker `<select>` instead of a button grid | b |
| 34 | `69b1071` | 05-17 | replay auto-play + speed picker | new control | d |
| 35 | `2decf76` | 05-17 | attached auras/equipment as pills (#73) | *"`playerSummary` now emits `attached_to`… so the client knows which aura/equipment is riding on which creature"* — the relationship was absent from the projection | **a** |
| 36 | `ec50f67` | 05-17 | real token artwork (#68) | *"XMage names the permanent 'Vampire Token' while Scryfall files the same printing under 'Vampire'"* — no printing identity on the wire, art joined by display name | **a** |
| 37 | `30f8dee` | 05-17 | restore battlefield during declare-attackers (#67) | *"The real engine prompt carries only available_attackers + defenders — no me/opponents snapshot — so me.battlefield was undefined and bfCells([]) wiped #myBfNonland"* | **a** |
| 38 | `259718f` | 05-16 | full god-view replay scrubber | new route | d |
| 39 | `6a42513` | 05-16 | legible token rendering (#66) | name band + P/T chip CSS | b |
| 40 | `95869d9` | 05-16 | engine-side log triggers | `GameLogWatcher` subscribes to XMage events | d |
| 41 | `e545315` | 05-15 | unfold creature stacks during declare-attackers (#65) | client grouping | b |
| 42 | `ce9ca5c` | 05-14 | hover-zoom over choose-card grid | drop a modal exclusion | b |
| 43 | `1c57bc5` | 05-14 | interactive damage-assignment prompt (multi_amount) | `getMultiAmountWithIndividualConstraints` used to auto-distribute; new prompt kind + reply | **a** |
| 44 | `099dd63` | 05-14 | {Q} icon + anchor mana modal at cursor | styling + positioning | b |
| 45 | `5d23b11` | 05-14 | multi-undo for manual-mana taps | LIFO bookmark stack | d |
| 46 | `2047321` | 05-14 | X-cost / announceX prompt | *"`announceX` was a stub returning the minimum, so every X cost silently resolved to X=0… The user had no way to know they were being asked, let alone to answer"* | **a** |
| 47 | `e4bba23` | 05-14 | mana icons in ACT-pane labels (#63) | `.textContent`→`renderManaSymbols` | b |
| 48 | `fb297f6` | 05-14 | action buttons hidden in ACT pane (#60) | `flex: 1 0 100%` banner ate the column | b |
| 49 | `38b9467` | 05-14 | #58 LOG rendering + #59 solo orientation | #59: *"Attackers prompts omit me/opponents; only available_attackers + defenders + kind… the user saw their creatures rendered as opp creatures"* | **a** |
| 50 | `f1b89a1` | 05-14 | #57 renderHidden auto-passes ignored-source frames | Two divergent client "trivial frame" definitions → *"the frame sat hidden, no swap, no auto-pass, 'game seems frozen'"* | **a** |
| 51 | `beb2362` | 05-14 | backend-driven game-log channel + LOG/DEV tabs | migration 0014 + `/events?since=N` | d |
| 52 | `955d8ea` | 05-13 | right-side LOG/ACT/STACK panel | layout redesign | d |
| 53 | `273d7ca` | 05-13 | rewrite scaling/layout pipeline | *"Every fix to one perturbed the others"* — 4 invariants + 2 specs | b |
| 54 | `0c99c49` | 05-13 | #55 solo + manualMana swaps focus | *"LAST_PROMPT_BY_SLOT[ACTIVE_SLOT] still held the (now-stale) prior priority frame"* — no supersede/void signal from the engine | **a** |
| 55 | `58fe892` | 05-13 | #54 tokens `is_token` + per-color fallback | Tokens unresolvable by name; engine had to tag `is_token`/`token_color` | **a** |
| 56 | `ceb1796` | 05-13 | #51 tap-pills inert, #52 me-pill obscured | *"`applyPriorityChoice`'s 'activate' case was looping through getPlayable() and skipping every ManaAbility — so the click resolved to FAILED"*: the engine offered options its own handler refused | **a** |
| 57 | `5acb18d` | 05-13 | #49 hand cropped, #50 manualmana re-renders | #50 needed a `{kind:"__refresh__"}` sentinel because there is no way to ask "re-send the decision I'm parked on" (#49 is CSS) | **a** |
| 58 | `c7a7ffc` | 05-12 | #48 mana icon chips + ignore-prompts toggle | icon rendering + a client-side per-card mute list | b |
| 59 | `a52231f` | 05-12 | #46 auto-pass through mana-only, #47 arrows on scroll | *"in manual-mana mode every untapped land is a tap-for-mana activate option, so the engine paused at every step"* → `is_mana` added so the client could re-classify | **a** |
| 60 | `8fb343e` | 05-12 | engine crash visibility, pile scaling, drag ghost | #45: *"The GAME thread's catch block was silently logging the exception — clients sat forever waiting for the next prompt that never arrived"* → new `game_crashed` frame | **a** |
| 61 | `c0c0cdb` | 05-12 | phase 3 manual-mana: tap-pill + modal | `is_mana` grouping + ChoiceColor surfaced | d |
| 62 | `ab13985` | 05-12 | phase 2 manual-mana toggle | prefs frame + mana abilities as activations | d |
| 63 | `e882561` | 05-12 | phase 1 manual-mana pool pips | per-color pips | d |
| 64 | `f84f01d` | 05-12 | revert row-split flex:2 | shrank every card | b |
| 65 | `7f5e5b2` | 05-12 | #42 dedup PlayLandAbility, #41 pile aspect | *"The engine's priority() emits one option per playable land via the dedicated play_land loop, AND another via the getPlayable loop's catch-all"* → double-rendered rows | **a** |
| 66 | `f963b59` | 05-12 | #39 self-vs-self alias | seat-index suffix in solo mode | b |
| 67 | `9e5351c` | 05-12 | #40 picker modal cards 1×1 | `--card-w` scoped to `.play-grid`, modals mount on body | b |
| 68 | `cef611b` | 05-12 | #36 Pass in overlay, #37 pile widths | UX; #34/#35/#38 filed as engine concerns | b |
| 69 | `faf94ce` | 05-12 | #32 actions layout, #33 pass arrow | inherited base.css rules | b |
| 70 | `ca62d5e` | 05-12 | #29 #30 #31 + preview index + 8 fixtures | corner cluster, discard routing, me-pill placement; note the `id` vs `card_id` shape mismatch between zone entries and prompt options | b |
| 71 | `7832ff1` | 05-12 | screenshot attached to bug reports | getDisplayMedia capture | d |
| 72 | `1767ceb` | 05-12 | 6 issues from match 41+40 | player colors, auto-pass delay, pill clipping, chrome, pass triangle, drag-to-play | b |
| 73 | `30b5a43` | 05-12 | 4 issues + match_issues triage tooling | #20: *"forwarded `{kind: opt.kind}`… dropping ability_id / source_id / mode_id / target_id. The engine got `{kind:"activate"}` with no routing fields and rejected the request"* — client dropped data it held, but only possible because the reply has no opaque option id | b |
| 74 | `03f67d6` | 05-12 | 5 issues from match 38 | commander art fallback, pretty JSON, error-toast lock, pass button | b |
| 75 | `e0dec7b` | 05-12 | 4 issues from match 37 | #12: *"the user clicked Cast 5x in 1.2 seconds; engine never responded… no visible cue that the first click had landed"* → client invents `AWAITING_ENGINE` + a 6 s non-response timeout | **a** |
| 76 | `6441f71` | 05-12 | life-val font 1.45→1.85rem | typography | b |
| 77 | `40e90a8` | 05-12 | smooth phase-strip transition | CSS transition | b |
| 78 | `dbd2f5f` | 05-12 | flex `.counter-badge` in corner-counters | chips stacked vertically | b |
| 79 | `3924b61` | 05-12 | mode-prompt fixture + corner styling | fixture + polish | b |
| 80 | `c68b6de` | 05-12 | actionsTitle headers on every prompt | uniform header element | b |
| 81 | `01c6341` | 05-12 | stack-active fixture, corner mana, 1×4 piles | polish | b |
| 82 | `9c2a65b` | 05-12 | compact stack zone, multi-opp fixture | polish | b |
| 83 | `20bf707` | 05-12 | edhplay-style compact layout | full board redesign | d |
| 84 | `fefba64` | 05-08 | drop mulligan banner; ellipsis buttons | overflow fix | b |
| 85 | `b39cfb5` | 05-08 | card-w-max 240 + always-on row-scrubber | scroll affordance | b |
| 86 | `8d4625b` | 05-08 | commander tile thin green border | stray flex override | b |
| 87 | `f87ca18` | 05-08 | pin actions section to 1/3 | `minmax(0,1fr)` | b |
| 88 | `d3366cf` | 05-08 | collapse empty bf/hand rows | `:has()` layout | b |
| 89 | `e986d10` | 05-08 | Revert auto-hide top nav | revert | e |
| 90 | `ee62bdf` | 05-07 | auto-hide top nav | reclaim vertical space | d |
| 91 | `0f3cdf6` | 05-07 | fix card scaling | measure row height directly | b |
| 92 | `976b45f` | 05-07 | override base.css label-input rule | toggle layout | b |
| 93 | `af3c3f6` | 05-07 | toggles as pill widgets | styling | b |
| 94 | `b3dd007` | 05-07 | single-line toggle labels | styling | b |
| 95 | `4f46f78` | 05-07 | per-half borders + in-pane scrubber | layout | b |
| 96 | `4caa85a` | 05-07 | match center-column border style | styling | b |
| 97 | `880da7c` | 05-07 | move stack-count emblem | styling | b |
| 98 | `366b9a3` | 05-07 | Advanced ▾ panel + debug JSON + hover preview | debug tooling | d |
| 99 | `44e1aa1` | 05-07 | stop bf-rows wrapping | specificity fix | b |
| 100 | `0fb7d99` | 05-07 | bracket center column, dot-on-line scrollbars | styling | b |
| 101 | `c06b989` | 05-07 | pin playLog as a grid row | floating log | b |
| 102 | `3799ebe` | 05-07 | extract HTML/CSS/JS to embedded templates | main.go 6687→443 lines | e |

**Totals — a: 27 · b: 49 · c: 0 · d: 21 · e: 5 (n = 102).**

### On the empty (c) column

Class (c) is 0 *by construction of the file filter*: a pure engine/XMage bug is fixed in
`mtgplay/` and never touches a play template. The adjacent engine-only population from the same
period is small and, notably, half of it is also interface-shaped:

| commit / patch | what | class |
|---|---|---|
| `1a7f833` #113 Ponder reorder | *"That overload is what `putCardsOnTopOfLibrary(...anyOrder=true)` calls in a loop to let a player ORDER cards… so Ponder silently auto-resolved and the player never saw a prompt"* | **a** |
| `1d43874` / patch `0015` #1 Chrome Mox | *"the picker exposed opponents' hand cards. (XMage's Swing client happens to restrict the view to the controller, hiding the latent bug upstream.)"* | **a** |
| `b9322cf` #118/#119 Crucible lands | bridge's hand-rolled land enumeration missed graveyard-playable lands | **a** |
| patch `0002` Mana Vault | trigger fired unconditionally and prompted on a no-op upkeep | c |
| patch `0014` Hearthhull | missing printed ability | c |
| patch `0001` watchdog | caps the SBA/trigger loop after two prod hangs | c |

## 2. User-visible misses (cross-reference)

- `tests/e2e/playwright/regressions/` holds **112 specs; 56 are play-domain and issue-numbered**
  (`issue1` … `issue128`). Per the repo's standing rule (*"every fixed issue lands with a
  regression Playwright spec… unless the user waives it"*, `ca62d5e`), that is a direct count of
  play misses that reached a user and were reported.
- Interface-limitation misses with their own locked-in spec:
  `issue51-tap-pill-functional`, `issue54-token-rendering`, `issue57-renderhidden-ignored`,
  `issue60/63/65/66/67/68`, `issue75-stack-card-art`, `issue76-choose-type-prompt`,
  `issue79/80/83/85`, `issue89-replay-gamelog-default`, `issue93/94`,
  `issue95-mandatory-no-cancel`, `issue96-97-target-prompt-clarity`,
  `issue103-autopass-undo-trivial`, `issue104-cross-row-attachment`,
  `issue111-reveal-hand`, `issue113-ponder-reorder`, `issue114-delve-pay-mana`,
  `issue125-mulligan-hand-render`, `issue128-mdfc-land-side`,
  `announce-x-stepper`, `multi-amount-distribution`, `hide-undo-toggle`,
  `pass-b-universal-undo`, `manual-mana-*`, `game-log-*`.
- `make issue-list-all` works locally (12 rows survive in the current prod DB; the #29–#130
  series referenced in commit messages predates a reset). Of the survivors, the **open** ones are
  disproportionately interface-shaped:
  - **#2 (wontfix)** — *"my mana pool shows 4 black 1 red, why can't I cast rakdos the muscle?"*
    Resolution required a human reading a captured `client_state`. There is no legality-reason
    channel; the answer was "commanders are sorcery-speed and the log ended at upkeep".
  - **#7 (open)** — *"goblin recruiter etb action has a strange interface. User isn't sure what
    the prompts are for and why multiple are needed. We need some modal that allows for
    'ordering' and subset selection that also displays what/why the prompt is being displayed."*
  - **#9 (open)** — *"instants or other interaction by the opposing player should exist in the
    log. Info about the stack resolution should get added."*
  - **#10 (open)** — *"pop up modal to select ordering is still not acceptable. We should wire up
    a 'drag and drop' into a new row representing the top of the deck."*
  - **#12 (open)** — *"Buttons should indicate source cards to play and follow up with cards
    using abilities on you — but why one? Need better UI."*
  - **#8 (open, engine)** — Fury evoke/damage-assignment misbehaviour.

## 3. Themes, with mechanism and file:line evidence

### T1 — Prompt frames are not self-contained views
**Mechanism.** Only `kind:"priority"` carries `me`/`opponents`; attackers, blockers, multi_amount,
announce_x, mode, yes_no and pay_mana carry only their own option lists. The client therefore
holds `LAST_PRIORITY_BY_SLOT` and repaints the board from a *previous* frame.

Repo A: `mtgplay/src/main/java/com/adams_shaun/mtgplay/WebSocketPlayer.java:1986`
(`p.add("me", …)` in `buildPriorityPrompt`) vs `:1601`/`:1699` — the attackers and blockers
prompts add `available_attackers`/`defenders`/`available_blockers` and nothing else. Client
compensations: `mtgserve/internal/views/templates/play/scripts/70-state.js:20,26`
(`LAST_PROMPT_BY_SLOT`, `LAST_PRIORITY_BY_SLOT`),
`.../scripts/40-combat.js:25`, `.../scripts/90-bootstrap.js:16,291-292`.
Commits: `30f8dee` (#67), `38b9467` (#59), `60e3995` (#125). Partially fixed for
target/choose_card in 2026-07; **still unfixed for combat prompts today**.

### T2 — The engine silently answers decisions that belong to the player
**Mechanism.** Every `Player` override the bridge did not implement returned a default, and the
player never learned a question had been asked.

Repo A, still live: `WebSocketPlayer.java:1500-1521` —
> *"Until we expose an interactive 'order your triggers' prompt (rare and only matters when 2+
> triggers fire simultaneously…) pick the first available… Triggered-ability ordering is a player
> choice per CR 603.3b but for almost all cases the order doesn't matter."*

and immediately above it, the cost of the *previous* answer:
> *"CRITICAL: returning null here caused the trigger-loop hangs we firefighted twice today
> (matches 69 + 70)… it re-enters its `while (player.canRespond())` loop… and the CPU spins
> forever."*

Same shape elsewhere: `announceX` stub → X=0 (`2047321`); `getMultiAmount…` auto-distribute
(`1c57bc5`); `choose(Choice)` auto-pick (`d38f60f` #76, and #62/#64 Offalsnout);
`choose(Outcome, Cards, TargetCard…)` auto-added everything so Ponder never prompted (`1a7f833`);
auto-tap committing a sacrifice cost (`b573200` #99).

### T3 — Legality and "does this decision matter" re-derived outside the rules engine
**Mechanism (i), bridge side.** `buildPriorityPrompt` hand-rolls land legality:
`WebSocketPlayer.java:2019-2023` (`isActivePlayer && main phase && stack empty && canPlayLand()`)
and then enumerates `getHand()` filtering on front-face `isLand()`. That missed MDFC backs
(`60e3995` #128: *"Enumeration used front-face-only isLand() and filtered the right-half
PlayLandAbility, so no play_land option was offered"*) and graveyard-playable lands (`b9322cf`).

**Mechanism (ii), client side.** Three independent "is this frame really just a yield in
disguise" predicates: the auto-pass branch in `60-render.js`, `isActionablePrompt` and
`renderHidden` in `90-bootstrap.js`. Divergence between them froze games (`f1b89a1` #57:
*"no longer a category of frames that's 'trivial enough to hide' but 'actionable enough to need a
user click'"*), and adding `kind:"undo"` to the engine's option set broke all three (`7467e96`
#103: *"The three checks must stay in sync"*). `a52231f` #46 is the same bug for mana options.

### T4 — Options are not the thing the engine will accept
**Mechanism.** The option list and the submit handler are separate implementations, and the reply
carries no opaque option identifier — the client must know which of
`{card_id, ability_id, source_id, mode_id, target_id}` keys each kind.
- `ceb1796` #51: *"`applyPriorityChoice`'s 'activate' case was looping through `getPlayable()` and
  skipping every ManaAbility… so the click resolved to FAILED with a 'couldn't perform that
  action' toast."* The engine offered an action it then refused.
- `7f5e5b2` #42: *"PlayLandAbility isn't a SpellAbility… and isn't a ManaAbility… so it fell
  through and double-rendered."* The option list was not canonical.
- `30b5a43` #20: forwarding `{kind: opt.kind}` alone made the engine reject the action.

### T5 — No prompt identity or sequence; stale and misrouted answers
**Mechanism.** `incomingChoices` is an unlabelled `LinkedBlockingQueue<JsonObject>`; whatever
object arrives next answers whatever prompt the GAME thread is parked on.
`WebSocketPlayer.java:110` and `:277-280`:
> *"Settings frames… apply to the player instance without unblocking the GAME thread. Intercept
> BEFORE pushing onto incomingChoices — feeding them through the queue would resolve whatever
> prompt the engine is currently parked on with garbage."*

Downstream consequences: the `{kind:"__refresh__"}` sentinel invented to re-emit the current
prompt (`5acb18d` #50, `WebSocketPlayer.java:361`); the client's `AWAITING_ENGINE` lock and 6 s
timeout after a double-click sent five cast frames (`e0dec7b` #12); superseded priority frames
kept as live client state (`0c99c49` #55). The same defect in the persistence interface: two
independent `seq` counters (`0b8cc24`) and a snapshot payload with no `seq` at all (`34840b4`).

### T6 — Hidden information: one leak and one missing channel
**Leak.** `mtgplay/patches/0015-issue-1-chrome-mox-imprint-own-hand.patch`:
> *"`TargetCard.possibleTargets(Zone.HAND)`… scans `getPlayersInRange(...)` — every player's hand
> — so in mtgbld's web serialization the picker exposed opponents' hand cards. (XMage's Swing
> client happens to restrict the view to the controller, hiding the latent bug upstream.)"*

There was no redaction layer between the engine's candidate set and the wire, so an over-broad
engine answer became a live information leak in production (`match_issues` #1, match 219).

**Missing channel.** `506f103` #111: `Player.lookAtCards` had no serialization at all —
*"PlayerImpl's default only records cards for a local Swing GUI — a headless WebSocket seat
surfaced nothing"* — so Gitaxian Probe silently did nothing visible.

### T7 — No stable printing identity; objects joined by display name
**Mechanism.** The wire carries names, and art/oracle data is resolved by a name lookup against
Scryfall. Every naming mismatch became a rendering miss: tokens (`58fe892` #54 → engine adds
`is_token`/`token_color`; `ec50f67` #68 → a whole new `/api/tokens/resolve` endpoint that strips
`" Token"` and gates on `layout='token'`), stack entries named by rules text (`d38f60f` #75 →
engine adds `card_name` + `source_id`), action rows rendering raw UUIDs (`b3f5786` #85), DFC back
faces and art-series printings (`aa7dd50`, `match_issues` #3/#5).

### T8 — No liveness, ack, or error channel
**Mechanism.** `8fb343e` #45: *"The GAME thread's catch block was silently logging the exception —
clients sat forever waiting for the next prompt that never arrived."* Fix invented a
`{type:"game_crashed"}` frame. Two production hangs (`fb297f6`: *"match 70, GAME thread RUNNABLE
for ~31 min inside `GameState.getTriggered` → `checkStateAndTriggered`"*; `beb2362`: *"match 69's
GAME thread spun ~58 minutes… pegging mtgplay at 100% CPU and 2 GB RAM"*) forced vendor patch
`0001` (an iteration cap in `GameImpl`). Client-side there is no ack, so `e0dec7b` #12 added a
lock and a timeout.

### T9 — No "why can't I do X" channel
**Mechanism.** The interface is an allowlist of legal actions with no reasons attached. When a
user asks *why* something is absent, nobody can answer from the wire. `match_issues` #2 was closed
`wontfix` only after a human reconstructed the answer from a captured `client_state`; #12 is open
asking for buttons that explain their source.

### T10 — Compound decisions decomposed into unexplained prompt loops
**Mechanism.** Ordering and subset-selection effects are driven by XMage looping a single-pick
overload, so the client sees N context-free modals instead of one ordering decision
(`1a7f833`: *"`putCardsOnTopOfLibrary(...anyOrder=true)` calls [it] in a loop"*). Users named this
directly: `match_issues` #7 (*"why multiple are needed… a modal that allows for 'ordering' and
subset selection that also displays what/why"*) and #10 (*"wire up a drag and drop into a new row
representing the top of the deck"*). Both still open.

### T11 — The stack, and anything about to hit it, is not observable
**Mechanism.** `stack[]` exists only on the priority frame (`WebSocketPlayer.java:2000-2020`), so
during a target/mode/combat prompt the client has no stack. Triggers that have matched but not yet
been put on the stack are never observable at all. `match_issues` #9 is exactly this, still open.

## 4. Would each theme recur under repo B's interface?

| # | Theme | Mechanism present in repo B? | Recurs? |
|---|---|---|---|
| T1 | Prompt frames not self-contained | **Present.** `view.Project` builds a full `View` and attaches the `Decision` to it (`view/view.go:169-236`, `:72`); `Seat.Decide(ctx, v, d)` receives both (`seat/seat.go:19`). Spec step 2: *"`view` projects state for that seat and attaches the decision."* There is no code path that emits a decision without a view. | **No** |
| T2 | Engine silently answers player decisions | **Present** for triggers: `KTriggerOrder` (`decision/decision.go:35`; `rules/trigger.go:561`) and `KTriggerOptional` (`decision/decision.go:41`; `rules/trigger.go:616`, *"an optional trigger reaches the stack only on an explicit yes"*). `Engine.Advance` blocks (`rules/engine.go:174-178`) — there is no auto-pick fallback anywhere. Repo A's exact hang (`chooseTriggeredAbility` returning `abilities.get(0)`) is structurally impossible. **Partial** elsewhere: `KMulligan`/`KModes` are declared but unused outside tests, and X-costs, divided damage, library ordering and cost-payment choices have no `Kind` yet — each is a future implementation that could re-take the shortcut. | **No** for triggers; **partly** for kinds not yet modelled |
| T3(i) | Legality re-derived outside the engine | **Present.** `rules/legal.go:16` — *"legalActions enumerates everything p may legally do with priority. The result is the complete rules surface a client ever sees."* One enumeration, inside the engine, feeding both the Decision and `handlePriority` (`rules/legal.go:78`). No bridge, no second implementation. | **No** |
| T3(ii) | Client infers whether a decision is trivial | **Absent.** Nothing on `Decision` says "this is a no-op / auto-passable"; a client that wants auto-pass must inspect `Options` and decide — exactly repo A's `isActionablePrompt`. Only the degenerate "one option, `pass`" case is unambiguous. | **Partly** |
| T4 | Offer/accept asymmetry, no option id | **Present.** `Intent.Choices []int` index the engine's own `Decision.Options` (`decision/decision.go:85`); `Chosen` resolves them back to the engine's stored `Option` (`:117-128`); routing data the client must never echo is server-side-only — `Option.Attacker` / `Option.AltCostIndex` / `Decision.Source` all `json:"-"` (`:56,65,78`). `Validate` rejects out-of-range and duplicate indices (`:100-109`). An offered option is by construction the object the handler acts on. | **No** |
| T5 | Stale / misrouted answers, unsequenced messages | **Present.** `Decision.Seq` is stamped from log length (`rules/engine.go:168`); `Validate` rejects seq mismatch and wrong player (`decision/decision.go:91-96`); `Submit` rejects when nothing is pending and after game over (`rules/engine.go:183-191`). Spec step 4: *"out-of-range, duplicate, wrong-player and wrong-sequence are all rejected."* One log, one `Seq` (`events/log.go:42`), so the two-counter replay bug (`0b8cc24`, `34840b4`) has no analogue. Re-asking "what am I being asked?" is a re-projection, so no `__refresh__` sentinel is needed. | **No** |
| T6 | Hidden-zone leak / missing reveal | **Present.** Hand and Pool only for `p.ID == viewer` (`view/view.go:215-224`); hidden zones contribute counts only (`:208-210`); `Decision` attached only to its owner and never to a spectator (`:199,228`); `Object.Remembered` is never projected (`:395-399`); `RedactEvents` is state-aware with a closed default — an unresolvable id is treated as hidden (`view/redact.go:78-124,131-137`), Secret events keep only their shape via an allowlist (`:24-35`), and pairs are dropped whole (`:152-162`). Reveal is a public `Note` (`:105-114`), which is the `lookAtCards` channel repo A lacked. **Residual gap:** `Project` copies `d.Options` verbatim (`view/view.go:232-234`) with no `visibleTo` filter, so the Chrome Mox shape is prevented only because `legalActions` is correct, not by defence in depth. | **No**, with one residual gap worth closing |
| T7 | No printing identity; name-based joins | **Absent.** `CardView` carries `ID`/`Name`/`Types` only (`view/view.go:112-131`) and `cards` IR `Face` carries `Name`, `Types`, `Oracle` — no set code, collector number, or image identity anywhere. Any client fetching artwork must again join by display name, which is precisely how #54/#66/#68/#93 and the DFC/art-series family arose. `ObjID` is stable and solves the *identity of an object in play*, not the *identity of a printing*. | **Yes** |
| T8 | Liveness / ack / crash channel | **Partly present.** The rules-side causes are designed out: bounded trigger cascades (`maxTriggerFires = 256`, `rules/trigger.go:59,120`), a bounded SBA fixed point (`maxSBAPasses`, `rules/sba.go:218`), a bounded resolve chain, and a resumable trigger drain (`rules/engine.go:44-57`) that cannot re-ask the same group. There is no watchdog to need. **But** there is no transport, so no ack, no crash frame, no heartbeat; the only timeout affordance is the `ctx` on `Seat.Decide` (`seat/seat.go:19`). | **Partly** — cause removed, symptom channel not built |
| T9 | No "why can't I do X" | **Absent.** `legalActions` omits illegal actions with no reason; `Option.Label` is a verb plus a name. It is arguably *worse*: `cast` is offered only when `e.adjustedCost(p, id).CanPay(pool)` (`rules/legal.go:41`), i.e. mana must already be floating, so "my card just isn't there" becomes the normal case with no explanation — repo A's `match_issues` #2 verbatim. | **Yes** |
| T10 | Compound / ordering decisions | **Partly present.** `KTriggerOrder` is the correct shape and proves the pattern: one decision, `Min == Max == n`, and *"Validate's existing 'N distinct in-range indices' rule already means 'a permutation' and no new wire format is needed"* (`decision/decision.go:21-34`). But it exists only for triggers — there is no `Kind` for library ordering, scry, or subset selection, which is what `match_issues` #7 and #10 are about. | **Partly** |
| T11 | Stack + pending observability (R3) | **Present.** `View.Stack` and `View.Pending` are public for every seat on every view (`view/view.go:66-71`); `Chars.PendingTriggers` is explicitly R3 (`:37-40`); `StackView` carries kind, name, source, text and chosen targets (`:133-145`); `PendingView` carries `Optional` and `Decider` (`:157-163`); ability objects get their source's name and `TriggerDescription$` rather than rules text (`:344,360-382`) — the exact defect `d38f60f` #75 patched. | **No** |

**Aggregate verdict.** Of 11 themes, **7 do not recur**, **3 recur partly**, **1 recurs
outright**, plus one new gap (T7) that repo A had already paid for and repo B has not yet
addressed. Weighted by the 27 interface-limitation commits: **21 of 27 map to themes repo B
closes** (T1 ×4, T2 ×6, T3(i) ×1, T4 ×2, T5 ×5, T6 ×2, T11 ×1), **4 map to themes it closes
partly** (T3(ii) ×3, T8 ×1), and **2 map to T7, which it does not address**.

## 5. What repo B does not have that repo A's history proves clients need

None of these are rules-interface faults; misses here would recur for unrelated reasons.

| Need | Evidence it is needed (repo A) | Repo B state |
|---|---|---|
| Transport | Browser talks WebSocket straight to mtgplay:8765 | D5 is design only. `cmd/` holds `forgec` alone; no server, no envelope, no versioning |
| Match host / single-writer loop | `MtgPlayServer.Match`, per-match executor, thread-leak incident #38 | D6 is design only |
| Reconnect + prompt replay | `swapSocket`, `getLastPromptSent`/`replayLastPrompt`, `/api/matches/{id}/events?since=N` backfill, re-issue attach-AI on reconnect, `tests/e2e/match_reconnect.py` | Nothing. Re-projection makes it *easy*, but no session, no resume, no backfill |
| Timers / idle TTL / auto-concede | *"Choice waits used to time out after 60s"*; `match.idle_ttl_seconds`; abandonment sweep | Only `ctx` on `Seat.Decide` |
| Concede | `smoke/concede.spec.ts`, concede control in the play UI | Absent |
| Spectate | `/api/share/replays/{token}`, anonymous full-board replay | `Project` treats an out-of-range viewer as a spectator (`view/view.go:193-199`) — a projection property only; no enumeration or authorization |
| Persistence / replay | migrations 0014 `match_events`, 0016 `match_snapshots`, share tokens, god-view snapshots, scrubber + autoplay | Log is in memory. D4's hash chain exists (`events/log.go`); checkpoints, storage and replay UI do not |
| Undo | engine bookmark stack, `UNDO_STACK_CAP = 16`, universal undo (`97c8633`), `HIDE UNDO` toggle | Append-only log, no rewind API. Note `97c8633`'s multiplayer guardrail — *"You can undo within your own priority window, never across a handoff"* — is a real constraint any future design inherits |
| Client prefs channel | `{type:"prefs", manualMana}`, `IGNORED_SOURCE_IDS`, `HIDE_UNDO`, auto-pass, zoom, sound | No non-decision inbound message type at all |
| Mulligan / free mulligans | `free_mulligans` match config, `mulligansTaken`, London bottoming, issues #4/#11/#116/#125 | `KMulligan` is declared (`decision/decision.go:19`) and referenced nowhere outside tests |
| Chat / issue reporting | `match_issues`, screenshot capture, triage bot | Absent |
| AI seat | `AIPlayer`, `SeatPlayer`, `attachAiSeat`, policy vocab gating | `Seat` interface exists (D8); no in-tree bot outside tests |
| Printing / art identity | `/api/cards/resolve`, `/api/tokens/resolve`, DFC front-face lookup, art-series skip | Absent (see T7) |
| Legality explanations | `match_issues` #2, #12 | Absent (see T9) |
| Trivial-decision hint | three client heuristics, `AUTO-PASS` toggle | Absent (see T3(ii)) |

## 6. Answer to the question asked

**Were interface limitations a primary factor?** They were a *substantial* factor, not the
primary one. 36% of bug-fix commits (27/76) were interface limitations; 64% were client
rendering and layout bugs that a perfect engine interface would not have prevented. What the
interface faults lack in count they make up in severity: they produced the two production hangs
and the vendored watchdog, the only information leak, every "the engine decided for me"
complaint, and the four themes that are still open in `match_issues` today.

**Would they repeat under repo B?** Mostly no. The four defects that did the most damage —
decisions the engine answered on the player's behalf (T2), legality re-derived outside the rules
(T3i), an offer the handler would then refuse (T4), and an unlabelled answer queue (T5) — are
each closed by a specific, cited mechanism, and R1/R2/R3 close the trigger-ordering, optional-
trigger and observability gaps that repo A never even attempted. The residual risks are
(1) decision kinds that do not exist yet being implemented with repo A's shortcuts, (2) no
printing identity, so the entire artwork miss family is unaddressed, (3) no way to explain an
absent action, and (4) everything above the rules layer — transport, reconnect, timers, concede,
persistence, undo — being unbuilt, which is where the majority class (b) of repo A's misses will
land again regardless.
