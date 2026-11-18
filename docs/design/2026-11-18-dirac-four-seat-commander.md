# dirac plays four-seat Commander pods

Status: plan, 2026-11-18. Base `origin/main` ae57987d13. It replaces sections 3.6, 3.7
and 10's first row of `2026-11-17-xengine-commander-diffs.md` (branch `wt/xeng-edh`).
Operator rule: Commander is played in 4-player pods only. A 1v1 duel never stands in
for a pod, so #1286's `edh` duel slice is superseded. When dirac cannot seat a pod,
the pod row records dirac as absent (`dirac=n/a(<reason>)`) and no duels are played
in its place.

## 1. Where two seats are assumed

**The size class.** Every seat-dependent knob lives in `dirac/seats_default.go`:
`MaxSeats = 2` (:5), the 8-bit `TCode`/`Obj` (:23, :46), the Action packing with an
8-bit code (:28), `MaxObj = 236` with only two player codes (`PlayerTarget0` 0xFC,
then 0xFE gone and 0xFF none, :38), one owner bit in `object.acts` (:60), and
`maxXTgt` 7, `embRemSlots` 4, `linkPadN` 40, `seatWord` uint8 and `maxFree` 8
(:75, :81, :88, :92, :109). The code is already seat-generic. It has 135
`[MaxSeats]` arrays in 26 files and 80 `MaxSeats == 2`/`> 2` branches in 35 files.
The raw-arithmetic ratchet (`dirac/seatarith_test.go:55`, `testdata/seatarith.tsv`)
has one site left. `InitN` (`engine.go:34`) deals n seats, `NextSeat`/`nSeats`
(`seatorder.go:18`, :50) give turn and priority order, and `listenerOrder`
(`triggers_gen.go:121`) walks every seat's triggers in APNAP order.
`attackOffersN` (`combat.go:202`) offers each attacker against every live
opponent, which covers multiple defenders, attacking each opponent and goad.
Commander damage is a per-seat tally (`state.go:437`, `combat_cmddamage.go:17`).
Genesis puts the commanders in the command zone (`zone_cmdinit.go:31`,
`engine.go:126`). A departed seat follows CR 800.4a (`zone_leavegame.go:37`, :54).
Only the size class pins all of this at two seats.

**Built and then removed.** The `dirac_mp` class set four seats, a 9-bit
`Obj`/`TCode`, `MaxObj` 488 and a 36 KiB bar of its own, `maxStateBytesMP`. It
was deleted on 11-14 in 8bfff46110 ("dirac has one build, two seats"), together
with 68 mp-only files: `seats_mp.go`, `canon_mp.go`, `repro_wire_mp.go`
(koracle-repro-2 kept 8-bit refs and an `mp` packing marker), `setup_mp.go`, the
mp State bar and layout pin, the four-seat lockstep tests, `make
dirac-mp-check`/`dirac-mp-lockstep`, and gate steps 8c and mp.

**What stays two-seat at HEAD, outside the size class:**
- canon: `Ref = uint8`, `Canon uint32` (`canon/canon_default.go:4`, :8).
- Repro and game wire formats: `koracle-repro-2` (`oracle/repro.go:68`) and
  `koracle-game-2` (`oracle/replay.go:80`). These carry default-class bytes only
  (`repro_wire_default.go`).
- The oracle Projection: `MaxInst = dirac.MaxObj` (`oracle/project.go:20`),
  `Life [2]int32` and the other two-seat arrays (:108-121), and
  `CmdDamage [2][2]` (:136). Seats 2 and 3 go through `Seats`/`CmdDamageFrom`
  (:154), and `oracle.MaxSeats = 4` (`oracle/setup.go:75`).
- The adapter: "dirac plays two seats" and "two-seat commander only"
  (`adapter/engine.go:90`, :92, :243), with `[2]` tallies in `board.go` and
  `game.go`. This is out of scope here because gorged serves gorge for pods.
- kfuzz: `-seats` (`main.go:287`) plays a Commander pool above two seats only cut
  with `-cmd-mini` (:333-343). `INFERRED`: in my runs, `-cmd-mini 100/60/40` all
  peaked at 215-236 objects, so the cut size is not applied. Ticket T3 checks this.
- Pools: `PodsFrom`/`PodAt` exist (`oracle/fuzz.go:34`, :51). No registered pool
  has pods: cmd5 is a 2-seat slice (`oracle/pools.go:42`), and the pool table
  holds 254 subtypes (`cardset/pooldata.go:88`), a matchup table 192
  (`kop/table.go:11`).
- support.md: the `4-seat` column is "a ceiling, not evidence: no 4-seat game is
  in the evidence runs" (`dirac/docs/support.md:36`).
- Card lowering: 44 kgen sites call `lc.twoSeat(...)` (`kgen/lower_twoseat.go:15`;
  `TestTwoSeatMarkerCensus`). `kop.CardPodLimited` (`kop/twoseat.go:40`) and
  `prepTwoSeat`/`initStall` (`stallclass.go:318`, :136) stop a game of three or
  more seats at its start with `StallTwoSeat`.

