package kgen

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// an as-enters "choose a creature type" narrowed by
// ValidTypes$ (Dawn-Blessed Pennant, Eclipsed Realms: "choose Elemental,
// Elf, ..."). gorge creatureTypeOptions filters its option list with
// effects.FilterTypeWords; the card carries the allowed vocabulary indexes
// (CardDef.TypeValid, in vocabulary order) and the instruction's B = 1
// tells the kernel's ask to offer only those (act_choosetype.go).

// chooseTypeValid reads ValidTypes$ (InvalidTypes$ refuses) onto in and
// the card's TypeValid list. Every listed word must be in gorge's
// creature-type vocabulary.
func (lc *lowerCtx) chooseTypeValid(ps *params, in *lInstr) error {
	if _, ok := ps.get("InvalidTypes"); ok {
		return refuse("ChooseType InvalidTypes$")
	}
	list, ok := ps.get("ValidTypes")
	if !ok {
		return nil
	}
	if lc.out.TypeValid != nil {
		return refuse("ChooseType ValidTypes$ twice on one card")
	}
	vocab := effects.CreatureTypeWordList()
	allow := map[string]bool{}
	for _, w := range strings.Split(list, ",") {
		w = strings.TrimSpace(w)
		found := false
		for _, v := range vocab {
			if v == w {
				found = true
			}
		}
		if !found {
			return refuse("ChooseType ValidTypes %q", w)
		}
		allow[w] = true
	}
	var idx []uint16
	for v, w := range vocab {
		if allow[w] {
			idx = append(idx, uint16(v))
		}
	}
	if len(idx) == 0 {
		return refuse("ChooseType ValidTypes$ empty")
	}
	lc.out.TypeValid = idx
	in.B = cst(1)
	in.CS = "valid=" + strings.TrimSpace(list)
	return nil
}

// cardTypeChoice maps gorge's chooseCardTypes vocabulary to kop type bits.
var cardTypeChoice = map[string]kop.TypeMask{
	"Artifact": kop.TArtifact, "Battle": kop.TBattle, "Creature": kop.TCreature, "Enchantment": kop.TEnchantment,
	"Instant": kop.TInstant, "Kindred": kop.TKindred, "Land": kop.TLand, "Planeswalker": kop.TPlaneswalker,
	"Sorcery": kop.TSorcery,
}

// chooseCardType lowers a mid-resolution ChooseType | Type$ Card |
// ValidTypes$ <card types> (Winding Way; gorge effChooseType over
// TypeChoiceLabels("Card", ...): one KChoose option per listed card type,
// in the vocabulary's alphabetical order) to OpChooseType with C = the
// offered types' kop.TypeMask. The answer is the resolution's chosen card
// type, which a later Card.ChosenType filter reads (F2ChosenType).
func (lc *lowerCtx) chooseCardType(ps *params) error {
	if d, ok := ps.get("Defined"); ok && d != "You" {
		return refuse("ChooseType Card Defined %q", d)
	}
	var m kop.TypeMask
	for _, v := range strings.Split(ps.str("ValidTypes"), ",") {
		b, ok := cardTypeChoice[strings.TrimSpace(v)]
		if !ok {
			return refuse("ChooseType Card ValidTypes %q", v)
		}
		m |= b
	}
	if bits.OnesCount16(uint16(m)) < 2 {
		return refuse("ChooseType Card with fewer than two types (gorge's no-ask fallback)")
	}
	if _, ok := ps.get("AtRandom"); ok {
		return refuse("ChooseType AtRandom")
	}
	lc.emit(lInstr{Op: "ChooseType", Obj: lRef{Kind: rYou}, C: int(m), Note: "choice among card types"})
	return nil
}

// A mid-resolution ChooseType | Type$ Basic Land whose SubAbility$ is an
// Animate | Types$ ChosenType "in addition to its other types" (Navigator's
// Compass): gorge effChooseType asks over TypeChoiceLabels("Basic Land") --
// the five basic land types, sorted -- and combatfx.go animateGrant adds the
// source's ChosenType at resolution, and with it the type's intrinsic mana
// ability (CR 305.6). kgen lowers the choice to OpChooseType with B =
// kop.ChooseTypeBasicLand and the Animate to five override records, one per
// basic land type in that order, under one OpOverride with B =
// kop.OvChosenType (the chosen-colour pattern, emitChosenColorAnimate). A
// RemoveLandTypes$ True Animate ("becomes the basic land type of your
// choice", Dream Thrush) refuses: CR 305.7's set-and-strip record has no
// kernel override yet.

// basicLandChoice is gorge's Type$ Basic Land option list, in its order.
var basicLandChoice = effects.TypeChoiceLabels("Basic Land", "", "")

