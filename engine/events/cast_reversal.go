package events

// CastReversals pairs the CR 733.1 reversals in a log with the casts they
// undid, for a reader that counts casts by walking the log backward (this
// turn's casts: Storm, the per-turn cast limit, "first spell you cast each
// turn"). A reversed cast's PutOnStack stays in the append-only log, but the
// spell was never cast: "no abilities trigger and no effects apply as a result
// of an undone action". The reversal (SpellCastReversed) is logged after its
// PutOnStack, so a backward walk meets the reversal first and the cast it
// undid next.
//
// Feed every event of the walk to Undone, newest first.
type CastReversals struct {
	owed [8]int32
}

// Undone reports whether ev is a PutOnStack that a later reversal undid: the
// reader skips it. A SpellCastReversed is recorded and answers false, being no
// cast itself.
func (r *CastReversals) Undone(ev *Event) bool {
	if ev.Player < 0 || int(ev.Player) >= len(r.owed) {
		return false
	}
	switch ev.Kind {
	case SpellCastReversed:
		r.owed[ev.Player]++
	case PutOnStack:
		if r.owed[ev.Player] > 0 {
			r.owed[ev.Player]--
			return true
		}
	}
	return false
}
