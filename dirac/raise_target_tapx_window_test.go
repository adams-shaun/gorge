package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// A target-dependent raise (Dragon's Prey's "this spell costs {2} more to
// cast if it targets a Dragon") drops from the target menu the candidates
// whose raised cost the caster cannot pay (gorge affordableTargetCandidates,
// CR 601.2h: choosing one can only end in the payment's abort). The menu's
// window probe prices the CR 601.2g window only while the payer holds an
// untapped mana source (gorge cast_pricing.go probeManaShort reads the
// abilities' payability): a tap-another mana source whose tap-another part
// has no candidate (Dragonbroods' Relic with no untapped creature to tap) is
// no source, so a pool-short menu drops its candidates instead of keeping
// them on a census the walk cannot price.

// raiseTapXScen seats Dragon's Prey in hand with {2}{B}{G} floating: the
// plain cast is payable, a Dragon target raises it to {4}{B}. seat 1's board
// holds a Dragon (Scavenger Regent) and a plain creature (Lasyd Prowler);
// seat 0 holds a Dragonbroods' Relic, and a Lasyd Prowler of its own when
// tapSource (the Relic's tap-another part then has a candidate).
func raiseTapXScen(t *testing.T, withRelic, tapSource bool) (*scen, Decision, Obj, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Dragon's Prey", Count: 2},
		{Name: "Dragonbroods' Relic", Count: 2},
		{Name: "Scavenger Regent", Count: 2},
		{Name: "Lasyd Prowler", Count: 2},
		{Name: "Swamp", Count: 12},
	}
	set := cardsettest.Synth(t, "std-tdm")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	dp := sc.hand(id["Dragon's Prey"], 0)
	if withRelic {
		sc.perm(id["Dragonbroods' Relic"], 0)
	}
	if tapSource {
		// seat 0's own untapped creature: the Relic's tap-another part has a
		// candidate, so the census the walk cannot price keeps the menu whole
		sc.perm(id["Lasyd Prowler"], 0)
	}
	dragon := sc.perm(id["Scavenger Regent"], 1)
	bear := sc.perm(id["Lasyd Prowler"], 1)
	sc.s.pl[0].pool[2] = 2 // {2}{B}
	sc.s.pl[0].pool[4] = 1 // {G}
	sc.do(t, AkCast, TCode(dp), scAny)
	d := sc.s.Step()
	return sc, d, dragon, bear
}

func raiseTapXOffers(d *Decision, o Obj) int {
	n := 0
	for i := 0; i < int(d.N); i++ {
		if TCode(o) == d.Options[i].Target() {
			n++
		}
	}
	return n
}

// TestRaiseTargetTapXWindowDeadSource: the Relic's tap-another part has no
// candidate (seat 0 controls no creature), so {4}{B} is unpayable and the
// Dragon target stays off the menu.
func TestRaiseTargetTapXWindowDeadSource(t *testing.T) {
	sc, d, dragon, bear := raiseTapXScen(t, true, false)
	_ = sc
	if raiseTapXOffers(&d, bear) == 0 {
		t.Fatalf("the plain creature's target vanished from the ask: %v", d.Options[:d.N])
	}
	if n := raiseTapXOffers(&d, dragon); n != 0 {
		t.Fatalf("the Dragon target is offered %d times with {4}{B} unpayable (the Relic's tap has no candidate): %v", n, d.Options[:d.N])
	}
}

// TestRaiseTargetTapXWindowLiveSource: the same board with an untapped
// creature for the Relic to tap. The census cannot price the tap-another
// source, so the menu keeps its candidates however short the window is
// (gorge's census fallback, castWindowCensusComplete) and the Dragon target
// stays on the ask.
func TestRaiseTargetTapXWindowLiveSource(t *testing.T) {
	sc, d, dragon, bear := raiseTapXScen(t, true, true)
	_ = sc
	if n := raiseTapXOffers(&d, dragon); n == 0 {
		t.Fatalf("the Dragon target vanished with the Relic activatable: %v", d.Options[:d.N])
	}
	if raiseTapXOffers(&d, bear) == 0 {
		t.Fatalf("the plain creature's target vanished from the ask: %v", d.Options[:d.N])
	}
}

// TestRaiseTargetTapXWindowNoSource: the same board with no Relic at all.
// Every mana source is tapped, so the window has nothing to tap and the
// Dragon target stays off the menu.
func TestRaiseTargetTapXWindowNoSource(t *testing.T) {
	sc, d, dragon, bear := raiseTapXScen(t, false, false)
	_ = sc
	if raiseTapXOffers(&d, bear) == 0 {
		t.Fatalf("the plain creature's target vanished from the ask: %v", d.Options[:d.N])
	}
	if n := raiseTapXOffers(&d, dragon); n != 0 {
		t.Fatalf("the Dragon target is offered %d times with {4}{B} unpayable and no untapped source: %v", n, d.Options[:d.N])
	}
}
