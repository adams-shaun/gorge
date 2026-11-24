package kgen

import (
	"fmt"
	"io"
)

// Umbra armor (CR 702.89a): "If enchanted permanent would be destroyed,
// instead remove all damage marked on it and destroy this Aura." gorge reads
// the printed keyword at its destruction choke points (engine/effects
// ReplaceUmbraArmor, after the regeneration shield and a shield counter), so
// kgen only marks the Aura: CardDef.UmbraArmor, read by the kernel's destroy
// paths (kw_umbraarmor.go). A granted umbra armor (Umbra Mystic) stays refused.

// lowerBareKeyword lowers the parameterless printed keywords that have a
// lowering of their own, reporting whether head is one of them.
func (lc *lowerCtx) lowerBareKeyword(head string) bool {
	switch head {
	case "Decayed":
		lc.lowerDecayed()
	case "Umbra armor":
		lc.out.UmbraArmor = true
	case "Banding":
		// CR 702.22: no KwMask bit is free, so the printed head is
		// CardDef.Banding; the kernel's hasBanding (dirac/kw_banding.go) reads
		// it for the band ask and the damage-division chooser, beside the
		// granted kop.Override.Banding records (lower_banding.go)
		lc.out.Banding = true
	default:
		return false
	}
	return true
}

// dumpKeywordFlags prints the printed-keyword flags of a card in the LIR dump.
func dumpKeywordFlags(w io.Writer, c *lCard) {
	if c.Wither {
		fmt.Fprintf(w, " wither")
	}
	if c.UmbraArmor {
		fmt.Fprintf(w, " umbra-armor")
	}
	if c.Dredge != 0 {
		fmt.Fprintf(w, " dredge=%d", c.Dredge)
	}
}
