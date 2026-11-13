package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Five filter words: greatestPowerControlledByTargeted
// (Consumed by Greed), CrewedBySourceThisTurn (Balthier and Fran, beside
// three Vehicles), ControlledBy Spawner>TriggeredTarget (Leatherhead,
// Swamp Stalker), CastSa Spell.Mayhem and its negation (Sandman's
// Quicksand, beside a discard outlet) and CastSa Spell.ManaSpent EQ0
// (Satoru, the Infiltrator, beside two 0-cost creatures and a reanimation).
// withWarp (Blade of the Swarm) lowers as PredWord7 11 but is not in this
// pool; TestBladeWithWarpLockstep (warp_test.go) drives it.
//
// Mayhem casts themselves are not reached: gorge offers the graveyard cast
// only from FLOATING mana (it prices no mayhem mode: see mayhem_test.go) and
// no card of the pool that lowers floats the {3}{B}; Quicksand therefore
// resolves from a hand cast (the !Mayhem branch, and the Mayhem read on a
// non-mayhem spell), and sandMayhem stays 0.

var kprops9Decks = [][2][]cardset.Entry{
	{ // M1: Balthier and Leatherhead with Vehicles against Consumed by Greed
		{
			{Name: "Balthier and Fran", Count: 4}, {Name: "Leatherhead, Swamp Stalker", Count: 6}, {Name: "Smuggler's Copter", Count: 3},
			{Name: "Renegade Freighter", Count: 3}, {Name: "Sky Skiff", Count: 2}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Raging Goblin", Count: 2}, {Name: "Llanowar Elves", Count: 2}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 7},
		},
		{
			{Name: "Consumed by Greed", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Scathe Zombies", Count: 4},
			{Name: "Bonesplitter", Count: 4}, {Name: "Bad Moon", Count: 4}, {Name: "Hill Giant", Count: 2}, {Name: "Swamp", Count: 18},
		},
	},
	{ // M2: Sandman's Quicksand and a discard outlet against Satoru and the 0-cost and reanimation lines
		{
			{Name: "Sandman's Quicksand", Count: 5}, {Name: "Tormenting Voice", Count: 5}, {Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 2},
			{Name: "Raging Goblin", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
		},
		{
			{Name: "Satoru, the Infiltrator", Count: 4}, {Name: "Memnite", Count: 4}, {Name: "Ornithopter", Count: 3}, {Name: "Zombify", Count: 3},
			{Name: "Walking Corpse", Count: 7}, {Name: "Scathe Zombies", Count: 3}, {Name: "Swamp", Count: 8}, {Name: "Island", Count: 8},
		},
	},
}

// kprops9Stats is what the gorge-side event log says each word decided.
type kprops9Stats struct {
	greedSac    int // Consumed by Greed made the opponent sacrifice a creature
	greedMulti  int // ... and the opponent still held a creature after (2+ candidates)
	balTrig     int // Balthier's Attacks trigger went on the stack
	vehAtkBal   int // a Vehicle attacked while its controller's Balthier was on the battlefield
	vehNoTrig   int // vehAtkBal less the triggers: crewed by someone else (or no trigger)
	leatherDmg  int // Leatherhead's combat-damage trigger went on the stack
	leatherDest int // an artifact or enchantment Leatherhead's ImmediateTrigger destroyed
	sandDisc    int // Sandman's Quicksand was discarded
	sandHand    int // Sandman's Quicksand resolved from a hand cast (all creatures)
	sandMayhem  int // ... from a mayhem cast (opponents' creatures only)
	satTrig     int // Satoru's batch trigger went on the stack
	satFree     int // a 0-cost creature resolved beside Satoru
	satReanim   int // Zombify resolved beside Satoru
}

type kprops9Tally struct {
	evAt     int
	cur      string
	sacs     int
	castFrom map[string]state.Zone
}

func kprops9Has(g *state.Game, pl state.PlayerID, name string) bool {
	for _, o := range battlefieldObjects(g, pl) {
		if chName(g, o.ID) == name {
			return true
		}
	}
	return false
}

func (p *kprops9Tally) flush(g *state.Game, st *kprops9Stats, ctrl state.PlayerID) {
	if p.cur == "Consumed by Greed" && p.sacs > 0 {
		st.greedSac++
		opp := 1 - ctrl
		n := 0
		for _, o := range battlefieldObjects(g, opp) {
			if o.Face() != nil && o.Face().IsCreature() {
				n++
			}
		}
		if n > 0 {
			st.greedMulti++
		}
	}
	p.cur, p.sacs = "", 0
}

var kprops9Vehicles = map[string]bool{"Smuggler's Copter": true, "Renegade Freighter": true, "Sky Skiff": true}

func (p *kprops9Tally) scan(ref *GorgeSide, st *kprops9Stats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	var ctrl state.PlayerID
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Priority, events.StepChange, events.TurnChange:
			p.flush(g, st, ctrl)
		case events.Resolve:
			p.flush(g, st, ctrl)
			p.cur = chName(g, e.Obj)
			if o := g.Obj(e.Obj); o != nil {
				ctrl = o.Controller
				switch p.cur {
				case "Memnite", "Ornithopter":
					if kprops9Has(g, ctrl, "Satoru, the Infiltrator") {
						st.satFree++
					}
				case "Zombify":
					if kprops9Has(g, ctrl, "Satoru, the Infiltrator") {
						st.satReanim++
					}
				case "Sandman's Quicksand":
					if p.castFrom[p.cur] == state.ZGraveyard {
						st.sandMayhem++
					} else {
						st.sandHand++
					}
				}
			}
		case events.MoveZone:
			n := chName(g, e.Obj)
			if e.To == state.ZStack && n == "Sandman's Quicksand" {
				if p.castFrom == nil {
					p.castFrom = map[string]state.Zone{}
				}
				p.castFrom[n] = e.From
			}
			if p.cur == "Consumed by Greed" && e.From == state.ZBattlefield && e.To == state.ZGraveyard {
				p.sacs++
			}
			if e.From == state.ZHand && e.To == state.ZGraveyard && n == "Sandman's Quicksand" {
				st.sandDisc++
			}
			if e.From == state.ZBattlefield && e.To == state.ZGraveyard && (n == "Bonesplitter" || n == "Bad Moon") {
				st.leatherDest++
			}
		case events.TriggerPush:
			switch chName(g, e.Obj) {
			case "Balthier and Fran":
				st.balTrig++
			case "Leatherhead, Swamp Stalker":
				st.leatherDmg++
			case "Satoru, the Infiltrator":
				st.satTrig++
			}
		case events.DeclareAttackers:
			ids := e.IDs
			if len(ids) == 0 {
				ids = []state.ObjID{e.Obj}
			}
			for _, id := range ids {
				if a := g.Obj(id); a != nil && kprops9Vehicles[chName(g, id)] && kprops9Has(g, a.Controller, "Balthier and Fran") {
					st.vehAtkBal++
				}
			}
		}
	}
	p.evAt = len(evs)
	st.vehNoTrig = st.vehAtkBal - st.balTrig
}

