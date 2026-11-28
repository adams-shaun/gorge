package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// A player who has left the game is asked no unless cost (CR 800.4a, 800.4f).
// In the Miss Highwater pod (miss_highwater_pods_mp_test.go) seeds 34 and 36
// have Miss Highwater's combat damage take a seat to 0 life: the seat loses
// to state-based actions before the trigger resolves, so its "discard your
// hand" unless (UnlessPayer$ TriggeredTarget | UnlessSwitched$ True) has a
// departed payer. Gorge (effects/unless.go unlessPayerTargetsFor) skips the
// Lost seat and takes the decline orientation; the kernel's execPayer
// (dirac/cost_departedpayer.go) maps the departed seat to no payer. The two
// must agree decision for decision.

// departedUnlessPayers counts the seats that lost the game while a Miss
// Highwater trigger waited on the stack, and the mode choices such a seat
// made afterwards (CR 800.4f: none).
func departedUnlessPayers(g *GorgeSide) (departed, askedAfter int) {
	var gone [MaxSeats]bool
	pending := 0 // Miss Highwater triggers pushed and not yet resolved
	for i := range g.E.L.Events {
		switch e := &g.E.L.Events[i]; e.Kind {
		case events.TriggerPush:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Miss Highwater" {
				pending++
			}
		case events.AbilityResolved:
			if pending > 0 {
				pending--
			}
		case events.PlayerLost:
			if pending > 0 && int(e.Player) < MaxSeats {
				gone[e.Player] = true
				departed++
			}
		case events.ModeChosen:
			if int(e.Player) < MaxSeats && gone[e.Player] {
				askedAfter++
			}
		}
	}
	return departed, askedAfter
}

func TestFourSeatUnlessDepartedPayerLockstep(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 4)
	gd := make([][]*cards.Card, 4)
	md := make([][]cardset.Entry, 4)
	var all []string
	for s, d := range missHighwaterPodDecks {
		names[s] = "highwater-pod"
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
	departed := 0
	for _, seed := range []uint64{34, 36} {
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
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 100})
		CloseSide(k)
		n, asked := departedUnlessPayers(g)
		departed += n
		if asked != 0 {
			t.Errorf("seed %d: gorge asked a departed seat %d mode choice(s) (CR 800.4f)", seed, asked)
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Stall != "" {
			t.Errorf("seed %d: stall %q", seed, res.Stall)
		}
	}
	// the precondition: a payer left the game with the trigger on the stack
	if departed == 0 {
		t.Fatal("no seat lost the game while a Miss Highwater trigger waited: the departed-payer case never arose")
	}
}
