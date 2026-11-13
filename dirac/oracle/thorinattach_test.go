package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Thorin, Mountain-king's ETB attaches its controller's
// Equipment to a creature and its reflexive trigger has that creature deal
// damage equal to its power (TriggerRemembered$CardPower, DamageSource$
// DelayTriggerRemembered) to up to one target creature. Deck B answers a
// waiting ability with Unsummon and Shock, so the captured creature
// sometimes leaves before the reflexive trigger resolves: it then deals its
// last known power, taken as the trigger was put on the stack (CR 608.2h;
// gorge rules rememberedPTLKI, the kernel's reflexBiteGone).

var thorinDecks = [2][]cardset.Entry{
	{
		{Name: "Thorin, Mountain-king", Count: 6}, {Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Bonesplitter", Count: 4}, {Name: "Short Sword", Count: 4}, {Name: "Leather Armor", Count: 2},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Shock", Count: 5}, {Name: "Unsummon", Count: 6},
		{Name: "Mountain", Count: 6}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 4},
	},
}

// thorinCounts: what gorge's event logs show of Thorin's reflexive damage.
type thorinCounts struct {
	games     int
	resolves  int // reflexive damage resolutions
	dealt     int // ... dealing damage with the captured creature on the battlefield
	gone      int // ... with the captured creature gone
	goneDealt int // ... gone and still dealing damage (its last known power)
}

// thorinScan reads gorge's event log: each Thorin reflexive DealDamage
// resolution, split by whether its captured creature (the DelayedPush's
// remembered IDs) is still on the battlefield.
func thorinScan(g *GorgeSide, c *thorinCounts) {
	if g == nil {
		return
	}
	gm := g.E.G
	evs := g.E.L.Events
	zone := map[state.ObjID]state.Zone{}
	captured := map[state.ObjID]state.ObjID{} // source -> captured creature (the latest push)
	for i, ev := range evs {
		switch ev.Kind {
		case events.MoveZone:
			zone[ev.Obj] = ev.To
			continue
		case events.DelayedPush:
			if ev.Counter == "TrigDamage" && firstFaceName(gm, ev.Obj) == "Thorin, Mountain-king" && len(ev.IDs) == 1 {
				captured[ev.Obj] = ev.IDs[0]
			}
			continue
		case events.Resolve:
		default:
			continue
		}
		o := gm.Obj(ev.Obj)
		if o == nil || o.Ability == nil || o.Ability.API != "DealDamage" || sourceCardName(gm, ev.Obj) != "Thorin, Mountain-king" {
			continue
		}
		cap, ok := captured[o.Source]
		if !ok {
			continue
		}
		delete(captured, o.Source)
		dmg := 0
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Priority && evs[j].Kind != events.Resolve; j++ {
			if evs[j].Kind == events.Damage && evs[j].Amount > 0 {
				dmg++
			}
		}
		c.resolves++
		switch {
		case zone[cap] != state.ZBattlefield:
			c.gone++
			if dmg > 0 {
				c.goneDealt++
			}
		case dmg > 0:
			c.dealt++
		}
	}
}

// thorinReflexWaiting reports the top of gorge's stack is Thorin's reflexive
// damage ability, and the creature it captured (0 when none).
func thorinReflexWaiting(gm *state.Game) state.ObjID {
	n := len(gm.Stack)
	if n == 0 {
		return 0
	}
	o := gm.Obj(gm.Stack[n-1])
	if o == nil || o.Ability == nil || o.Ability.API != "DealDamage" || sourceCardName(gm, o.ID) != "Thorin, Mountain-king" ||
		len(o.Remembered) != 1 || o.Remembered[0].IsPlayer {
		return 0
	}
	return o.Remembered[0].Obj
}

// thorinPolicy is kremw4Policy for deck A. Deck B holds its mana: on three
// of its own turns in four it only plays a land, on deck A's turn it passes
// until Thorin's reflexive trigger waits, which it then answers with
// Unsummon on the captured creature (three times in four), so the creature
// leaves before the trigger resolves.
func thorinPolicy(seed uint64) Policy {
	base := kremw4Policy(seed)
	answers := uint64(0)
	pick := func(c *PickCtx, want func(Canon) bool) (Canon, bool) {
		for _, a := range c.D.Options {
			if want(a) {
				return a, true
			}
		}
		return 0, false
	}
	isKind := func(k CanonKind) func(Canon) bool { return func(a Canon) bool { return a.Kind() == k } }
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		if g == nil || c.D.Player != 1 {
			return base.Pick(c)
		}
		gm := g.E.G
		if int(gm.Active) == 1 {
			if gm.Turn%8 < 6 {
				if a, ok := pick(c, isKind(ActPlayLand)); ok {
					return a
				}
				if a, ok := pick(c, isKind(ActPass)); ok {
					return a
				}
			}
			return base.Pick(c)
		}
		if captured := thorinReflexWaiting(gm); captured != 0 && (seed+answers)%4 != 3 {
			if a, ok := pick(c, func(a Canon) bool {
				o, ok := g.GorgeOption(a)
				return ok && (a.Kind() == ActCast && chName(gm, o.Obj) == "Unsummon" || a.Kind() == ActTarget && o.Obj == captured)
			}); ok {
				if a.Kind() == ActTarget {
					answers++
				}
				return a
			}
		}
		if a, ok := pick(c, isKind(ActPass)); ok {
			return a
		}
		return base.Pick(c)
	})
}

func thorinRun(t *testing.T, seeds uint64, mutant bool) (c thorinCounts, diverged bool, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("thorinattach")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range thorinDecks {
		for _, e := range d {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], cd)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(thorinDecks[0], thorinDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"thorin-a", "thorin-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := thorinPolicy(seed) // deck A casts first, deck B answers the reflexive trigger
		if seed%8 == 0 {
			inner = RandomPolicy(seed)
		}
		pol := PolicyFunc(func(pc *PickCtx) Canon {
			if pc.G != nil {
				gs = pc.G
			}
			return inner.Pick(pc)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 14})
		CloseSide(k)
		c.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		thorinScan(gs, &c)
		if d := res.Div; d != nil {
			diverged = true
			if mutant {
				t.Logf("mutant: seed %d diverged at decision %d: %s %s", seed, d.At, d.What, d.Field)
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

// TestThorinAttachLockstep: zero divergences, with at least two reflexive
// resolutions whose captured creature had left and still dealt damage.
func TestThorinAttachLockstep(t *testing.T) {
	t.Parallel()
	c, _, stalls := thorinRun(t, 200, false)
	t.Logf("%d games: %d Thorin reflexive resolutions: %d dealt live, %d captured creature gone (%d dealt its last known power); kernel stalls %v",
		c.games, c.resolves, c.dealt, c.gone, c.goneDealt, stalls)
	if c.dealt < 10 {
		t.Errorf("%d reflexive damages from a creature still in play, want at least 10", c.dealt)
	}
	if c.goneDealt < 2 {
		t.Errorf("%d reflexive damages from a gone creature, want at least 2", c.goneDealt)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestThorinAttachMutation: with the gone creature's bite skipped (the
// kernel before the last known power was kept), the lockstep must diverge.
func TestThorinAttachMutation(t *testing.T) {
	dirac.DebugNoReflexBiteLKI = true
	defer func() { dirac.DebugNoReflexBiteLKI = false }()
	if _, diverged, _ := thorinRun(t, 200, true); !diverged {
		t.Fatal("no divergence in 200 seeds with the gone creature's bite skipped")
	}
}
