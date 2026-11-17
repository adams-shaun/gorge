package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The BecomesTarget forms this pool exercises (dirac/kgen/lower_trigger.go's
// BecomesTarget arms; kernel trig_becomestarget.go's btWatchers and
// becameTargetPlayer, and trig_counterfirst.go's Tf2FirstTime latch):
//   - Rayne, Academy Chancellor watches You,Permanent.YouCtrl for an
//     opponent's spell or ability (the player half queues through the kernel
//     player walk);
//   - Loki, God of Mischief watches Player,Permanent for an ability you
//     control (Prodigal Sorcerer's ping is the ability source);
//   - Dormant Gomazoa watches You for a spell (the single player target);
//   - Kira, Great Glass-Spinner's FirstTime line counters the targeting
//     spell or ability (Defined$ TriggeredSourceSA, the queue's trigSA).
//
// Prodigal Sorcerer's activated ping is the one ability source the pool
// needs: it targets a player (Loki, Rayne, Dormant) or Kira (the counter
// body), and Lightning Bolt adds a spell source. The original Fey Steed /
// Angelic Cub latch pair lives in TestBecomesTargetListLatchLockstep: its
// attack-trigger target and the FirstTime/reversal interaction predate this
// pool and are not what this test measures. Silverfur Partisan's ValidTarget$
// Wolf...,Werewolf... list over a ValidSource$ Instant,Sorcery spell runs in
// its own decks below, so the spell-source arm still has games of its own.
func becomestargetlistDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Loki, God of Mischief", Count: 2},
		{Name: "Rayne, Academy Chancellor", Count: 2},
		{Name: "Dormant Gomazoa", Count: 2},
		{Name: "Kira, Great Glass-Spinner", Count: 2},
		{Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Lightning Bolt", Count: 4},
		{Name: "Island", Count: 8},
		{Name: "Plains", Count: 4},
		{Name: "Mountain", Count: 3},
		{Name: "Forest", Count: 1},
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

// btPreferredTarget aims the burn and the ping at the target decision's
// rotating subject so every arm fires: a Kira (the counter body, spell or
// ability), the opposing player (Loki's player half, Rayne's and Dormant
// Gomazoa's "you become the target"), or a plain creature (the object half).
// When the preferred subject is not legal it falls back to the next
// preference, and with no preferred target the caller picks randomly.
func btPreferredTarget(c *PickCtx, want int) (Canon, bool) {
	var best Canon
	bestScore := 0
	for _, a := range c.D.Options {
		o, ok := c.G.GorgeOption(a)
		if !ok {
			continue
		}
		score := 0
		if o.Obj != 0 {
			ob := c.G.E.G.Obj(o.Obj)
			if ob == nil || ob.Zone != state.ZBattlefield || ob.Face() == nil {
				continue
			}
			switch name := ob.Face().Name; {
			case want == 0 && name == "Kira, Great Glass-Spinner":
				score = 3
			case name == "Kira, Great Glass-Spinner":
				score = 2
			case want == 2:
				score = 1
			}
		} else if uint8(o.Player) != c.D.Player {
			// the opponent: Loki's player half and Rayne's/Dormant Gomazoa's
			// "you become the target of a spell or ability an opponent controls"
			if want == 1 {
				score = 3
			} else {
				score = 1
			}
		}
		if score > bestScore {
			best, bestScore = a, score
		}
	}
	return best, bestScore > 0
}

// btcastFirstPolicy casts, activates and plays lands whenever it can (so the
// burn flies and creatures sit on the battlefield as targets), aims every
// spell target through btPreferredTarget, else random.
func btcastFirstPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	targets := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DTarget {
			targets++
			if a, ok := btPreferredTarget(c, targets%3); ok {
				return a
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
	if st.PlayerQueues == 0 {
		t.Fatal("no BecomesTarget player-target trigger was queued in the games; the player walk went untested")
	}
	if st.SrcAbilityQueues == 0 {
		t.Fatal("no BecomesTarget ability-source trigger was queued in the games; Loki's Ability.YouCtrl gate went untested")
	}
	var kiraFired, kiraRefused int
	for _, r := range recs {
		if r.Card != "Kira, Great Glass-Spinner" {
			continue
		}
		if r.Fired {
			kiraFired++
		} else {
			kiraRefused++
		}
	}
	if kiraFired == 0 {
		t.Fatal("Kira, Great Glass-Spinner's FirstTime line never queued; the TriggeredSourceSA counter went untested")
	}
	if kiraRefused == 0 {
		t.Fatal("the FirstTime latch never refused a later Kira targeting; the once-per-turn gate went untested")
	}
	t.Logf("BecomesTarget trigger queues: %d (player %d, ability source %d); Kira fired %d, refused %d",
		st.Queues, st.PlayerQueues, st.SrcAbilityQueues, kiraFired, kiraRefused)

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
