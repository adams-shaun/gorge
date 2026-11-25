package kgen

import "testing"

// TestTapsForManaRemainingShapesRefused pins the TapsForMana census cards that
// wave 4 (1a5de63a29) refused by name: the four `ALL` Effect-EOT rows and the
// four card-level refusals. Wave 4 pinned only Bubbling Muck and Extraplanar
// Lens; the other six could move silently. This table locks the seven that
// still refuse: Mana Web's Land.OppCtrl clause now lowers
// (TestTapsForManaManaWebLockstep), so it is no longer a remaining shape.
func TestTapsForManaRemainingShapesRefused(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Bubbling Muck", refused: "Effect TapsForMana trigger until end of turn"},
		{card: "High Tide", refused: "Effect TapsForMana trigger until end of turn"},
		{card: "Pyre Rhymer", refused: "token refused"},
		{card: "Selesnya Loft Gardens", refused: "Effect TapsForMana trigger until end of turn"},
		{card: "Barbflare Gremlin", refused: "PresentDefined$"},
		{card: "Runaway Growth", refused: "Card.EnchantedBy"},
		{card: "Extraplanar Lens", refused: "sharesNameWith Imprinted"},
	})
}
