package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/effects"
)

// selfOnlyCond reports whether a static's condition reads nothing a
// characteristic derivation produces: none, the turn, or the source's own
// counters (a level-up or station static: IsPresent$ Card.Self+counters).
func selfOnlyCond(c lCond) bool {
	switch c.Kind {
	case cNone, cYourTurn:
		return true
	case cAnd:
		// a Level up band: two Self counter tests (lower_levelband.go)
		for _, sub := range c.Sub {
			if !selfOnlyCond(sub) {
				return false
			}
		}
		return len(c.Sub) > 0
	case cPresent:
		if c.F == nil || c.F.Players != 0 || len(c.F.Clauses) == 0 {
			return false
		}
		for _, cl := range c.F.Clauses {
			if cl.Flags&^fSelf != 0 || cl.Types != 0 || cl.TypesAll != 0 || cl.NotTypes != 0 || cl.Subtype != "" || cl.NotSubtype != "" ||
				cl.Colors != "" || cl.NotColors != "" || cl.CmpStat != cmpCounters {
				return false
			}
			if cl.Flags&fSelf == 0 {
				return false
			}
		}
		return true
	}
	return false
}

// creatureOnly reports whether every clause of f requires a creature.
func creatureOnly(f *lFilter) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Types != tCreature {
			return false
		}
	}
	return true
}

// selfAffectedOnly reports whether a static's affected set is its own source
// alone: every clause is Self with no other restriction, so the layer walk
// applies the record to the carrying permanent and the live P/T anchor
// (static_layerwalk.go livePTSrc) names it.
func selfAffectedOnly(f *lFilter) bool {
	if f == nil || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Flags != fSelf || c.Types != 0 || c.TypesAll != 0 || c.NotTypes != 0 ||
			c.Subtype != "" || c.NotSubtype != "" || c.Subtype2 != "" ||
			c.MoreNotSubtype != [4]string{} ||
			c.Ctrl != 0 || c.Owner != 0 || c.CmpStat != 0 ||
			c.Colors != "" || c.NotColors != "" || c.Name != "" {
			return false
		}
	}
	return true
}

// loseAllNeedsCreatures reports whether a global override's lose-all reaches
// beyond creatures, which the walk does not serve (a land's mana abilities are
// read directly by the mana planner). A land-type setter's loss is exempt: the
// planner reads it through landSetView (dirac/static_landtypesetter.go).
func loseAllNeedsCreatures(o *lOverride, f *lFilter) bool {
	return o.LoseAbilities && !o.SetLandSub && !creatureOnly(f)
}

