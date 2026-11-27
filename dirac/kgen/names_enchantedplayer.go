package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	enum[kop.Ref]("kop.RefEnchantedPlayer", kop.RefEnchantedPlayer)
	enum[kop.Whose]("kop.WhoseEnchantedOpp", kop.WhoseEnchantedOpp)
}

// whoEnchantedOpp is the trigger Whose of ValidPlayer$ Opponent.EnchantedBy:
// the player the source Aura enchants, when an opponent of its controller. It
// must equal kop.WhoseEnchantedOpp (build_ab.go casts it).
const whoEnchantedOpp uint8 = 204

var _ = [1]struct{}{}[int(whoEnchantedOpp)-int(kop.WhoseEnchantedOpp)]
