package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The ThisTurnEntered exact-count lockstep: constructed matchups over the
// generated watchent pool. The generic Count$ThisTurnEntered head is refused
// (dirac/kgen/value.go): the kernel keeps only zoneTurn, which cannot reproduce
// gorge's per-add state.Entered list. The one exact spelling that remains,
// ThisTurnEntered_Graveyard_from_Battlefield_Creature, lowers to the per-add
// creatures-died tally (ExCreaturesDied), which is what Bloodcrazed Paladin
// and Khabál Ghoul read. The decks give the policy burn and destroy spells
// and vanilla creatures to kill, so the tally fires; the policy never blocks,
// so combat never kills.
var watchentDecks = [][2][]cardset.Entry{{
	{
		{Name: "Bloodcrazed Paladin", Count: 4}, {Name: "Khabál Ghoul", Count: 2},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Doom Blade", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Swamp", Count: 10},
		{Name: "Mountain", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Forest", Count: 10},
		{Name: "Mountain", Count: 10},
	},
}}

// watchentPolicy casts, activates, plays lands and attacks when it can, but
// never blocks: combat is always unblocked, so only the removal spells kill.
// It counts the carrier casts the gorge side poses.
func watchentPolicy(seed uint64, played map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate, ActPlayLand, ActAttack:
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 {
			for _, o := range c.D.Options {
				if o.Kind() == ActPass || o.Kind() == ActDone {
					return o
				}
			}
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActCast && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					played[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

// watchentMutate returns a copy of tbl with the creatures-died expr replaced
// by the constant zero: the carrier's lowering is reverted, so a game that
// exercises it must diverge.
func watchentMutate(tbl *kop.Table) *kop.Table {
	m := *tbl
	m.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
	for i := range m.Exprs {
		if m.Exprs[i].Kind == kop.ExCreaturesDied {
			m.Exprs[i] = kop.Expr{Kind: kop.ExConst}
			return &m
		}
	}
	return nil
}

func TestWatchentLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("watchent")
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 48
	games := 0
	played := map[string]int{}
	paladin := nameHash("Bloodcrazed Paladin")
	fired := false
	var mutTbl *kop.Table
	var mutDecks [2][]*cards.Card
	for mi, md := range watchentDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("matchup %d: %s not in the corpus", mi, e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		if mi == 0 {
			mutTbl, mutDecks = tbl, decks
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			m, err := NewMatchup(reg, [2]string{"watchent-a", "watchent-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			check := func(_ *GorgeSide, k Side) *Divergence {
				var p Projection
				k.Project(&p)
				for i := 0; i < int(p.NInst); i++ {
					c := &p.Cards[i]
					if c.NameHash == paladin && c.Zone == ZoneBattlefield && c.Counters[CtrP1P1] > 0 {
						fired = true
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, watchentPolicy(seed, played), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, carrier casts %v, paladin fired %v", games, played, fired)
	if played["Bloodcrazed Paladin"] < 8 {
		t.Errorf("Bloodcrazed Paladin cast %d times, want at least 8", played["Bloodcrazed Paladin"])
	}
	if played["Khabál Ghoul"] < 2 {
		t.Errorf("Khabál Ghoul cast %d times, want at least 2", played["Khabál Ghoul"])
	}
	if !fired {
		t.Error("Bloodcrazed Paladin never entered with a +1/+1 counter; the count never fired")
	}

	// Mutation: revert the creatures-died expr to the constant zero and
	// require the first matchup to diverge. Bloodcrazed Paladin's entry
	// replacement and Khabál Ghoul's end-step trigger both read it.
	mut := watchentMutate(mutTbl)
	if mut == nil {
		t.Fatal("no ExCreaturesDied expr in the watchent table")
	}
	diverged := false
	for seed := uint64(1); seed <= 24; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"watchent-a", "watchent-b"}, mutDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, mut, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, watchentPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			diverged = true
			t.Logf("mutation diverged at seed %d: %s %s %v", seed, res.Div.What, res.Div.Field, res.Div.Diff)
			break
		}
	}
	if !diverged {
		t.Error("mutation 'creatures died ignored' never diverged")
	}
}
