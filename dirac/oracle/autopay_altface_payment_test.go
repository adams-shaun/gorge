package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// altFacePaymentDecks is a small constructed matchup over the faces table: an
// Adventure card is cast from the hand, so the autopay offer builder prices
// the alternate spell face (decision.PlannedCast.CastMode "adventure_alt")
// beside the plain casts.
var altFacePaymentDecks = [2][]cardset.Entry{{
	{Name: "Beanstalk Giant", Count: 4}, {Name: "Rimrock Knight", Count: 4},
	{Name: "Curious Pair", Count: 2}, {Name: "Garenbrig Carver", Count: 2},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
}, {
	{Name: "Beanstalk Giant", Count: 4}, {Name: "Rimrock Knight", Count: 4},
	{Name: "Curious Pair", Count: 2}, {Name: "Garenbrig Carver", Count: 2},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
}}

// TestAutopayAltFacePaymentIsKnown pins that an alternate-spell-face cast the
// autopay planner prices has a canonical action: the offered "cast" option
// projects as ActCastFace, and the planner-paid loop must answer the same way
// instead of counting "priority/payment/hand" as an unknown option kind
// (dr-mbc-bbbb pair 98, the autopay surface).
func TestAutopayAltFacePaymentIsKnown(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("faces")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range altFacePaymentDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d has %d cards, want 40", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(altFacePaymentDecks[0], altFacePaymentDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: true}
	planned := 0
	for seed := uint64(1); seed <= 8; seed++ {
		m, err := NewMatchup(reg, [2]string{"altface-payment-a", "altface-payment-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30}
		rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
			d := ref.E.Pending()
			if d == nil || d.Kind != decision.KPriority {
				return nil
			}
			for _, pa := range ref.E.EnsurePaymentActions() {
				if pa.Cast.CastMode() == "adventure_alt" {
					planned++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, faceCastPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d diverged at decision %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if len(res.Stats.Unknown) != 0 {
			t.Errorf("seed %d: unknown option kinds %v", seed, res.Stats.Unknown)
		}
	}
	// The precondition the assertion rests on: at least one priority really
	// priced an Adventure face, or the game never exercised the mapping.
	if planned == 0 {
		t.Fatal("the run never planned an alternate-face cast; the test pins nothing")
	}
}
