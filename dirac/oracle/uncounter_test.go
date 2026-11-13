package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The uncounter lockstep: a battlefield "spells
// you control can't be countered" replacement (R:Event$ Counter | Layer$
// CantHappen | ActiveZones$ Battlefield | ValidSA$ Spell[.types]+YouCtrl).
// Chimil, the Inner Sun protects every spell its controller casts, Sphinx of
// the Final Word their instants and sorceries (its creature spells stay
// counterable); both seats also run instants, sorceries, creatures, hard
// counters (Counterspell, Cancel), a creature counter, a noncreature counter
// and a counter-unless-pay spell (Mana Leak). The expected answer at every
// decision is gorge's.
//
// gorge asks a CR 616.1 order question when two such replacements match one
// Counter event (this spell's own plus a battlefield source, or two sources),
// which dirac poses in uncGate. This pool keeps one source
// per seat (one Chimil each, or one Sphinx each; the Sphinx spell itself is
// the lone own-replacement, a battlefield Sphinx never meets its own spell) so
// it also covers the ordinary single-match path; the two-source and own-line
// competitions are TestStdEclReprosLockstep/uncounter_order and
// TestStdEclReprosLockstep/uncounter_order_own_line.
func kuncounterDecks(source string) [2][]cardset.Entry {
	seat := func(counters, blue []cardset.Entry) []cardset.Entry {
		d := []cardset.Entry{{Name: source, Count: 1}}
		d = append(d, counters...)
		return append(d, blue...)
	}
	blue := []cardset.Entry{
		{Name: "Opt", Count: 3}, {Name: "Divination", Count: 3}, {Name: "Unsummon", Count: 2},
		{Name: "Wind Drake", Count: 5}, {Name: "Air Elemental", Count: 4}, {Name: "Island", Count: 14},
	}
	counters := []cardset.Entry{
		{Name: "Counterspell", Count: 3}, {Name: "Cancel", Count: 2}, {Name: "Essence Scatter", Count: 1},
		{Name: "Mana Leak", Count: 1}, {Name: "Negate", Count: 1},
	}
	return [2][]cardset.Entry{seat(counters, blue), seat(counters, blue)}
}

var kuncounterSources = []string{"Chimil, the Inner Sun", "Sphinx of the Final Word"}

func TestUncounterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugUncounterStats = true
	defer func() { dirac.DebugUncounterStats = false }()
	dirac.UncounterHeld, dirac.UncounterPassed, dirac.UncounterStalls = 0, 0, 0
	for _, src := range kuncounterSources {
		// serial: the Uncounter* counters are plain package ints
		lockstepSetSerial(t, "uncounter", kuncounterDecks(src), 300, "KUNCOUNTER_SEEDS")
	}
	t.Logf("counters stopped by a battlefield source: %d; let through on a table with one: %d; order stalls: %d",
		dirac.UncounterHeld, dirac.UncounterPassed, dirac.UncounterStalls)
	if dirac.UncounterHeld < 20 {
		t.Errorf("only %d Counter attempts on a protected spell: the pool does not exercise the replacement", dirac.UncounterHeld)
	}
	if dirac.UncounterPassed < 10 {
		t.Errorf("only %d Counter attempts on an unprotected spell: the pool does not exercise the filter", dirac.UncounterPassed)
	}
	if dirac.UncounterStalls != 0 {
		t.Errorf("%d games stalled on the unposed CR 616.1 order ask: the pool must hold one source per seat", dirac.UncounterStalls)
	}
}

// TestUncounterMutation proves the pool exercises the scan: with the
// battlefield scan off the kernel counters a spell gorge refuses to, in each
// matchup.
func TestUncounterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugUncounterOff = true
	defer func() { dirac.DebugUncounterOff = false }()
	for _, src := range kuncounterSources {
		if !lockstepDiverges(t, "uncounter", kuncounterDecks(src), 128) {
			t.Errorf("%s: no seed diverged with the battlefield scan off: the pool does not exercise the replacement", src)
		}
	}
}
