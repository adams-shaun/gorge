package oracle

import (
	"errors"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestFourSeatLifeReplOrderLockstep plays four-seat pods whose decks hold a
// life-gain doubler (Alhammarret's Archive) beside a life-gain plus-N (Pest
// Rescuer) and lone GainLife sources that resolve on the stack (Soul Warden,
// Essence Warden, Wellwisher's activation, Graypelt Refuge's and Radiant
// Fountain's enters triggers). A player who controls both is asked gorge's
// CR 616.1 order for each gain, applied as picked (CR 616.1e): the doubler
// first gives 2n+1, the plus-N first 2(n+1). Both orders are picked, no
// divergence and no stall, and with the picked order ignored
// (dirac.DebugLifeReplFixed) some seed diverges. Lifelink, extort, trigger
// batches and the other call sites that cannot suspend are not in these decks
// (they stall, StallLifeRepl: repl_lifeask.go).
func TestFourSeatLifeReplOrderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	var deck []string
	for _, e := range []cardset.Entry{
		{Name: "Pest Rescuer", Count: 4}, {Name: "Alhammarret's Archive", Count: 3}, {Name: "Soul Warden", Count: 3},
		{Name: "Essence Warden", Count: 3}, {Name: "Wellwisher", Count: 3}, {Name: "Graypelt Refuge", Count: 2},
		{Name: "Radiant Fountain", Count: 2}, {Name: "Sol Ring", Count: 1}, {Name: "Mind Stone", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Plains", Count: 8},
	} {
		deck = append(deck, deckOf(e.Count, e.Name)...)
	}
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	es := make([][]cardset.Entry, 4)
	for i := range es {
		es[i] = ReproEntries(reg, set, deck)
	}
	tbl, _, err := set.MatchupN(es)
	if err != nil {
		t.Fatal(err)
	}
	type tally struct{ asks, dblFirst, plusFirst int }
	play := func(seed uint64) (Result, tally) {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{deck, deck},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{deck, deck}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 6000, 40
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		var n tally
		inner := castFirstPolicy(seed)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			a := inner.Pick(c)
			if a.Kind() == ActReplacement && c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
						n.asks++
						if strings.HasPrefix(ob.Face().Name, "Alhammarret") {
							n.dblFirst++
						} else {
							n.plusFirst++
						}
					}
				}
			}
			return a
		})
		return Lockstep(m, seed, k, pol, rc), n
	}
	const seeds = 12
	var total tally
	for seed := uint64(1); seed <= seeds; seed++ {
		res, n := play(seed)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		total.asks += n.asks
		total.dblFirst += n.dblFirst
		total.plusFirst += n.plusFirst
		t.Logf("seed %d: %d decisions, over %v; %d order asks", seed, res.Decisions, res.Over, n.asks)
	}
	if total.dblFirst == 0 || total.plusFirst == 0 {
		t.Fatalf("order asks reached: %d doubler-first, %d plus-first picks; want both orders", total.dblFirst, total.plusFirst)
	}
	dirac.DebugLifeReplFixed = true
	defer func() { dirac.DebugLifeReplFixed = false }()
	for seed := uint64(1); seed <= seeds; seed++ {
		if res, _ := play(seed); res.Div != nil {
			t.Logf("mutation: seed %d diverges at %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("ignoring the picked CR 616.1 order diverged on no seed")
}
