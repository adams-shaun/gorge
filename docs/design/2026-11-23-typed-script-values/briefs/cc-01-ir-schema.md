# feat(cardc): CC-01 the typed card IR schema (engine/cardc/ir)

Kind: feature

Target branch: `cardc`. **Complexity: high. Review: Opus** (this ticket defines the representation every later
ticket builds on). Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 1. This brief fixes the
exact fields and supersedes the plan's sketch table. Runs after CC-00 (`cc-00-branch`).

## Goal

Create `engine/cardc/ir`, the engine-neutral typed card IR that mtgish lowers into and gorge's backend reads. It is
stdlib-only data plus:
- a canonical printer;
- structural equality;
- a deterministic JSON codec;
- name tables for every enum.

The four core leaf families are references, filters, values and conditions. They follow oracle-grammar's
ObjectRef/PlayerRef, ObjectFilter, Expr and Condition shapes (taken as shapes only, no code).

## Behaviour

- `ir.Equal(a, b any) bool` compares two node pointers (`*Card`, `*Ref`, `*Filter`, …) structurally, ignoring every
  `Src` field. Different dynamic types are unequal; two nils of the same type are equal.
- `(*Card).String()` and each node's `String()` print a canonical, single-line S-expression. Two nodes print the same
  text iff `Equal` holds.
- `ir.EncodeCard(*Card) []byte` and `ir.DecodeCard([]byte) (*Card, error)` round-trip every node, `Src` included.
  - Enums are encoded by **name** (from the name tables), never by ordinal, so appending a const never changes old
    encodings.
  - The output is deterministic: fixed field order, nil and empty omitted.
- No IR type contains a map, a func, a channel or an interface field.
- The package imports only stdlib (`encoding/json`, `strings`, `strconv`, `fmt`, `sort`). CC-00's archtest rows forbid
  `engine/cards` and `engine/state`.

## Design

Decided. Every type and field below is written exactly. Each enum is `uintN` with a zero value `XxxNone` meaning
"absent", and an unexported `xxxCount` sentinel last. Later tickets append consts before the sentinel, in their
family's file, and add the name in the same file. Every node struct ends with `Src string`: the mtgish variant path
that produced it, for example `Rule.TriggerA/Trigger.WhenAPermanentEntersTheBattlefield`. Never Oracle text.

**Files and types:**

`card.go`
- `type Card struct { Name string; Layout Layout; Faces []*Face; Src string }`
- `type Layout uint8`: `LayoutNone, LayoutNormal, LayoutTransform, LayoutModalDFC, LayoutSplit, LayoutAdventure,
  LayoutFlip, LayoutMeld, LayoutPrepare, LayoutRoom`.
- `type Face struct`, with fields:
  - `Name string`
  - `ManaCost []ManaSym`
  - `HasManaCost bool`: false means no mana cost, as distinct from {0}.
  - `ColorIndicator ColorSet`
  - `Supertypes []string`, `Types []string`, `Subtypes []string`
  - `Power *PT`, `Toughness *PT`
  - `Loyalty *int32`, `Defense *int32`
  - `Abilities []*Ability`
  - `Refusals []Refusal`
  - `Src string`
- `type PT struct { N int32; Star bool; Src string }`. `Star` means a characteristic-defining `*`, with N added
  (`*+1` is Star, N=1).
- `type Refusal struct { Key string; Path string }`. Key is `Enum.Variant`; Path is the Src path where it was met.

`mana.go`
- `type ColorSet uint8`, bits `ColorW=1, ColorU=2, ColorB=4, ColorR=8, ColorG=16`.
- `type ManaKind uint8`: `ManaNone, ManaGeneric, ManaColored, ManaHybrid, ManaPhyrexian, ManaHybridPhyrexian,
  ManaTwoHybrid, ManaColorless, ManaX, ManaSnow`.
- `type ManaSym struct { Kind ManaKind; Colors ColorSet; N int32 }`. N is the generic amount, or 2 for
  ManaTwoHybrid.

`zone.go`
- `type Zone uint8`: `ZoneNone, ZoneBattlefield, ZoneHand, ZoneLibrary, ZoneGraveyard, ZoneExile, ZoneStack,
  ZoneCommand`.
- `type ZoneSet uint16`, bit `1<<Zone`.
- `type CmpOp uint8`: `CmpNone, CmpEQ, CmpNE, CmpLT, CmpLE, CmpGT, CmpGE`.
- `type Compare struct { Op CmpOp; V *Value; Src string }`.

`ref.go` (references; CC-11 appends)
- `type RefKind uint16`: `RefNone, RefSelf, RefYou, RefTarget, RefEventObject, RefEventPlayer, RefControllerOf,
  RefOwnerOf, RefAttachedTo, RefActivePlayer, RefDefendingPlayer, RefIterPlayer, RefIterObject, RefReceipt, RefAll,
  RefUnion, RefPlayerOrControllerOf, RefChoose, RefTopOfLibrary`.
