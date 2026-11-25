package dirac

// NoColoredMana$ True on a SpellCast line (Void Mirror: "Whenever a player
// casts a spell, if no colored mana was spent to cast it, counter that
// spell"; kop.Tf5CastNoColoredMana). gorge stamps the cast's per-colour
// spend (Object.ManaColorSpent) while such a reader is on the battlefield
// (rules noColoredManaReaderOut) and trigmatch SpellCastEval rejects a cast
// with any coloured unit in it, a fire-time match filter that is not
// rechecked on resolution. The spend is the same delta gorge's converge
// reads (payManaCastSpent), which the kernel tallies into the spell entry's
// spc while the table reads colours spent (genPrep.anyColSpent,
// mana_converge.go): a generic pip paid from a coloured unit counts toward
// that colour, a spell cast without paying mana spent none.

// DebugNoColoredManaOff is the lockstep mutation switch (test only): the
// gate always admits, as if the reader were a plain SpellCast trigger.
var DebugNoColoredManaOff bool

// castNoColoredOK is castTrigGen's Tf5CastNoColoredMana gate: spell e spent
// no coloured mana to be cast.
//
//go:noinline
func (s *State) castNoColoredOK(e *stackEntry) bool {
	if DebugNoColoredManaOff {
		return true
	}
	return spcColours(s.spellSpc(e.obj)) == 0
}