## 2. Budgets, measured

I revived the mp class at ae57987d13 in a scratch worktree with `git revert
8bfff46110`, taking HEAD's side of the 6 conflicts. I then applied the
measurement fixes T1 lists and ran `make test-one TAGS=dirac_mp`.

| | default (HEAD) | mp (revived) |
|---|---|---|
| `TestStateSize` | 21456 B | 29432 B (pin from 11-14: 28728) |
| `TestEveryPoolFitsTheStateBar`, largest of 990 pools | 24536 B, bar 24576 (40 B left) | 34616 B, old mp bar 36864 (2248 B left) |
| share of the 48 KiB L1d (Ryzen 9950X, `lscpu`) | 50% | 70% |
| `perm` (`TestPermSize`) | 64 B | 68 B (fails the old 64 pin) |
| `TestColdArraysHoldNoPointers` | pass | pass |

**Objects.** A pod is 4 × 100 = 400 objects at genesis, inside mp's `MaxObj` 488,
which leaves 88 for tokens. The default class's 236 cannot hold even the decks.
Before it was removed, the mp class measured a 4-seat Commander pod's peak at
480 (p99 452; `seats_mp.go` comment at 8bfff46110^), so 88 objects of token
headroom is tight but holds at p99. The edh pods below never reached a token
peak, because none played past decision 499.

**Lockstep on the revived class.** `TestFourSeatVanillaKernelGenesisAgrees` ran 4
games clean. `TestFourSeatCommanderMiniPodsLockstep` ran 11 cmd5 40-card pods
clean, with command-zone asks, tax, 3-defender commander damage and departed
seats all exercised. `TestPartnerPodLibraryLockstep` passed. #1286's 52 edh decks
in full 100-card pods (seed 1, pods 0-29, random policy) gave:
- 19 pods stall `twoseat` at genesis.
- 10 pods are refused by the interaction guard (atktax 4, replplus 2, liferepl 2,
  comboanymix 2).
- 1 pod played and diverged on `options` at decision 499 (pod 29).

The edh pool holds 44 pod-limited cards: 41 kgen `TwoSeat` (39 lowering sites
plus 2 operand cases) and 3 `StatGoad`. Ghostly Prison, Propaganda and Windborn
Muse are attack tax; the Vow cycle is CantAttack-you; Fact or Fiction and
Fractured Identity are among them. Pod tables reached **191 subtypes of 192**:
6 of the 20 built tables had 187 or more.

## 3. Tickets (Suspect + Repro first)

Every ticket runs capped (`make test-one`). The mp build is `TAGS=dirac_mp`.
`.cards` must be linked into the worktree.