// overrideStatic lowers the layer-hard statics (Witness Protection,
// Imprisoned in the Moon) to an override record applied while the static
// applies.
func (lc *lowerCtx) overrideStatic(sp *params, st *lStatic) ([]lStatic, error) {
	var pump lStatic
	havePump := false
	if _, okP := sp.m["AddPower"]; okP {
		havePump = true
	} else if _, okT := sp.m["AddToughness"]; okT {
		havePump = true
	}
	if havePump {
		pm := map[string]string{}
		for _, k := range []string{"Affected", "AffectedZone", "AddPower", "AddToughness"} {
			if v, ok := sp.m[k]; ok {
				pm[k] = v
			}
		}
		pump = lStatic{Kind: "Continuous", Cond: st.Cond}
		if _, err := lc.continuous(newParams(pm), &pump); err != nil {
			return nil, err
		}
		if pump.Kind != "Continuous" {
			return nil, refuse("override pump shape")
		}
		sp.consume("AddPower", "AddToughness")
	}
	o := &lOverride{}
	pS, okP := sp.get("SetPower")
	tS, okT := sp.get("SetToughness")
	if okP || okT {
		// a static may set just one of them (Maha's base toughness 1);
		// the other keeps its earlier-layer value (gorge StaticSet)
		pn, e1 := strconv.Atoi(pS)
		tn, e2 := strconv.Atoi(tS)
		if okP && e1 != nil || okT && e2 != nil {
			// a non-integer value is read live at every layer-7b check,
			// anchored on the static's source (an Equipment's "your life
			// total" is its controller's). AffectedX is the per-affected
			// word, a different anchor, and stays refused by name. Only an
			// attached static is lowered here: the walk would apply a global
			// one too, but that shape (Porcelain Gallery's creature count)
			// has no lockstep repro yet, so it stays refused by policy.
			if pS == "AffectedX" || tS == "AffectedX" {
				return nil, refuse("set P/T %q/%q: the per-affected word", pS, tS)
			}
			// A Self-affected static is admitted beside the attached ones
			// now that the shape has a lockstep repro: Duplicant's
			// "has the power, toughness, and creature types of the last
			// creature card exiled with it" (X/Y = Imprinted$CardPower/
			// CardToughness, zone_imprint.go impRefSrc). The walk applies a
			// Self record to its carrying permanent only, and livePTSrc
			// anchors the read there.
			if !attachedOnly(st.F) && !selfAffectedOnly(st.F) {
				return nil, refuse("set P/T %q/%q: a live value on a non-attached static", pS, tS)
			}
			pv, tv := cst(0), cst(0)
			var err error
			if okP {
				if pv, err = lc.value(pS); err != nil {
					return nil, refuse("set P/T %q/%q: %v", pS, tS, err)
				}
			}
			if okT {
				if tv, err = lc.value(tS); err != nil {
					return nil, refuse("set P/T %q/%q: %v", pS, tS, err)
				}
			}
			o.HasSetPT, o.LivePT, o.KeepP, o.KeepT = true, true, !okP, !okT
			o.PVal, o.TVal = pv, tv
		} else {
			o.HasSetPT, o.SetP, o.SetT, o.KeepP, o.KeepT = true, pn, tn, !okP, !okT
		}
	}
	if c, ok := sp.get("SetColor"); ok {
		o.HasSetColors = true
		cols, ok := colorWordSet(c)
		if !ok {
			return nil, refuse("SetColor %q", c)
		}
		o.SetColors = cols
	}
	if sp.is("RemoveAllAbilities", "True") {
		o.LoseAbilities = true
	}
	removeCard := sp.is("RemoveCardTypes", "True")
	removeCreature := sp.is("RemoveCreatureTypes", "True")
	if v, ok := sp.get("RemoveType"); ok {
		// layer 4 (CR 613.1d, 205.1b): the devotion gods' "isn't a creature".
		// Only the exact Creature word is lowered (gorge applyTypeEffect
		// RemoveTypes strips the named word from the type list, before this
		// effect's AddType$ words); any other value stays refused by name.
		if strings.TrimSpace(v) != "Creature" {
			return nil, refuse("RemoveType %q", v)
		}
		// The runtime walk takes CR 205.1a's correlated subtypes with the
		// Creature mask (chars_gen.go subsGen, static_layerwalk.go lfxApply4):
		// gorge chars/types.go's RemoveTypes block strips them beside the
		// card type, and a RemoveCreatureSub set here would refuse the
		// conditioned devotion statics in selfCondCheck.
		o.RemoveTypes = uint16(typeBits["Creature"])
	}
	if v, ok := sp.get("AddAllCreatureTypes"); ok {
		// layer 4 (CR 205.3m / 702.73a): the object has every creature
		// type. Only the exact True shape is lowered; any other value
		// (a False, an expression) stays refused by name.
		if !sp.is("AddAllCreatureTypes", "True") {
			return nil, refuse("AddAllCreatureTypes %q", v)
		}
		o.AllCreatureTypes = true
	}
	if t, ok := sp.get("AddType"); ok {
		for _, w := range strings.Split(t, "&") {
			if w = strings.TrimSpace(w); w == "ChosenType" {
				// The chosen land-type setter's word (Convincing Mirage,
				// Phantasmal Terrain, Multiversal Passage): the static reads
				// the source's as-enters ChooseType answer (st.AddChosen,
				// kop.Static.AddChosen), and only the CR 305.7 setter behind
				// it grants it — landTypeSetter marks the record SetLandSub
				// with no added subtype of its own. Any other shape refuses
				// after the RemoveLandTypes read below, by the word's own
				// class, exactly as this loop refused it before the setter
				// existed.
				st.AddChosen, o.Chosen = true, true
				continue
			}
			if err := lc.overrideAddTypeWord(w, st, o); err != nil {
				return nil, err
			}
		}
	}
	landwalk, err := lc.overrideAddKeyword(sp, o) // lower_overridelandwalk.go
	if err != nil {
		return nil, err
	}
	if v, ok := sp.get("RemoveLandTypes"); ok {
		if err := landTypeSetter(v, st, o, removeCard || removeCreature); err != nil {
			return nil, err
		}
	}
	if o.Chosen && !o.SetLandSub {
		return nil, refuse("AddType %q", "ChosenType")
	}
	if removeCard {
		o.HasSetTypes = true
		o.HasSetSubtypes = true
		if err := o.keepSubtypes(strings.Split(sp.str("AddType"), "&")); err != nil {
			return nil, err
		}
	} else {
		// gorge continuous layers: without RemoveCardTypes$ the AddType$
		// words are added to the object's types (layer 4), the rest of the
		// record (P/T, colour, abilities) applies as with it
		if removeCreature {
			// gorge applyTypeEffect strips the creature subtypes and the
			// all-creature-types mark, then adds the effect's AddType$
			// words; the kernel's strip is a per-override mask step walked
			// for attached overrides with subtype-only additions
			if !attachedOnly(st.F) {
				return nil, refuse("global override RemoveCreatureTypes without RemoveCardTypes")
			}
			if o.SetTypes != 0 {
				return nil, refuse("override RemoveCreatureTypes adds a card type")
			}
			o.RemoveCreatureSub = true
		}
		for _, w := range o.SetSubtypes {
			switch w {
			case "Plains", "Island", "Swamp", "Mountain", "Forest":
				// CR 305.6: a basic land type carries its mana ability. gorge
				// appends the intrinsic "{T}: Add [c]" for every derived basic
				// type word (mana_activation.go's CR 305.6 block), so the
				// kernel's grant is lowerable exactly when the same AddType$
				// list also makes the object a LAND (Ashaya, Soul of the Wild's
				// "nontoken creatures you control are Forest lands", Life and
				// Limb's Saprolings): the mana walk's blg views (static_landtype.go)
				// cover any plain-shaped card then. Without the Land type the
				// intrinsic still applies in gorge but the kernel's blgMask
				// reads only lands, so that shape stays refused.
				if o.SetTypes&typeBits["Land"] == 0 && !o.SetLandSub {
					return nil, refuse("override adds basic land type %q", w)
				}
			}
		}
		o.AddTypes, o.SetTypes = o.SetTypes, 0
		o.AddSubtypes, o.SetSubtypes = o.SetSubtypes, nil
	}
	if r, ok := sp.get("RemoveKeyword"); ok {
		ks, err := removeKeywords(r)
		if err != nil {
			return nil, err
		}
		o.RemoveKw = ks
	}
	if !attachedOnly(st.F) {
		// a global override static (CR 613: Maha, Ygra) is walked
		// in layers 4-7 (its layer-6 parts too); its layer-3 name is
		// not (the naming path reads attached overrides only)
		if _, ok := sp.get("SetName"); ok {
			return nil, refuse("global override with SetName")
		}
		l6 := o.LoseAbilities || len(o.AddKw) > 0 || len(o.RemoveKw) > 0
		if l6 && !selfOnlyCond(st.Cond) {
			// a counting condition read while every permanent's
			// abilities are derived would re-enter the derivation
			return nil, refuse("global layer-6 override with condition %d", st.Cond.Kind)
		}
		if loseAllNeedsCreatures(o, st.F) {
			// a land's mana abilities (CR 305.6) are not walked: the
			// mana planner reads them directly
			return nil, refuse("global lose-all override over non-creatures")
		}
	}
	if err := walkFilterOK(st.F); err != nil {
		return nil, err
	}
	if err := lc.finishOverrideStatic(sp, st, o); err != nil {
		return nil, err
	}
	if havePump {
		// CR 613.6: an override part before layer 6 keeps the whole effect,
		// the pump beside it too, applying once its source loses all abilities
		pump.OutlivesLoss = o.beginsBeforeLayer6()
		return lc.landwalkCompanion(landwalk, st, o, []lStatic{pump, *st})
	}
	return lc.landwalkCompanion(landwalk, st, o, []lStatic{*st})
}

