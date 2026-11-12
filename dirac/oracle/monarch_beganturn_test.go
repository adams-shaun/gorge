package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Knights of the Black Rose's BeginTurn$ You, the intervening-if "if you were
// the monarch as the turn began" (CR 603.4; gorge trigmatch.BecomeMonarchMatches
// reads the TurnStartMonarch snapshot, the kernel genState.mon's high nibble),
// played decision by decision against gorge. Seat 0 casts Knights, whose
// entry makes it the monarch, and attacks with them, which steals the crown
// back. Seat 1 holds only lands and Throne of the High City, whose {4}, {T},
// sacrifice activation makes it the monarch at instant speed, and activates it
// whenever seat 0 is the monarch: on the turn seat 0 took the crown (seat 0 was
// not the monarch as that turn began, so Knights does not drain) or on a later
// turn (it was, so seat 1 loses 2 life and seat 0 gains 2). On odd seeds seat 1
// takes the crown only on seat 0's turns, so it keeps its lands untapped for
// the turn seat 0 takes it.
var kknightsCrownDecks = [2][]cardset.Entry{
	{{Name: "Knights of the Black Rose", Count: 10}, {Name: "Plains", Count: 15}, {Name: "Swamp", Count: 15}},
	{{Name: "Throne of the High City", Count: 16}, {Name: "Island", Count: 24}},
}

// knightsCrown is what gorge's logs played: seat 1 became the monarch from
// seat 0, a Knights on the battlefield, on a turn seat 0 began without the
// crown (same) or with it (later), and the life seat 0 gained on each kind of
// turn. Only Knights' drain gains seat 0 life in this matchup.
type knightsCrown struct {
	same, later           int
	sameDrain, laterDrain int
}

// read walks one finished gorge log (RunConfig.Finish).
func (c *knightsCrown) read(g *GorgeSide) {
	mon, start, knights := -1, -1, 0
	sameTurn, laterTurn := false, false
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.TurnChange:
			start, sameTurn, laterTurn = mon, false, false // gorge's TurnStartMonarch snapshot
		case events.MoveZone:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Knights of the Black Rose" {
				if e.To == state.ZBattlefield {
					knights++
				}
				if e.From == state.ZBattlefield {
					knights--
				}
			}
		case events.MonarchChange:
			prev := mon
			mon = int(e.Player)
			if mon != 1 || prev != 0 || knights <= 0 {
				break
			}
			if start == 0 {
				c.later++
				laterTurn = true
			} else {
				c.same++
				sameTurn = true
			}
		case events.LifeChange:
			if e.Player == 0 && e.Amount > 0 {
				if sameTurn {
					c.sameDrain++
				}
				if laterTurn {
					c.laterDrain++
				}
			}
		}
	}
}

// knightsCrownPolicy: seat 0 casts and plays lands first, else at random
// (its attacks among them); seat 1 activates Throne of the High City's
// monarch ability (gorge ability index 1) whenever seat 0 is the monarch (odd
// seeds: on seat 0's turns only), else plays a land, else passes.
func knightsCrownPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	cast := castFirstPolicy(seed)
	theirTurnOnly := seed%2 == 1
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority || c.G == nil {
			return rnd.Pick(c)
		}
		if c.D.Player == 0 {
			return cast.Pick(c)
		}
		g := c.G.E.G
		take := g.IsMonarch(0) && (!theirTurnOnly || g.Active == 0)
		var land, pass Canon
		hasLand, hasPass := false, false
		for _, o := range c.D.Options {
			switch {
			case take && o.Kind() == ActActivate && o.Mode() == 1 && m.InstanceCard(int(o.Src())) == "Throne of the High City":
				return o
			case o.Kind() == ActPlayLand && !hasLand:
				land, hasLand = o, true
			case o.Kind() == ActPass && !hasPass:
				pass, hasPass = o, true
			}
		}
		if hasLand {
			return land
		}
		if hasPass {
			return pass
		}
		return rnd.Pick(c)
	})
}

// knightsCrownRun plays seeds games of kknightsCrownDecks dirac against gorge,
// mutate (when set) applied to the kernel's table. report fails the test on
// each divergence; otherwise the first divergence ends the run. It returns
// what the gorge logs played and the divergent games.
func knightsCrownRun(t *testing.T, seeds uint64, mutate func(*kop.Table), report bool) (c knightsCrown, div int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("monarch")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kknightsCrownDecks {
		for _, e := range d {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], cd)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kknightsCrownDecks[0], kknightsCrownDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kknights-a", "kknights-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, knightsCrownPolicy(m, seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Finish: c.read})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s %s)", seed, d.At, d.What, d.Field)
				return c, div
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return c, div
}

// dropBeganTurn clears every Mode$ BecomeMonarch trigger's BeginTurn$ You
// (kop.TrigNMonarchBeganTurn), on copies of the table's slices.
func dropBeganTurn(tbl *kop.Table) {
	cs := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cs {
		abs := append([]kop.Ability(nil), cs[i].Abilities...)
		for j := range abs {
			if abs[j].Kind == kop.AbTriggered && abs[j].Trig.Event == kop.TrigBecomeMonarch {
				abs[j].Trig.N = 0
			}
		}
		cs[i].Abilities = abs
	}
	tbl.Cards = cs
}

// TestMonarchBeganTurnLockstep: every decision matches gorge, both kinds of
// turn were played, and gorge drained on the later turns only (CR 603.4).
func TestMonarchBeganTurnLockstep(t *testing.T) {
	t.Parallel()
	c, _ := knightsCrownRun(t, monarchSeeds(48), nil, true)
	t.Logf("seat 1 took the crown from seat 0 %d time(s) on turns seat 0 began without it (%d life gains for seat 0 on those turns) and %d on turns it began with it (%d gains)",
		c.same, c.sameDrain, c.later, c.laterDrain)
	if c.same == 0 || c.later == 0 {
		t.Fatalf("same-turn takes %d, later-turn takes %d: the matchup must play both", c.same, c.later)
	}
	if c.sameDrain != 0 || c.laterDrain == 0 {
		t.Fatalf("gorge gained seat 0 life %d time(s) on the same turns, %d on the later ones: Knights drains on the later turns only", c.sameDrain, c.laterDrain)
	}
}

// TestMonarchBeganTurnMutation: with BeginTurn$ You dropped from the table
// Knights drains on the turn seat 0 took the crown too, and some seed
// diverges.
func TestMonarchBeganTurnMutation(t *testing.T) {
	t.Parallel()
	if _, div := knightsCrownRun(t, monarchSeeds(48), dropBeganTurn, false); div == 0 {
		t.Fatal("no seed diverged with BeginTurn$ You dropped: the matchup never takes the crown on the turn seat 0 took it")
	}
}
