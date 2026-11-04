package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed mirror matchups over the brazenbluff pool.
// Brazen Bluff targets an opponent's noncreature spell (its head is a Pump
// that only carries the spell target); that spell's controller may exile it
// (an optional ChangeZone Origin$ Stack asked of the non-active player at
// times, remembering the exiled spell), and if nothing was exiled the caster
// may cast a noncreature spell from hand without paying its mana cost. The
// deck holds creature spells as illegal targets and creature cards in hand
// that the free cast must not offer.

var brazenBluffDeck = []cardset.Entry{
	{Name: "Brazen Bluff", Count: 5}, {Name: "Shock", Count: 5}, {Name: "Lightning Bolt", Count: 3},
	{Name: "Opt", Count: 3}, {Name: "Divination", Count: 2},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
	{Name: "Mountain", Count: 12}, {Name: "Island", Count: 4},
}

type bluffTally struct {
	cast         int // Brazen Bluff casts begun
	yes, no      int // the spell controller's answers to the exile ask
	askedOpp     int // exile asks put to a player who is not the Bluff's caster
	free         int // free casts begun after a decline
	freeOffered  int // free-cast asks posed
	creatureHand int // free-cast asks with a creature card in the caster's hand
	creatureOnly int // priority asks with only creature spells on the stack, a Bluff in hand and no cast of it offered
}

func bluffPolicy(seed uint64, t *bluffTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate:
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) > 0 {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		} else {
			a = rnd.Pick(c)
		}
		if c.G != nil {
			bluffCount(c, a, t)
		}
		return a
	})
}

func bluffCount(c *PickCtx, a Canon, t *bluffTally) {
	pd := c.G.Pending()
	if pd == nil {
		return
	}
	g := c.G.E.G
	src := ""
	if o := g.Obj(pd.Source); o != nil && o.Face() != nil {
		src = o.Face().Name
	}
	opt, ok := c.G.GorgeOption(a)
	if pd.Kind == decision.KPriority && bluffCreatureOnly(g, pd) {
		t.creatureOnly++
	}
	switch {
	case pd.Kind == decision.KPriority && ok && opt.Kind == "cast":
		if o := g.Obj(opt.Obj); o != nil && o.Face() != nil && o.Face().Name == "Brazen Bluff" {
			t.cast++
		}
	case pd.ResumeKind == "changezone_optional" && ok:
		switch opt.Kind {
		case "yes":
			t.yes++
		case "no":
			t.no++
		}
		for _, id := range g.Stack {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Brazen Bluff" && o.Controller != state.PlayerID(pd.Player) {
				t.askedOpp++
				break
			}
		}
	case pd.ResumeKind == "play" && src == "Brazen Bluff":
		t.freeOffered++
		for _, id := range g.Zone(state.ZHand, state.PlayerID(pd.Player)) {
			if o := g.Obj(id); o != nil && o.Face() != nil && (o.Face().Name == "Grizzly Bears" || o.Face().Name == "Hill Giant") {
				t.creatureHand++
				break
			}
		}
		if ok && opt.Kind == "mode" {
			t.free++
		}
	}
}

// bluffCreatureOnly: the stack holds creature spells only, the asked seat
// holds a Brazen Bluff, and no option casts it (a creature spell is no legal
// target).
func bluffCreatureOnly(g *state.Game, pd *decision.Decision) bool {
	if len(g.Stack) == 0 {
		return false
	}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || (o.Face().Name != "Grizzly Bears" && o.Face().Name != "Hill Giant") {
			return false
		}
	}
	for _, o := range pd.Options {
		if o.Kind == "cast" {
			if x := g.Obj(o.Obj); x != nil && x.Face() != nil && x.Face().Name == "Brazen Bluff" {
				return false
			}
		}
	}
	for _, id := range g.Zone(state.ZHand, state.PlayerID(pd.Player)) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Brazen Bluff" {
			return true
		}
	}
	return false
}

func bluffSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, [2][]cardset.Entry) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("brazenbluff")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = brazenBluffDeck
		for _, e := range brazenBluffDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	return reg, set, decks, ents
}

// bluffRun plays seeds 1..n and returns the number of divergences (the first
// is logged) and stalls.
func bluffRun(t *testing.T, n uint64, tally *bluffTally, stopOnDiv bool) (div, stalls int) {
	t.Helper()
	reg, set, decks, ents := bluffSetup(t)
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"bluff-a", "bluff-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, bluffPolicy(seed, tally), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div++
			if div == 1 {
				t.Logf("seed %d diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if stopOnDiv {
				return div, stalls
			}
		}
	}
	return div, stalls
}

func TestBrazenBluffLockstep(t *testing.T) {
	t.Parallel()
	tally := &bluffTally{}
	div, _ := bluffRun(t, 300, tally, false)
	if div != 0 {
		t.Errorf("%d divergent games", div)
	}
	t.Logf("%+v", *tally)
	if tally.cast < 30 {
		t.Errorf("Brazen Bluff cast %d times, want at least 30", tally.cast)
	}
	if tally.yes < 5 || tally.no < 5 {
		t.Errorf("exile answers yes %d no %d, want at least 5 of each", tally.yes, tally.no)
	}
	if tally.askedOpp != tally.yes+tally.no {
		t.Errorf("%d of %d exile asks went to the Bluff's own caster", tally.yes+tally.no-tally.askedOpp, tally.yes+tally.no)
	}
	if tally.freeOffered < 5 || tally.free < 3 {
		t.Errorf("free-cast asks %d, free casts %d, want at least 5 and 3", tally.freeOffered, tally.free)
	}
	if tally.creatureHand < 1 {
		t.Errorf("no free-cast ask with a creature card in hand")
	}
	if tally.creatureOnly < 20 {
		t.Errorf("priority asks over a creature-only stack with a Bluff in hand: %d, want at least 20", tally.creatureOnly)
	}
}

// TestBrazenBluffMutation lets the kernel cast even when the spell was
// exiled (the exiled spell is not remembered) and requires a divergence.
func TestBrazenBluffMutation(t *testing.T) {
	dirac.DebugBluffNoRemember = true
	defer func() { dirac.DebugBluffNoRemember = false }()
	if div, _ := bluffRun(t, 300, &bluffTally{}, true); div == 0 {
		t.Errorf("mutation: no divergence in 300 games")
	}
}
