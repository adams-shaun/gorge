package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: gorge, CR 704.5p. An attached Equipment
// that becomes a creature (Ensouled Scimitar animates itself; Relic's Roar,
// Karn's Touch, Alloy Animist, Toymaker, I Am Iron Man, Tough Cookie and the
// Auras Ensoul Artifact, Zoetic Glyph and Mightstone's Animation animate
// another artifact) becomes unattached and stays on the battlefield.

var kunattachDeck = []cardset.Entry{
	{Name: "Ensouled Scimitar", Count: 4}, {Name: "Bonesplitter", Count: 2}, {Name: "Short Sword", Count: 1},
	{Name: "Vulshok Morningstar", Count: 1}, {Name: "Avacyn's Collar", Count: 2},
	{Name: "Relic's Roar", Count: 2}, {Name: "Karn's Touch", Count: 2}, {Name: "Alloy Animist", Count: 1},
	{Name: "Toymaker", Count: 1}, {Name: "Ensoul Artifact", Count: 1}, {Name: "Zoetic Glyph", Count: 1},
	{Name: "Mightstone's Animation", Count: 1}, {Name: "I Am Iron Man", Count: 1}, {Name: "Tough Cookie", Count: 1},
	{Name: "Grizzly Bears", Count: 1}, {Name: "Llanowar Elves", Count: 1}, {Name: "Memnite", Count: 1},
	{Name: "Ornithopter", Count: 1}, {Name: "Gold Myr", Count: 1},
	{Name: "Island", Count: 8}, {Name: "Forest", Count: 6},
}

const kunattachWhy = "attached creature or battle becomes unattached"

// kunattachCounts is read off gorge's finished event log.
type kunattachCounts struct {
	unattached         int // 704.5p unattaches of a creature
	selfAnim           int // ... where the object is Ensouled Scimitar
	otherAnim          int // ... where it is another Equipment
	auraAnim           int // ... an Equipment an animating Aura was attached to
	equipWhileAnimated int // an Attach of an Equipment unattached as a creature earlier and still on the battlefield
}

func kunattachFinish(c *kunattachCounts) func(*GorgeSide) {
	return func(g *GorgeSide) {
		auraOn := map[state.ObjID]state.ObjID{} // animating Aura -> the object it is attached to
		animated := map[state.ObjID]bool{}      // an Equipment unattached as a creature
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			o := g.E.G.Obj(e.Obj)
			name := ""
			if o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			switch {
			case e.Kind == events.Attach && len(e.IDs) > 0:
				switch name {
				case "Ensoul Artifact", "Zoetic Glyph", "Mightstone's Animation":
					auraOn[e.Obj] = e.IDs[0]
				default:
					if animated[e.Obj] {
						c.equipWhileAnimated++
					}
				}
			case e.Kind == events.MoveZone && e.From == state.ZBattlefield:
				delete(auraOn, e.Obj)
				delete(animated, e.Obj)
			case e.Kind == events.Unattached && e.Text == kunattachWhy:
				c.unattached++
				animated[e.Obj] = true
				if name == "Ensouled Scimitar" {
					c.selfAnim++
				} else {
					c.otherAnim++
				}
				for _, tgt := range auraOn {
					if tgt == e.Obj {
						c.auraAnim++
						break
					}
				}
			}
		}
	}
}

// kunattachRun plays seeds 1..seeds of the matchup (the deck on both seats)
// with the random policy; it returns whether one diverged and the kernel
// stalls.
func kunattachRun(t *testing.T, seeds uint64, report bool, finish func(*GorgeSide)) (diverged bool, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unattach")
	if err != nil {
		t.Fatal(err)
	}
	both := [2][]cardset.Entry{kunattachDeck, kunattachDeck}
	decks := kcastDeck(t, reg, both)
	tbl, _, _, err := set.Matchup(kunattachDeck, kunattachDeck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kunattach-a", "kunattach-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25, Finish: finish})
		CloseSide(k)
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if report {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			diverged = true
			if !report {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestUnattachLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var c kunattachCounts
	div, stalls := kunattachRun(t, 192, true, kunattachFinish(&c))
	t.Logf("192 seeds, %d divergences, %d stalls; unattached %d (Ensouled Scimitar %d, other Equipment %d, Equipment animated by an Aura %d), Attach of an already-unattached-as-creature Equipment %d",
		b2i(div), stalls, c.unattached, c.selfAnim, c.otherAnim, c.auraAnim, c.equipWhileAnimated)
	if div || stalls > 0 {
		return
	}
	if c.unattached < 10 {
		t.Errorf("unattached: %d, want at least 10", c.unattached)
	}
	if c.selfAnim < 3 {
		t.Errorf("Ensouled Scimitar unattached: %d, want at least 3", c.selfAnim)
	}
	if c.otherAnim < 3 {
		t.Errorf("other Equipment unattached: %d, want at least 3", c.otherAnim)
	}
}

// TestUnattachMutation: leaving an attached creature attached
// (DebugAttachedCreatureStays) must make the lockstep diverge.
func TestUnattachMutation(t *testing.T) {
	dirac.DebugAttachedCreatureStays = true
	defer func() { dirac.DebugAttachedCreatureStays = false }()
	var c kunattachCounts
	if diverged, _ := kunattachRun(t, 128, false, kunattachFinish(&c)); !diverged {
		t.Fatal("DebugAttachedCreatureStays: no divergence in 128 seeds")
	}
}
