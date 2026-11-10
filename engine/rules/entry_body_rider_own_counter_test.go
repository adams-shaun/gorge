package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// std-tmt farm rows: Michelangelo, Weirdness to 11 cast through Mikey & Don,
// Party Planners entered with two +1/+1 counters in gorge and one in the
// kernel. Mikey & Don's "enters with an additional +1/+1 counter" body chains
// a SubAbility$ rider, so the entry fold does not absorb it and it runs on the
// ordinary Updated body path. CR 614.12: Michelangelo's "a creature you
// control" plus-one line is a general subset, so it does not modify how
// Michelangelo itself enters, on that path as on the folded one.

// entryLenderScript lends every other creature its controller's entering an
// additional +1/+1 counter through a body with a SubAbility$ rider (the shape
// the fold leaves to the ordinary body path).
const entryLenderScript = `Name:Entry Lender
ManaCost:2
Types:Artifact
R:Event$ Moved | ValidCard$ Creature.YouCtrl | Destination$ Battlefield | ReplacementResult$ Updated | ReplaceWith$ ETBCounter | Description$ Creatures you control enter with an additional +1/+1 counter.
SVar:ETBCounter:DB$ PutCounter | ETB$ True | Defined$ ReplacedCard | CounterType$ P1P1 | CounterNum$ 1 | SubAbility$ Rider
SVar:Rider:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:Creatures you control enter with an additional +1/+1 counter.
`

func entryBodyRiderCounters(t *testing.T, seed uint64, enter string, alreadyOut ...string) int32 {
	t.Helper()
	lender, ds := cards.ParseBytes("entry_lender.txt", []byte(entryLenderScript))
	if lender == nil || len(ds) > 0 {
		t.Fatalf("parse Entry Lender: %v", ds)
	}
	if ds := lender.Link(); len(ds) > 0 {
		t.Fatalf("link Entry Lender: %v", ds)
	}
	if r := lender.Faces[0].Repls; len(r) != 1 || r[0].With == nil || r[0].With.Sub == nil {
		t.Fatalf("Entry Lender's body must carry its SubAbility$ rider (the unabsorbed shape): %+v", r)
	}
	seat0 := []*cards.Card{lender}
	var out []*cards.Card
	for _, n := range alreadyOut {
		c := tokenReplCorpusCard(t, n)
		out = append(out, c)
		seat0 = append(seat0, c)
	}
	entrant := tokenReplCorpusCard(t, enter)
	seat0 = append(seat0, entrant)
	e, cfg := tokenReplGame(t, seed, seat0...)
	moveSeededCard(t, e, 0, lender, state.ZBattlefield)
	for _, c := range out {
		moveSeededCard(t, e, 0, c, state.ZBattlefield)
	}
	life := e.G.Players[0].Life
	id := moveSeededCard(t, e, 0, entrant, state.ZBattlefield)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("%s did not reach the battlefield: %+v", enter, o)
	}
	if got := e.G.Players[0].Life; got != life+1 {
		t.Fatalf("the lent body's rider did not run on %s's entry (life %d -> %d)", enter, life, got)
	}
	replayCheck(t, e, cfg)
	return o.Counter("P1P1")
}

func TestEntryBodyRiderOwnCounterReplacementExcluded(t *testing.T) {
	t.Parallel()
	const mikey = "Michelangelo, Weirdness to 11"
	// Precondition: Michelangelo's plus-one line is live for another creature
	// entering through the lent body, so the self case is not vacuous.
	if got := entryBodyRiderCounters(t, 331, "Grizzly Bears", mikey); got != 2 {
		t.Fatalf("Grizzly Bears entered with %d +1/+1 counters beside Michelangelo, want 2 (the lent one, plus one)", got)
	}
	if got := entryBodyRiderCounters(t, 337, mikey); got != 1 {
		t.Fatalf("Michelangelo entered with %d +1/+1 counters through the lent body, want 1: its own general plus-one does not apply to its entry (CR 614.12)", got)
	}
}
