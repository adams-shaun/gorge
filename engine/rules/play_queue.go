package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// queuedPlays is the not-yet-begun remainder of one answered Play (Etali,
// Primal Storm's Amount$ All "cast any number of spells from among them"),
// with the riders every card of that answer is begun under.
type queuedPlays struct {
	player           state.PlayerID
	ids              []state.ObjID
	free             bool
	playCost         string
	replaceGraveyard bool
	copyCard         bool
	// manaConversion is the Play SA's ManaConversion$ rider, copied onto
	// each card the Play casts (Forge PlayEffect's per-spell grant).
	manaConversion string
	// imprintOn is the ImprintPlayed$ True source (0 = no imprint): every
	// card the play actually begins is imprinted on it.
	imprintOn state.ObjID
}

func (q *queuedPlays) clone(m *cloneRemap) *queuedPlays {
	if q == nil {
		return nil
	}
	c := *q
	c.ids = append([]state.ObjID(nil), q.ids...)
	return &c
}

// runPlays begins q's cards in order. A cast that parks on its own question
// (e.cast still in flight, a pending decision, or a suspended resolution)
// stops the walk; q stays in e.queuedPlays while it still holds cards to
// begin or a parked continuation. It reports whether any card was begun.
func (e *Engine) runPlays(q *queuedPlays) bool {
	begun := false
	for len(q.ids) > 0 {
		id := q.ids[0]
		q.ids = q.ids[1:]
		from := state.Zone(0)
		if o := e.G.Obj(id); o != nil {
			from = o.Zone
		}
		mint := e.G.NextID
		e.beginPlay(q.player, id, q.free, q.playCost, q.replaceGraveyard, q.copyCard, q.manaConversion)
		begun = true
		if played, pfrom := e.playedObject(q.copyCard, id, from, mint); q.imprintOn != 0 && played != 0 && pfrom.Valid() {
			if o := e.G.Obj(played); o != nil && o.Zone != pfrom {
				e.emit(events.Event{Kind: events.Imprint, Obj: q.imprintOn,
					IDs: []state.ObjID{played}})
			}
		}
		if e.Suspended() || e.cast != nil || e.pending != nil {
			break
		}
	}
	if len(q.ids) > 0 {
		e.queuedPlays = q
	} else {
		e.queuedPlays = nil
	}
	return begun
}

// playedObject names the object a Play began and the zone it is judged to
// have left. A CopyCard$ play casts the copy beginPlay minted (id mint, first
// in ZLibrary), not the offered original, which deliberately stays put
// (Forge's PlayEffect imprints the copy). It is 0 when no copy was minted.
func (e *Engine) playedObject(copyCard bool, id state.ObjID, from state.Zone, mint state.ObjID) (state.ObjID, state.Zone) {
	if !copyCard {
		return id, from
	}
	if o := e.G.Obj(mint); o != nil && o.IsCopy {
		return mint, state.ZLibrary
	}
	return 0, 0
}

// startQueuedPlay drains e.queuedPlays before priority, the suspendedCasts /
// defeatedCasts shape (engine/rules/turn.go): once the parked cast that stopped
// runPlays has completed and nothing is pending, the next chosen card is
// begun; when none is left, the parked resolution continuation resumes.
func (e *Engine) startQueuedPlay() bool {
	q := e.queuedPlays
	if q == nil || e.cast != nil || e.pending != nil || e.Suspended() {
		return false
	}
	if len(q.ids) > 0 {
		e.runPlays(q)
		return true
	}
	e.queuedPlays = nil
	return true
}
