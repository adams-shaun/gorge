package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The demo Charm-shapes lockstep: Inscription of Abundance (the charm's
// minimum and maximum both read off the kicker: a kicked cast chooses any
// number of modes, an unkicked one exactly one), Kozilek's Command (a
// multi-pick charm whose fourth mode is "up to X target cards from
// graveyards": a 0..X slot among sequential per-mode asks), Masterful
// Replication (a mode Clone whose template is its own target beside
// CloneTarget$ Valid ...+NotDefinedTargeted) and Dromoka's Command (a mode
// that is a remembered-source prevention Effect). The pool is
// dirac/cardset/pools/fixtures/democharmshapes. The kernel's CharmShapeStats
// count the casts each shape decided, so a green game is not one where no
// shape was reached.
const demoCharmShapesPool = "democharmshapes"

var demoCharmShapesDecks = [2][]cardset.Entry{
	{
		{Name: "Inscription of Abundance", Count: 4}, {Name: "Kozilek's Command", Count: 4}, {Name: "Dromoka's Command", Count: 4},
		{Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2}, {Name: "Pacifism", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Glory Seeker", Count: 2}, {Name: "Wastes", Count: 4}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Masterful Replication", Count: 4}, {Name: "Lightning Bolt", Count: 8},
		{Name: "Sol Ring", Count: 3}, {Name: "Mind Stone", Count: 3}, {Name: "Ornithopter", Count: 3},
		{Name: "Merfolk of the Pearl Trident", Count: 6}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 5},
	},
}

// demoCharmShapesPolicy is castFirstPolicy that holds Dromoka's Command while
// a Lightning Bolt is on the stack and takes the first mode of a charm two
// times in three. gorge does not prevent the damage of its first mode's
// target: effectRemembered has no RememberObjects$ TargetedSource, so that
// Effect remembers nothing (a gorge defect, ticketed), while the kernel
// prevents it as Forge does (TestPrevSpellShield). The first mode still plays
// against gorge over a spell that deals no damage; the other three modes
// always do.
func demoCharmShapesPolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x64726f6d6f6b61)) // "dromoka"
	var buf []Canon
	return PolicyFunc(func(p *PickCtx) Canon {
		switch p.D.Kind {
		case DModes:
			for _, a := range p.D.Options {
				if a.Kind() == ActMode && a.Mode() == 0 && prng.IntN(3) != 0 {
					return a
				}
			}
		case DPriority:
			bolt := false
			for _, id := range p.G.E.G.Stack {
				if o := p.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Lightning Bolt" {
					bolt = true
				}
			}
			if !bolt {
				break
			}
			buf = buf[:0]
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && refCardName(p.G, a.Src()) == "Dromoka's Command" {
					continue
				}
				buf = append(buf, a)
			}
			if len(buf) > 0 && len(buf) < len(p.D.Options) {
				d := *p.D
				d.Options = buf
				return base.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
			}
		}
		return base.Pick(p)
	})
}

// demoCharmShapesGames plays the seeds and returns the clean games, the
// divergent ones and the shape counts.
func demoCharmShapesGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs []lockstepDivergence, st *dirac.CharmShapeStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoCharmShapesPool, demoCharmShapesDecks)
	st = &dirac.CharmShapeStats{}
	dirac.DebugCharmShapeStats = st
	defer func() { dirac.DebugCharmShapeStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, demoCharmShapesPool, demoCharmShapesDecks, gd, 1, hi, mode, nil, nil, demoCharmShapesPolicy)
	return c, d, st
}

// TestDemoCharmShapesLockstep: no divergence and no kernel stall over 64
// seeds, and across them every shape decided a cast.
func TestDemoCharmShapesLockstep(t *testing.T) {
	clean, divs, st := demoCharmShapesGames(t, 128, lockstepFailEach)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.KickedAnyNumber == 0 {
		t.Error("no kicked Inscription of Abundance chose its modes with no minimum")
	}
	if st.UpToXSlots == 0 {
		t.Error("no Kozilek's Command asked its up-to-X graveyard slot")
	}
	if st.CloneBecomers == 0 {
		t.Error("no Masterful Replication made an artifact a copy of its target")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDemoCharmShapesMutation: with any one of the new reads ignored (the
// kicked charm minimum, the sequential 0..X slot, the spell-keyed shield),
// some game diverges from gorge.
func TestDemoCharmShapesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []struct {
		name string
		sw   *bool
	}{
		{"DebugCharmMinKicked", &dirac.DebugCharmMinKicked},
		{"DebugCharmUpToX", &dirac.DebugCharmUpToX},
	} {
		*m.sw = true
		_, divs, _ := demoCharmShapesGames(t, 64, lockstepQuiet)
		*m.sw = false
		if len(divs) == 0 {
			t.Errorf("%s: no divergence in 64 games", m.name)
			continue
		}
		t.Logf("%s diverged in %d games", m.name, len(divs))
	}
}

// TestFourSeatDemoCharmShapesLockstep: four seats, gorge leading.
func TestFourSeatDemoCharmShapesLockstep(t *testing.T) {
	a := pod4Deck(deckOf(4, "Inscription of Abundance"), deckOf(4, "Kozilek's Command"), deckOf(4, "Dromoka's Command"),
		deckOf(2, "Sol Ring"), deckOf(2, "Mind Stone"), deckOf(2, "Pacifism"), deckOf(4, "Grizzly Bears"), deckOf(2, "Glory Seeker"),
		deckOf(4, "Wastes"), deckOf(6, "Forest"), deckOf(6, "Plains"))
	b := pod4Deck(deckOf(4, "Masterful Replication"), deckOf(8, "Lightning Bolt"),
		deckOf(3, "Sol Ring"), deckOf(3, "Mind Stone"), deckOf(3, "Ornithopter"),
		deckOf(6, "Merfolk of the Pearl Trident"), deckOf(8, "Island"), deckOf(5, "Mountain"))
	if n := pod4FixtureLockstep(t, demoCharmShapesPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
