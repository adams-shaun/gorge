package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep: the view-only MayLookAt$ over face-down
// battlefield creatures. DSK Found Footage carries the printed static; MKM
// Lumbering Laundry carries it as an Effect-delivered static (its {2}
// activation resolves doing nothing, like gorge's). The pool also holds the
// facedown Morph/Disguise carriers so face-down creatures exist for the look
// to cover, Grizzly Bears as the vanilla body and Moxen/lands as mana.
var kmaylookatDeck = []cardset.Entry{
	{Name: "Found Footage", Count: 4},
	{Name: "Lumbering Laundry", Count: 4},
	{Name: "Kolaghan Stormsinger", Count: 3},
	{Name: "Bolrac-Clan Basher", Count: 2},
	{Name: "Basilica Stalker", Count: 2},
	{Name: "Concealed Weapon", Count: 2},
	{Name: "Zoetic Cavern", Count: 2},
	{Name: "Grizzly Bears", Count: 5},
	{Name: "Mox Jet", Count: 3},
	{Name: "Mox Ruby", Count: 3},
	{Name: "Mountain", Count: 4},
	{Name: "Forest", Count: 4},
	{Name: "Swamp", Count: 2},
}

// kmaylookatCounts counts the two carriers' activations from gorge's own
// options, plus the unimplemented Note gorge emits when Lumbering Laundry's
// Effect resolves (the Stop rule: no continuous effect is registered).
type kmaylookatCounts struct {
	laundry, footage int // chosen activations across every game
	notes            int // "continuous effect Continuous unimplemented" Notes

	// per-game caps, so one game cannot churn the same no-op activation
	// forever and starve the other carrier.
	laundryGame, footageGame int
}

// kmaylookatPolicy prefers the two carriers' activations (capped per game),
// then plays any cast/land/attack/block/turn-up, and falls back to the seeded
// random policy. It answers multi-picks with a mode (like m11Policy) so the
// games reach the carriers.
func kmaylookatPolicy(seed uint64, c *kmaylookatCounts) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(ctx *PickCtx) Canon {
		var footage, laundry Canon
		haveFootage, haveLaundry := false, false
		hasMode, hasDone := false, false
		buf = buf[:0]
		for _, a := range ctx.D.Options {
			switch a.Kind() {
			case ActDone:
				hasDone = true
			case ActMode:
				hasMode = true
			case ActActivate:
				if ctx.G != nil {
					switch optionFaceName(ctx, a) {
					case "Found Footage":
						footage, haveFootage = a, true
					case "Lumbering Laundry":
						laundry, haveLaundry = a, true
					}
				}
				buf = append(buf, a)
			case ActCast, ActPlayLand, ActAttack, ActBlock, ActSpecial:
				buf = append(buf, a)
			}
		}
		switch {
		case hasDone && hasMode:
			for _, o := range ctx.D.Options {
				if o.Kind() == ActMode {
					return o
				}
			}
		case haveFootage && c.footageGame < 2:
			c.footageGame++
			c.footage++
			return footage
		case haveLaundry && c.laundryGame < 3:
			c.laundryGame++
			c.laundry++
			return laundry
		case len(buf) > 0:
			d := *ctx.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: ctx.G, Seq: ctx.Seq})
		}
		return rnd.Pick(ctx)
	})
}