// overrideAddTypeWord lowers one AddType$ word onto o: a card type, the
// ImprintedCreatureType persist read, or a known creature subtype.
func (lc *lowerCtx) overrideAddTypeWord(w string, st *lStatic, o *lOverride) error {
	if b, ok := typeAddBit(w); ok {
		o.SetTypes |= b
		return nil
	}
	if w == "ImprintedCreatureType" {
		// Duplicant's AddType$: the last still-exiled creature card
		// in the source's kept imprints lends its printed creature
		// types (gorge layers.go: the list read from its end, the
		// live object's zone the CR 607.2a exile test). The kernel's
		// layer walk reads the store (chars_gen.go impCreatureSub),
		// so only the persist shape's own static lowers it.
		if !lc.impPersist {
			return refuse("AddType %q outside the persist shape", w)
		}
		if !selfAffectedOnly(st.F) {
			return refuse("AddType %q on a static that reaches past its source", w)
		}
		o.ImpSub = true
		return nil
	}
	if !lc.known(w) {
		return refuse("AddType %q", w)
	}
	o.SetSubtypes = append(o.SetSubtypes, w)
	return nil
}

// beginsBeforeLayer6 reports whether o has a part in layers 1 to 5 (the
// reading of ovrKeptOnLoss in the kernel, for a lowered override).
func (o *lOverride) beginsBeforeLayer6() bool {
	return o.HasSetTypes || o.SetTypes != 0 || o.AddTypes != 0 || o.HasSetSubtypes || len(o.SetSubtypes) > 0 ||
		len(o.AddSubtypes) > 0 || o.AllCreatureTypes || o.RemoveCreatureSub || o.RemoveTypes != 0 || o.HasSetColors || o.AddColors != "" || o.Name != "" || o.ImpSub
}

