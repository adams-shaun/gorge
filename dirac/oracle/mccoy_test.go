package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMcCoyLockstep: Dr. McCoy, Surly Surgeon replaces the destruction of
// another creature its controller controls that has a +1/+1 counter: remove
// all +1/+1 counters and heal all damage instead (CR 614.1). McCoy's enters
// trigger puts the counter on Emergency Medical Hologram, then the finisher
// destroys it: Planetary Bombardment is the lethal-damage SBA path and
// Impossible Choice is the destroy-effect path. The Hologram survives with no counters
// and no damage.
func TestMcCoyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, d := range []mccoyDeck{mccoyDecks("Planetary Bombardment"), mccoyDecks("Impossible Choice")} {
		t.Run(d.finisher, func(t *testing.T) {
			reg := synthReg(t)
			set := cardsettest.Synth(t, "std-trk")
			decks, tbl := mccoyBoard(t, reg, set, d.entries)
			fired := 0
			for seed := uint64(1); seed <= 32; seed++ {
				m, err := NewMatchup(reg, [2]string{"mccoy-a", "mccoy-b"}, decks, 0)
				if err != nil {
					t.Fatal(err)
				}
				cfg := Config{}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				sc := &mccoyScenario{finisher: d.finisher}
				tl := &mccoyTally{}
				res := Lockstep(m, seed, k, sc, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20, Check: tl.check})
				CloseSide(k)
				if sc.fired == 0 {
					continue
				}
				fired++
				if res.Div != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
						seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
						res.Div.RefOptions, res.Div.KerOptions)
				}
				if tl.healed == 0 {
					t.Fatalf("seed %d: %s resolved but no +1/+1 counter was removed", seed, d.finisher)
				}
				if tl.died != 0 {
					t.Fatalf("seed %d: the Hologram went to the graveyard despite McCoy's replacement", seed)
				}
				t.Logf("seed %d: counter removed and damage healed, no divergence", seed)
				break
			}
			if fired == 0 {
				t.Fatalf("no seed reached McCoy's destroy replacement with %s", d.finisher)
			}
		})
	}
}

// TestMcCoyMutation turns the replacement off in the kernel only: each
// scenario must diverge (the Hologram dies in the kernel, survives in gorge).
func TestMcCoyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugDestroyHealOff
	dirac.DebugDestroyHealOff = true
	defer func() { dirac.DebugDestroyHealOff = old }()
	for _, d := range []mccoyDeck{mccoyDecks("Planetary Bombardment"), mccoyDecks("Impossible Choice")} {
		t.Run(d.finisher, func(t *testing.T) {
			reg := synthReg(t)
			set := cardsettest.Synth(t, "std-trk")
			decks, tbl := mccoyBoard(t, reg, set, d.entries)
			for seed := uint64(1); seed <= 32; seed++ {
				m, err := NewMatchup(reg, [2]string{"mccoy-a", "mccoy-b"}, decks, 0)
				if err != nil {
					t.Fatal(err)
				}
				cfg := Config{}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				sc := &mccoyScenario{finisher: d.finisher}
				res := Lockstep(m, seed, k, sc, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20})
				CloseSide(k)
				if sc.fired == 0 {
					continue
				}
				if res.Div == nil {
					t.Fatalf("seed %d: mutation did not diverge", seed)
				}
				t.Logf("seed %d: mutation diverged at decision %d", seed, res.Div.At)
				return
			}
			t.Fatalf("no seed reached McCoy's destroy replacement with %s", d.finisher)
		})
	}
}

type mccoyDeck struct {
	finisher string
	entries  [2][]cardset.Entry
}

func mccoyDecks(finisher string) mccoyDeck {
	lands := []cardset.Entry{{Name: "Sacred Foundry", Count: 16}, {Name: "Plains", Count: 12}}
	if finisher == "Impossible Choice" {
		lands = []cardset.Entry{{Name: "Blood Crypt", Count: 8}, {Name: "Sacred Foundry", Count: 8}, {Name: "Plains", Count: 12}}
	}
	a := []cardset.Entry{
		{Name: "Emergency Medical Hologram", Count: 4},
		{Name: "Dr. McCoy, Surly Surgeon", Count: 4},
		{Name: finisher, Count: 4},
	}
	a = append(a, lands...)
	return mccoyDeck{finisher: finisher, entries: [2][]cardset.Entry{a, {{Name: "Mountain", Count: 40}}}}
}

func mccoyBoard(t *testing.T, reg *cards.Registry, set cardset.Set, entries [2][]cardset.Entry) ([2][]*cards.Card, *kop.Table) {
	t.Helper()
	var decks [2][]*cards.Card
	for s := range entries {
		for _, e := range entries[s] {
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
	tbl, _, _, err := set.Matchup(entries[0], entries[1])
	if err != nil {
		t.Fatal(err)
	}
	return decks, tbl
}

type mccoyScenario struct {
	finisher string
	fired    int
}

func (p *mccoyScenario) Pick(c *PickCtx) Canon {
	d := c.D
	g := c.G.E.G
	switch d.Kind {
	case DMulligan:
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActKeep }); ok {
			return a
		}
		return c.D.Options[0]
	case DTarget:
		if hol := reproBattlefieldObj(g, "Emergency Medical Hologram", 0); hol != nil {
			ref := c.G.InstRef(hol.ID)
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActTarget && a.Tgt() == ref }); ok {
				return a
			}
		}
		return c.D.Options[0]
	case DModes:
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActMode && a.Mode() == 0 }); ok {
			return a
		}
		return c.D.Options[0]
	case DAttackers, DBlockers:
		return pickKind(c, ActDone)
	case DPriority:
		return p.priority(c, g)
	}
	return c.D.Options[0]
}

func (p *mccoyScenario) priority(c *PickCtx, g *state.Game) Canon {
	if c.D.Player != 0 {
		return pickKind(c, ActPass)
	}
	hol := reproBattlefieldObj(g, "Emergency Medical Hologram", 0)
	mccoy := reproBattlefieldObj(g, "Dr. McCoy, Surly Surgeon", 0)
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
		return a
	}
	if hol == nil {
		if a, ok := reproHandCastAction(c, g, 0, "Emergency Medical Hologram"); ok {
			return a
		}
	}
	if hol != nil && mccoy == nil {
		if a, ok := reproHandCastAction(c, g, 0, "Dr. McCoy, Surly Surgeon"); ok {
			return a
		}
	}
	if hol != nil && mccoy != nil && hol.Counter("P1P1") > 0 {
		if a, ok := reproHandCastAction(c, g, 0, p.finisher); ok {
			p.fired++
			return a
		}
	}
	return pickKind(c, ActPass)
}

type mccoyTally struct {
	ref    *GorgeSide
	evAt   int
	healed int
	died   int
}

func (t *mccoyTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if t.ref != ref || len(evs) < t.evAt {
		t.ref, t.evAt = ref, 0
	}
	for j := t.evAt; j < len(evs); j++ {
		e := &evs[j]
		o := g.Obj(e.Obj)
		if o == nil || o.Face() == nil || o.Face().Name != "Emergency Medical Hologram" {
			continue
		}
		switch {
		case e.Kind == events.CounterChange && e.Counter == "P1P1" && e.Amount < 0:
			t.healed++
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard:
			t.died++
		}
	}
	t.evAt = len(evs)
	return nil
}
