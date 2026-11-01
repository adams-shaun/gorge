package dirac

// CastSa Spell.ManaFromTreasure: the cast ring's per-push
// Treasure-spend bit.
//
// gorge tags every mana a mana ability produces with the source's types
// (engine/effects/mana_effect.go ManaProducerTag; an artifact Treasure is
// TypedArtifactTreasure) and folds the spend into the caster's bucket until
// the next PutOnStack consumes it (rules/host_cast_ledger.go castSpendFacts).
// The kernel's planner is the only way a spell's mana is paid, so a tap of
// a Treasure source during the spell's build marks that build; histCast
// copies the mark onto the ring entry noteCast appended at the push
// (CastSpecTreasure, thisTurnCastSpec).
//
// The bit follows the push, not the object: a recast gets its own bit, and
// an aborted cast that never reached histCast leaves its entry unmarked.
//
// The Treasure test is the planner's pcard view (printed subtypes, set by
// prepareGen when the table turns the cast ring on), the same printed-face
// view pc.manaSac already uses: a continuous effect that grants the Treasure
// subtype to a mana source is not read here. No pool carries such an effect.

// noteCastTreasure marks the ring entry of the in-flight cast (castN, set by
// noteCast) when its payment tapped a Treasure. A saturated ring recorded no
// entry (castN == 0), so it marks nothing.
//
//go:noinline
func (s *State) noteCastTreasure(e *stackEntry) {
	if s.bld().castN == 0 {
		return
	}
	s.castPushes(e.ctrl)[s.bld().castN-1].prov |= castTreasurePush
}