// keepSubtypes records what removing o's card types leaves, from the words the
// effect adds: gorge's own reading of CR 205.1a, so the two cannot differ on a
// word. A kept creature set is refused: nothing records a kept all-creature-types
// mark, and RemoveCreatureTypes$ would have to take it back.
func (o *lOverride) keepSubtypes(added []string) error {
	for i, w := range added {
		added[i] = strings.TrimSpace(w)
	}
	o.Kept = effects.SubtypesKeptBy(added)
	if o.Kept.Creature() {
		return refuse("RemoveCardTypes$ keeps creature subtypes")
	}
	return nil
}

// finishOverrideStatic reads an override static's name and granted ability
// and records it.
func (lc *lowerCtx) finishOverrideStatic(sp *params, st *lStatic, o *lOverride) error {
	if n, ok := sp.get("SetName"); ok {
		// The name filters compare CardIDs (Hare Apparent), so the rename
		// only names the object: lowerPool emits a name card carrying it
		// (dirac/feat reads the renamed object's name rows from it).
		o.Name = n
		lc.out.Tokens = append(lc.out.Tokens, nameCardKey+n)
	}
	if ab, ok := sp.get("AddAbility"); ok {
		idx, err := lc.grantedAbility(ab)
		if err != nil {
			return err
		}
		o.GrantAbility = append(o.GrantAbility, idx)
	}
	st.Kind = "Override"
	st.Override = o
	lc.out.overrides = append(lc.out.overrides, o)
	return nil
}

// attachedOnly reports whether every clause of f is limited to the
// source's host (EnchantedBy / EquippedBy).
func attachedOnly(f *lFilter) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Flags&(fEquippedBy|fEnchantedBy) == 0 {
			return false
		}
	}
	return true
}

// walkFilterOK refuses an override static whose affected filter reads a
// characteristic the layer walk cannot serve from the characteristics
// built so far (P/T, keywords): matching it would re-enter the derivation
// (CR 613, kernel matchPermCur).
func walkFilterOK(f *lFilter) error {
	if f == nil {
		return nil
	}
	for _, c := range f.Clauses {
		switch c.CmpStat {
		case cmpPower, cmpToughness, cmpPowerLTToughness, cmpPowerGTBase, cmpPTBox, cmpSpill:
			return refuse("override static filter reads P/T")
		}
		if c.Flags&(fWithFlying|fWithoutFlying) != 0 {
			return refuse("override static filter reads keywords")
		}
	}
	return nil
}
