package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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
