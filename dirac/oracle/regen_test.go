package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRegenCombatLockstep: CR 701.19b. A blocked attacker that regenerates
// is removed from combat (506.4) -- gorge emits EndCombatReset and the
// blocker's BlockedBy entry is tombstoned -- so the blocker deals no combat
// damage. The kernel's regenerate must leave the same state: the attacker
// stops attacking, is no longer blocked, and its blockers stay blocking but
// block nothing.
//
// The recipe: a Dragon-Scarred Bear attacks into a Grizzly Bears (so the
// Bear is blocked and the stale blocking pointer is exercised), then the
// Bear's controller casts Fated Conflagration at its own Bear in the declare
// blockers step. The 5 damage is lethal to the 3/2 Bear; the regeneration
// shield (activated the same step) replaces the destruction, taps the Bear
// and removes it from combat before combat damage. The Bear's regeneration
// is gated on Formidable, so deck A also plays a Kree Sentinel (3 + 5 = 8
// total power).
func TestRegenCombatLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("validaggs")
	if err != nil {
		t.Fatal(err)
	}
	entries := [2][]cardset.Entry{
		{
			{Name: "Dragon-Scarred Bear", Count: 4}, {Name: "Kree Sentinel", Count: 4},
			{Name: "Fated Conflagration", Count: 8}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 12},
		},
		{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Forest", Count: 16}, {Name: "Mountain", Count: 16},
		},
	}
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
	fired := 0
	for seed := uint64(1); seed <= 24; seed++ {
		m, err := NewMatchup(reg, [2]string{"regen-a", "regen-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		sc := &regenScenario{rnd: RandomPolicy(seed)}
		saw, resets := 0, 0
		alive := false
		check := func(g *GorgeSide, _ Side) *Divergence {
			evs := g.E.L.Events
			for _, e := range evs[saw:] {
				if e.Kind != events.EndCombatReset || e.Obj == 0 || e.Obj != sc.bearID {
					continue
				}
				resets++
				if o := g.E.G.Obj(e.Obj); o != nil && o.Zone == state.ZBattlefield && o.Damage == 0 {
					alive = true
				}
			}
			saw = len(evs)
			return nil
		}
		res := Lockstep(m, seed, k, sc, RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if sc.castConf == 0 {
			continue
		}
		fired++
		if sc.targeted == 0 {
			t.Fatalf("seed %d: Conflagration cast but the Bear was never targeted", seed)
		}
		if res.Div != nil {
			lo := res.Div.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Fatalf("seed %d: diverged after the regenerate at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
				actionStrings(res.Actions[lo:res.Div.At]), res.Div.RefOptions, res.Div.KerOptions)
		}
		if resets == 0 {
			t.Fatalf("seed %d: no gorge EndCombatReset for the regenerated Bear (bearID %d)", seed, sc.bearID)
		}
		if !alive {
			t.Fatalf("seed %d: the Bear is not on the battlefield with no damage after regenerating", seed)
		}
		t.Logf("seed %d: Conflagration cast and targeted at the Bear, %d EndCombatReset event(s), no divergence", seed, resets)
		break
	}
	if fired == 0 {
		t.Fatal("no seed fired the blocked-attacker regeneration scenario")
	}
}

// regenScenario is the deterministic policy for TestRegenCombatLockstep.
// Player 0 plays lands, keeps exactly one Dragon-Scarred Bear and one Kree
// Sentinel (Formidable: 8 total power) on the battlefield, attacks only
// with the Bear, activates its regeneration shield in the declare blockers
// step and then casts Fated Conflagration at it. Player 1 plays lands,
// casts Grizzly Bears and blocks the Bear with one. Every other decision
// falls back to the seeded random policy.
type regenScenario struct {
	rnd         Policy
	regenTurn   int32
	fired       bool
	wantTarget  bool
	blockPicked bool
	bearID      state.ObjID
	castConf    int
	targeted    int
}

func (p *regenScenario) Pick(c *PickCtx) Canon {
	d := c.D
	g := c.G.E.G
	switch d.Kind {
	case DTarget:
		if p.wantTarget && p.bearID != 0 {
			ref := c.G.InstRef(p.bearID)
			for _, a := range d.Options {
				if a.Kind() == ActTarget && a.Tgt() == ref {
					p.wantTarget = false
					p.targeted++
					return a
				}
			}
		}
		return p.rnd.Pick(c)
	case DMulligan:
		for _, a := range d.Options {
			if a.Kind() == ActKeep {
				return a
			}
		}
		return p.rnd.Pick(c)
	case DAttackers:
		p.blockPicked = false
		if d.Player == 0 {
			if bear := reproBattlefieldObj(g, "Dragon-Scarred Bear", 0); bear != nil {
				ref := c.G.InstRef(bear.ID)
				for _, a := range d.Options {
					if a.Kind() == ActAttack && a.Src() == ref {
						return a
					}
				}
			}
		}
		return pickKind(c, ActDone)
	case DBlockers:
		if d.Player == 1 && !p.blockPicked {
			if bear := reproBattlefieldObj(g, "Dragon-Scarred Bear", 0); bear != nil && bear.IsAttacking {
				ref := c.G.InstRef(bear.ID)
				for _, a := range d.Options {
					if a.Kind() == ActBlock && a.Tgt() == ref {
						p.blockPicked = true
						return a
					}
				}
			}
		}
		return pickKind(c, ActDone)
	case DPriority:
		return p.priority(c, g)
	}
	return p.rnd.Pick(c)
}

func (p *regenScenario) priority(c *PickCtx, g *state.Game) Canon {
	bear := reproBattlefieldObj(g, "Dragon-Scarred Bear", 0)
	if bear != nil && bear.IsAttacking && g.Step == state.StepDeclareBlockers && !p.fired {
		bearRef := c.G.InstRef(bear.ID)
		if bear.Counter("Shield") == 0 {
			if p.regenTurn != g.Turn {
				if a, ok := findAction(c, func(a Canon) bool {
					return a.Kind() == ActActivate && a.Src() == bearRef
				}); ok {
					p.regenTurn = g.Turn
					return a
				}
			}
		} else if a, ok := reproHandCastAction(c, g, 0, "Fated Conflagration"); ok {
			p.fired = true
			p.bearID = bear.ID
			p.wantTarget = true
			p.castConf++
			return a
		}
	}
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
		return a
	}
	if p.count(g, "Dragon-Scarred Bear", 0) == 0 {
		if a, ok := reproHandCastAction(c, g, 0, "Dragon-Scarred Bear"); ok {
			return a
		}
	}
	if p.count(g, "Kree Sentinel", 0) == 0 {
		if a, ok := reproHandCastAction(c, g, 0, "Kree Sentinel"); ok {
			return a
		}
	}
	if c.D.Player == 1 {
		if a, ok := reproHandCastAction(c, g, 1, "Grizzly Bears"); ok {
			return a
		}
	}
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPass }); ok {
		return a
	}
	return c.D.Options[0]
}

func (p *regenScenario) count(g *state.Game, name string, ctrl state.PlayerID) int {
	n := 0
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Controller != ctrl || o.Card == nil {
			continue
		}
		if f := o.Face(); f != nil && f.Name == name {
			n++
		}
	}
	return n
}

func findAction(c *PickCtx, match func(Canon) bool) (Canon, bool) {
	for _, a := range c.D.Options {
		if match(a) {
			return a, true
		}
	}
	return 0, false
}

// pickKind picks the first option of kind, or the first option when the
// decision has none (a mandatory pick).
func pickKind(c *PickCtx, kind CanonKind) Canon {
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == kind }); ok {
		return a
	}
	return c.D.Options[0]
}
