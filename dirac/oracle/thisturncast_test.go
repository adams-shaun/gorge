package oracle

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// thisturncast pool, whose carriers read Count$ThisTurnCast_<spec>
// through the kernel's per-push cast ring:
//
//   - static conditions: Stoic Sphinx (hexproof LT1), Brightspear Zealot
//     (+2/+0 GE2: the projection's layer-applied power), Leapfrog (flying),
//     Haunting Figment (CantBlockBy), Mogg Conscripts and Goblin Cohort
//     (CantAttack, Creature.YouCtrl);
//   - cost composition (the in-flight push hidden): Focus the Mind, Raging
//     Battle Mouse;
//   - hand provenance: Canyon Crab (wasCastFromYourHandByYou);
//   - !CastSaSource: Thunder Salvo, Storm Entity (unscoped), Dream Thief;
//   - an unscoped opponent count: Delightful Discovery (a cost reduction too);
//   - Slick Sequence's resolution-time count.
//   - Impatience: an ActivePlayerCtrl count (its 2 damage is counted).
//   - Relic Runner (Card.Historic+YouCtrl, with Mind Stone as the
//     historic spell; its attacks after a historic cast are counted) and
//     Predatory Advantage (Creature.ActivePlayerCtrl; its Lizards are counted).
//
// Unsummon bounces a creature so one object can be cast twice in a turn.
// The frozen FDN and BLB pair pools exercise none of these arms.

var kcastDecks = [][2][]cardset.Entry{{
	{
		// The active player only ever casts creatures, so once its Bears are
		// out it attacks on turns it casts no spell. Kill Shot (below) is
		// castable only while a creature is attacking, and B has no
		// creatures, so B holds it until A attacks: B's cast lands on a turn
		// A cast nothing. That is the ActivePlayerCtrl false branch: the
		// correct count (A's spells) is 0 while the any-controller count is 1.
		{Name: "Grizzly Bears", Count: 12}, {Name: "Forest", Count: 28},
	},
	{
		{Name: "Impatience", Count: 4}, {Name: "Kill Shot", Count: 4},
		{Name: "Plains", Count: 16}, {Name: "Mountain", Count: 16},
	},
}, {
	{
		{Name: "Brightspear Zealot", Count: 3}, {Name: "Canyon Crab", Count: 2}, {Name: "Thought Scour", Count: 3},
		{Name: "Unsummon", Count: 3}, {Name: "Focus the Mind", Count: 2}, {Name: "Dream Thief", Count: 2},
		{Name: "Stoic Sphinx", Count: 2}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 11}, {Name: "Island", Count: 11},
	},
	{
		{Name: "Delightful Discovery", Count: 2}, {Name: "Storm Entity", Count: 3}, {Name: "Thunder Salvo", Count: 3},
		{Name: "Raging Battle Mouse", Count: 3}, {Name: "Mogg Conscripts", Count: 2}, {Name: "Goblin Cohort", Count: 2},
		{Name: "Slick Sequence", Count: 2}, {Name: "Leapfrog", Count: 2}, {Name: "Haunting Figment", Count: 1},
		{Name: "Plains", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 8},
	},
}, {
	{
		{Name: "Relic Runner", Count: 2}, {Name: "Mind Stone", Count: 5},
		{Name: "Thought Scour", Count: 2}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Unsummon", Count: 2}, {Name: "Vision Skeins", Count: 4},
		{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12},
	},
	{
		{Name: "Predatory Advantage", Count: 4}, {Name: "Stoic Sphinx", Count: 4},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Unsummon", Count: 2},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 10},
	},
}, {
	// reductions paying into {X} (CR 601.2f): Zimone's counter-scaled
	// hasXCost reduction on Ivy Elemental and Mind Spring
	{
		{Name: "Zimone, Infinite Analyst", Count: 4}, {Name: "Ivy Elemental", Count: 4}, {Name: "Mind Spring", Count: 4},
		{Name: "Unsummon", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 12}, {Name: "Island", Count: 12},
	},
	// Mocking Sprite is a plain generic reducer (no hasXCost filter)
	// admitting Mind Spring's {X}{U}{U}
	{
		{Name: "Mocking Sprite", Count: 4}, {Name: "Mind Spring", Count: 4}, {Name: "Thought Scour", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Unsummon", Count: 2}, {Name: "Zimone, Infinite Analyst", Count: 2},
		{Name: "Island", Count: 16}, {Name: "Forest", Count: 6},
	},
}, {
	// Laboratory Drudge: "cast a spell from a graveyard"
	// (the CastFlags read on the object now, Bulk Up's flashback) plus
	// "activated an ability of a card in a graveyard" (a Mind Stone
	// sacrificed this turn sits in a graveyard when the end step reads it).
	// Unsummon gives the Drudge's controller a second chance at an end step.
	{
		{Name: "Laboratory Drudge", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Bulk Up", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Unsummon", Count: 1},
		{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12},
	},
	{
		{Name: "Laboratory Drudge", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Bulk Up", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Unsummon", Count: 1},
		{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12},
	},
}}

