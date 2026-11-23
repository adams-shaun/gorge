package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestYannikReflexiveDividedCountersLowers: Yannik, Scavenging Sentinel's
// "when you do, distribute X +1/+1 counters among any number of target
// creatures, where X is the exiled creature's power" lowers as a fourth
// divided PutCounter total (lower_dividedcounters.go dividedCountersRemLKI):
// the reflexive trigger handed the battle-link pick's remembered set
// (RememberObjects$ RememberedLKI) captures the set's summed last-known
// power as it is put on the stack (OpReflexiveTrigger B = ExRemLKIPT, the
// pick's remLK accumulation), the slot is 0..X targets (TgfBoundsV over
// ExTriggerAmount) with DivValue the captured amount, and the body's
// CounterNum is the same capture. Every other divided shape keeps refusing.
func TestYannikReflexiveDividedCountersLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const name = "Yannik, Scavenging Sentinel"
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	c := lowerCard(p, card.Faces[0].Name, card, false)
	if len(c.Refusals) != 0 {
		t.Fatalf("%s refusals = %v, want none", name, c.Refusals)
	}
	var etb, lat *lAbility
	for _, a := range c.Abilities {
		if a.Latent && a.Name == "ReflexiveTrigger" {
			lat = a
		}
		for i := range a.Prog {
			if a.Prog[i].Op == "ReflexiveTrigger" {
				etb = a
			}
		}
	}
	if etb == nil || lat == nil {
		t.Fatalf("no ETB ability or reflexive latent: %d abilities", len(c.Abilities))
	}
	// the parent's program: the battle-link remember pick, the gate, the
	// ReflexiveTrigger whose B is the pick's remLK sum, the cleanup
	var reflex *lInstr
	hasPick, hasClear := false, false
	for i := range etb.Prog {
		in := &etb.Prog[i]
		switch in.Op {
		case "ChooseMove":
			fl := flagsOf(in.CS)
			_, ul := fl["UntilLeaves"]
			_, rm := fl["Remember"]
			hasPick = hasPick || fl["from"] == "Battlefield" && ul && rm
		case "ReflexiveTrigger":
			reflex = in
		case "ClearRemembered":
			hasClear = true
		}
	}
	if !hasPick || reflex == nil || !hasClear {
		t.Fatalf("the ETB program lacks the pick, the reflexive trigger or the cleanup: %s", progDump(etb.Prog))
	}
	if reflex.B.Kind != vRemLKIPT || reflex.B.N != 0 {
		t.Fatalf("ReflexiveTrigger B = %v, want RemLKIPT(n=0)", reflex.B)
	}
	// the latent: one 0..X battlefield slot divided by the captured amount,
	// and one DividedCounters body over it
	if len(lat.Prog) != 1 || lat.Prog[0].Op != "DividedCounters" {
		t.Fatalf("the latent's program = %s, want one DividedCounters", progDump(lat.Prog))
	}
	if len(lat.Targets) != 1 {
		t.Fatalf("the latent has %d target slots, want one", len(lat.Targets))
	}
	tg := lat.Targets[0]
	if tg.Min.Kind != vConst || tg.Min.N != 0 {
		t.Fatalf("slot Min = %v, want const 0", tg.Min)
	}
	if tg.Max.Kind != vTriggerAmount {
		t.Fatalf("slot Max = %v, want the captured trigger amount", tg.Max)
	}
	if tg.Divided != kop.DivValue || tg.DividedV.Kind != vTriggerAmount {
		t.Fatalf("slot divided = %d %v, want DivValue over the captured trigger amount", tg.Divided, tg.DividedV)
	}
	if tg.Zone != zBattlefield {
		t.Fatalf("slot zone = %d, want the battlefield", tg.Zone)
	}
}

// progDump renders a program for a failure message.
func progDump(prog []lInstr) string {
	var parts []string
	for _, in := range prog {
		parts = append(parts, in.String())
	}
	return "[" + strings.Join(parts, "; ") + "]"
}
