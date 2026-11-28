package rules

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// mayPlayBoardStatics is the battlefield half of every may-play walk: the
// printed Mode$ Continuous statics (activeStatics) followed by the
// AddStaticAbility$-granted may-play statics that carry a RaiseCost$
// surcharge (grantedRaiseMayPlayStatics).
//
// A granted static without RaiseCost$ (Rogue Class, Fortune Teller's Talent)
// rides the layer walk's registered effect instead (layers.go's MayPlay
// rules-mod, through mayPlayGrant's whitelist); that effect has no field for
// a surcharge, so mayPlayGrant refuses a RaiseCost$ body and the grant was
// inert. Reading exactly those bodies here, with the HOST as source as
// grantedStatics resolves it, gives them the printed route's one gate chain
// (mayPlayStatic: Condition$, MayPlayLimit$, the RaiseCost$ price), and no
// grant is ever offered by both routes: Exploration Broodship's STATION 8+
// "cast a permanent spell from your graveyard by sacrificing a land".
func (e *Engine) mayPlayBoardStatics() []staticView {
	bf := e.activeStatics("Continuous")
	g := e.grantedRaiseMayPlayStatics()
	if len(g) == 0 {
		return bf
	}
	return append(slices.Clip(bf), g...)
}

// grantedRaiseMayPlayStatics returns the live granted Continuous statics with
// MayPlay$ True and a RaiseCost$, in grantedStatics' deterministic order.
func (e *Engine) grantedRaiseMayPlayStatics() []staticView {
	var out []staticView
	for _, sv := range e.grantedStatics("Continuous") {
		if raiseMayPlayBody("Continuous", sv.Params) {
			out = append(out, sv)
		}
	}
	return out
}

// grantsRaiseMayPlay reports whether st (a static on face f) grants, through
// AddStaticAbility$, a Continuous MayPlay$ True body carrying RaiseCost$: the
// shape mayPlayBoardStatics reads. The quiet proof's board scan uses it so
// its MayPlay$ hit stays a superset of mayPlayBoardGrantsOpen.
func grantsRaiseMayPlay(f *cards.Face, st *cards.Static) bool {
	name := strings.TrimSpace(st.ParamStr(cards.PKAddStaticAbility))
	if name == "" || f == nil {
		return false
	}
	for _, in := range parseGrantedStatics(f, name) {
		if raiseMayPlayBody(in.Mode, in.Params) {
			return true
		}
	}
	return false
}

// raiseMayPlayBody is the one shape test: a Continuous MayPlay$ True body
// with a RaiseCost$.
func raiseMayPlayBody(mode string, params map[string]string) bool {
	return mode == "Continuous" && strings.TrimSpace(params["MayPlay"]) == "True" && strings.TrimSpace(params["RaiseCost"]) != ""
}
