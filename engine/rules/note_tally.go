package rules

import "github.com/adams-shaun/gorge/engine/state"

// note_tally.go holds the note-attribution tally: engine scratch that
// names the card or ability behind the notes whose event carries no source
// object (the aborted cast/activation Notes of abortCast, and the
// default-answered ask Note of Engine.Ask). The cardval note census reads
// the tallies at game end; nothing in the engine reads them for behaviour.
//
// The tally is keyed text <US> label so a plain map[string]int clone tag
// carries it. It rides Clone (clone:"deep", not reset) deliberately: the
// tape kernel's aborted-run rollback RESTOREs the engine from a checkpoint
// clone taken mid-run, and a reset tag would drop the counts of everything
// before the checkpoint while the log prefix keeps their notes. The
// surviving cost of that choice is the discarded run's own aborts, which the
// census reports as tally-vs-log overcounts rather than losing.

// tallySep separates the note text from its attribution label inside a
// tally key (US, which no note text or card name contains).
const tallySep = "\x1f"

// noteAttribution is the attribution label for a note whose event carries
// no source object: the card's first face name, or the arena id when the
// object or its face is already gone. ability marks the label of an
// activated ability's abort (the pending cast's card id names the source
// permanent, not the ability itself).
func noteAttribution(o *state.Object, ability bool) string {
	name := "<unknown>"
	if o != nil && o.Face() != nil {
		name = o.Face().Name
	}
	if ability {
		name += " (activation)"
	}
	return name
}

// tallyNote records one attributed note into a tally map, lazily
// initialising it so a call site stays a one-line record.
func tallyNote(t map[string]int, text, label string) map[string]int {
	if t == nil {
		t = map[string]int{}
	}
	t[text+tallySep+label]++
	return t
}
