package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAutopayNoPlanFilterFirstX is the shrink of the four-seat edh panic:
// Hydroid Krasis ({X}{G}{U}) is cast with a Forest, an Island and a Simic
// Signet ({1}, {T}: Add {G}{U}), a filter that is its card's first and only
// mana ability. At X = 0 the printed sources pay {G}{U}, so the cast is
// offered, and the X ask's filter bound (filtMaxX, CR 605.3b) then offers
// X = 1: the Signet's fee can be paid in the CR 601.2g window. The planner's
// census prices no first-only filter, so the payment had no plan and the
// kernel panicked ("autopay found no plan") before the window could pay it.
func TestAutopayNoPlanFilterFirstX(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Hydroid Krasis", Count: 1},
		{Name: "Forest", Count: 8},
		{Name: "Island", Count: 8},
		{Name: "Simic Signet", Count: 1},
	}
	tbl, da, _, err := edhSet(t).Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	forest := sc.perm(id["Forest"], 0)
	island := sc.perm(id["Island"], 0)
	signet := sc.perm(id["Simic Signet"], 0)
	krasis := sc.hand(id["Hydroid Krasis"], 0)

	// Precondition: the Signet is a census gap the planner cannot tap. If the
	// kernel ever prices it as an ordinary unit the panic cannot arise from it.
	if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(signet)) {
		t.Fatal("Simic Signet is no first-only filter census gap")
	}

	d := sc.s.Step()
	var cast Action
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == krasis {
			cast = a
		}
	}
	if cast == 0 {
		t.Fatalf("Hydroid Krasis not offered; decision kind %d opts %v", d.Kind, d.Options[:d.N])
	}
	sc.s.Apply(cast)

	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision %d/%d after the cast, want the X ask", d.Kind, d.Sub)
	}
	x1 := false
	for i := 0; i < int(d.N); i++ {
		x1 = x1 || d.Options[i].Mode() == 1
	}
	if !x1 {
		t.Fatalf("X = 1 not offered over the Signet's filter: %v", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1))

	// The payment: the plan has no witness for {1}{G}{U} (the Signet is no
	// unit), so it must go through the CR 601.2g window.
	for i := 0; i < 8; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if got := sc.s.objs[krasis].zone; got != kop.ZoneStack {
		t.Fatalf("Hydroid Krasis in zone %d, want the stack", got)
	}
	// Precondition for the stack check: the cast really cost the filter its
	// activation; an X = 0 payment would leave both basics and the Signet alone.
	if !sc.tapped(forest) || !sc.tapped(island) {
		t.Error("the basics are untapped after {1}{G}{U}")
	}
	if !sc.tapped(signet) {
		t.Error("the Signet is untapped: the filter did not pay the {1}")
	}
}
