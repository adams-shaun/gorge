package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// castOfferedFor reports whether the seat's decision offers a cast of o.
func castOfferedFor(d Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkCast && d.Options[i].Source() == TCode(o) {
			return true
		}
	}
	return false
}

// callousHostScen is the tail-45-c001 pair 6 board: an Island and a Sandstone
// Needle (its counters set to ctrs) beside Callous Dismissal ({1}{U}, return
// target nonland permanent), the only nonland permanent being Curator's Ward
// attached to the Needle.
func callousHostScen(t *testing.T, ctrs int) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Sandstone Needle", Count: 1}, {Name: "Callous Dismissal", Count: 1},
		{Name: "Curator's Ward", Count: 1}, {Name: "Island", Count: 17}}
	tbl, _, _, err := cardsettest.Synth(t, "tail-45").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	for _, n := range []string{"Island", "Sandstone Needle", "Callous Dismissal", "Curator's Ward"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("precondition: %s is not in the table", n)
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true, GorgeLowering: true})
	sc.perm(id["Island"], 0)
	needle := sc.perm(id["Sandstone Needle"], 0)
	ward := sc.perm(id["Curator's Ward"], 0)
	nsl := sc.slot(needle)
	a, ok := sc.s.riderAbilityOf(sc.k, nsl)
	if !ok {
		t.Fatal("precondition: Sandstone Needle is not the depletion-rider shape")
	}
	sc.s.perms[nsl].flags &^= pfTapped // it entered tapped; it has untapped since
	for n := sc.s.perms[nsl].ctrN(sc.s, int(a.ManaRiderCtr)); n > ctrs; n-- {
		sc.s.perms[nsl].subCtrN(sc.s, int(a.ManaRiderCtr), 1)
	}
	if n := sc.s.perms[nsl].ctrN(sc.s, int(a.ManaRiderCtr)); n != ctrs {
		t.Fatalf("precondition: the Needle holds %d depletion counters, want %d", n, ctrs)
	}
	wsl := sc.slot(ward)
	if sc.s.perms[wsl].attached != nsl {
		sc.s.attachTo(sc.k, wsl, nsl)
	}
	if sc.s.perms[wsl].attached != nsl {
		t.Fatal("precondition: Curator's Ward is not attached to the Needle")
	}
	return sc, sc.hand(id["Callous Dismissal"], 0)
}

// CR 704.5m, 605.3b: the Needle's {R}{R} on its last counter sacrifices it,
// and the Ward on it falls off; the Island alone cannot pay {1}{U}, so every
// plan strands the spell's only target and gorge's search offers no cast
// (potential_script.go targetsGone).
func TestCallousDismissalWardOnLastCounterNeedleNotOffered(t *testing.T) {
	sc, spell := callousHostScen(t, 1)
	if d := sc.s.Step(); castOfferedFor(d, spell) {
		t.Fatalf("Callous Dismissal is offered although paying it sacrifices the Needle under its only target; opts %v", d.Options[:d.N])
	}
}

// Control: a Needle with a counter to spare survives the payment, so the Ward
// stays a legal target and the cast is offered.
func TestCallousDismissalWardOnTwoCounterNeedleOffered(t *testing.T) {
	sc, spell := callousHostScen(t, 2)
	if d := sc.s.Step(); !castOfferedFor(d, spell) {
		t.Fatalf("Callous Dismissal is not offered with the Needle on two counters; opts %v", d.Options[:d.N])
	}
}
