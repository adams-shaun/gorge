package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAttachedBySourceSpellDamageLockstep plays a mirror of Spirit Link (an
// Aura whose one trigger reads ValidSource$ Card.AttachedBy), a Memnite to
// attach it to and Chandra's Pyrohelix (a spell that deals noncombat damage,
// so the kernel walks its card matcher, dirac's fireDamageSrc), 40 seeds,
// gorge leading. The enchanted creature never deals damage in these games, so
// the trigger must never queue; before the kernel's card matcher refused
// kop.FAttachedBySrc it queued Spirit Link's gain-life trigger at the
// Pyrohelix damage ("projection NStack") and the game diverged.
func TestAttachedBySourceSpellDamageLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	const aura, spell, creature = "Spirit Link", "Chandra's Pyrohelix", "Memnite"
	reg := synthReg(t)
	var pool string
	var set cardset.Set
	for _, name := range cardsettest.SynthNames(t) {
		if !strings.HasPrefix(name, cardset.SynthDraftPrefix+"mbc") {
			continue
		}
		s := cardsettest.Synth(t, name)
		hold := true
		for _, c := range []string{aura, spell, creature} {
			i, ok := s.Lookup(c)
			if !ok {
				hold = false
				break
			}
			if s.Status(i) == cardset.StatusRefused {
				hold = false
				break
			}
		}
		if hold {
			pool, set = name, s
			break
		}
	}
	if pool == "" {
		t.Fatalf("no MBC draft pool holds %s, %s and %s", aura, spell, creature)
	}
	deck := []cardset.Entry{
		{Name: aura, Count: 12}, {Name: spell, Count: 12},
		{Name: creature, Count: 8}, {Name: "Plains", Count: 4}, {Name: "Mountain", Count: 4},
	}
	var cs [2][]*cards.Card
	for s := range cs {
		for _, e := range deck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatalf("%s: %v", pool, err)
	}
	played := map[string]int{}
	attached := 0
	for seed := uint64(1); seed <= 40; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"attachedby-a", "attachedby-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		inner := castActivatePlayedPolicy(seed, played)
		var gs *GorgeSide
		pol := PolicyFunc(func(pc *PickCtx) Canon {
			if pc.G != nil {
				gs = pc.G
			}
			return inner.Pick(pc)
		})
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", pool, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		if res.StallUnmodelled {
			t.Fatalf("%s seed %d: unmodelled stall %s", pool, seed, res.Stall)
		}
		if gs == nil {
			continue
		}
		for s := 0; s < 2; s++ {
			for _, oid := range gs.E.G.Zone(state.ZBattlefield, state.PlayerID(s)) {
				if o := gs.E.G.Obj(oid); o != nil && o.Face() != nil && o.Face().Name == aura {
					attached++
				}
			}
		}
	}
	t.Logf("%s: %s cast %d times, %s cast %d times, %d games ended with a %s attached", pool, aura, played[aura], spell, played[spell], attached, aura)
	if played[aura] == 0 {
		t.Errorf("%s was never cast in 40 games", aura)
	}
	if attached == 0 {
		t.Errorf("%s never sat attached on a battlefield in 40 games", aura)
	}
	if played[spell] == 0 {
		t.Errorf("%s was never cast in 40 games", spell)
	}
}
