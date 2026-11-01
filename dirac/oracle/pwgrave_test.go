package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The "target player or planeswalker" and YouCtrl-graveyard-pick lockstep
//   - Boros Charm's DealDmg mode and Viashino Pyromancer's enters trigger
//     (ValidTgts$ Player,Planeswalker): a player or a planeswalker, never a
//     creature;
//   - Another Chance's "return up to two creature cards from your graveyard"
//     (ChangeType$ Creature.YouCtrl, a hidden pick with no owner word);
//   - Deadly Brew's "return another permanent card from your graveyard"
//     (ChangeType$ Permanent.YouCtrl+!IsRemembered).
//
// The expected answer at every decision is gorge's.

var kpwgraveDecks = [2][]cardset.Entry{
	// A: the player-or-planeswalker burn, two cheap planeswalkers to aim it
	// at, the two graveyard picks, creatures of each requested colour and a
	// self-mill spell so the graveyard holds creatures.
	{
		{Name: "Boros Charm", Count: 4}, {Name: "Viashino Pyromancer", Count: 4},
		{Name: "Ajani, Caller of the Pride", Count: 2}, {Name: "Kaito, Cunning Infiltrator", Count: 2},
		{Name: "Another Chance", Count: 4}, {Name: "Deadly Brew", Count: 2},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Walking Corpse", Count: 2}, {Name: "Thought Scour", Count: 2},
		{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3}, {Name: "Swamp", Count: 3},
		{Name: "Island", Count: 2}, {Name: "Forest", Count: 2},
	},
	// B: the same mechanisms with the graveyard payoffs weighted up.
	{
		{Name: "Boros Charm", Count: 3}, {Name: "Viashino Pyromancer", Count: 3},
		{Name: "Ajani, Caller of the Pride", Count: 2}, {Name: "Kaito, Cunning Infiltrator", Count: 2},
		{Name: "Another Chance", Count: 3}, {Name: "Deadly Brew", Count: 3},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Walking Corpse", Count: 2}, {Name: "Thought Scour", Count: 2},
		{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3}, {Name: "Swamp", Count: 3},
		{Name: "Island", Count: 2}, {Name: "Forest", Count: 3},
	},
}

// kpwgraveStats counts the resolutions the pool must reach. dmg is a landed
// noncombat damage event from Boros Charm or Viashino Pyromancer; dmgPw is
// one whose recipient was a planeswalker. another and brew count cards
// returned from the graveyard to hand by those spells.
type kpwgraveStats struct {
	dmg, dmgPw, another, brew int
}

// kpwgraveTally reads the gorge event log during the game (the objects are
// still live) and attributes a graveyard-to-hand move to the last resolved
// source.
type kpwgraveTally struct {
	evAt int
	src  string
}

func (p *kpwgraveTally) scan(ref *GorgeSide, st *kpwgraveStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Resolve:
			p.src = chName(g, e.Obj)
		case events.DamageProvenance:
			if e.Amount <= 0 || strings.HasPrefix(e.Text, events.DamageProvenanceCombat) {
				continue
			}
			switch chName(g, e.Obj) {
			case "Boros Charm", "Viashino Pyromancer":
				st.dmg++
				if len(e.IDs) > 0 {
					if _, isPlayer := e.IDs[0].PlayerRef(); !isPlayer {
						if o := g.Obj(e.IDs[0]); o != nil && o.Face() != nil && o.Face().IsPlaneswalker() {
							st.dmgPw++
						}
					}
				}
			}
		case events.MoveZone:
			if e.From != state.ZGraveyard || e.To != state.ZHand {
				continue
			}
			switch p.src {
			case "Another Chance":
				st.another++
			case "Deadly Brew":
				st.brew++
			}
		}
	}
	p.evAt = len(evs)
}

// kpwgravePolicy develops mana and casts (the carriers are reached), picks
// Boros Charm's DealDmg mode, aims a player-or-planeswalker target at a
// planeswalker whenever one is offered, and feeds the event log to the tally.
func kpwgravePolicy(seed uint64, st *kpwgraveStats, tl *kpwgraveTally) Policy {
	cast := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		switch c.D.Kind {
		case DTarget:
			if c.G != nil {
				for _, a := range c.D.Options {
					if a.Kind() != ActTarget || int(a.Tgt()) >= int(RefPlayer) {
						continue
					}
					if o := c.G.E.G.Obj(c.G.id.obj[a.Tgt()]); o != nil && o.Face() != nil && o.Face().IsPlaneswalker() {
						return a
					}
				}
			}
		case DModes:
			// Boros Charm is the pool's only modal card: mode 0 is DealDmg.
			for _, a := range c.D.Options {
				if a.Kind() == ActMode && a.Mode() == 0 {
					return a
				}
			}
		}
		return cast.Pick(c)
	})
}

// runPwgrave plays the matchup for seeds games and returns the stats,
// divergences and kernel capacity stalls. mutate makes the kernel's TgtAny
// object branch ignore its filter; the lockstep must then diverge.
func runPwgrave(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kpwgraveStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("pwgrave")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kpwgraveDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	old := dirac.DebugTgtAnyNoFilter
	dirac.DebugTgtAnyNoFilter = mutate
	defer func() { dirac.DebugTgtAnyNoFilter = old }()
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		tbl, _, _, err := set.Matchup(kpwgraveDecks[0], kpwgraveDecks[1])
		if err != nil {
			t.Fatal(err)
		}
		m, err := NewMatchup(reg, [2]string{"kpwgrave-a", "kpwgrave-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		var tl kpwgraveTally
		res := LockstepLed(m, seed, g, k, kpwgravePolicy(seed, &st, &tl), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		tl.scan(g, &st)
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, st
}

func TestPwgraveLockstep(t *testing.T) {
	t.Parallel()
	games, divs, stalls, st := runPwgrave(t, 300, false)
	t.Logf("%d games, %d divergences, %d kernel stalls", games, divs, stalls)
	t.Logf("Boros Charm/Viashino damage resolutions %d (planeswalker %d); Another Chance returns %d; Deadly Brew returns %d",
		st.dmg, st.dmgPw, st.another, st.brew)
	if divs != 0 || stalls != 0 {
		t.Fatalf("lockstep: %d divergences, %d kernel stalls", divs, stalls)
	}
	if st.dmg < 20 {
		t.Fatalf("player-or-planeswalker damage resolutions %d, want >= 20", st.dmg)
	}
	if st.dmgPw < 3 {
		t.Fatalf("planeswalker damage resolutions %d, want >= 3", st.dmgPw)
	}
	if st.another < 10 {
		t.Fatalf("Another Chance returns %d, want >= 10", st.another)
	}
	if st.brew < 5 {
		t.Fatalf("Deadly Brew returns %d, want >= 5", st.brew)
	}
}

// TestPwgraveMutation proves the pool exercises the filtered any-target
// shape: with the kernel's TgtAny object branch ignoring its filter, a
// creature becomes a legal target beside the players, and some seed diverges.
func TestPwgraveMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	_, divs, _, st := runPwgrave(t, 128, true)
	t.Logf("mutation: %d divergences; %+v", divs, st)
	if divs == 0 {
		t.Fatal("no seed diverged with the TgtAny object filter dropped: the pool does not exercise the filtered any-target")
	}
}
