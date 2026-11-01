package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// K:Storied (CR 702.175): "If you control three or more
// artifacts, legendaries, and/or Sagas, you have an enduring story for the
// rest of the game."
//
// The keyword is read DIRECTLY here rather than expanded in
// engine/cards/keywords.go, exactly like kw:Ascend (engine/rules/ascend.go) and kw:Start
// your engines (engine/rules/speed.go): Forge's own expansion mints a static
// ability this engine has no representation for, and the equivalent read is
// the emit-side scan below plus state.Player.EnduringStory. Registering the
// primitive makes the coverage census count the 9 K:Storied carriers as
// supported.
func init() {
	effects.RegisterNonAPI("kw:Storied")
}

// checkEnduringStoryGrants is the continuous half of CR 702.175a's grant:
// called from Engine.emit's post-fold hook on every battlefield entry
// (MoveZone), token mint (TokenCreate, CardToken) and control transfer
// (ControlChange) -- the same hook shape checkBlessingGrants uses. For each
// still-alive seat without the designation it checks the gate directly off
// the FOLDED state: the seat controls at least three permanents that are
// artifacts, legendaries and/or Sagas (each permanent counted ONCE, even if
// it matches several), and controls at least one permanent whose derived
// keywords include Storied. It then emits the one-way EnduringStoryChange
// latch. Running after the fold means a third qualifying permanent's own
// arrival grants it, and the recursive emit the scan makes sees a now-set
// seat, so the scan terminates.
func (e *Engine) checkEnduringStoryGrants() {
	if e.G.Over {
		return
	}
	if !e.storiedPossible() {
		return
	}
	for _, p := range e.G.AliveFrom(0) {
		if int(p) >= len(e.G.Players) || e.G.Players[p].Lost || e.G.Players[p].EnduringStory {
			continue
		}
		board := e.G.Zone(state.ZBattlefield, p)
		storied, count := false, 0
		for _, id := range board {
			o := e.G.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield {
				continue
			}
			if e.hasKeywordH(id, kwhStoried) {
				storied = true
			}
			d := e.Derived(id)
			if slices.Contains(d.Types, "Artifact") ||
				slices.Contains(d.Types, "Saga") ||
				(o.Face() != nil && o.Face().IsLegendary()) {
				count++
			}
		}
		if storied && count >= 3 {
			e.emit(events.Event{Kind: events.EnduringStoryChange, Player: p,
				Text: "enduring story"})
		}
	}
}

// playerHasEnduringStory reports whether seat p holds the CR 702.175
// enduring-story latch. The ONE read of state.Player.EnduringStory for the
// rules-side gates: layers.go's continuousConditionHolds (Condition$
// EnduringStory) and replacement.go's replacementConditionHolds
// (EnduringStory$ True/False, Bombur, Gentle Dreamer) both go through it, so
// the bounds guard cannot drift between them. A seat index outside the
// player slice reads false, the gates' documented deny direction.
func (e *Engine) playerHasEnduringStory(p state.PlayerID) bool {
	if int(p) < 0 || int(p) >= len(e.G.Players) {
		return false
	}
	return e.G.Players[p].EnduringStory
}

// storiedPossible is checkEnduringStoryGrants' amortised pre-filter, the
// exact shape of ascendPossible (engine/rules/ascend.go): the scan reads every
// permanent's DERIVED keywords, and the post-fold hook runs it on every
// battlefield entry, so a game with no Storied card in its arena pays a
// full derived walk per entry for nothing. No object can carry Storied
// unless some card in the game's object arena mentions it (printed
// K:Storied, or an AddKeyword$/KW$ grant naming it), so the arena is scanned
// once, incrementally (objects are only ever appended), and the per-seat
// walk runs only once such a card exists. It is a pure cache over state and
// emits nothing.
type storiedScan struct {
	game    *state.Game
	scanned int
	seen    bool
}

func (e *Engine) storiedPossible() bool {
	s := &e.storied
	if s.game != e.G || s.scanned > len(e.G.Objs) {
		*s = storiedScan{game: e.G}
	}
	for ; !s.seen && s.scanned < len(e.G.Objs); s.scanned++ {
		if e.cardScanHas(e.G.Objs[s.scanned].Card, faceScanStoried) {
			s.seen = true
		}
	}
	return s.seen
}
