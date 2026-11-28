package kgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMutateLowering covers K:Mutate and Mode$ Mutates: every corpus card that
// prints K:Mutate (but the two still refused for another reason) lowers with
// the cost and target set, the named refusals still refuse, and Essence
// Symbiote's lord-shape Mutates trigger lowers over the event's object.
func TestMutateLowering(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	// Brokkos' "Spell.Mutate" ValidSA and Porcuparrot's Count$TimesMutated are
	// out of this ticket's scope: they stay refused for those other reasons,
	// not for the keyword.
	skips := []string{"Brokkos, Apex of Forever", "Porcuparrot"}
	seenSkip := map[string]bool{}
	cardsWithMutate := 0
	for _, card := range reg.AllCards() {
		prints := false
		for _, face := range card.Faces {
			for _, k := range face.Keywords {
				if cards.KeywordHead(k) == "Mutate" {
					prints = true
				}
			}
		}
		if !prints {
			continue
		}
		cardsWithMutate++
		name := card.Faces[0].Name
		if name == skips[0] || name == skips[1] {
			seenSkip[name] = true
			continue
		}
		p := &pool{byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, name, card, false)
		for _, r := range lc.Refusals {
			if r.Reason == `keyword "Mutate"` {
				t.Errorf("%s: %s", name, r.Reason)
			}
		}
		if lc.Mutate == nil || lc.MutateT == nil {
			t.Errorf("%s: Mutate=%v MutateT=%v, want both set", name, lc.Mutate, lc.MutateT)
		}
	}
	// Precondition: the corpus really is the wave-2 set this test measures.
	if cardsWithMutate != 34 {
		t.Fatalf("corpus cards printing K:Mutate = %d, want 34", cardsWithMutate)
	}
	for _, name := range skips {
		if !seenSkip[name] {
			t.Fatalf("%s no longer prints K:Mutate; the skip list is stale", name)
		}
	}

	t.Run("named refusals", func(t *testing.T) {
		const probe = `Name:Mutate Probe
ManaCost:1 G
Types:%s
PT:2/2
%s
Oracle:probe
`
		for _, tc := range []struct {
			name    string
			types   string
			keyword string
			want    string
		}{
			{"x cost", "Creature Beast", "K:Mutate:X", `mutate cost "X"`},
			{"non-creature", "Enchantment", "K:Mutate:1 G", "mutate on a non-creature"},
			{"beside bestow", "Enchantment Creature Beast", "K:Bestow:2 W\nK:Mutate:1 G", "mutate beside another alternative cost"},
		} {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				lc := lowerProbe(t, fmt.Sprintf(probe, tc.types, tc.keyword), "Mutate Probe", "")
				if got := probeRefusals(lc); !strings.Contains(got, tc.want) {
					t.Fatalf("refusals = %q, want %q", got, tc.want)
				}
			})
		}

		// The other ValidCard$ shape has no kernel read: only Card.Self and
		// Creature.YouCtrl lower.
		lc := lowerProbe(t, `Name:Mutate Trigger Probe
ManaCost:1 G
Types:Creature Beast
PT:2/2
T:Mode$ Mutates | ValidCard$ Creature.OppCtrl | Execute$ ProbeDraw7 | TriggerDescription$ probe
SVar:ProbeDraw7:DB$ Draw | NumCards$ 7 | Defined$ You
Oracle:probe
`, "Mutate Trigger Probe", "")
		if got := probeRefusals(lc); !strings.Contains(got, `Mutates ValidCard$ "Creature.OppCtrl"`) {
			t.Fatalf("refusals = %q, want the Mutates ValidCard$ refusal", got)
		}
	})

	t.Run("lord shape", func(t *testing.T) {
		card, ok := reg.Lookup("Essence Symbiote")
		if !ok || card == nil {
			t.Fatal("Essence Symbiote not in the corpus")
		}
		// Precondition: the trigger is the Creature.YouCtrl shape, not the
		// Card.Self one, so a pass cannot come from the self arm.
		var line cards.Trigger
		found := false
		for _, tr := range card.Faces[0].Triggers {
			if tr.Mode == "Mutates" {
				line, found = tr, true
			}
		}
		if !found || line.ParamStr(cards.PKValidCard) != "Creature.YouCtrl" {
			t.Fatalf("Essence Symbiote Mutates trigger = %+v, want ValidCard$ Creature.YouCtrl", line)
		}
		if eventMap["Mutates"] != kop.TrigMutates {
			t.Fatalf("eventMap[Mutates] = %v, want kop.TrigMutates", eventMap["Mutates"])
		}
		name := card.Faces[0].Name
		p := &pool{byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, name, card, false)
		var got *lTrigger
		for i := range lc.Abilities {
			if tr := lc.Abilities[i].Trigger; tr != nil && tr.Event == "Mutates" {
				got = tr
			}
		}
		if got == nil {
			t.Fatalf("Essence Symbiote trigger did not lower: %v", lc.Refusals)
		}
		// The lord shape matches the event's object (the pile) with the
		// listener's `you`, never the self flag.
		if len(got.F.Clauses) != 1 || got.F.Clauses[0].Types != tCreature || got.F.Clauses[0].Ctrl != whoYou {
			t.Fatalf("lord-shape filter = %+v, want one Creature.YouCtrl clause", got.F)
		}
	})
}
