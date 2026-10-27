package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// host_read.go is the engine's implementation of effects.HostRead, the read
// role of effects.Host (rules-engine refactor spec W1d): the live game and
// the characteristics queries an effect reads it through.

// boardLayers is the always-published pair of the board's derived-
// characteristic tables: the layer-3 rename table (setname.go) and the
// layer-4 derived type table (layer4types.go), both refreshed after every
// emitted event. The other LayerTables (static goads, layer-5 colours,
// layer-6 keywords) are built on demand; LayerTables adds them.
func (e *Engine) boardLayers() effects.LayerTables {
	return effects.LayerTables{EffectiveNames: e.renames, DerivedTypes: e.layer4Types}
}

// LayerTables is effects' optional layerTablesHost: the board's derived-
// characteristic tables a resolving Ctx binds (effects.Resolve reads it at
// walk entry and at every body boundary). The rename and type tables always;
// the static-goad set, the layer-5 colour and layer-6 keyword tables only when
// want asks, because rules builds those on demand.
func (e *Engine) LayerTables(want effects.LayerTableSet) effects.LayerTables {
	t := e.boardLayers()
	if want&effects.LayerGoads != 0 {
		// The static-goad set (staticgoad1): a resolving IsGoaded read agrees
		// with the combat requirement's combat.staticGoaders derivation instead of
		// seeing the event-backed goad list alone.
		t.StaticGoads = e.staticallyGoaded()
	}
	if want&effects.LayerColors != 0 {
		// The layer-5 colour table (layer5colors.go), for a body that names
		// a colour word.
		t.DerivedColors = e.derivedColorTable()
	}
	if want&effects.LayerKeywords != 0 {
		t.DerivedKeywords = e.EffectiveKeywords()
	}
	if want&effects.LayerCDAPT != 0 {
		t.CDAPTs = e.cdaPTTable()
	}
	if want&effects.LayerBattlefieldPT != 0 {
		t.BattlefieldPTs = e.battlefieldPTTable()
	}
	return t
}

// cdaPTTable is LayerTables.CDAPTs: every object, in any zone, whose face
// prints a characteristic-defining P/T, with its Derived current and base
// P/T. Derived applies the CDA wherever the card is (CR 604.3), so a
// resolving filter reads a library Consuming Aberration as its CDA value
// instead of its printed star as 0 (CR 208.2a). Built in g.Objs order (no
// map range); nil when no such object exists.
func (e *Engine) cdaPTTable() []effects.ObjectPT {
	var out []effects.ObjectPT
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if f := o.Face(); f == nil || !f.CharacteristicDefining() {
			continue
		}
		p, t, bp, bt, ok := e.FilterDerivedPT(o.ID)
		if !ok {
			continue
		}
		out = append(out, effects.ObjectPT{ID: o.ID, Power: p, Toughness: t, BasePower: bp, BaseToughness: bt})
	}
	return out
}

// battlefieldPTTable is LayerTables.BattlefieldPTs: every battlefield object
// whose Derived current P/T differs from the printed face plus counters, or
// whose Derived base P/T differs from the printed face, with those values.
// An object whose Derived P/T is the object-alone read is omitted, so the
// effects fallback stays exact for it and the table is nil on a plain board.
// Built in g.Objs order (no map range).
func (e *Engine) battlefieldPTTable() []effects.ObjectPT {
	var out []effects.ObjectPT
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		f := o.Face()
		if o.Zone != state.ZBattlefield || f == nil {
			continue
		}
		p, t, bp, bt, ok := e.FilterDerivedPT(o.ID)
		if !ok {
			continue
		}
		dp, dt := o.CounterPTTotals()
		fp, ft := int32(f.Power()), int32(f.Toughness())
		if p == fp+int32(dp) && t == ft+int32(dt) && bp == fp && bt == ft {
			continue
		}
		out = append(out, effects.ObjectPT{ID: o.ID, Power: p, Toughness: t, BasePower: bp, BaseToughness: bt})
	}
	return out
}

