package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// paidCountConspireCards are the corpus cards printing K:Conspire (CR 702.78a)
// that lower whole: the keyword's expansion reads Count$Conspired, which
// lowers to the kernel's paid count (vPaidN), beside CardDef.Conspire. Traitor's
// Roar (DamageSource$ to TargetController) and Giantbaiting (its token script
// needs the pool's token tables) print it too but lower no further for a reason
// of their own.
var paidCountConspireCards = []string{
	"Barkshell Blessing", "Burn Trail", "Memory Sluice", "Mine Excavation", "Aethertow",
	"Disturbing Plot", "Rally the Galadhrim", "Ghastly Discovery", "Gleeful Sabotage",
}

// TestPaidCountConspireLowers: a Conspire spell lowers with no refusal, builds
// CardDef.Conspire, and its expansion trigger's Count$Conspired reads vPaidN.
func TestPaidCountConspireLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range paidCountConspireCards {
		t.Run(name, func(t *testing.T) {
			lc := lowerRepeatedCard(t, reg, name)
			for _, rf := range lc.Refusals {
				t.Errorf("refused: %s", rf.Reason)
			}
			if !lc.Conspire {
				t.Fatal("lowered without Conspire")
			}
			_, all, pt, err := buildPool(&lowered{cards: []*lCard{lc}})
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			d := &all[0].def
			if !d.Conspire {
				t.Fatal("CardDef.Conspire not set")
			}
			found := false
			for _, e := range pt.exprs {
				if e.Kind == kop.ExKicked && e.N == 2 {
					found = true
				}
			}
			if !found {
				t.Fatal("no ExKicked N 2 (the paid count) read in the built card")
			}
		})
	}
}

// TestPaidCountRefusals: Count$Conspired, Count$SquadPaid and
// Count$ReplicatePaid read the paid count only on a card whose own keyword is
// the matching one; Conspire beside another cast route or cost refuses by name.
func TestPaidCountRefusals(t *testing.T) {
	probe := func(t *testing.T, name, script, want string) {
		t.Helper()
		card, diags := cards.ParseBytes("probe.txt", []byte(script))
		if card == nil {
			t.Fatalf("parse: %v", diags)
		}
		if d := card.Link(); len(d) != 0 {
			t.Fatalf("link: %v", d)
		}
		p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		lc := lowerPool(p).cards[0]
		for _, rf := range lc.Refusals {
			if strings.Contains(rf.Reason, want) {
				return
			}
		}
		t.Fatalf("refusals %v, want one containing %q", lc.Refusals, want)
	}
	t.Run("count without keyword", func(t *testing.T) {
		probe(t, "Paid Probe", "Name:Paid Probe\nManaCost:2 R\nTypes:Sorcery\nSVar:X:Count$Conspired\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ X\n", "Count$Conspired without the matching keyword")
	})
	t.Run("conspire beside casualty", func(t *testing.T) {
		probe(t, "Conspire Casualty Probe", "Name:Conspire Casualty Probe\nManaCost:2 R\nTypes:Sorcery\nK:Casualty:1\nK:Conspire\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\n", "Conspire beside another cast route or additional cost")
	})
}
