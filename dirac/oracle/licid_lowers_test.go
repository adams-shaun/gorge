package oracle

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLicidLowers plays Convulsing Licid and Tempting Licid in lockstep with
// gorge over the licid fixture pool (dirac/cardset/pools/fixtures/licid).
// Their AB$ Animate | Defined$ Self | Duration$ Permanent | Types$
// Enchantment,Aura | RemoveCardTypes$ True | RemoveEnchantmentTypes$ True |
// Keywords$ Enchant:Creature lowers as a permanent self override plus the
// card's Enchant spec (kgen lower_animateaura.go), and the SubAbility$ DB$
// Attach attaches the new Aura to the target creature.
//
// Gorge reads neither RemoveThisAbility$ nor RevertCost$ (engine/effects
// combatfx.go effAnimate): the Aura keeps the ability and no revert is
// offered, so an attached Licid may activate again and move to another
// creature. The lockstep counts, from gorge's side, the activations that
// resolved into an attach, the re-activations of an already-attached Licid
// (the RevertCost$ special action gorge does not offer: the kernel matches
// the re-attach instead), and the attached Licids the
// attachment state-based action put into the graveyard once their bearer
// left.
//
// The mutation drops the Override from both Licids' programs, so the kernel
// attaches a creature, and must diverge.
func TestLicidLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("table", func(t *testing.T) {
		set, err := cardset.RawByName("licid")
		if errors.Is(err, cardset.ErrNoCorpus) {
			t.Skip("no card corpus: " + err.Error())
		}
		if err != nil {
			t.Fatal(err)
		}
		tbl, _, _, err := set.Matchup(licidDecks[0][0], licidDecks[0][1])
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Convulsing Licid", "Tempting Licid"} {
			found := false
			for i := range tbl.Cards {
				cd := &tbl.Cards[i]
				if cd.Name != name {
					continue
				}
				found = true
				if cd.Types&kop.TAura != 0 || cd.Types&kop.TCreature == 0 {
					t.Errorf("%s printed types %#x: want a non-Aura creature card", name, cd.Types)
				}
				if cd.Enchant.Kind != kop.TgtObject || cd.Enchant.Filter == 0 {
					t.Errorf("%s Enchant %+v: want the granted enchant creature spec", name, cd.Enchant)
				}
			}
			if !found {
				t.Errorf("%s is not in the table", name)
			}
		}
	})
	t.Run("lockstep", func(t *testing.T) {
		run, st := runLicidLockstep(t, 24, true, nil)
		t.Logf("%d games, %d divergences, %d stalls; Licid stats %+v", run.games, run.divs, run.stalls, st)
		if st.attached < 4 {
			t.Errorf("a Licid activation resolved into an attach %d times, want at least 4", st.attached)
		}
		if st.reattached < 1 {
			t.Errorf("an attached Licid activated again and moved %d times, want at least 1", st.reattached)
		}
		if st.fell < 1 {
			t.Errorf("an attached Licid went to the graveyard %d times, want at least 1", st.fell)
		}
	})
	t.Run("mutation-Override", func(t *testing.T) {
		run, _ := runLicidLockstep(t, 24, false, func(tbl *kop.Table) {
			for _, name := range []string{"Convulsing Licid", "Tempting Licid"} {
				mutateCard(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpOverride {
						*in = kop.Instr{Op: kop.OpNop}
					}
				})
			}
		})
		run.requireDivergence(t, "a Licid that never becomes an Aura")
	})
}

var licidDecks = []fixtureDecks{{
	{{Name: "Convulsing Licid", Count: 9}, {Name: "Tempting Licid", Count: 9}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9}},
	{{Name: "Grizzly Bears", Count: 12}, {Name: "Raging Goblin", Count: 10}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9}},
}}

var licidNames = map[string]bool{"Convulsing Licid": true, "Tempting Licid": true}

// licidStats are the preconditions measured from gorge's side.
type licidStats struct {
	attached   int // Attach events of a Licid onto a permanent
	reattached int // ... of a Licid that was already attached to another one
	fell       int // an attached Licid moved from the battlefield to a graveyard
	offered    int // priority decisions offering an attached Licid's ability
}

// licidObserve scans the events gorge appended since *seen; bearer keeps
// each Licid's last bearer while it stays on the battlefield (0: never
// attached), so a move that unattaches first still counts as a re-attach.
func licidObserve(ref *GorgeSide, seen *int, bearer map[state.ObjID]state.ObjID, st *licidStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; *seen < len(evs); *seen++ {
		e := evs[*seen]
		o := g.Obj(e.Obj)
		if o == nil || o.Face() == nil || !licidNames[o.Face().Name] {
			continue
		}
		switch {
		case e.Kind == events.Attach && len(e.IDs) == 1:
			if b := bearer[e.Obj]; b != 0 && b != e.IDs[0] {
				st.reattached++
			}
			bearer[e.Obj] = e.IDs[0]
			st.attached++
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield:
			if bearer[e.Obj] != 0 && e.To == state.ZGraveyard {
				st.fell++
			}
			bearer[e.Obj] = 0
		}
	}
}

// licidPolicy plays randomly but activates a Licid's ability three times in
// four when one is offered, and casts a Licid the same way.
func licidPolicy(seed uint64, casts map[string]int, st *licidStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6c696364)) // "licd"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				if a.Kind() == ActActivate && licidNames[refCardName(c.G, a.Src())] && int(a.Src()) < len(c.G.id.obj) {
					if o := c.G.E.G.Obj(c.G.id.obj[a.Src()]); o != nil && o.AttachedTo != 0 {
						st.offered++
						break
					}
				}
			}
			for _, kind := range []canon.CanonKind{ActActivate, ActCast} {
				for _, a := range c.D.Options {
					if a.Kind() != kind || !licidNames[refCardName(c.G, a.Src())] || prng.IntN(4) == 0 {
						continue
					}
					if kind == ActCast {
						casts[refCardName(c.G, a.Src())]++
					}
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runLicidLockstep plays seeds 1..seeds of the licid matchup, gorge leading,
// alternating the option surface by seed and measuring the preconditions;
// mutate rewrites the table first.
func runLicidLockstep(t *testing.T, seeds int, report bool, mutate func(tbl *kop.Table)) (fixtureRun, licidStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("licid")
	if err != nil {
		t.Fatal(err)
	}
	md := licidDecks[0]
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	run := fixtureRun{casts: map[string]int{}}
	var st licidStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"licid-a", "licid-b"}, decks, 1)
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
		seen, bearer := 0, map[state.ObjID]state.ObjID{}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			licidObserve(ref, &seen, bearer, &st)
			return nil
		}
		res := LockstepLed(m, seed, g, k, licidPolicy(seed, run.casts, &st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, st
}
