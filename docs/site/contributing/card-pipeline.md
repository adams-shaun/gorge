# The card pipeline

How a Forge card script becomes behaviour, and how coverage is measured: the
how-to view for someone changing a card, a parser or a primitive. IR types and
runtime dispatch are on [the architecture page](../architecture/gorge/cards.md).

```mermaid
flowchart LR
  fetch["forgec fetch<br/>.cards/cardsfolder/"] --> parse["cards.ParseBytes<br/>script → Card"]
  parse --> link["Card.Link<br/>SVars resolved, keywords expanded"]
  link --> intr["Face.ApplyIntrinsics"]
  intr --> cache[".cards/ir-*.gob.gz"]
  cache --> reg["cards.Registry"]
  reg --> prims["Face.Primitives<br/>api: trig: stat: repl: kw: count:"]
  prims --> cov["Registry.Coverage<br/>vs effects.Supported()"]
  reg --> game["rules.Engine<br/>effects.Resolve dispatches by name"]
```

## 1. Fetch

`make fetch-cards` builds `bin/forgec` and runs `forgec fetch` (`engine/cards/fetch.go`): a sparse
checkout of `cardsfolder` and the token scripts from
[adams-shaun/forge](https://github.com/adams-shaun/forge) at `FORGE_REF` into the gitignored `.cards/`,
plus `.cards/cards.lock` (ref, commit, license). A changed lock clears Go's test cache. The scripts
are GPL-3.0: nothing under `.cards/` is committed (`engine/cards/boundary_test.go`).

## 2. Parse and link

`engine/cards` is the compiler. It depends on nothing else in the module.

- `cards.ParseBytes(path, src)` (`engine/cards/parse.go`) turns one script
  into a `*cards.Card` with one `*cards.Face` per printed face, each holding
  its `Abilities`, `Triggers`, `Statics`, `Repls`, `Keywords`, `SVars` and
  raw `Params` maps (`engine/cards/ir.go`). Problems come back as
  non-fatal `cards.Diag`s.
- `Card.Link()` (`engine/cards/link.go`) resolves the SVar references
  (`Execute$`, `SubAbility$`, …) into sub-ability trees, and first runs
  `expandKeywords` (`engine/cards/keywords.go`). That rewrites a keyword the
  engine implements through ordinary machinery into the lines Forge itself
  would expand it to. For example, `kw_prowess.go` turns Prowess into a
  `SpellCast` trigger with a `Pump` effect. Each expander is one
  `engine/cards/kw_*.go` file registered with `registerKeyword`.
- `Face.ApplyIntrinsics()` (`engine/cards/intrinsic.go`) adds what the rules
  grant rather than the script prints, such as a basic land's mana ability.

`cards.CompileDir` does all three for every script, in sorted path order, so
the output is byte-identical run to run.

## 3. The IR cache

`make compile-cards` (`forgec compile`) saves the registry to `.cards/ir-<fingerprint>.gob.gz`
(`cards.CachePath`, `engine/cards/open.go`). The fingerprint, `cards.CompilerFingerprint`
(`engine/cards/fingerprint.go`), hashes the `engine/cards` sources that affect the IR, so a parser
change reads and writes its own cache. Beside it sits a binary image,
`.cards/ir-<fingerprint>.seg`, that keeps the corpus out of the garbage collector's way; compiling
prunes older caches.

Programs open the corpus with `cards.OpenCorpus(dir)` or `cards.SharedCorpus(dir)` (the cache, or a
recompile when missing or older than `cards.lock`). Tests use `testutil.CorpusRegistry(t)`: one
shared registry per test binary, **skipping** when `.cards/` is absent.

## 4. The registry

`cards.Registry` (`engine/cards/registry.go`) holds every card and token.
`Registry.Lookup(name)` finds a card by name, and `Registry.Tokens` the token
scripts. Decks (`engine/deck`) resolve card names against it, and
`rules.New(rules.Config{Decks: ...})` deals the resulting `*cards.Card`s
into a game.

## 5. Primitives and their implementations

`Face.Primitives()` (`engine/cards/primitive.go`) lists every engine symbol
a face needs, prefixed by kind, after keyword expansion and following
sub-ability chains and SVar bodies: `api:DealDamage`, `trig:ChangesZone`,
`stat:Continuous`, `repl:Moved`, `kw:Flying`. `Card.ValueHeads()`
(`engine/cards/value_heads.go`) adds the `count:` heads that its value
SVars read.

The engine side registers what it implements:

- `effects.Register("DealDamage", fn)` in an `init()` in `engine/effects`
  installs an `api:` effect, an `effects.Effect` function
  `func(h Host, c *Ctx, sa *cards.SA)`. At resolution, `effects.Resolve`
  walks an ability and its `SubAbility$` chain and calls each node's
  registered function. The function reads the game and proposes events
  through `effects.Host`, never touching `rules` directly.
- `effects.RegisterNonAPI("kw:Flying", ...)` declares keyword, trigger,
  static and replacement primitives. Those are implemented in `engine/rules`
  (combat, layers, the trigger matcher, replacement handling); the
  registration only records that the build supports them.
- `count:` heads are evaluated in `engine/effects/count*.go` and listed in
  `modelledValueHeads` (`engine/effects/value_heads.go`).

`effects.Supported()` returns the union, in the same prefixed form.

## 6. Coverage

`Registry.Unsupported(card, effects.Supported())` (`engine/cards/validate.go`)
is a card's missing primitives. An empty result means the card is
*playable*. `Registry.Coverage` aggregates that over the corpus.

```sh
make report     # prints cards, playable and token counts, then the top missing primitives
make coverage   # the same as markdown tables in .coverage/ (gitignored), by type, colour, mana value
```

"Playable" means every primitive is registered, not that every parameter is honoured. Three checks
narrow the gap: the param census (`TestEveryRepoDeckParamsAreRead`), oracle scenarios (behaviour
against printed text) and the compliance oracles (gorge against XMage and Forge). The last two live
in [gorge-compliance](https://github.com/adams-shaun/gorge-compliance); per-set figures are on
[Support by set](../support/sets.md).
