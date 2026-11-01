package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: when deciding whether
// an entry rider applies, an entering permanent's own statics count (CR
// 614.12). A second Metallic Mimic naming the same type gets the first
// Mimic's +1/+1 counter; Hardened Scales and Kami of Whispered Hopes and
// Endless One's X put the counters the rider adds to.
//
// Two Mimics only: at three or more copies the pre-existing board-capacity
// kernel stalls pass 16 of 128 (45 of 128 at six copies, at both pins).
var mimicDeck = []cardset.Entry{
	{Name: "Metallic Mimic", Count: 2}, {Name: "Endless One", Count: 5},
	{Name: "Hardened Scales", Count: 4}, {Name: "Kami of Whispered Hopes", Count: 3},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Silvercoat Lion", Count: 4},
	{Name: "Forest", Count: 10}, {Name: "Plains", Count: 8},
}

// mimicCounter counts the counters a permanent enters with (a CounterChange
// within 8 events after its MoveZone to the battlefield) while a Metallic
// Mimic is on the battlefield, from gorge's log at each agreed decision.
type mimicCounter struct {
	evAt, entries int
}

func (c *mimicCounter) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
	}
	mimic := false
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Metallic Mimic" {
				mimic = true
			}
		}
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.CounterChange || e.Amount <= 0 {
			continue
		}
		for k := j - 1; k >= 0 && k >= j-8; k-- {
			if m := &evs[k]; m.Kind == events.MoveZone && m.Obj == e.Obj && m.To == state.ZBattlefield {
				if mimic {
					c.entries++
				}
				break
			}
		}
	}
	c.evAt = len(evs)
	return nil
}

func TestMimicLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mimic")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range mimicDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	decks := [2][]*cards.Card{deck, deck}
	tbl, _, _, err := set.Matchup(mimicDeck, mimicDeck)
	if err != nil {
		t.Fatal(err)
	}
	var cnt mimicCounter
	stalls, divs := 0, 0
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mimic-a", "mimic-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := Policy(RandomPolicy(seed))
		if seed%2 == 1 {
			pol = HeuristicPolicy(seed)
		}
		cnt.evAt = 0
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25, Check: cnt.check})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("128 seeds, %d divergences, %d kernel stalls, %d counter-bearing entries beside a Metallic Mimic", divs, stalls, cnt.entries)
	if divs > 0 {
		return
	}
	if cnt.entries < 40 {
		t.Errorf("counter-bearing entries beside a Mimic: %d, want at least 40", cnt.entries)
	}
	if stalls > 16 {
		t.Errorf("kernel stalls: %d, want at most 16 of 128", stalls)
	}
}
