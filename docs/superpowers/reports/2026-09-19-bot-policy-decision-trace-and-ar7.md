# Bot policy decision trace and AR7 evaluation

Date: 2026-09-19  
Corpus pin: `95f04e8a04c8925fa97cb226fc3341cabcc90a53`  
Trace implementation commits: `14d3e37`, `c8f047c`, `2a87693`

The completion audit subsequently hardened atomic publication: injected write
and publication failures now prove that no destination or temporary sibling is
left behind, and a destination created concurrently at the publication
boundary is preserved rather than overwritten. Publication uses an atomic
same-directory hard link followed by removal of the temporary name, avoiding
the check-then-rename overwrite race while retaining a fully-written file at
the instant the destination appears.

## Method

The suite is the approved ten unordered pairs over `mono-white-equipment`,
`mono-blue-tempo`, `mono-black-aggro`, `mono-red-prowess`, and
`mono-green-stompy`. Policies trade seats every game. Development uses seed 0
and 100 games per pair; held-out uses seed 1,000,000 and 400 games per pair.
Intervals are botbench's 95% normal-approximation binomial intervals.

The common command shape was:

```sh
/tmp/gorge-botbench-pressure \
  -a <A> -b <B> \
  -pairs 'mono-white-equipment:mono-blue-tempo,mono-white-equipment:mono-black-aggro,mono-white-equipment:mono-red-prowess,mono-white-equipment:mono-green-stompy,mono-blue-tempo:mono-black-aggro,mono-blue-tempo:mono-red-prowess,mono-blue-tempo:mono-green-stompy,mono-black-aggro:mono-red-prowess,mono-black-aggro:mono-green-stompy,mono-red-prowess:mono-green-stompy' \
  -games <100-or-400> -seed <0-or-1000000> -workers 0 -out json
```

The baseline and held-out head-to-heads additionally used
`-decision-trace /tmp/<name>.jsonl`; diagnostics were produced with
`/tmp/gorge-botbench-pressure -analyze-trace /tmp/<name>.jsonl`. Raw JSONL
files remain outside git.

## Trace baseline and family selection

The baseline `bot` self-control was 515-485 over 1,000 development games:
51.5%, 95% CI [48.40%, 54.60%], zero draws, stalls, errors, or livelocks.
All ten pair intervals included 50%. Seat 0 won 38.5% [35.48%, 41.52%]; seat
0 started 471 games and won 180 of those, while seat 1 started 529 and won
324.

The trace contained 427,480 records (1.8 GiB). The principal non-priority
diagnostic proxies for `bot` were:

| family | opportunities | singleton | offered options |
| --- | ---: | ---: | ---: |
| attackers | 11,243 | 4,166 | 26,520 |
| target | 8,172 | 1,358 | 30,904 |
| blockers | 3,476 | 1,374 | 8,812 |
| choose | 2,735 | 473 | 11,107 |
| trigger order | 1,288 | 0 | 3,704 |
| trigger optional | 789 | 0 | 1,578 |
| modes | 588 | 0 | 1,890 |

These are diagnostic proxies, not counterfactual regret. Combat was selected
because attacker choices were the highest-frequency non-priority family and
63% were non-singletons. The narrow hypothesis was that a creature which is
lethal if unblocked should attack through an otherwise unfavorable block: it
either wins or forces the defender to spend a blocker. Missing life remains
unknown, and all ties retain deterministic offered-index ordering.

The historical `legacy` comparison was 568-332 over 900 completing games
(63.11%, [59.96%, 66.26%]) with 100 intent stalls. Every stall was at the
20,000-intent watchdog; there were no errors or livelocks. Because stalled
games are excluded, this is not a clean strength estimate.

## Development experiments

The first experiment—proven lethal burn face before a future-turn creature
threat—was rejected. Candidate versus baseline was 517-483 (51.7%, [48.60%,
54.80%]), with no pair interval excluding 50% and only two changed outcomes.
Candidate versus legacy was byte-for-byte identical to the baseline-versus-
legacy result. The policy code was removed.

AR7 lethal combat pressure was retained provisionally for held-out testing.
Candidate versus baseline was 529-471 (52.9%, [49.81%, 55.99%]), zero stalls
or livelocks. Pair A wins were 56, 56, 52, 52, 48, 52, 50, 53, 56, 54 in
manifest order; no pair point estimate regressed relative to the baseline
self-control. Candidate self-control was 518-482 (51.8%, [48.70%, 54.90%]),
zero stalls/livelocks. Seat/start counts were:

| run | seat 0 rate (95% CI) | seat 0 starts/wins | seat 1 starts/wins |
| --- | --- | ---: | ---: |
| candidate vs baseline | 38.5% [35.48%, 41.52%] | 471 / 180 | 529 / 324 |
| candidate self-control | 38.6% [35.58%, 41.62%] | 471 / 182 | 529 / 325 |

## Held-out gate

Candidate versus baseline passed the clean gate: 2,066-1,934 over 4,000
games, 51.65%, 95% CI [50.10%, 53.20%], with zero draws, stalls, errors, or
livelocks. Nine pair point estimates favored the candidate and one was 49%;
the improvement is broad rather than supplied by one pair, although no
individual 400-game pair interval excludes 50%.