// kcastCarriers names the cards whose casts and activations the test
// counts (each must be played at least kcastMinPlays times).
var kcastCarriers = []string{
	"Stoic Sphinx", "Brightspear Zealot", "Focus the Mind", "Raging Battle Mouse", "Canyon Crab",
	"Thunder Salvo", "Storm Entity", "Dream Thief", "Delightful Discovery", "Leapfrog",
	"Haunting Figment", "Slick Sequence", "Mogg Conscripts", "Goblin Cohort", "Unsummon", "Impatience",
	"Relic Runner", "Predatory Advantage", "Mind Stone",
	"Zimone, Infinite Analyst", "Ivy Elemental", "Mind Spring", "Mocking Sprite",
	"Laboratory Drudge", "Bulk Up",
}

const kcastMinPlays = 20

// kcastLog is what one game's gorge event log shows about the cast ledger:
// the most spells one player pushed (PutOnStack) in one turn, whether one
// object was pushed twice in one turn, and the CR 733.1 reversals.
type kcastLog struct {
	maxPush   int
	recast    int
	reversed  int
	impDmg    int // Impatience's 2 damage (its intervening-if count was 0)
	impOff    int // ...on a turn the non-active cast a spell and the active cast none
	runner    int // Relic Runner declared as an attacker after its controller cast a historic spell that turn
	lizard    int // Predatory Advantage's 2/2 Lizard token
	lizardOff int // ...on a turn the non-active cast a creature and the active cast none
	drudge    int // end-step triggers pushed by Laboratory Drudge (TriggerPush Obj is the source)
	flashback int // spells pushed from a graveyard (Bulk Up's flashback)
}

// add sums o's per-game counts (not maxPush) into l.
func (l *kcastLog) add(o kcastLog) {
	l.recast += o.recast
	l.reversed += o.reversed
	l.impDmg += o.impDmg
	l.impOff += o.impOff
	l.runner += o.runner
	l.lizard += o.lizard
	l.lizardOff += o.lizardOff
	l.drudge += o.drudge
	l.flashback += o.flashback
}