// Chars is effects.HostRead's characteristics query: the object's current,
// layer-derived characteristics (effects.Chars, which Derived aliases). It is
// Derived(id) answered by pointer: inside a Derived memo scope (a legal-actions
// walk, a BeginDerivedReads board build) the pointer is the memo entry
// itself, otherwise it is the engine's one charsScratch record.
//
// The pointer -- and its Keywords/Types -- is valid until the next Chars or
// Derived call or the next emit, whichever comes first; a caller that holds
// characteristics across either copies the record (and the slices it keeps),
// and never makes two Chars reads in one expression: Go does not order the
// first read's field load before the second call, so both can see the second
// record (the api:ExchangeTextBox capture hit exactly this).
// derivedMemoVerify (the rules test binary) recomputes every memo hit, the
// empirical check that a served entry is the current answer.
func (e *Engine) Chars(id state.ObjID) *effects.Chars {
	if e.derivedMemoDepth > 0 && e.derivedMemoUsable() {
		if d := e.derivedMemoRef(id, 0); d != nil {
			return d
		}
	}
	e.charsScratch = e.derivedCompute(id, 0)
	return &e.charsScratch
}

func (e *Engine) Game() *state.Game { return e.G }

func (e *Engine) ObjectColors(o *state.Object) string { return e.objColors(o) }

// CastProhibited is effects' optional castProhibitedHost read (task
// play-prohibited-election): the same CantBeCast gate beginPlay enforces
// (engine/rules/statics_cast.go castRestrictedFree -- every Play election is
// a free or PlayCost$ cast, CR 107.3b, so no CR 601.3a X lookahead),
// exposed so effPlay's Play election never
// OFFERS a cast that CR 601.3 would refuse. It is a pure read -- no event,
// no state change -- and the beginPlay recheck stays the enforcement site.
func (e *Engine) CastProhibited(p state.PlayerID, id state.ObjID) bool {
	// A land is played, not cast: its offer follows beginPlay's land gate
	// (CR 305.2a/305.3), so the election never offers a land play the
	// hand-off would refuse.
	if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().IsLand() {
		return e.effectLandPlayRefusal(p, id, o.Zone) != ""
	}
	return e.castRestrictedFree(p, id)
}

// CastTargetsUnavailable is effects' optional castTargetsHost read: the
// spell's mandatory targets (root and SubAbility$ chain) cannot all be chosen
// now, the same census the hand offer applies (castTargetsAvailable), so a
// Play election never offers a cast CR 601.2c forbids. A card with no spell
// ability (a land, a permanent without a targeting clause) is never judged
// unavailable. A pure read.
func (e *Engine) CastTargetsUnavailable(p state.PlayerID, id state.ObjID) bool {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false
	}
	sa := o.Face().SpellAbility()
	if sa == nil {
		return false
	}
	return !e.castTargetsAvailable(p, id, sa)
}

// CopyTargetCandidates is effects' optional copyTargetCandidatesHost read
// (CR 707.10d, CopySpellAbility CopyForEachCanTarget$): every target the
// spell's first targeting declaration could legally choose now, announced by
// the copier, in the target census's deterministic order. A pure read.
func (e *Engine) CopyTargetCandidates(copier state.PlayerID, spell state.ObjID) []state.Target {
	o := e.G.Obj(spell)
	if o == nil {
		return nil
	}
	root, _ := e.copyDeclarationSplit(o)
	for _, sa := range root {
		if sa == nil || !effects.TargetsOf(sa).Targeted() {
			continue
		}
		var out []state.Target
		for _, cand := range e.legalTargetCandidates(copier, spell, spell, sa) {
			out = append(out, candidateTarget(cand))
		}
		return out
	}
	return nil
}

// GraveyardKeywordParam is effects' optional graveyardDredgeHost read: a
// graveyard card's derived keyword parameter (derivedKeywordParam, the object's
// CURRENT layer-derived keyword list). Dredge (CR 702.55) is offered from the
// graveyard, where the board-wide derived table effects publishes bounds no
// entry, so the single-object derivation is the only way an ability loss
// (CR 613.1f -- Yixlid Jailer's RemoveAllAbilities$ static) or a layer-6
// keyword grant can be honoured at that read site. A pure read: no event,
// no state change.
func (e *Engine) GraveyardKeywordParam(id state.ObjID, head string) (string, bool) {
	return e.derivedKeywordParam(id, head)
}
