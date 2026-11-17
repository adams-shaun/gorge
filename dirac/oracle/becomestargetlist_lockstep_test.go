package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Fey Steed's, Angelic Cub's and Silverfur Partisan's lockstep
// (dirac/kgen/lower_trigger.go's BecomesTarget list and FirstTime arms;
// kernel trig_becomestarget.go's btWatchers and trig_counterfirst.go's
// Tf2FirstTime latch): Fey Steed watches a comma list of permanent filters (a
// creature or planeswalker you control) targeted by an opponent's spell or
// ability and may draw a card; Angelic Cub's FirstTime$ True self line puts a
// +1/+1 counter on it the first time it is targeted each turn (a second
// Lightning Bolt on the same Cub in one turn triggers nothing, the next turn's
// first one does); Silverfur Partisan's ValidTarget$ Wolf...,Werewolf... list
// over a ValidSource$ Instant,Sorcery spell fires when a Wolf or Werewolf is
// targeted by an instant or sorcery. The burn can also aim at a player, which
// none of the forms answer.
//
// The other named carriers do not play here: Kira, Great Glass-Spinner's
// Counter body refuses (TriggeredSourceSA outside a SpellCast trigger) and
// Dormant Gomazoa's player target is the pre-existing inert lowering (the
// kernel's targetedOne walk sees object targets only, the ticket's PARTIAL
// player-target item) — either would diverge by design.
func becomestargetlistDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Fey Steed", Count: 4},
		{Name: "Angelic Cub", Count: 4},
		{Name: "Lightning Bolt", Count: 8},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Hill Giant", Count: 4},
		{Name: "Plains", Count: 8},
		{Name: "Mountain", Count: 2},
		{Name: "Forest", Count: 2},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// becomestargetlistSpellSourceDecks is the spell-source list arm in games of
// its own: Silverfur Partisan is the only BecomesTarget carrier in the deck,
// so the run's queue count can only be this arm's. Watchwolf is the Wolf the
// opponent's Lightning Bolt targets; Silverfur Partisan is itself a Wolf, so
// either body proves the list.
func becomestargetlistSpellSourceDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Silverfur Partisan", Count: 4},
		{Name: "Watchwolf", Count: 10},
		{Name: "Lightning Bolt", Count: 8},
		{Name: "Forest", Count: 8},
		{Name: "Plains", Count: 5},
		{Name: "Mountain", Count: 5},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// btcastFirstPolicy casts, activates and plays lands whenever it can (so the
// burn flies and creatures sit on the battlefield as targets), and aims every
// spell target at a battlefield Angelic Cub when one is legal (the Cub's own
// first trigger puts a +1/+1 counter on it, so a second Bolt in the same turn
// finds it still on the battlefield and exercises the once-per-turn latch),
// else random.
func btcastFirstPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DTarget {
			for _, a := range c.D.Options {
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZBattlefield &&
					ob.Face() != nil && ob.Face().Name == "Angelic Cub" {
					return a
				}
			}
		}
		buf := make([]Canon, 0, 8)
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActPlayLand:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// btwolfTargetPolicy is btcastFirstPolicy aiming the burn at a Wolf or
// Werewolf on the battlefield (an opponent's body first) so Silverfur
// Partisan's spell-source list arm sees a target.
func btwolfTargetPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DTarget {
			var opp, any []Canon
			for _, a := range c.D.Options {
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				ob := c.G.E.G.Obj(o.Obj)
				if ob == nil || ob.Zone != state.ZBattlefield || ob.Face() == nil {
					continue
				}
				if !slices.Contains(ob.Face().Types, "Wolf") && !slices.Contains(ob.Face().Types, "Werewolf") {
					continue
				}
				any = append(any, a)
				if ob.Controller != state.PlayerID(c.D.Player) {
					opp = append(opp, a)
				}
			}
			if len(opp) > 0 {
				return opp[int(c.Seq)%len(opp)]
			}
			if len(any) > 0 {
				return any[int(c.Seq)%len(any)]
			}
		}
		buf := make([]Canon, 0, 8)
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActPlayLand:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// Serial, not t.Parallel: the test installs the process-global kernel hooks
// dirac.DebugBtStats and dirac.CounterFirstLog. Other tests' kernel games run
// counterFirstUsed/markCounterFirst concurrently and would race the slice
// header (or panic when the defer nils the pointer between a nil check and a
// deref); e.g. enterprised_test.go is serial for the same reason. Serial
// tests finish before the package's parallel tests resume, so there is no
// overlap.
func TestBecomesTargetListLockstep(t *testing.T) {
	st := &dirac.BtStats{}
	dirac.DebugBtStats = st
	recs := []dirac.CounterFirstRec{}
	dirac.CounterFirstLog = &recs
	defer func() {
		dirac.DebugBtStats = nil
		dirac.CounterFirstLog = nil
	}()
	lockstepRunPolicy(t, "becomestargetlist", becomestargetlistDecks(), 60, true, nil, nil, btcastFirstPolicy)
	if st.Queues == 0 {
		t.Fatal("no BecomesTarget trigger was queued in the games; the lockstep proves nothing")
	}
	var fired, refused int
	for _, r := range recs {
		if r.Card != "Angelic Cub" {
			continue
		}
		if r.Fired {
			fired++
		} else {
			refused++
		}
	}
	if fired == 0 {
		t.Fatal("Angelic Cub's FirstTime line never queued in the games; the lockstep proves nothing")
	}
	if refused == 0 {
		t.Fatal("the FirstTime latch never refused a later targeting in the games; the once-per-turn gate went untested")
	}
	t.Logf("BecomesTarget trigger queues: %d; Angelic Cub latch fired %d, refused %d", st.Queues, fired, refused)

	// The spell-source list arm in its own games: Silverfur Partisan is the
	// only BecomesTarget carrier in these decks, so a queue here is the
	// ValidTarget$ Wolf...,Werewolf... list over a ValidSource$
	// Instant,Sorcery spell firing.
	st.Queues = 0
	lockstepRunPolicy(t, "becomestargetlist", becomestargetlistSpellSourceDecks(), 60, true, nil, nil, btwolfTargetPolicy)
	if st.Queues == 0 {
		t.Fatal("Silverfur Partisan's spell-source ValidTarget$ list never queued in its own games; the lockstep proves nothing for it")
	}
	t.Logf("Silverfur Partisan spell-source list trigger queues: %d", st.Queues)
}
