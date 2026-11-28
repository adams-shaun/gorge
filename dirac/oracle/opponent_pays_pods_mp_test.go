package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// "Any opponent may pay" (UnlessPayer$ Player.Opponent) in a four-seat pod:
// both engines ask the controller's opponents in APNAP order (CR 101.4) until
// one pays. Clackbridge Troll and Reservoir Kraken carry UnlessResolveSubs$
// WhenPaid, so a payment by a later payer must run the whole chain and a walk
// everybody declines must run none of it; Vexing Devil is the switched
// damage offer.

var opponentPaysPodDecks = [4][]cardset.Entry{
	{{Name: "Clackbridge Troll", Count: 10}, {Name: "Swamp", Count: 14}},
	{{Name: "Reservoir Kraken", Count: 8}, {Name: "Savannah Lions", Count: 4}, {Name: "Island", Count: 8}, {Name: "Plains", Count: 4}},
	{{Name: "Vexing Devil", Count: 10}, {Name: "Mountain", Count: 14}},
	{{Name: "Savannah Lions", Count: 10}, {Name: "Plains", Count: 14}},
}

// TestFourSeatOpponentPaysLockstep plays the pod over seeds in lockstep: no
// game diverges or stalls, and across them a payer pays after an earlier
// payer of the same walk declined.
func TestFourSeatOpponentPaysLockstep(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 4)
	gd := make([][]*cards.Card, 4)
	md := make([][]cardset.Entry, 4)
	var all []string
	for s, d := range opponentPaysPodDecks {
		names[s] = "opponent-pays-pod"
		md[s] = d
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			all = append(all, e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, [2][]string{all, nil}))
	tbl, _, err := set.MatchupN(md)
	if err != nil {
		t.Fatal(err)
	}
	// asks counts the unless asks; latePaid the payments made by a payer
	// asked after another declined the same source's walk.
	var asks, latePaid int
	for seed := uint64(1); seed <= 16; seed++ {
		cfg := Config{}
		m, err := NewMatchupN(reg, names, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		inner := castFirstPolicy(seed)
		var last state.ObjID // the source of the ask just declined
		pol := PolicyFunc(func(c *PickCtx) Canon {
			a := inner.Pick(c)
			d := c.G.Pending()
			if d == nil || d.ResumeKind != "unless_pay" {
				last = 0
				return a
			}
			asks++
			paid := false
			for i, o := range c.D.Options {
				if o == a && len(d.Options) == len(c.D.Options) && d.Options[i].Mode == decision.ModeUnlessPay {
					paid = true
				}
			}
			if paid && last != 0 && last == d.Source {
				latePaid++
			}
			last = d.Source
			if paid {
				last = 0
			}
			return a
		})
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 100})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Stall != "" {
			t.Fatalf("seed %d: stall %q", seed, res.Stall)
		}
	}
	t.Logf("%d unless asks, %d paid by a later payer", asks, latePaid)
	if latePaid == 0 {
		t.Fatalf("no payer paid after an earlier one declined (%d asks): the walk was not exercised", asks)
	}
}
