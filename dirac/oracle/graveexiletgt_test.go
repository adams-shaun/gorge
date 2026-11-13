package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Sorceress's Schemes lockstep: "return target instant or sorcery card
// from your graveyard or exiled card with flashback you own to your hand"
// (kop.TgtGraveExileCard, one target slot across the graveyard and exile). A
// scripted policy fills the graveyard with Shock on even seeds, and on odd
// seeds casts Think Twice and then its flashback (the card is exiled as that
// resolves); it casts Schemes only once its own zone's card is there and aims
// the target at that card. Both zones are tallied from gorge's MoveZone
// events, so neither arm is covered by accident.

var kgxDeck = []cardset.Entry{
	{Name: "Sorceress's Schemes", Count: 8},
	{Name: "Shock", Count: 6},
	{Name: "Think Twice", Count: 6},
	{Name: "Mountain", Count: 12},
	{Name: "Island", Count: 8},
}

// kgxEnv builds the graveexiletgt pool's registry, decks and table.
func kgxEnv(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("graveexiletgt")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kgxDeck {
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
	tbl, _, _, err := set.Matchup(kgxDeck, kgxDeck)
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

// kgxHolds reports whether player me has a card named name in zone z.
func kgxHolds(c *PickCtx, me state.PlayerID, z state.Zone, name string) bool {
	for _, id := range c.G.E.G.Zone(z, me) {
		if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// kgxPolicy plays the zone the seed names: wantExile is the flashback arm.
func kgxPolicy(seed uint64, wantExile bool) Policy {
	rnd := RandomPolicy(seed)
	pick := func(c *PickCtx, opts []Canon) Canon {
		d := *c.D
		d.Options = opts
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		me := state.PlayerID(c.D.Player)
		g := c.G.E.G
		var schemes, shock, twice, flash, tgt, rest []Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActTarget:
				if int(a.Tgt()) >= int(RefPlayer) {
					continue
				}
				o := g.Obj(c.G.id.obj[a.Tgt()])
				if o == nil || o.Face() == nil {
					continue
				}
				if wantExile && o.Zone == state.ZExile || !wantExile && o.Zone == state.ZGraveyard {
					tgt = append(tgt, a)
				}
				continue
			case ActCast:
				opt, ok := c.G.GorgeOption(a)
				if !ok {
					break
				}
				ob := g.Obj(opt.Obj)
				if ob == nil || ob.Face() == nil {
					break
				}
				switch {
				case ob.Face().Name == "Sorceress's Schemes" && opt.Mode != "flashback":
					schemes = append(schemes, a)
				case ob.Face().Name == "Shock":
					shock = append(shock, a)
				case ob.Face().Name == "Think Twice" && ob.Zone == state.ZGraveyard:
					flash = append(flash, a)
				case ob.Face().Name == "Think Twice":
					twice = append(twice, a)
				}
				continue
			}
			rest = append(rest, a)
		}
		switch {
		case len(tgt) > 0:
			return pick(c, tgt)
		case wantExile && len(schemes) > 0 && kgxHolds(c, me, state.ZExile, "Think Twice"):
			return pick(c, schemes)
		case !wantExile && len(schemes) > 0 && (kgxHolds(c, me, state.ZGraveyard, "Shock") || kgxHolds(c, me, state.ZGraveyard, "Think Twice")):
			return pick(c, schemes)
		case wantExile && len(flash) > 0:
			return pick(c, flash)
		case wantExile && len(twice) > 0 && !kgxHolds(c, me, state.ZGraveyard, "Think Twice") && !kgxHolds(c, me, state.ZExile, "Think Twice"):
			return pick(c, twice)
		case !wantExile && len(shock) > 0 && !kgxHolds(c, me, state.ZGraveyard, "Shock"):
			return pick(c, shock)
		}
		if len(rest) == 0 {
			return rnd.Pick(c)
		}
		return pick(c, rest)
	})
}

// kgxRun plays the seeds; it returns the divergence count and the MoveZone
// tallies of cards leaving a graveyard / exile for a hand.
func kgxRun(t *testing.T, seeds uint64, errf func(string, ...any)) (divs, fromGrave, fromExile int) {
	t.Helper()
	reg, set, decks, tbl := kgxEnv(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 >= 2}
		m, err := NewMatchup(reg, [2]string{"gx-a", "gx-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		finish := func(g *GorgeSide) {
			for _, ev := range g.E.L.Events {
				if ev.Kind != events.MoveZone || ev.To != state.ZHand {
					continue
				}
				switch ev.From {
				case state.ZGraveyard:
					fromGrave++
				case state.ZExile:
					fromExile++
				}
			}
		}
		res := Lockstep(m, seed, k, kgxPolicy(seed, seed%2 == 1), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Finish: finish})
		CloseSide(k)
		if d := res.Div; d != nil {
			divs++
			errf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			errf("seed %d: stall %q", seed, res.Stall)
		}
	}
	return divs, fromGrave, fromExile
}

func TestGraveExileTargetedReturnLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	seeds := uint64(24)
	if testing.Short() {
		seeds = 8
	}
	_, fromGrave, fromExile := kgxRun(t, seeds, t.Errorf)
	t.Logf("Schemes returned from a graveyard %d, from exile %d", fromGrave, fromExile)
	if fromGrave == 0 || fromExile == 0 {
		t.Errorf("Schemes returned %d cards from a graveyard and %d from exile, want both", fromGrave, fromExile)
	}
}

// TestGraveExileTargetedReturnMutation: with the kernel's exile walk off the
// exile arm's targets are missing from its options, so the lockstep diverges.
func TestGraveExileTargetedReturnMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugGraveExileNoExile = true
	defer func() { dirac.DebugGraveExileNoExile = false }()
	// the divergences are the expected outcome: count them, do not fail
	divs, _, _ := kgxRun(t, 24, t.Logf)
	if divs == 0 {
		t.Fatal("DebugGraveExileNoExile: no divergence over the seeds")
	}
}
