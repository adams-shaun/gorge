# State

A dirac game is one Go value, `dirac.State`, plus one small buffer it owns. This page shows what is inside
it and why, for readers who want to know what a cheap clone costs and what it rules out.

Sizes were read on 2026-10-31 with Go 1.26.3 on linux/amd64 by a scratch test printing `unsafe.Sizeof` of each
part. dirac's own `TestStateSize` logs the total.

## Three parts

```go
type State struct {
    stateCore            // the rules state: fixed-size, no pointers
    perms [MaxPerm]perm  // the battlefield, hot, copied only as far as it is used
    cold  coldArrays     // arrays only some card tables need
    pool  *ForkPool      // the States that rules probes run on
}
```

| Part | Bytes | What it holds |
|---|--:|---|
| `stateCore` | 8,344 | the rules state, below |
| `perms` | 6,272 | 112 battlefield slots at 56 bytes each |
| `cold` headers and `pool` | 280 | slice headers and a pointer; the arrays themselves are in one buffer |
| **`State`, total** | **14,896** | |

The core and `perms` hold no pointers, maps, slices, strings or interfaces. Every collection is a fixed array
and a count, so three things follow:

1. **A clone is one or two memory copies.** `dst.CopyFrom(src)` assigns the core, copies `perms` up to
   `permHi` (one past the highest slot this game has used), clears any slots above that in the destination, and
   copies the cold buffer. It allocates only if the destination's buffer is too small. A copy of a mid-game FDN
   position takes about 41 ns ([Performance](../../performance/index.md#transitions-and-copies)).
2. **The garbage collector scans almost nothing.** The cold buffer is a `[]uint64`, which it doesn't look
   inside, and the only pointers in a `State` are in the cold headers and the fork pool.
3. **Positions compare directly.** `SameGame` compares the core, `perms` and the buffer. `Projection`, below,
   compares with `==`.

An assignment of a `State` would share the cold buffer, so a `noCopy` field makes `go vet` reject one.

What's variable-length about the cards (definitions, filters, op programs) is in the read-only card table
(`kop.Table`, see [Cards](cards.md)). A `State` names its table by a 16-bit index.

## The core

| Region | Bytes | Contents |
|---|--:|---|
| header | about 16 | turn, step, active player, priority holder, passes, control state, config bits, result |
| RNG | 168 | the PCG state, draw count, chance record ([RNG](step-apply.md#rng)) |
| players | 556 | two records: life, zones, mana pools |
| objects | 1,536 | 192 card instances at 8 bytes: card, zone, slot, version, flags |
| layer stamps | 224 | a layer timestamp per battlefield slot |
| slot masks | about 480 | occupied, by controller, per event kind, static sources |
| stack | 864 | 24 entries at 36 bytes |
| pending triggers | 384 | 24 records, then the trigger-order round's picks |
| delayed triggers | 96 | 16 records |
| resolution in progress | 80 | the suspended program and its position |
| change marks | 24 + 192 | one bit per object, one stamp per object ([below](#change-marks)) |
| generated-table extension | 3,448 | continuous effects, last known information, per-turn history and other pools |

## Hot and cold

Moving an array out of the core changes which tables pay for it:

- **`perms` stays inline.** Reaching it costs no extra load, and most of a battlefield is empty, so `CopyFrom`
  copies only the slots in use.
- **Cold arrays are per table.** These are arrays only some tables' cards reach: exiled-with links,
  attacker groups, this turn's zone entries and spell pushes, held discards, mechanic masks. A `coldLayout`,
  computed once per table, gives each one its length, and zero when no card in the table can write to it. Each
  array is a view into one `[]uint64`, so a `State` makes one allocation for them all.

The mechanic masks show the idea at its smallest. A mask that almost no pool uses (goaded, monstrous, suspected
and so on) gets an index only if a card in the table uses it. Any other mechanic's index is past the end of the
mask array, so the bounds test is also the off switch and the unused mechanic touches no memory.

A full cold array stalls by name, and an empty one is simply full. A table that can't write to one never gets
there, because its size follows the same flags that switch its writers on.

A rules probe (does this option leave a legal play?) runs on a fork from the `ForkPool`, one `State` per
nesting depth. `NewForkPool` makes eight, past the deepest nesting measured in random play, so a warmed search
doesn't allocate for them. All `State`s sharing a pool must run on one goroutine.

## Objects and slots

An `Obj` is a `uint8` card instance index. Seat 0's deck comes first in deck order, then seat 1's, then tokens
in creation order, the same relative order as gorge's object IDs. An object keeps its index across zones.

A permanent also has a **slot**, its index into `perms`. Battlefield loops walk bitmasks (`slotMask`, two 64-bit
words) rather than scanning 112 records: `occ` for occupied slots, `ctrlM[p]` by controller, one mask per
kind of event a trigger can watch, `statM` for slots with a static ability. Finding the attackers or the
sources that can react to an event is a few word operations.

Each object has a **version**, bumped on every zone change. A stack target stores the object and its version,
so a target that left and came back no longer matches and is illegal (CR 400.7). An ability's source works
the same way. An ability that must see a creature as it last existed snapshots it into the last-known-information
pool when the trigger is queued or the cost is paid.

## Change marks

A search copies a position many times, and most objects in a copy match the base. Two readers use that: the
network input encoder and the search's evaluation cache.

- `chg` is a 192-bit set. A write to an object or its slot marks its bit, bumps a touch counter and stamps the
  object.
- `SetEpoch` clears the marks and names the current position as a base. A copy carries the epoch and the marks,
  so an unmarked object in any copy is field for field the base's.
- `ObjHash` is a pure function of an object's fields, so two positions holding the same object hash the same.

Marks may over-report and cost a recomputation, but never miss.

## The size bar

Every byte of the core is copied on every clone, so its size is a decision rather than a side effect.

- **`State` must stay under 16 KiB.** `TestStateSize` fails above that, and moving the bar is a deliberate
  decision. It is 14,896 bytes today.
- **The table holds what's printed and `State` holds what changes.** No field in `State` belongs to one card.
- **Per-slot bytes are expensive.** One byte on `perm` costs 112 bytes of `State`, one on `object` 192. Rare
  state goes in a pool sized for the worst case seen in play, or in a cold array.
- **A mechanic that's off costs one branch.** Each block is gated by a flag computed once per table, and the code
  behind the gate stays out of line.

## The Projection

`Project` writes a `dirac.Projection` of 33,744 bytes: a comparable view of a whole position, for tools and tests.

| Part | Contents |
|---|---|
| per object, in `Obj` order | card, owner, controller, zone, position in the zone, token bit, tapped, attacking, blocking and summoning-sick flags, damage, counters, derived power and toughness, attached-to, blocked attacker, attacked defender |
| derived characteristics, per object | types, subtypes, colours and keywords after the layer system |
| per seat | life, lands played, mana pool, zone sizes |
| the stack, bottom to top | kind, source, card, ability index, controller, targets, modes, X |
| the rest | turn, step, active player, priority holder, RNG draw count, game over, winner |

Every field is a fixed array, and equal positions project equal. A position's stack matches gorge's at the
points that matter for lockstep: a spell goes on the stack after its X and cost picks and before its modes and
targets, an activated ability after its targets and costs, and a triggered ability before its targets. dirac
also splits gorge's combat-damage step into a first-strike step and a regular step, which the oracle maps back.

`ProjectDelta` and `ProjectSince` update an existing projection from the change marks. The oracle compares its
own canonical projection, built from both engines ([Verification](oracle.md#what-is-compared)).
