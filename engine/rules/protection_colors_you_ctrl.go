package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// protectionColorsYouCtrl is Forge's "Protection from colorsYouCtrl" quality
// (Empty-Shrine Kannushi, Pledge of Loyalty): "protection from the colors of
// permanents you control". It is relative to the protected permanent's
// controller, so protectedFrom judges it rather than sourceHasQuality.
const protectionColorsYouCtrl = "colorsyouctrl"

// colorsYouCtrlReader is the slice of Engine sourceSharesControlledColor
// needs: the live controller and colours (layers applied) of an object, and
// the per-colour source test.
type colorsYouCtrlReader interface {
	controllerOf(state.ObjID) state.PlayerID
	Colors(state.ObjID) string
	sourceHasQuality(state.ObjID, string) bool
}

// sourceSharesControlledColor reports whether source has a colour that some
// battlefield permanent controlled by you has (Forge Card.hasProtectionFrom's
// colorsYouCtrl arm). Colours are read live at the time of the check, the
// protected permanent itself included, so a colour change on either side moves
// the answer. Objects are walked in ObjID order; the union is order-free.
func sourceSharesControlledColor(g *state.Game, r colorsYouCtrlReader, you state.PlayerID, source state.ObjID) bool {
	var union string
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil || r.controllerOf(o.ID) != you {
			continue
		}
		for _, c := range r.Colors(o.ID) {
			if !strings.ContainsRune(union, c) {
				union += string(c)
			}
		}
	}
	for _, c := range union {
		for _, w := range [...]string{"white", "blue", "black", "red", "green"} {
			if protecColourLetter(w) == c && r.sourceHasQuality(source, w) {
				return true
			}
		}
	}
	return false
}