// kprops9Policy plays at random among casting, activating and playing lands
// (so the carriers are reached) and attacks whenever it can.
func kprops9Policy(seed uint64, st *kprops9Stats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x70726f7073390a)) // "props9"
	var buf []Canon
	var tl kprops9Tally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		if c.G != nil && c.D.Kind != DPriority && c.D.Kind != DAttackers {
			g := c.G.E.G
			for _, a := range c.D.Options {
				oa, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				// a trigger's optional cost (Leatherhead's counter, Balthier's {1}{R}{G}) is mostly paid,
				// and a discard (Tormenting Voice) feeds Sandman's Quicksand to the graveyard
				if (oa.Kind == "trigger_cost_pay" || oa.Kind == "trigger_cost_counter" || oa.Obj != 0 && chName(g, oa.Obj) == "Sandman's Quicksand") && prng.IntN(4) != 0 {
					return a
				}
			}
		}
		if c.G != nil && c.D.Kind == DPriority {
			g := c.G.E.G
			for _, a := range c.D.Options {
				oa, ok := c.G.GorgeOption(a)
				if !ok || a.Kind() != ActCast || oa.Obj == 0 {
					continue
				}
				// the mayhem cast is taken when gorge offers it; the outlet is cast to discard the spell
				if (oa.Mode == "mayhem" || chName(g, oa.Obj) == "Tormenting Voice") && prng.IntN(4) != 0 {
					return a
				}
			}
		}
		switch c.D.Kind {
		case DAttackers:
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return buf[prng.IntN(len(buf))]
			}
		case DPriority:
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

// runFiltercrewedcast plays the constructed matchups, each in both seatings, and
// returns the games, divergences, stalls and the word counts. mutate negates
// the kernel's props9 answers (the mutation must diverge).
func runFiltercrewedcast(t *testing.T, seeds int, mutate bool) (games int, divs, stalls [2]int, st [2]kprops9Stats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("filtercrewedcast")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugWord9Flip
	dirac.DebugWord9Flip = mutate
	defer func() { dirac.DebugWord9Flip = old }()
	for mi, md0 := range kprops9Decks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
			}
			var decks [2][]*cards.Card
			for s, d := range md {
				for _, e := range d {
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
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
			for seed := uint64(1); seed <= uint64(seeds); seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"kprops9-a", "kprops9-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, kprops9Policy(seed, &st[mi]), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls[mi]++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
					}
				}
				if d := res.Div; d != nil {
					divs[mi]++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestFilterWordsCrewedAndCastLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 per matchup
	games, divs, stalls, st := runFiltercrewedcast(t, 96, false)
	t.Logf("%d games, %v divergences, %v stalls", games, divs, stalls)
	t.Logf("M1 %+v", st[0])
	t.Logf("M2 %+v", st[1])
	if divs != [2]int{} || stalls != [2]int{} {
		t.Fatalf("%v divergences, %v stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Consumed by Greed sacrifices", st[0].greedSac}, {"Greed sacrifices with 2+ candidates", st[0].greedMulti},
		{"Balthier triggers", st[0].balTrig}, {"Vehicle attacks beside Balthier without a trigger", st[0].vehNoTrig},
		{"Leatherhead combat-damage triggers", st[0].leatherDmg}, {"Leatherhead destroys", st[0].leatherDest},
		{"Sandman resolutions from hand", st[1].sandHand}, {"Quicksand discards", st[1].sandDisc},
		{"Satoru triggers", st[1].satTrig}, {"0-cost creatures beside Satoru", st[1].satFree}, {"Zombify beside Satoru", st[1].satReanim},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestFilterWordsCrewedAndCastMutation negates the kernel's props9 answers (PredWord7
// sub-kinds 12-13, the Mayhem read of wasCastSpell, the ManaSpent-EQ0 read of
// wasCastRead): each matchup must diverge. A green mutation means the
// lockstep never reaches a decision the words shape.
func TestFilterWordsCrewedAndCastMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runFiltercrewedcast(t, 32, true)
	for mi, d := range divs {
		if d == 0 {
			t.Errorf("matchup %d: mutation negated the props9 answers with no divergence", mi+1)
		}
	}
	t.Logf("mutation diverged %v in %d games", divs, games)
}