func kcastScan(g *GorgeSide) kcastLog {
	var out kcastLog
	if g == nil {
		return out
	}
	var per, creat [8]int
	var endPer, endCreat [8]int // the per-turn tallies frozen when the end step begins
	var active, endActive state.PlayerID
	var step state.Step
	seen := map[state.ObjID]bool{}
	var historic [8]bool // the player has pushed a historic spell this turn
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TurnChange:
			per = [8]int{}
			creat = [8]int{}
			clear(seen)
			historic = [8]bool{}
			active = ev.Player
		case events.PutOnStack:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && int(ev.Player) < len(historic) {
				if n := ob.Face().Name; n == "Mind Stone" && state.StackKindOf(g.E.G, ob) == state.StackKindSpell {
					historic[ev.Player] = true
				}
				if f := ob.Face(); f != nil {
					for _, typ := range f.Types {
						if typ == "Creature" {
							creat[ev.Player]++
							break
						}
					}
				}
			}
			if int(ev.Player) < len(per) {
				per[ev.Player]++
				out.maxPush = max(out.maxPush, per[ev.Player])
			}
			if ev.From == state.ZGraveyard {
				out.flashback++
			}
			if seen[ev.Obj] {
				out.recast++
			}
			seen[ev.Obj] = true
		case events.DeclareAttackers: // Player is the defender; the attacker's controller is the caster
			for _, id := range ev.IDs {
				if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Relic Runner" &&
					int(ob.Controller) < len(historic) && historic[ob.Controller] {
					out.runner++
				}
			}
		case events.TokenCreate:
			if ev.Text == "g_2_2_lizard" {
				out.lizard++
				if int(endActive) < len(endCreat) && endCreat[endActive] == 0 && endCreat[1-endActive] > 0 {
					out.lizardOff++
				}
			}
		case events.SpellCastReversed:
			out.reversed++
		case events.TriggerPush: // Obj is the trigger's source
			if ob := g.E.G.Obj(ev.Obj); step == state.StepEnd && ob != nil && ob.Face() != nil && ob.Face().Name == "Laboratory Drudge" {
				out.drudge++
			}
		case events.StepChange:
			step = ev.Step
			if step == state.StepEnd {
				endPer, endCreat, endActive = per, creat, active
			}
		case events.Damage:
			// 2 to a player in an end step: nothing else in the pool does it
			if step == state.StepEnd && ev.Obj == 0 && ev.Amount == 2 {
				out.impDmg++
				if ev.Player == endActive && int(endActive) < len(endPer) &&
					endPer[endActive] == 0 && endPer[1-endActive] > 0 {
					out.impOff++
				}
			}
		}
	}
	return out
}

// kcastDump logs gorge's events since the last TurnChange (a divergence's
// context: the turn's casts and moves, by name).
func kcastDump(t *testing.T, g *GorgeSide) {
	if g == nil {
		return
	}
	for i := range g.E.G.Objs {
		ob := &g.E.G.Objs[i]
		if ob.Face() != nil && (ob.Zone == state.ZHand || ob.Zone == state.ZBattlefield) {
			t.Logf("  obj %d ref #%d %q zone %v ctrl %d", i+1, g.InstRef(state.ObjID(i+1)), ob.Face().Name, ob.Zone, ob.Controller)
		}
	}
	evs := g.E.L.Events
	i := len(evs) - 1
	for i > 0 && evs[i].Kind != events.TurnChange {
		i--
	}
	for ; i < len(evs); i++ {
		ev := evs[i]
		name := ""
		if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
			name = ob.Face().Name
		}
		t.Logf("  ev %v obj %d %q player %d from %v to %v amt %d %q", ev.Kind, ev.Obj, name, ev.Player, ev.From, ev.To, ev.Amount, ev.Text)
	}
}

// kcastPolicy is eagerPolicy that also remembers the gorge side (for the log
// scan) and, on one seed in eight, counts the priority picks at which a
// Brightspear Zealot shows its +2/+0 in gorge's derived power (the
// lockstep's projection compares that power with the kernel's).
func kcastPolicy(seed uint64, played map[string]int, gs **GorgeSide) Policy {
	inner := eagerPolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			*gs = c.G
		}
		if c.G != nil && seed%8 == 1 && c.D.Kind == DPriority {
			for i := range c.G.E.G.Objs {
				ob := &c.G.E.G.Objs[i]
				if ob.Zone == state.ZBattlefield && ob.Face() != nil && ob.Face().Name == "Brightspear Zealot" && c.G.E.Power(state.ObjID(i+1)) >= 4 {
					played["brightspear+2"]++
				}
			}
		}
		return inner.Pick(c)
	})
}

func kcastSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("thisturncast")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestThisturncastLockstep(t *testing.T) {
	t.Parallel()

	reg, set := kcastSetup(t)
	games := 0
	played := map[string]int{}
	pushHist := map[int]int{} // the most pushes by one player in one turn, per game
	stalls := map[string]int{}
	var sum kcastLog // maxPush unused: pushHist holds it
	matchupChunks(t, len(kcastDecks), 1, 512, 128, func(t *testing.T, c seedChunk, mu *sync.Mutex) {
		mi, md := c.mi, kcastDecks[c.mi]
		n, part, partHist, partStalls := 0, map[string]int{}, map[int]int{}, map[string]int{}
		var tl kcastLog
		defer func() {
			mu.Lock()
			defer mu.Unlock()
			games += n
			for k, v := range part {
				played[k] += v
			}
			for k, v := range partHist {
				pushHist[k] += v
			}
			for k, v := range partStalls {
				stalls[k] += v
			}
			sum.add(tl)
		}()
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := c.lo; seed <= c.hi; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcast-a", "kcast-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			var pol Policy = kcastPolicy(seed, part, &gs)
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			n++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				partStalls[res.Stall]++
			}
			lg := kcastScan(gs)
			partHist[lg.maxPush]++
			tl.add(lg)
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				kcastDump(t, gs)
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	})
	if t.Failed() {
		return
	}
	recast, reversed, impDmg, impOff, runner, lizard, lizardOff, drudge, flashback := sum.recast, sum.reversed, sum.impDmg, sum.impOff, sum.runner, sum.lizard, sum.lizardOff, sum.drudge, sum.flashback
	var hist []int
	for n := range pushHist {
		hist = append(hist, n)
	}
	sort.Ints(hist)
	var hs []string
	for _, n := range hist {
		hs = append(hs, fmt.Sprintf("%d:%d", n, pushHist[n]))
	}
	t.Logf("%d games, max pushes by one player in one turn (per game, max:games) %s, recasts %d, reversals %d, impatience damage %d (off-turn %d), relic runner historic attacks %d, lizards %d (off-turn %d), drudge end-step triggers %d, flashback casts %d, kernel stalls %v, carrier plays %v",
		games, strings.Join(hs, " "), recast, reversed, impDmg, impOff, runner, lizard, lizardOff, drudge, flashback, stalls, played)
	for _, n := range kcastCarriers {
		if played[n] < kcastMinPlays {
			t.Errorf("carrier %s played %d times, want at least %d", n, played[n], kcastMinPlays)
		}
	}
	if played["brightspear+2"] == 0 {
		t.Errorf("Brightspear Zealot never showed its +2/+0")
	}
	if impDmg < kcastMinPlays {
		t.Errorf("Impatience dealt its damage %d times, want at least %d", impDmg, kcastMinPlays)
	}
	if impOff < kcastMinPlays {
		t.Errorf("Impatience dealt its damage on a turn the non-active cast a spell and the active did not %d times, want at least %d", impOff, kcastMinPlays)
	}
	if runner < kcastMinPlays {
		t.Errorf("Relic Runner attacked after a historic cast %d times, want at least %d", runner, kcastMinPlays)
	}
	if lizard < kcastMinPlays {
		t.Errorf("Predatory Advantage made a Lizard %d times, want at least %d", lizard, kcastMinPlays)
	}
	if lizardOff < kcastMinPlays {
		t.Errorf("Predatory Advantage made a Lizard on a turn the non-active cast a creature and the active did not %d times, want at least %d", lizardOff, kcastMinPlays)
	}
	if drudge < kcastMinPlays {
		t.Errorf("Laboratory Drudge triggered %d times at an end step, want at least %d", drudge, kcastMinPlays)
	}
	if recast == 0 {
		t.Errorf("no object was cast twice in one turn")
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestThisturncastMutation flips the debug switch that reverts the cast
// ring reader to the caster's payment-time histSpells tally (the reader
// kgen refused before): it ignores the spec, the object's
// current controller and the cost-composition exclusion. The lockstep must
// diverge.
func TestThisturncastMutation(t *testing.T) {
	reg, set := kcastSetup(t)
	md := kcastDecks[1]
	decks := kcastDeck(t, reg, md)
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCastRingLive
	dirac.DebugCastRingLive = true
	defer func() { dirac.DebugCastRingLive = old }()
	for seed := uint64(1); seed <= 64; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcast-a", "kcast-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("mutation diverged at seed %d: %s %s %v", seed, res.Div.What, res.Div.Field, res.Div.Diff)
			return
		}
	}
	t.Fatal("debug switch did not diverge in 64 seeds")
}