// kmaylookatEnv loads the corpus and the generated maylookat pool, builds
// both 40-card decks and the matchup table.
func kmaylookatEnv(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("maylookat")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kmaylookatDeck {
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
	tbl, _, _, err := set.Matchup(kmaylookatDeck, kmaylookatDeck)
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

// kmaylookatFinish counts the unimplemented Note gorge emits when Lumbering
// Laundry's Effect resolves. A Note is not a lockstep-compared event; the
// count proves gorge registered nothing for the battlefield look (Stop rule).
func kmaylookatFinish(c *kmaylookatCounts) func(*GorgeSide) {
	return func(g *GorgeSide) {
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind != events.Note || !strings.HasPrefix(e.Text, "continuous effect Continuous unimplemented") {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Lumbering Laundry" {
				c.notes++
			}
		}
	}
}

func TestMaylookatLockstep(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbl := kmaylookatEnv(t)
	var counts kmaylookatCounts
	var kernelStalls, decisionCaps, turnCaps int
	seeds := uint64(256)
	if testing.Short() {
		seeds = 16
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"km-a", "km-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		counts.laundryGame, counts.footageGame = 0, 0
		res := Lockstep(m, seed, k, kmaylookatPolicy(seed, &counts), RunConfig{
			Config: cfg, MaxDecisions: 2000, MaxTurns: 25, Finish: kmaylookatFinish(&counts),
		})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		switch {
		case strings.HasPrefix(res.Stall, KernelStallPrefix):
			kernelStalls++
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		case res.Stall == "decisions":
			decisionCaps++
		case res.Stall == "turns":
			turnCaps++
		}
	}
	t.Logf("%d seeds: Lumbering Laundry activations %d, Found Footage activations %d, unimplemented Notes %d, kernel stalls %d, decision-cap ends %d, turn-cap ends %d",
		seeds, counts.laundry, counts.footage, counts.notes, kernelStalls, decisionCaps, turnCaps)
	if counts.laundry < 10 {
		t.Errorf("Lumbering Laundry's Effect activated %d times, want at least 10", counts.laundry)
	}
	if counts.footage < 5 {
		t.Errorf("Found Footage's surveil-draw activated %d times, want at least 5", counts.footage)
	}
	if counts.notes == 0 {
		t.Errorf("no unimplemented continuous-effect Note: gorge registered the battlefield look")
	}
}

// kmaylookatMutate rewrites the generated table so Lumbering Laundry's
// activation creates an emblem granting +1/+0 to Creature.YouCtrl until end
// of turn: a stand-in for a look that changes state. The kernel's activation
// is currently empty (the view-only look lowers to nothing).
func kmaylookatMutate(tbl *kop.Table) bool {
	f := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	e := int16(len(tbl.Emblems))
	tbl.Emblems = append(tbl.Emblems, kop.Emblem{
		Abilities: []kop.Ability{{
			Kind:   kop.AbStatic,
			Static: kop.Static{Kind: kop.StatContinuous, Filter: f, A: 1},
		}},
		UntilEOT: true,
	})
	for i := range tbl.Cards {
		if tbl.Cards[i].Name != "Lumbering Laundry" {
			continue
		}
		for j := range tbl.Cards[i].Abilities {
			ab := &tbl.Cards[i].Abilities[j]
			if ab.Kind == kop.AbActivated && ab.Cost.Mana.Generic == 2 && ab.Cost.Flags == 0 && len(ab.Prog) == 0 {
				ab.Prog = append(ab.Prog, kop.Instr{Op: kop.OpCreateEmblem, Obj: kop.RefController, C: e})
				return true
			}
		}
	}
	return false
}

// TestMaylookatMutation: the mutated table's Laundry activation changes
// state the projection reads, so the lockstep must diverge within 128 seeds.
func TestMaylookatMutation(t *testing.T) {
	reg, set, decks, tbl := kmaylookatEnv(t)
	if !kmaylookatMutate(tbl) {
		t.Fatal("Lumbering Laundry's empty {2} activation not found in the generated table")
	}
	seeds := uint64(128)
	if testing.Short() {
		seeds = 8
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"km-a", "km-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var counts kmaylookatCounts
		res := Lockstep(m, seed, k, kmaylookatPolicy(seed, &counts), RunConfig{
			Config: cfg, MaxDecisions: 2000, MaxTurns: 25,
		})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d: mutation diverges at decision %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("the +1/+0 emblem mutation diverged in no game: the lockstep does not see the activation")
}
