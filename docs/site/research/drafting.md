# Drafter and builder knobs

*How `cmd/draftres` measures a drafter or deck-builder variant against others*

Does a change to how a bot drafts, or to how it builds its 40 cards, win more games? `cmd/draftres` answers this in self-play: the variants draft pods together, every pool is built, and the decks play each other in dirac.

## Method

- **Pods.** Every pod is `-seats` seats (8 by default, the sweep's pod; `-seats 4` for a draft event's pod size) opening 3 packs of the set (default collation). Pod `i` opens and drafts with seed `seed+i`.
- **Seat rotation.** With `K` variants, seat `s` of pod `i` drafts with variant `(s+i) mod K`, so every variant sits in every seat once every `K` pods. When `K` divides 8, every pod also holds `8/K` drafters of each variant.
- **Building.** Each pool is built by each builder in `-builders`. With more than one builder, a deck's label is `variant/builder`.
- **Pairs.** Each pair of decks from the same pod with different labels plays `-games` games (default 2), with seats swapped between games. With `-builder-ab`, the only pairs are decks built from the same seat's pool, so the builders are compared on identical pools.
- **Play.** dirac's `dplay` plays the pairs hbot against hbot on the `lockstep-autopay` surface, the one dirac/oracle checks against gorge. A pair whose deck holds a card the set refuses is skipped (`-skip-refused`). Draws, stalls and errors are left out of the win rate.
- **Summary.** A label's win rate is its share of the decided games it played. The 95% interval comes from a pod bootstrap: pods are resampled with replacement 1000 times, and a pod's games stay together because its decks share packs. The resampling is seeded from `-seed` alone, so the same run gives the same interval.

## Running

From the module root, with the corpus in `.cards`:

```sh
go build -o /tmp/dplay ./dirac/cmd/dplay
go run ./cmd/draftres -set FDN -pods 200 -seed 1 \
  -variants committed,'knob:cfm=0.5;sig=0.3;deny=1' \
  -builders balanced -chunk 25 -out runs/knob -dplay /tmp/dplay -workers 6
```

`-dplay` is the command that plays a chunk (default `go run -trimpath ./dirac/cmd/dplay`; `none` only drafts). A built binary saves a compile per chunk.

### Variants

| Variant | Drafter |
|---|---|
| `random` | a uniformly random pick |
| `rating` | ratings alone (no colour fit) |
| `rc[:w]` | rating plus colour fit, colour-fit cap `w` |
| `committed` | the committed-pair drafter |
| `signals[:w]` | committed plus a table-signal term of weight `w` |
| `knob:k=v;...` | committed plus knobs: `cfm` colour-fit cap, `need` need weight, `sig` signal weight, `deny` hate-draft weight, `bias=G3/B-2` per-colour bias, `pbias=BG+1` per-pair bias (a card of exactly that pair); the same knobs are `pickbot.Tuning` keys a `knobs:<path>` JSON file sets |
| `knobs:<path>` | the same drafter with the knobs a `pickbot` knob file sets (the keys are `pickbot.Tuning`'s JSON names); an unknown key is refused, and a file and the inline `knob:` string that mean the same knobs draft identically |

The human-deck gauntlet, `cmd/draftsim -gauntlet fdn-limited`, takes the same knob files as `-drafter knobs:<path>` and `-builder knobs:<path>`, and its `-ratings-file` applies to a knob-file drafter.

Builders take the draftlab names (`balanced`, `best23`, ...). `balanced:main=;cmin=;cmax=;top=` sets the main-deck size, the creature floor and cap, and the cap on 6+ drops. `knobs:<path>` builds every pool with Best23 under the knob file's knobs (`pickbot.NewTuned`; the Best23 shape knobs — `min_creatures`, `max_high_value`, `high_value_mv`, `short_pair_penalty`, `curve_min`, `curve_max`, `creature_max`, `main_size`/`lands` — so they can be swept like a `balanced:` shape); `knobs:<path>@<hash>` is accepted when the hash matches the file. Any variant or builder takes `+r=file#field` to read its ratings from a ratings file's `field` (default `gih`).

### Output and resume

`-out DIR` holds one `chunk-NNNN` directory per `-chunk` pods: `decks/`, `pool.tsv`, `pairs.tsv`, `decks.jsonl` (each deck's pod, seat, label, colours and cards), and dplay's `games.jsonl` and `summary.json`. A chunk is done once its `done` file is written. Rerunning with the same flags and `DIR` skips the done chunks, redoes a partial one, and prints the summary over every done chunk. A chunk's pair seeds depend only on its first pod, but keep `-chunk` the same across reruns: the chunk directories are numbered by chunk, not by pod.
