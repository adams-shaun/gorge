package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tableHasLockGain: some ability of t runs a DealDamageEx with DmgLockGain, so
// a player can be barred from gaining life for the rest of the game and the
// gain sites must consult playerBanned (gen.go anyCantGain).
func tableHasLockGain(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpDealDamageEx && in.C&kop.DmgLockGain != 0 {
					return true
				}
			}
		}
	}
	return false
}
