package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ledgerCastFaceCopy returns a detached copy of o showing the spell face the
// PutOnStack at log[i] cast, when that face is no longer the one o shows;
// nil when o shows its cast face (or the cast was an ordinary one).
//
// CR 715.3b: "While on the stack as an Adventure, the spell has only its
// alternative characteristics"; CR 720.3 gives an Omen cast the same. Once
// the spell leaves the stack the card is its normal face again (CR 715.4,
// 720.4: events.Apply's MoveZone fold resets the face), but the cast
// ledger counts the cast as it was made: Bubble Up resolved into exile is
// still the turn's first sorcery spell (Aquatic Alchemist's
// ActivatorThisTurnCast$ EQ1; Forge's ThisTurnCast list holds cast-time
// copies, XMage's SpellsCastWatcher holds the Spell objects). The cast face
// is read from the cast's own pay-time CastInfo (FlagAdventure, FlagOmen),
// which sits below the push and before any later zone change of the card.
// The copy is a read-only hypothetical for the filter match; it never
// reaches the game state.
func ledgerCastFaceCopy(log []events.Event, i int, o *state.Object) *state.Object {
	if o == nil || o.FaceIdx != 0 || o.CopyFace != nil {
		return nil
	}
	adventure, omen := adventureSpellFace(o) != nil, omenSpellFace(o) != nil
	if !adventure && !omen {
		return nil
	}
	var flags uint64
	for j := i + 1; j < len(log); j++ {
		ev := &log[j]
		if ev.Obj != o.ID {
			continue
		}
		if ev.Kind == events.CastInfo {
			flags = events.FlagsFrom(ev.Counter)
			break
		}
		if ev.Kind == events.PutOnStack || ev.Kind == events.MoveZone {
			break
		}
	}
	if adventure && flags&state.FlagAdventure == 0 || omen && flags&state.FlagOmen == 0 {
		return nil
	}
	cp := *o
	cp.SetFaceIdx(1)
	return &cp
}
