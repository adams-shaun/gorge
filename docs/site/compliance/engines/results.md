# Reading results

`xengine diff` gives every item one class per follower and writes:

| File | What it holds |
|---|---|
| `verdicts.jsonl` | One line per item: `id`, `engine`, `class`, and where it differed (`checkpoint`, `field`, `gorge`, `other`, `step`, `msg`) |
| `finds/<class>/<id>/` | A repro directory for every item that isn't `agree` ([what's in one](howto.md#debug-one-item)) |
| `compared.tsv` | Per card and ability, how many compared items exercised it. The sweep adds it to the coverage ledger |

A finding names the first field that differs:

```json
{"id":"xe:FDN:0:0:tape","engine":"toy","class":"diverge","checkpoint":"step 2 (cast)","field":"p1.life","gorge":"20","other":"19","step":2}
```

## The classes

The classifier tries the classes in roughly this order and stops at the first that fits. Only one class
is a lead on an engine bug.

### A lead

| Class | Means | What to do |
|---|---|---|
| `diverge` | Your engine and gorge reached different observable states, and nothing below explains it | Read the card and the rules. One of the two engines, or your driver, is wrong |

A `diverge` whose message says `the driver then stopped: …` means your row stopped early with a harness
message, but gorge already differed within the part you did replay. The difference is real.

### Agreement

| Class | Means |
|---|---|
| `agree` | Every compared field matched at every checkpoint |

### Your driver couldn't say

None of these is a verdict on either engine. They measure how much of the corpus your driver covers.

| Class | Means | What to do |
|---|---|---|
| `harness` | Your row set `harness`, and gorge didn't differ before the stop | Fix the driver, or leave it until it matters |
| `decision_shape` | Your engine asked something gorge recorded no answer for (`strict_miss`), or never asked something it did (`leftover`) | Check the prompt mapping. If gorge doesn't log a choice your engine needs, tell us |
| `xmage_lacks` | A harness message names a card your engine lacks, using your `LacksCardMsg`. The name is historical; it applies to any follower | Refuse the card in the preflight instead |
| `turn_cap` | A tape stopped at your driver's turn cap (`TurnCapMsg`) after agreeing up to there | Raise the cap |
| `not_sent` | The item uses a card your preflight refused | Nothing; it's working as intended |

### Known differences that aren't engine bugs

| Class | Means |
|---|---|
| `payment_choice` | The only difference is which pool mana paid a cost neither side fixed: a legal choice (CR 601.2h), not a rules difference. Applies only to a driver without `ConsumesPayment` |
| `rng` | A hand, library or graveyard difference after a random event gorge recorded. Each engine drew its own randomness |
| `projection_gap` | A field neither side can describe in the shared vocabulary: an unmapped keyword (`other:…`) or two token names for the same token |
| `forge_skips_damage_step` | Forge skips a combat damage step when no creature is left in combat, which the CR doesn't allow. A driver quirk (`SkipsDamageStep`) |
| `crew_type_dependency` | A type-adding static misses a Vehicle that became a creature. XMage and Forge both apply a CR 613.8a dependency by timestamp instead; gorge applies the dependency. Documented in [upstream-bugs.md](https://github.com/adams-shaun/gorge-compliance/blob/main/docs/upstream-bugs.md) |

### Never sent

| Class | Means |
|---|---|
| `lower_gap` | gorge's own runner didn't reproduce the item when it was cut from the game. That's a harness bug on our side, and the item never reaches any follower |

## Which numbers to watch

While you build the runner, the share of sent items that are `agree` is the headline number. But the
classes worth reading one by one are:

1. **`diverge`.** Every one is either an engine bug or a driver bug. Expect most early ones to be
   driver bugs, typically about priority, setup or snapshot vocabulary.
2. **`decision_shape`, by message.** Group the `strict_miss` messages. A few prompt kinds usually make
   up most of them, and each one you map clears a block of items.
3. **`harness`, by message.** Group them the same way to find what to build next.

A quick histogram from a chunk:

```sh
jq -r .class diff-toy/verdicts.jsonl | sort | uniq -c | sort -rn
jq -r 'select(.class=="decision_shape") | .msg' diff-toy/verdicts.jsonl | cut -c1-60 | sort | uniq -c | sort -rn
```

## When engines disagree with each other

With Forge and XMage both following, `xengine adjudicate` combines their diffs into a pattern per item:
`FX|G` means Forge and XMage agree and gorge stands alone, `GF|X` means XMage stands alone, and so on
(see [the disagreement classes](../harness.md#disagreement-classes)). It doesn't include a third engine yet.
Counting by family across any number of engines is planned. Meanwhile, compare your `verdicts.jsonl`
with Forge's and XMage's for the same chunk. They share item ids.

Even a unanimous pattern isn't a ruling. A person reads the printed card and the rules before anything is
called a bug, in gorge or in your engine.
