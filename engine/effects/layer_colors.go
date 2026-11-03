package effects

import "github.com/adams-shaun/gorge/engine/state"

// layerColorsHost is the optional Host read of an object's colours through
// layer 5 only (rules.Engine.LayerColors).
type layerColorsHost interface {
	LayerColors(*state.Object) string
}

// LiveColors is o's colour letters as the layers leave them after layer 5
// (CR 613.1e). It never evaluates layer 7, so a count that runs inside a
// power/toughness-defining ability may use it; a Host without the cut-off
// read answers with ObjectColors.
func LiveColors(h Host, o *state.Object) string {
	if lh, ok := h.(layerColorsHost); ok {
		return lh.LayerColors(o)
	}
	return h.ObjectColors(o)
}
