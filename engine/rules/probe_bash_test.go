package rules

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TEMP PROBE (delete before commit): rebuild the divergence decision's p1
// board and dump gorge's potential plans for Bash to Bits' flashback cast.
func probeCardSrc(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile("../../.cards/cardsfolder/" + path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestProbeBashFlashbackOffer(t *testing.T) {
	bash := probeCardSrc(t, "b/bash_to_bits.txt")
	e, _, spell := newFixtureDeck(t, 9701, bash)
	toGraveyard(t, e, spell)
	mk := func(path string, sick bool) state.ObjID {
		id := onBoard(t, e, 0, probeCardSrc(t, path))
		e.G.Obj(id).SummonSick = sick
		return id
	}
	mk("m/mystic_gate.txt", false)
	mk("m/mountain.txt", false)
	mk("o/orzhova_the_church_of_deals.txt", false)
	vivid := mk("v/vivid_meadow.txt", false)
	mk("f/forest.txt", false)  // Forest#56
	mk("f/forest.txt", true)   // Forest#54, just played
	mk("e/energy_refractor.txt", false)
	mk("b/bubbling_cauldron.txt", false)
	e.emit(events.Event{Kind: events.CounterChange, Obj: vivid, Counter: "CHARGE", Amount: 2})
	paymentPlanReask(t, e)

	var rows string
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Obj != spell {
			continue
		}
		rows += pp.Action.Mode + ": reason=" + pp.Reason + " detail=" + pp.Detail +
			" plan=" + itob(pp.Plan != nil) + " script=" + itob(len(pp.Script) > 0)
		if pp.Plan != nil {
			for _, a := range pp.Plan.Activations {
				rows += " act(src=" + probeItoa(int(a.Source)) + ")"
			}
		}
		rows += "\n"
	}
	if rows == "" {
		t.Fatal("precondition: no potential plan row for the Bash to Bits cast")
	}
	// census + manual script search
	e.beginDerivedMemo()
	hyp, opts := e.potentialWalkOf(0, true)
	e.endDerivedMemo()
	c := e.paymentPlanCensusOf(0, &hyp)
	rows += "census: complete=" + itob(c.complete) + " relaxable=" + itob(c.relaxable) +
		" bounded=" + itob(c.bounded) + " gapGains=" + itob(c.gapGains) + " fees=" + probeItoa(c.fees()) + " gaps=["
	for _, g := range c.gaps {
		rows += probeItoa(int(g)) + "=" + e.G.Obj(g).Face().Name + " "
	}
	rows += "] opts="
	for _, o := range opts {
		if o.Obj == spell {
			rows += "(kind=" + o.Kind + " mode=" + o.Mode + " alt=" + probeItoa(o.AltCostIndex) + " cost=" + o.Cost + ")"
		}
	}
	rows += "\n"
	for _, o := range opts {
		if o.Obj != spell || o.Kind != "cast" {
			continue
		}
		sc, ok := e.potentialPrefixScript(0, o, c.gaps, c.fees())
		rows += "scriptSearch(" + o.Mode + "): found=" + itob(ok) + " steps=" + probeItoa(len(sc))
		for _, st := range sc {
			rows += " {kind=" + string(st.Kind)
			for _, pk := range st.Picks {
				rows += " pick=" + pk.Kind + "/" + pk.Label + "/" + pk.ManaSymbol
			}
			rows += "}"
		}
		rows += "\n"
	}
	t.Fatal("rows:\n" + rows)
}



func itob(b bool) string {
	if b {
		return "y"
	}
	return "n"
}

func probeItoa(n int) string { return string([]byte{byte(48 + n%10)}) }
