package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// This file is the ONE bridge between rules' layer walk and the effects tier's
// ORDINARY type comparisons (a filter spec's bases and type predicates, as
// reached by a target offer, cost site, Count$Valid census or CantTarget spec).
//
// A layer-4 effect (CR 613.1d/613.1c: AddTypes$, RemoveCardTypes$,
// RemoveCreatureTypes$, AddAllCreatureTypes$, a face-down CR 708.5 set) changes
// an object's types, and only the layer walk can say which effect applies and
// which of two competing effects wins on timestamp. The layer walk already
// answers its OWN Affected$ match with a types-so-far list (SpecContext.
// ExtraTypes). But every filter OUTSIDE the walk -- the target offer a
// Goblin-killer's ValidTgts$ Goblin drives, a cost site's candidate census, a
// Count$Valid, a CantTarget -- read the printed face plus Changeling's CDA, so
// a creature a static made a Goblin was counted by a lord but not targetable
// by the Goblin-killer.
//
// The effects tier cannot import rules and must not re-derive any of that from
// a battlefield scan. So rules computes the derived type list itself and hands
// it down as DATA on the SpecContext the filter call already carries -- the
// same shape setname.go uses for layer-3 names and SpecContext.ExtraTypes
// uses for the walk's per-object types: a plain value slice, deliberately not
// a callable resolver field (which poisons escape analysis) and never a
// back-pointer from state.Game into a live engine (which would make an
// authoritative Game depend on something outside its event fold, and would
// survive a Game.Clone aimed at a different board).
//
// The table is a FIELD, refreshed after each emitted event, and specCtxSVars
// binds it with a plain field read. That is load-bearing, not a style choice:
// calling a function from inside specCtxSVars pushes it over the inline budget,
// its Resolve closure then escapes, and every SpecContext construction on the
// Derived/legal-actions hot path allocates
// (TestDerivedWithContinuousEffectsDoesNotAllocate,
// TestLegalActionsReusesActionStaticMembership). The refresh itself is gated on
// layer4InPool so a match whose cards carry no type-changing script -- most
// matches -- pays one predictable branch per event and nothing else, and it
// short-circuits on anyLayer4Active so a carrier still in hand costs only the
// active() scan.
//
// The entries are only the objects whose DERIVED list differs from the printed
// face (a handful at most, nil on the overwhelmingly common board), so the
// filter tier's linear scan is cheaper than building a map and an unchanged
// object keeps the compiled-predicate fast path (effects/filter.go's
// hasDerivedTypeEntry).

// poolHasLayer4Static reports whether any card this match can put on the
// battlefield prints a layer-4 type-changing effect. Computed once, at
// genesis, over the decks and the token table: a card outside the pool can
// never register one of these effects from its printed text. The per-card
// probe is cards.ChangesTypes -- card-data introspection, deliberately not a
// statics-parameter read on a rules path (rules/paramcensus_test.go attributes
// those to the primitive whose activeStatics root reaches them, and this probe
// has no such root because it runs before any board exists).
func poolHasLayer4Static(cfg Config) bool {
	for _, deck := range cfg.Decks {
		for _, c := range deck {
			if c.ChangesTypes() {
				return true
			}
		}
	}
	for _, c := range cfg.Tokens {
		if c.ChangesTypes() {
			return true
		}
	}
	return false
}

// refreshDerivedTypes recomputes the derived-type table if the board or the
// continuous effects moved since the last build. The key is active()'s own --
// the log head plus the continuous-effect version -- because the derived type
// set is a pure function of exactly those two inputs.
func (e *Engine) refreshDerivedTypes() {
	if e.typesBuilding {
		// Re-entry: a Derived/typeCharacteristics call below reached something
		// that re-emitted. The half-built table must never be published; the
		// outer call finishes it.
		return
	}
	// The key also carries len(e.G.Objs): a test helper (or any caller) may
	// place a permanent with a direct AddObject -- no event, so the log head
	// would not move -- and the table must not stay a stale cache hit. An
	// emitted move changes neither the object count nor the derived list of
	// any other object, so this adds no work on the ordinary path.
	if e.typesEpoch == len(e.L.Events) && e.typesVersion == e.continuousVersion && e.typesObjs == len(e.G.Objs) {
		return
	}
	e.typesEpoch, e.typesVersion, e.typesObjs = len(e.L.Events), e.continuousVersion, len(e.G.Objs)
	buf := e.layer4Types[:0]
	if !e.anyLayer4Active() {
		e.layer4Types = buf[:0]
		return
	}
	e.typesBuilding = true
	defer func() { e.typesBuilding = false }()
	// e.G.Objs is append-ordered, so this walk is deterministic; only the
	// battlefield is scanned because a layer effect applies nowhere else.
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil {
			continue
		}
		ty := e.typeCharacteristics(o.ID, 0)
		if sameTypeWordSet(ty, o.Face().Types) {
			continue
		}
		// The list may alias a scratch buffer or the face's own slice, so copy
		// it into the table (only for the few changed objects).
		buf = append(buf, effects.ObjectTypes{ID: o.ID, Types: append([]string(nil), ty...)})
	}
	e.layer4Types = buf
}

// anyLayer4Active reports whether any active continuous effect changes a type.
// Without this gate every refresh would pay for a full battlefield type walk on
// a board whose type-changing carrier is still in a library.
func (e *Engine) anyLayer4Active() bool {
	for _, ce := range e.active() {
		if ce.Layer == LType {
			return true
		}
	}
	return false
}

// EffectiveTypes publishes the current layer-4 derived type table to the
// effects tier, which reads it once at the top of every effects.Resolve walk
// (effects' typeTableHost) and binds it on the resolving Ctx. That is what
// makes a resolving effect's own filter calls -- a target offer, a Count$Valid
// census, a CantTarget spec -- agree with the layer walk instead of reading
// the printed face. It is a plain value-slice read, never a live engine
// pointer: effects answer type filters during a resolution without a
// back-pointer on state.Game, and a cloned game cannot read another game's
// board. The table is refreshed by emit and continuousChanged, so it is
// current at Resolve entry.
func (e *Engine) EffectiveTypes() []effects.ObjectTypes { return e.layer4Types }

// sameTypeWordSet reports whether two type lists carry the same words,
// case-insensitively and ignoring order. It decides whether an object's derived
// types differ from its printed face and therefore need a table entry. The
// lists are tiny (a handful of words), so the quadratic scan is cheaper than
// building a set on this per-object refresh path.
func sameTypeWordSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		found := false
		for _, y := range b {
			if strings.EqualFold(x, y) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
