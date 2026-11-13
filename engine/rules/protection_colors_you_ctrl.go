package rules

import "github.com/adams-shaun/gorge/engine/state"

// protectionColorsYouCtrl is Forge's "Protection from colorsYouCtrl" quality
// (Empty-Shrine Kannushi, Pledge of Loyalty): "protection from the colors of
// permanents you control" (Forge matches it case-sensitively). It is relative to the protected permanent's
// controller, so protectedFrom judges it rather than sourceHasQuality.
const protectionColorsYouCtrl = "colorsYouCtrl"

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
	var seen [5]bool // WUBRG
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil || r.controllerOf(o.ID) != you {
			continue
		}
		for _, c := range r.Colors(o.ID) {
			switch c {
			case 'W':
				seen[0] = true
			case 'U':
				seen[1] = true
			case 'B':
				seen[2] = true
			case 'R':
				seen[3] = true
			case 'G':
				seen[4] = true
			}
		}
	}
	for i, w := range [...]string{"white", "blue", "black", "red", "green"} {
		if seen[i] && r.sourceHasQuality(source, w) {
			return true
		}
	}
	return false
}
