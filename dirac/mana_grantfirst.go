package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// windowGrantFirstCol is the colour a cast window's source sl answers when
// its own mana ability makes no colour the cost still owes but an ability an
// effect granted it does (an Island under Chromatic Lantern, owing {B}).
// Gorge's ability ask lists that ability first (owedAbilitiesFirst reads the
// cast's unreduced cost, so it holds whether or not a convoke pick has
// reduced it), and the payment bot takes the first option. The colour ask
// that follows carries no ManaPayment (a plan that fell back, or a convoke
// cast the planner priced before its picks), so the bot answers the granted
// ability's lowest colour, W for an any-colour grant, not the pip owed.
// ok is false when sl's own ability is the first listed (its own mask owes a
// colour, or nothing is owed) or no grant makes an owed colour.
func (s *State) windowGrantFirstCol(k *prep, p uint8, sl Slot, useful *kop.ManaCost) (col int, ok bool) {
	if k.gen == nil || len(k.gen.grantPC) == 0 || !s.grantMay(k, sl) || !s.ownManaUsable(k, sl) {
		return 0, false
	}
	owed := s.windowDemand(k, p, useful) & 0x1f
	if owed == 0 || s.windowOwnMask(k, sl)&owed != 0 {
		return 0, false
	}
	grant := s.windowGrantMask(k, sl) & 0x1f
	if grant&owed == 0 {
		return 0, false
	}
	return bits.TrailingZeros8(grant), true
}
