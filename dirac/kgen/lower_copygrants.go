package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// lower_copygrants.go: CopyPermanent AddStaticAbilities$ naming one equip
// cost reduction of the copy itself (Firion, Wild Rose Warrior: "except it
// has 'This Equipment's equip abilities cost {2} less to activate'", gorge
// copypermanent.go grantCostStatics). The copy is a token of the COPIED
// card, which is not the card that read the grant, so the static cannot be
// one of the reading card's own: it is lowered once here, then lowerPool
// adds it to every pool card with an Equip ability, switched on by a bit of
// the copy exception's override record (kop.Static.OvGate,
// kop.Override.GrantStatics; the kernel's ovGranted reads a copy exception's
// record as a copiable value, CR 707.9b).

// equipGrantBit is the OvGate bit of an equip reduction of n generic mana,
// 1 to 4: the top four bits, which an Animate staticAbilities$ grant (the
// low bits, allocated upward) does not reach in a card of one equip
// reduction. 0 = not a supported amount.
func equipGrantBit(n int) uint8 {
	if n < 1 || n > 4 {
		return 0
	}
	return 0x80 >> (n - 1)
}

// copyEquipGrant lowers the AddStaticAbilities$ value v, one SVar name whose
// body is a Mode$ ReduceCost static of this Equipment's own equip abilities
// (ValidCard$ Card.Self, ValidSpell$ Activated.Equip, a literal Amount$), and
// returns the GrantStatics bit that switches it on; the static waits in
// lCard.EquipGrants for lowerPool.
func (lc *lowerCtx) copyEquipGrant(v string) (uint8, error) {
	name := strings.TrimSpace(v)
	ss, ok := cards.ParseStaticLines(lc.face.SVars[name])
	if !ok || len(ss) != 1 || strings.ContainsAny(name, ", ") {
		return 0, refuse("CopyPermanent AddStaticAbilities$ %q", v)
	}
	s := ss[0]
	n, err := strconv.Atoi(strings.TrimSpace(s.Params["Amount"]))
	if s.Mode != "ReduceCost" || s.Params["ValidCard"] != "Card.Self" || s.Params["ValidSpell"] != "Activated.Equip" || err != nil || equipGrantBit(n) == 0 {
		return 0, refuse("CopyPermanent AddStaticAbilities$ %q: only a copy's own equip abilities costing a literal 1 to 4 less", v)
	}
	before := len(lc.out.Statics)
	if err := lc.lowerStatic(s); err != nil {
		return 0, err
	}
	if len(lc.out.Statics) != before+1 || lc.out.Statics[before].AbilityRed != 5 || lc.out.Statics[before].F2 != nil {
		return 0, refuse("CopyPermanent AddStaticAbilities$ %q lowered to an unexpected static", v)
	}
	st := lc.out.Statics[before]
	lc.out.Statics = lc.out.Statics[:before] // not the reading card's own
	st.OvGate = equipGrantBit(n)
	for _, g := range lc.out.EquipGrants {
		if g.OvGate == st.OvGate {
			return st.OvGate, nil
		}
	}
	lc.out.EquipGrants = append(lc.out.EquipGrants, st)
	return st.OvGate, nil
}

// injectEquipGrants adds each pool card's EquipGrants (once per bit) to every
// pool card with an Equip ability, in card order; a card that already uses
// the bit for another gated static is refused.
func injectEquipGrants(all []*lCard) {
	var grants []lStatic
	for _, c := range all {
		for _, g := range c.EquipGrants {
			dup := false
			for _, h := range grants {
				dup = dup || h.OvGate == g.OvGate
			}
			if !dup {
				grants = append(grants, g)
			}
		}
	}
	if len(grants) == 0 {
		return
	}
	for _, c := range all {
		equip := false
		for _, a := range c.Abilities {
			equip = equip || a.Kind == akActivated && a.Name == "Equip"
		}
		if !equip {
			continue
		}
		for _, g := range grants {
			used := false
			for _, st := range c.Statics {
				used = used || st.OvGate&g.OvGate != 0
			}
			if used {
				c.Refusals = append(c.Refusals, refusal{Card: c.Name, Node: "equip grant", Reason: "an equip reduction a copy exception grants shares its OvGate bit with the card's own gated static"})
				continue
			}
			c.Statics = append(c.Statics, g)
		}
	}
}
