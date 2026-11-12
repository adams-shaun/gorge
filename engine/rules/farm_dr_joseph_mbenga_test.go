package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TRK farm windows (3way:FX|G, field permanents): with Deanna Troi, Psychic
// Counsel out ("other creatures you control enter with an additional +1/+1
// counter"), Dr. Joseph M'Benga entered as a 5/5 with two +1/+1 counters in
// gorge and a 4/4 with one in Forge and XMage. CR 614.12: a replacement that
// modifies how a permanent enters comes from the permanent itself only if it
// affects that permanent alone; M'Benga's "a creature you control" doubler is
// a general subset, so it does not apply to its own entry counter. A creature
// entering after M'Benga is on the battlefield is still doubled, and a
// self-scoped line (Mowu's "on CARDNAME") still applies to its own entry.

func mbengaEntryCounters(t *testing.T, seed uint64, enter string, alreadyOut ...string) int32 {
	t.Helper()
	troi := corpusAlternativeCard(t, "Deanna Troi, Psychic Counsel")
	seat0 := []*cards.Card{troi}
	var out []*cards.Card
	for _, n := range alreadyOut {
		c := corpusAlternativeCard(t, n)
		out = append(out, c)
		seat0 = append(seat0, c)
	}
	entrant := corpusAlternativeCard(t, enter)
	seat0 = append(seat0, entrant)
	e, cfg := tokenReplGame(t, seed, seat0...)
	moveSeededCard(t, e, 0, troi, state.ZBattlefield)
	for _, c := range out {
		moveSeededCard(t, e, 0, c, state.ZBattlefield)
	}
	id := moveSeededCard(t, e, 0, entrant, state.ZBattlefield)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("%s did not reach the battlefield: %+v", enter, o)
	}
	replayCheck(t, e, cfg)
	return o.Counter("P1P1")
}

func TestFarmDrJosephMBenga(t *testing.T) {
	t.Parallel()
	// Precondition: M'Benga's doubler is live for another creature entering
	// under Troi's rider, so the self case below is not vacuous.
	if got := mbengaEntryCounters(t, 311, "Grizzly Bears", "Dr. Joseph M'Benga"); got != 2 {
		t.Fatalf("Grizzly Bears entered with %d +1/+1 counters beside M'Benga and Troi, want 2 (Troi's one, doubled)", got)
	}
	if got := mbengaEntryCounters(t, 313, "Dr. Joseph M'Benga"); got != 1 {
		t.Fatalf("Dr. Joseph M'Benga entered with %d +1/+1 counters under Troi's rider, want 1: its own general doubler does not apply to its entry (CR 614.12)", got)
	}
	if got := mbengaEntryCounters(t, 317, "Mowu, Loyal Companion"); got != 2 {
		t.Fatalf("Mowu entered with %d +1/+1 counters under Troi's rider, want 2: its self-only \"plus one\" applies to its own entry (CR 614.12)", got)
	}
}
