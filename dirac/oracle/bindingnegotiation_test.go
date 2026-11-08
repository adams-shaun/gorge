package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Binding Negotiation's second half is a game-wide hidden pick out of exile
// (ChangeZone Origin$ Exile Destination$ Graveyard Hidden$ True):
// ChooseMove from=Exile|Confirm. The kernel's exwPickCands walks seat 0's
// then seat 1's exile, matching each card with its own owner, which is
// gorge effHiddenPick's gameWide walk. Seat A casts the card; seat B owns
// the face-up exiled cards (its plotted black/white/red plot creatures).
var bindingDecks = [2][]cardset.Entry{
	{
		{Name: "Binding Negotiation", Count: 8},
		{Name: "Swamp", Count: 32},
	},
	{
		{Name: "Blacksnag Buzzard", Count: 4},
		{Name: "Dust Animus", Count: 4},
		{Name: "Cunning Coyote", Count: 4},
		{Name: "Swamp", Count: 10},
		{Name: "Plains", Count: 9},
		{Name: "Mountain", Count: 9},
	},
}

// bindingTally counts the card's two arms from gorge's event log. The
// cursor resets with a rebuilt side, as kotjTally's.
type bindingTally struct {
	evAt        int
	lastResolve string
	sawDiscard  bool
	sawConfirm  bool
	resolutions int
	discards    int
	moves       int
	confirmYes  int
	confirmNo   int
	eq0         int
	moveNames   map[string]int
}

func (p *bindingTally) reset() {
	p.evAt = 0
	p.lastResolve = ""
	p.sawDiscard, p.sawConfirm = false, false
}

func (p *bindingTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.reset()
	}
	if p.moveNames == nil {
		p.moveNames = map[string]int{}
	}
	name := func(id state.ObjID) string {
		if o := g.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	for i := p.evAt; i < len(evs); i++ {
		e := &evs[i]
		switch e.Kind {
		case events.Resolve:
			p.lastResolve = e.Text
			if e.Text == "Binding Negotiation" {
				p.resolutions++
				p.sawDiscard, p.sawConfirm = false, false
			}
		case events.MoveZone:
			if e.From == state.ZStack && e.To != state.ZStack && name(e.Obj) == "Binding Negotiation" {
				// the spell leaves the stack: this resolution's discard
				// arm (if any) saw no confirmation
				if p.sawDiscard && !p.sawConfirm {
					p.eq0++
				}
				p.sawDiscard, p.sawConfirm = false, false
				continue
			}
			if p.lastResolve != "Binding Negotiation" {
				continue
			}
			o := g.Obj(e.Obj)
			if o == nil || o.Owner != 1 {
				continue
			}
			switch {
			case e.From == state.ZHand && e.To == state.ZGraveyard:
				p.discards++
				p.sawDiscard = true
			case e.From == state.ZExile && e.To == state.ZGraveyard:
				p.moves++
				p.moveNames[name(e.Obj)]++
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// bindingPolicy prefers seat A's cast of Binding Negotiation and seat B's
// plot special action, alternating the Discard may-choice between taking
// and declining and the hidden pick's confirmation between yes and no.
type bindingPolicy struct {
	rnd        Policy
	tally      *bindingTally
	take       bool
	confirmYes bool
}

func (p *bindingPolicy) Pick(c *PickCtx) Canon {
	g := c.G
	if g == nil {
		return p.rnd.Pick(c)
	}
	if d := g.Pending(); d != nil {
		src := ""
		if o := g.E.G.Obj(d.Source); o != nil && o.Face() != nil {
			src = o.Face().Name
		}
		switch d.ResumeKind {
		case "discard":
			if src == "Binding Negotiation" && d.Min == 0 {
				if p.take {
					for _, a := range c.D.Options {
						if o, ok := g.GorgeOption(a); ok && o.Kind == "discard" {
							p.take = false
							return a
						}
					}
				}
				p.take = true
				for _, a := range c.D.Options {
					if a.Kind() == ActDone {
						return a
					}
				}
			}
		case "hidden_pick_confirm":
			if src == "Binding Negotiation" {
				p.tally.sawConfirm = true
				want := "no"
				if p.confirmYes {
					want = "yes"
				}
				p.confirmYes = !p.confirmYes
				for _, a := range c.D.Options {
					if o, ok := g.GorgeOption(a); ok && o.Kind == want {
						if want == "yes" {
							p.tally.confirmYes++
						} else {
							p.tally.confirmNo++
						}
						return a
					}
				}
			}
		}
	}
	for _, a := range c.D.Options {
		o, ok := g.GorgeOption(a)
		if !ok || o.Kind != "cast" {
			continue
		}
		if o.Mode == "plot" {
			return a // seat B's plot special action
		}
		if ob := g.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Binding Negotiation" {
			return a
		}
	}
	return p.rnd.Pick(c)
}

func TestBindingNegotiationExileReturnLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set := cardsettest.Synth(t, "std-otj")
	decks := kcastDeck(t, reg, bindingDecks)
	tbl, _, _, err := set.Matchup(bindingDecks[0], bindingDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var p bindingTally
	p.reset()
	games, divs, stalls := 0, 0, 0
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"binding-a", "binding-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, &bindingPolicy{rnd: RandomPolicy(seed), tally: &p},
			RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: p.check})
		CloseSide(k)
		games++
		if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
			// a turn or decision cap is the run's bound; a kernel stall or
			// a gorge trigger cap is what the lockstep must not see
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d Binding Negotiation resolutions, %d discards, %d exile->graveyard moves %v, %d confirmations (%d yes, %d no), %d discard resolutions with no confirmation",
		games, p.resolutions, p.discards, p.moves, p.moveNames, p.confirmYes+p.confirmNo, p.confirmYes, p.confirmNo, p.eq0)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if p.moves < 5 {
		t.Errorf("%d Binding Negotiation exile->graveyard moves of seat B's card, want at least 5", p.moves)
	}
	if p.confirmNo < 5 {
		t.Errorf("%d hidden_pick_confirm answers of no, want at least 5", p.confirmNo)
	}
	if p.eq0 < 5 {
		t.Errorf("%d Binding Negotiation resolutions discarded a card and posed no confirmation, want at least 5", p.eq0)
	}
}