1. **T1 Revive the dirac_mp class at HEAD.** *Suspect:* the revert of 8bfff46110,
   plus code that has assumed one build since then. `turnFT`/`turnDamagedShift`
   are missing from mp (`seats_default.go:11-17`). mp needs `turnDamagedShift`
   8 and `turnFT` uint16. `turnUnlessWin uint8 = 1<<7` (`mana_window.go:1179`)
   is seat 3's Exhaust bit at four seats, so it becomes a per-class `turnFT` bit
   (1<<15 in mp). `rootF []uint8` (`statecopy.go:80`), `rootTurnF` and
   `withRootExhaust` become `turnFT`. `ctr_addorremove.go:69,79,101` assign
   `uint8` to `res.chosen` (objWord). In mp, `rsvN` (`gen.go:2473`) is -3, so
   the reserve block needs 4 more bytes, and perms' phase needs `linkPadN` 88. The
   `TestPermSize` pin becomes 68 under mp. *Repro:* at HEAD `-tags dirac_mp` is a
   no-op: `kfuzz -kernel vanilla -seats 4` prints "this dirac build holds 2
   seats". *Done means:* `make test-one PKG=./dirac TAGS=dirac_mp
   RUN='^(TestStateSize|TestEveryPoolFitsTheStateBar|TestColdArraysHoldNoPointers|TestStateLayoutPinnedMP|TestPermSize)$'`
   and `make test-one PKG=./dirac/oracle TAGS=dirac_mp
   RUN='^TestFourSeatCommanderMiniPodsLockstep$'` PASS. The default class stays
   byte-identical (`make test-one PKG=./dirac RUN='^TestStateLayoutPinned$'`
   unchanged, no head re-pin). Needs D1.
2. **T2 Gate and CI for mp.** Restore `dirac-mp-check`/`dirac-mp-lockstep`, gate
   steps 8c and mp, the CI `dirac-slices` mp step and the LANDGEN mp layout row
   from 8bfff46110^. Add the allocation step (BenchmarkTransitionFDN, 0 allocs/op)
   for mp. *Done means:* `make dirac-mp-check` green and `make test-one
   PKG=./dirac/oracle TAGS=dirac_mp RUN='^TestFourSeatVanillaKernelGenesisAgrees$'`
   PASS. Needs D1.
3. **T3 kfuzz plays full Commander pods.** *Suspect:* `kfuzz/main.go:333-343`
   (podKernel needs `-cmd-mini`) and `addMiniDecks` (:91), whose cut did not
   change the object peak. *Repro:* `kfuzz -kernel cmd5 -seats 4 -pods 20` exits
   with "a kernel game of more than two seats plays ... a -cmd-mini Commander
   pool". *Done means:* `make test-one PKG=./dirac/oracle/cmd/kfuzz TAGS=dirac_mp
   RUN='^TestFullCommanderPods$'` (new) plays 400-object pods, and
   `-cmd-mini 40` reaches 160 objects at genesis.
4. **T4 Pod-limited census.** Count `kop.CardPodLimited` per pool into a ratchet,
   `dirac/testdata/podlimited/<pool>.txt`, that only shrinks. `decks.py --seats 4`
   treats a pod-limited card as refused (the cmd5 basic-land recipe, recorded in
   deck notes). *Repro:* 19 of 30 edh pods stall `twoseat`. *Done means:*
   `make test-one PKG=./dirac/oracle TAGS=dirac_mp
   RUN='^TestPodLimitedCensus$'` PASS, with edh pinned at 44.
5. **T5 The `edh` pool becomes 4-seat.** Reuse #1286's selection and
   `cards.txt`/`ids.tsv`. The registry row gets `Seats: 4`. The slice becomes
   `pods.tsv` (1500 pods at seed 1), and the duel `pairs.tsv` is dropped.
   `TestEDHPodTables`: every pod's table fits `kop.MaxSubtypes`, or counts as a
   `subtypes` refusal, and 400 + 88 fits `MaxObj`. Retry Sultai Arisen against the
   mp bar. *Done means:* `make test-one PKG=./dirac/oracle TAGS=dirac_mp
   RUN='^(TestEDHPodTables|TestPoolRegistry|TestSliceMatchesPool)$'` PASS.
6. **T6 Lockstep baseline `kfuzz -kernel edh -seats 4`** under the random and
   heuristic policies, with `-keep-going -allow-unverified -mulligans 1 -surface
   autopay`. Divergences go to backlog-scope. The first one is known: pod 29,
   `options` at decision 499. *Done means:* `make test-one PKG=./dirac/oracle
   TAGS=dirac_mp RUN='^TestEDHPodsLockstep$'` plays the slice's first 20 pods with
   no divergence, hang or unmodelled stall. Capacity stalls are counted.
7. **T7 Generalise two-seat lowerings by pod count**, the highest count first:
   attack tax (`kgen/lower_attacktax.go:126`), CantAttack-you
   (`lower_combatstatics.go:33-39`) and static goad (`act_goad.go:41`). Each is
   one census-and-ratchet ticket. *Done means:* `make test-one PKG=./dirac/kgen
   RUN='^TestTwoSeatMarkerCensus$'` shrinks, plus a `TAGS=dirac_mp` pod test for
   that card.
8. **T8 Pod evidence.** Clean edh pods credit a separate 4-seat evidence list, so
   support.md's `4-seat` column becomes evidence instead of a ceiling.
   *Done means:* `make test-one PKG=./dirac/kgen TESTENV=DIRAC_SUPPORT_STRICT=1
   RUN='^TestSupportDocIsCurrent$'` PASS.
9. **T9 Close the loop.** The xengine pod rows read `dirac=lockstep` when edh holds
   all four decks with nothing pod-limited, else `dirac=n/a(<card>|seats)`. #1286
   closes once T5 lands.

## 4. Decisions

**Chosen:**
- Four seats come from a second size class, `dirac_mp`, revived rather than
  rewritten: at HEAD it plays 4-seat Commander mini-pods clean after about 10
  lines of fixes. The default two-seat build, `maxStateBytes` and its gate stay
  untouched.
- Pod-limited and refused cards become basics under the cmd5 recipe, with T4's
  census and T7 generalising them back.
- More than 192 subtypes is a counted `subtypes` refusal. The pool past 254 is
  split into shards (edh-a, edh-b), not widened.
- Pod evidence is kept apart from 2-seat evidence.

**For the operator:**
- **D1. Reverse the 11-14 "one build, two seats" ruling.** Four seats cannot fit
  the default class: the State would be 29432 B against the 24 KiB
  `maxStateBytes`, which has 40 B left, and the 9-bit `Obj` and the 68 B perm
  would slow every 2-seat game. The only path that leaves `maxStateBytes` alone is
  the mp class. It needs three things:
  - its own bar again, `maxStateBytesMP` 36 KiB, which the operator approved on
    11-12 and 11-14 (largest pool 34616 B, 70% of L1d);
  - gate steps 8c and mp back in `scripts/dirac-gate.sh`, a protected file,
    costing about 60 s or more a dirac change;
  - the hard rule applied to both builds: a gorge change fixes dirac in both.
    Before the removal, the CI's 30-day dirac reds were all one mp build break
    that only promote CI saw.
- **D2.** Accept mp's `perm` at 68 B, past one cache line, or ask T1 to pack it
  back to 64.
- **D3.** Close #1286 when T5 lands, or now.
