package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugLandGrantIdentOff is the mutation: a commander-identity land ignores the
// granted basic land type's intrinsic ability (Command Tower under Stormtide
// Leviathan), as the kernel did before it merged the granted colour.
var DebugLandGrantIdentOff bool

// identGrantView is manaPCGen's view of a commander-identity land (Command
// Tower) under a granted basic land type: the identity view with the granted
// colours merged into its mask.
//
// gorge lists the identity ability's per-colour alternatives (Combo W, Combo G)
// and appends the granted type's intrinsic {U}: Produced$ ColorIdentity is no
// single colour, so ManaAbilitiesProduce never dedups the intrinsic one
// (pay.ManaAbilitiesProduce keys on the raw text). The source ranks by the
// number of distinct mana types over every alternative (planalts.go Flex), so
// the planner view holds the union. nil when the card is no identity land or
// the grant adds nothing. A controller with no commander identity stalls: gorge
// offers the intrinsic ability alone (CR 903.4f), which the view does not run.
//
//go:noinline
func (s *State) identGrantView(k *prep, sl Slot, c CardID, vi int16) *pcard {
	g := k.gen
	if DebugLandGrantIdentOff || g.flags[c]&gfManaRefl == 0 || !g.varPC[vi].manaIdent {
		return nil
	}
	gm := s.blgMask(k, sl) & kop.ColAny
	if gm == 0 {
		return nil
	}
	m := s.reflColours(k, sl, c)
	if m == 0 {
		s.capFull(StallLandGrant)
		return nil
	}
	return &g.varPC[int(vi)+int(m|gm)]
}

// DebugLandGrantThrivingOff is the mutation: a Thriving land under a granted
// basic land type is refused again.
var DebugLandGrantThrivingOff bool

// blgThriving: pc is a land whose only mana ability is a plain {T} "Combo W
// Chosen" one: one fixed colour or the permanent's chosen colour (Thriving
// Heath, Citadel Gate). Under a granted basic land type gorge lists the
// intrinsic ability beside it (the raw Produced$ is no single colour, so it is
// never deduplicated), and the planner ranks the source over the union of the
// fixed, the chosen and the granted colour, as it does a dual's.
func blgThriving(c *kop.CardDef, pc *pcard) bool {
	if DebugLandGrantThrivingOff || c.Types&kop.TLand == 0 || pc.manaAb < 0 || !pc.manaTap || pc.manaN > 1 || bits.OnesCount8(uint8(pc.manaCol)&0x1f) != 1 {
		return false
	}
	n := 0
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
			n++
		}
	}
	a := &c.Abilities[pc.manaAb]
	return n == 1 && a.AFlags&^kop.AfManaChoice == kop.AfManaChosen && a.ManaRestrict == 0 && a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && len(a.Prog) == 0
}

// DebugLandGrantThrivingView is the mutation: a Thriving land's granted view
// keeps its printed colours alone, the intrinsic colour dropped.
var DebugLandGrantThrivingView bool

// blgDropsGrant: the mutation switches that leave a granted colour off view v.
func blgDropsGrant(v *pcard) bool {
	return v.painCol != 0 && DebugLandGrantPainView || v.manaChosen && DebugLandGrantThrivingView
}

// blgOwnUsable: a card with no mana ability of its own still has the intrinsic
// ones its granted basic land types give it (Ashaya's Forest {G}), which gorge's
// planner counts as the source's own unit; a granted ability (Citanul
// Hierophants' "{T}: Add {G}") is then beside it, not the source's only one.
//
//go:noinline
func (s *State) blgOwnUsable(k *prep, sl Slot) bool {
	return !DebugLandGrantOwnOff && k.gen.blgOn && s.blgMask(k, sl)&kop.ColAny != 0
}

// DebugLandGrantOwnOff is the mutation: the intrinsic ability of a land-typed
// card with no own ability is no usable one beside a granted ability, as before.
var DebugLandGrantOwnOff bool