// chooseBasicLandType lowers the ChooseType half.
func (lc *lowerCtx) chooseBasicLandType(ps *params) error {
	if d, ok := ps.get("Defined"); ok && d != "You" {
		return refuse("ChooseType Basic Land Defined %q", d)
	}
	for _, k := range []string{"AtRandom", "ValidTypes", "InvalidTypes", "Secretly"} {
		if _, ok := ps.get(k); ok {
			return refuse("ChooseType Basic Land %s$", k)
		}
	}
	n := lc.curNode.Sub
	if n == nil || n.API != "Animate" || strings.TrimSpace(n.Params["Types"]) != "ChosenType" {
		return refuse("ChooseType Basic Land without a Types$ ChosenType Animate")
	}
	lc.landTypeGrant = true
	lc.emit(lInstr{Op: "ChooseType", Obj: lRef{Kind: rYou}, B: cst(kop.ChooseTypeBasicLand), Note: "choice among the basic land types"})
	return nil
}

// animateChosenLandType lowers the Animate half on obj: the grant without
// its Types$, copied into the five records.
func (lc *lowerCtx) animateChosenLandType(ps *params, obj lRef) error {
	landSet := ps.is("RemoveLandTypes", "True")
	for _, k := range []string{"Optional", "AtEOT", "RemoveCreatureTypes", "RemoveCardTypes"} {
		if _, ok := ps.get(k); ok {
			return refuse("Animate Types$ ChosenType with %s$", k)
		}
	}
	if obj.Kind == rTarget {
		t := &lc.ab.Targets[obj.N]
		if t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N != 1 {
			return refuse("chosen-type Animate target slot: not one battlefield permanent")
		}
	} else if obj.Kind != rSelf {
		return refuse("chosen-type Animate object %v", obj.Kind)
	}
	// the grant without its Types$: a copy of the params (the IR's map is
	// shared), its reads marked on ps
	m := make(map[string]string, len(ps.m))
	for k, v := range ps.m {
		if k != "Types" {
			m[k] = v
		}
	}
	rest := newParams(m)
	o, dur, chosen, err := lc.animateOverride(rest, obj)
	if err != nil {
		return err
	}
	ps.consume("Types")
	for k := range rest.used {
		ps.used[k] = true
	}
	if chosen || o.WhileHost {
		return refuse("chosen-type Animate beside a chosen colour or a host-bound duration")
	}
	lc.landTypeGrant = false
	start := len(lc.out.overrides)
	for _, w := range basicLandChoice {
		r := *o
		if landSet {
			// CR 305.7: the land's land types become the chosen one and it
			// loses its rules-text abilities, keeping the intrinsic mana
			// ability of the type (kop.Override.LandSet)
			r.LandSet, r.LoseAbilities = true, true
		}
		r.AddSubtypes = []string{w}
		lc.out.overrides = append(lc.out.overrides, &r)
	}
	in := lInstr{Op: "Override", Obj: obj, CS: dur + "|chosenType"}
	in.Note = overrideString(lc.out.overrides[start])
	in.C = start
	lc.emit(in)
	return nil
}

// chosenTypeOverridesOK asserts the OvChosenType contract: the five records
// from first are identical apart from their one added subtype, which is
// basicLandChoice's in order, as subIndexes stores it. A subtype the pool
// table does not hold yet (the first build pass) is not compared.
func (cb *cardBuilder) chosenTypeOverridesOK(first int) error {
	if first+5 > len(cb.pt.overrides) || len(basicLandChoice) != 5 {
		return fmt.Errorf("chosen-type override: %d records from %d", len(cb.pt.overrides)-first, first)
	}
	base := cb.pt.overrides[first]
	for i, w := range basicLandChoice {
		if cb.pt.overrides[first+i] != base {
			return fmt.Errorf("chosen-type override %d differs beyond its subtype", first+i)
		}
		add := cb.pt.ovSubs[first+i].add
		if len(add) != 1 || len(cb.pt.ovSubs[first+i].set) != 0 {
			return fmt.Errorf("chosen-type override %d adds %d subtypes, want one", first+i, len(add))
		}
		if j, ok := cb.pt.subIdx[w]; ok && add[0] != uint8(j) {
			return fmt.Errorf("chosen-type override %d adds subtype %d, want %s", first+i, add[0], w)
		}
	}
	return nil
}

// chooseCreatureTypeMid lowers a mid-resolution creature-type ChooseType
// (Kindred Charge; A Killer Among Us's enters trigger, "then secretly
// choose Human, Merfolk, or Goblin"). gorge effChooseType: an absent
// Defined$ is the controller, ValidTypes$ narrows the option list exactly as
// the as-enters ask's (h.TypeChoices over the same filter), so it rides the
// card's TypeValid list and B = 1 (the kernel's mid-resolution ask offers
// only those, act_choosetype.go midTypeOptions). Secretly$ True changes no
// decision: gorge marks the Choose event Secret so the view redacts the
// answer from the other seats, and the options, the chooser and the stored
// ChosenType are the public ask's.
func (lc *lowerCtx) chooseCreatureTypeMid(ps *params) error {
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "You" {
		return refuse("ChooseType shape")
	}
	if s, ok := ps.get("Secretly"); ok && !strings.EqualFold(strings.TrimSpace(s), "True") {
		return refuse("ChooseType Secretly$ %q", s)
	}
	in := lInstr{Op: "ChooseType", Obj: lRef{Kind: rYou}}
	if err := lc.chooseTypeValid(ps, &in); err != nil {
		return err
	}
	lc.emit(in)
	return nil
}