- `type ReceiptKind uint8`: `RcNone, RcExiled, RcChosen, RcRevealed, RcCreated, RcSacrificed, RcDiscarded, RcMilled,
  RcDestroyed, RcReturned`.
- `type Ref struct`, with fields:
  - `Kind RefKind`
  - `Slot uint8`: target declaration index.
  - `Of *Ref`
  - `Filter *Filter`
  - `N *Value`: the count, for RefChoose and RefTopOfLibrary.
  - `Receipt ReceiptKind`
  - `Union []*Ref`
  - `Src string`
- Meanings:
  - RefTarget uses Slot.
  - RefControllerOf, RefOwnerOf, RefAttachedTo and RefPlayerOrControllerOf use Of.
  - RefAll is every object or player matching Filter.
  - RefChoose is N objects matching Filter, chosen by Of.
  - RefTopOfLibrary is the top N cards of Of's library.
  - For both, a nil Of means the acting player: the enclosing Step's `Who`, else you. mtgish leaves this player
    implicit (`Exilable.ACardFromHand` inside a `PlayerAction` is that player's hand).
  - RefReceipt is the objects an earlier step of the same program produced, of kind Receipt.
  - RefIterPlayer and RefIterObject are the current element of the enclosing for-each.
- `type Target struct`, with fields:
  - `Min int32`, `Max int32`
  - `MaxV *Value`: set when the count is a value (X targets).
  - `Filter *Filter`
  - `Distinct bool`
  - `Src string`

  The Ability's `Targets` index is the RefTarget Slot.

`filter.go` (filters; CC-12 appends)
- `type Subject uint8`: `SubjNone, SubjObject, SubjPermanent, SubjPlayer, SubjSpell, SubjCard`.
- `type FilterOp uint8`: `FNone, FAll, FAny, FNot, FPred`.
- `type Pred uint16`: `PNone, PAny, PCardType, PSubtype, PSupertype, PColor, PColorless, PMulticolored,
  PPermanentCard, PManaValue, PPower, PToughness, PControlledBy, POwnedBy, PIs, PToken, PTapped, PAttacking,
  PBlocking, PHasKeyword, PCounters, POpponentOf, PPlayerIs, PAttackedThisTurn, PControls, PLifeTotal,
  PLostLifeThisTurn, PCardsInGraveyard, PCardsInHand, PControlsMoreThan, PName`.
- `type Filter struct`, with fields:
  - `Subject Subject`
  - `Zones ZoneSet`
  - `Op FilterOp`
  - `Args []*Filter`
  - `Pred Pred`
  - `Word string`
  - `Colors ColorSet`
  - `Ref *Ref`
  - `Cmp CmpOp`
  - `Value *Value`
  - `Src string`
- Meanings:
  - FAll, FAny and FNot combine Args.
  - FPred tests Pred with its operands: Word for type, subtype, supertype, keyword, counter kind and name; Colors;
    Ref; and Cmp/Value for numeric predicates.
  - PControls takes `Args[0]` as the object filter.
  - PControlsMoreThan takes Ref (the other player) and `Args[0]`.

`value.go` (values; CC-13 appends)
- `type ValueKind uint16`: `VNone, VConst, VX, VAdd, VSub, VMul, VHalfDown, VHalfUp, VCount, VPower, VToughness,
  VManaValue, VLife, VStartingLife, VLifeGainedThisTurn, VDevotion, VCountersAmong, VTotalPower,
  VDistinctManaValues, VSpellsCastBeforeThisTurn, VChosenNumber, VEventAmount, VReceiptAmount, VExcessDamage,
  VCountReceipt`.
- `type Value struct`, with fields:
  - `Kind ValueKind`
  - `N int32`
  - `Args []*Value`
  - `Filter *Filter`
  - `Ref *Ref`
  - `Word string`
  - `Colors ColorSet`
  - `Receipt ReceiptKind`
  - `Src string`

`cond.go` (conditions; CC-14 appends)
- `type CondKind uint16`: `CNone, CAll, CAny, CNot, CCompare, CRefMatches, CExistsPlayer, CCountCompare, CCostPaid,
  CIsTurnOf, CNthTurn, CReceiptNonEmpty, CReceiptCount, CDiedThisTurn, CLeftBattlefieldThisTurn`.
- `type Cond struct`, with fields:
  - `Kind CondKind`
  - `Args []*Cond`
  - `L *Value`, `R *Value`
  - `Cmp CmpOp`
  - `Filter *Filter`
  - `Ref *Ref`
  - `Word string`
  - `Receipt ReceiptKind`
  - `Src string`

`ability.go`
- `type AbilityKind uint8`: `AbNone, AbSpell, AbActivated, AbTriggered, AbStatic, AbReplacement, AbKeyword`.
- `type Ability struct`, with fields:
  - `Kind AbilityKind`
  - `Cost *Cost`
  - `Trigger *Trigger`: the trigger, or the replaced event for AbReplacement.
  - `Cond *Cond`
  - `Targets []*Target`
  - `Body *Program`
  - `Keyword *Keyword`
  - `Zones ZoneSet`
  - `Src string`
- `type Keyword struct { Name string; N *Value; Cost *Cost; Filter *Filter; Src string }`. Name is the keyword's
  rules name (`Flying`, `Ward`), never reminder text.
- `type EventKind uint16`: `EvNone` only (CC-16 appends).
- `type Trigger struct { Event EventKind; Subject *Filter; Ref *Ref; Zones ZoneSet; OncePerTurn bool; Src string }`.

`program.go`
- `type Verb uint16`: `VerbNone` only (CC-15 appends).
- `type Duration uint8`: `DurNone, DurEndOfTurn, DurPermanent, DurUntilYourNextTurn, DurWhileSourceOnBattlefield,
  DurEndOfCombat, DurWhileActive`.
- `type Program struct { Steps []*Step; Src string }`.
- `type Step struct`, with fields:
  - `Verb Verb`
  - `Optional bool`
  - `Who *Ref`
  - `Obj *Ref`
  - `N *Value`
  - `Cond *Cond`
  - `Then *Program`, `Else *Program`
  - `Dur Duration`
  - `From Zone`, `To Zone`
  - `Word string`
  - `Colors ColorSet`
  - `Src string`

`cost.go`
- `type CostKind uint8`: `CostNone, CostTapSelf, CostUntapSelf, CostSacrifice, CostDiscard, CostPayLife, CostExile,
  CostRemoveCounters, CostReturnToHand, CostTapUntapped, CostReveal, CostMill`.
- `type Cost struct { Mana []ManaSym; HasMana bool; Parts []*CostPart; Src string }`.
- `type CostPart struct { Kind CostKind; Obj *Ref; N *Value; Word string; Src string }`.

**Name tables.** One per enum in the enum's own file:
- `var refKindNames = [...]string{RefNone: "none", RefSelf: "self", ...}`, lower-kebab names (`event-object`,
  `controller-of`);
- an exported `func (k RefKind) String() string`;
- an exported `func ParseRefKind(s string) (RefKind, bool)` for the codec.

**Printer.** Examples:
- `(ref self)`, `(ref target 0)`, `(ref controller-of (ref target 1))`;
- `(filter all (filter pred card-type "Creature") (filter not (filter pred token)))`;
- `(value add (value const 2) (value x))`.
- Fields print in declaration order, and only when non-zero.
- Strings are quoted with `strconv.Quote`.
- `Src` never prints.

## Implementation steps

1. Write the files above. Every enum gets its `xxxCount` sentinel, its name table and String/Parse.
2. Write `equal.go` (`Equal` via one hand-written function per type; no `reflect.DeepEqual`) and `print.go`.
3. Write `codec.go`. Each node encodes as a JSON object with lower-camel keys and enum names as strings.
   `DecodeCard` rejects an unknown enum name with an error naming it.

## Tests

In `engine/cardc/ir`:
- `TestEnumNamesComplete`: every enum's name table has a non-empty, unique name for every value from None up to the
  sentinel.
- `TestIRHoldsNoMaps`: reflect over every exported struct type and fail on a map, func, chan or interface field.
- `TestCodecRoundTrip`: hand-built cards covering every struct type and every enum value at least once.
  DecodeCard(EncodeCard(c)) is Equal to c, and its Src fields are equal too.
- `TestCodecDeterministic`: encoding the same card twice gives identical bytes.
- `TestPrintMatchesEqual`: for pairs of hand-built nodes that differ in exactly one field, `String()` differs and
  `Equal` is false. With only Src different, both agree.
- `TestCodecUsesNames`: the encoded JSON of a Ref with `Kind: RefControllerOf` contains `"controller-of"` and no
  ordinal.

No test reads card data.

## Done means

- [ ] `make test-one PKG=./engine/cardc/ir RUN='^(TestEnumNamesComplete|TestIRHoldsNoMaps|TestCodecRoundTrip|TestCodecDeterministic|TestPrintMatchesEqual|TestCodecUsesNames)$'` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc)$'` passes.
- [ ] `make test-targeted BASE=origin/cardc` passes.
- [ ] `make test-smart BASE=origin/cardc` passes.

## Out of scope

- Any mtgish reading (CC-02).
- Any lowering (CC-10..16).
- Any gorge backend (CC-20).
- Verb, EventKind and further Pred/RefKind/ValueKind/CondKind consts beyond those listed: later tickets append them.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

The hub ticket. Every later cardc ticket depends on it. After it lands:
- each family ticket appends only to its own file (`ref.go` CC-11, `filter.go` CC-12, `value.go` CC-13, `cond.go`
  CC-14, `program.go` CC-15, `ability.go` triggers CC-16);
- `equal.go`, `print.go` and `codec.go` are generic over fields, so appending a const needs no edit there.