| pair | candidate-baseline | rate | 95% CI |
| --- | ---: | ---: | ---: |
| white-blue | 203-197 | 50.75% | [45.85%, 55.65%] |
| white-black | 203-197 | 50.75% | [45.85%, 55.65%] |
| white-red | 202-198 | 50.50% | [45.60%, 55.40%] |
| white-green | 216-184 | 54.00% | [49.12%, 58.88%] |
| blue-black | 204-196 | 51.00% | [46.10%, 55.90%] |
| blue-red | 196-204 | 49.00% | [44.10%, 53.90%] |
| blue-green | 211-189 | 52.75% | [47.86%, 57.64%] |
| black-red | 204-196 | 51.00% | [46.10%, 55.90%] |
| black-green | 215-185 | 53.75% | [48.86%, 58.64%] |
| red-green | 212-188 | 53.00% | [48.11%, 57.89%] |

Candidate self-control was 2,017-1,983 (50.43%, [48.88%, 51.97%]), zero
stalls/livelocks. Candidate-versus-baseline seat 0 was 35.50% [34.02%,
36.98%]; seat 0 started 1,936 games and won 696, while seat 1 started 2,064
and won 1,340. Self-control seat 0 was 35.08% [33.60%, 36.55%]; its starting
split was 1,936/691 and 2,064/1,352.

Candidate versus legacy was 2,274-1,360 over 3,634 completing games: 62.58%,
[61.00%, 64.15%], with 366 intent stalls and zero livelocks. Pair results in
manifest order were 207-93 (100 stalls), 125-176 (99), 231-80 (89), 186-136
(78), 212-188, 251-149, 233-167, 269-131, 298-102, and 262-138. Eight pair
intervals favored the candidate, one favored legacy (white-black), and one
included 50%. Seat 0 was 30.71% [29.21%, 32.21%]; starting counts/wins were
1,936/548 and 2,064/1,300. This comparison is qualified by its 9.15% stall
rate and is not the retention gate.

Held-out trace diagnostics recorded 21,728 attacker decisions for the
candidate (8,339 singleton, 49,865 offered options, 28,355 selected attacker
options) and 21,624 for baseline (8,170 singleton, 50,347 offered, 27,341
selected). The candidate generated fewer mean turns (16.1665 head-to-head;
15.8335 self-control) than the development baseline control (16.537).

## Decision

Retain AR7 as the opt-in `lethal-pressure` policy. It is deterministic,
changes only attacker selection, has focused unit coverage for the positive
and missing-life cases, improved the direct held-out head-to-head with a
confidence interval above 50%, moved nine of ten pair point estimates in the
favorable direction, and introduced no stalls or livelocks. Both view-shaped
and game-shaped adapters select the same policy implementation.

Do not replace production `bot` yet. Promotion changed three pinned legacy-deck
`TestHeads` values, and this work was explicitly forbidden from regenerating
goldens. Keeping the measured policy opt-in preserves the replay goldens while
leaving a direct `-a lethal-pressure -b bot` comparison available. The next
experiment should refine multi-creature lethal pressure: evaluate combined
unblocked power and the minimum blocker set rather than the current
per-attacker lethal test, then decide promotion in work authorized to update
policy-driven goldens.

## Verification

Fresh commands on the retained tree:

```sh
go test ./botpolicy ./seat -count=1
# PASS: botpolicy 0.027s, seat 15.376s

go test ./cmd/botbench -count=1 \
  -skip 'TestFullPairsIteratesSorted|TestConstructedDefaultIsByteIdentical'
# PASS: 23.696s

go test ./cmd/botbench -run 'TestTrace|TestDecisionTrace' -count=1
# PASS: includes injected write/publication failures, concurrent-destination
# preservation, redaction, replay isolation, and two-pair worker determinism

go test ./rules -run TestHeads -count=1
# PASS: 2.479s; no golden regenerated

go vet ./...
# PASS

git diff --check
# PASS

make sim
# PASS: 20/20 games replay OK
```

`go test ./... -count=1` was run before implementation and failed on stale
deck-pool expectations introduced by the already-present `avengers-assemble`
fixture (`cmd/botbench` two tests and `cmd/gorged` one test). A fresh broad
run, excluding only those three named tests, exposed additional pre-existing
failures in `host`, `internal/archtest`, and `internal/searchprobe`; none of
the failing files are touched by this work. The focused packages above and
the unchanged rules chain-head gate pass.

## Remaining risks

- Full board snapshots make raw traces large: 1,000 bot-v-bot development
  games produced 1.8 GiB; the legacy run produced 8.9 GiB because 100 games
  reached 20,000 intents. Consumers need streaming analysis and deliberate
  retention cleanup.
- AR7 evaluates each attacker independently. It does not yet calculate a
  multi-attacker lethal set or the defender's minimum blocking assignment.
- `legacy` is unsuitable as a clean strength gate for the four white-deck
  pairs because of its reproducible intent stalls; direct candidate-versus-
  baseline is the retention signal.
