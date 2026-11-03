package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// A continuous static's "as long as any player controls a black permanent"
// (Knight of Grace) is checked when the static is applied in its layer, by then
// past layer 5 (CR 613.1e, 613.4c), so it counts the permanent Eaten by Piranhas
// made black and not the one Witness Protection made green and white. Forge
// checks a static's conditions per layer and reads the same answer
// (StaticAbility.shouldApplyContinuousAbility, read).
//
// staticEffects decides the gate while it is still building the list the
// colours come from, so the walk's matchesSpec reads printed colours. A build
// that met such a gate and has a colour effect live therefore walks a second
// time, with the colours the first walk's list gives. A colour effect gated on a
// colour would need a third; the corpus has none.
//
// Colours move on events the quiet admission accepts and on the registry, and the
// gate re-check has no table, so such a build counts as a state read: it is
// rescanned on every run but a layer-inert one.

// noteStaticColorGate records a gate naming a colour; its reads are the second
// walk's business.
func (e *Engine) noteStaticColorGate(sv staticView) {
	for _, key := range [...]cards.ParamKey{cards.PKIsPresent, cards.PKIsPresent2} {
		if spec, ok := sv.Param(key); ok && specBindFacts(spec).colors {
			e.staticColorGate, e.staticMemoStateRead = true, true
			return
		}
	}
}

// rewalkStaticColorGates is the second walk. It runs after the first build's
// sequence bump, so the nested Derived reads build their own list from it.
func (e *Engine) rewalkStaticColorGates() {
	if !e.anyColorEffectListed() {
		return
	}
	// One list for the whole read: a nested active() builds a private one and
	// stamps the sequence, after which every other read would hit the outer
	// activeBuf, the previous build's.
	outer := e.activeBuf
	e.activeBuf = e.active()
	defer func() { e.activeBuf = outer }()
	var table []effects.ObjectColors
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil {
			continue
		}
		printed := effects.ColorMaskOf(o)
		if derived := effects.ColorMaskFromLetters(e.derivedComputeFresh(o.ID, 0).Colors); derived != printed {
			table = append(table, effects.ObjectColors{ID: o.ID, Mask: derived})
		}
	}
	if len(table) == 0 {
		return
	}
	e.staticGateColors = table
	e.rescanStatic()
	e.staticBuildSeq++
}

func (e *Engine) anyColorEffectListed() bool {
	for i := range e.staticContinuous {
		if e.staticContinuous[i].Layer == LColor {
			return true
		}
	}
	for i := range e.continuous {
		if e.continuous[i].Layer == LColor {
			return true
		}
	}
	return false
}
