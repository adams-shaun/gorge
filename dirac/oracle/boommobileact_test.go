package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 106.6, 605.1a): Boommobile's enters trigger adds four mana
// of one colour that may be spent only to activate abilities. The units float
// restricted (State.rb, the kop.SpmActivated marker filter) and, the source
// being an artifact, typed (poolArtRestr). The mechanism is counted from
// gorge's state at each decision: the restricted "Activated" units that float,
// an ability activation that spent them, and a spell cast while they float
// (which they must never pay).

var kbmDeck = []cardset.Entry{
	{Name: "Boommobile", Count: 5}, {Name: "Furnace Whelp", Count: 7}, {Name: "Lightning Bolt", Count: 4},
	{Name: "Shock", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Goblin Chieftain", Count: 2},
	{Name: "Mountain", Count: 16},
}

const (
	kbmFloat     = iota // a decision with Boommobile's units floating
	kbmActPaid          // an ability activation spent them
	kbmSpellCast        // a spell cast while they float
	kbmSpellPaid        // a spell the units paid (a gorge bug if ever > 0)
	kbmWithheld         // a priority decision where a hand spell is affordable only with the units and gorge does not offer it
	kbmN
)

var kbmNames = [kbmN]string{"decision with units floating", "activation paid from them", "spell cast while they float", "spell paid from them", "spell withheld though the units would cover it"}

const (
	pushSpell = iota + 1
	pushAbility
)

// kbmState tracks the "Activated" restricted units of each seat between two
// decisions.
type kbmState struct {
	prev  [2]int32
	seen  int // gorge log events already read
	picks int // priority decisions with units floating
	push  int // the last stack push read: pushSpell or pushAbility (a payment follows its push, over several decisions)
}

// kbmCountWithheld counts a priority decision where the actor holds Boommobile's
// units and a nonland hand card costs more than its untapped lands pay but no
// more than the lands and the units together: gorge offers no cast of it.
func kbmCountWithheld(c *PickCtx, n *[kbmN]int) {
	g := c.G.E.G
	p := state.PlayerID(c.D.Player)
	units := int(kbmUnits(g, int(p)))
	if c.D.Kind != DPriority || units == 0 {
		return
	}
	lands := 0
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().IsLand() && !o.Tapped {
			lands++
		}
	}
	offered := map[state.ObjID]bool{}
	for _, a := range c.D.Options {
		if a.Kind() == ActCast {
			if o, ok := c.G.GorgeOption(a); ok {
				offered[o.Obj] = true
			}
		}
	}
	for _, id := range g.Zone(state.ZHand, p) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || o.Face().IsLand() || offered[id] {
			continue
		}
		if mv := int(o.Face().ManaValue()); mv > lands && mv <= lands+units {
			n[kbmWithheld]++
			return
		}
	}
}

func kbmUnits(g *state.Game, p int) (n int32) {
	for _, r := range g.Players[p].RestrictedMana {
		if r.Valid == "Activated" {
			n += r.Amount
		}
	}
	return n
}

// kbmObserve reads the log since the last decision: units that left a seat's
// pool with no ManaClear/TurnChange between were spent by the push they follow.
func kbmObserve(c *PickCtx, st *kbmState, n *[kbmN]int) {
	g := c.G.E.G
	evs := c.G.E.L.Events
	var cleared, cast bool
	for ; st.seen < len(evs); st.seen++ {
		switch evs[st.seen].Kind {
		case events.ManaClear, events.TurnChange:
			cleared = true
		case events.PutOnStack:
			st.push, cast = pushSpell, true
		case events.AbilityPush:
			st.push = pushAbility
		}
	}
	spell, ability := st.push == pushSpell, st.push == pushAbility
	for p := range st.prev {
		cur := kbmUnits(g, p)
		if st.prev[p] > 0 && cast {
			n[kbmSpellCast]++
		}
		if cur < st.prev[p] && !cleared {
			switch {
			case spell:
				n[kbmSpellPaid]++
			case ability:
				n[kbmActPaid]++
			}
		}
		if cur > 0 {
			n[kbmFloat]++
		}
		st.prev[p] = cur
	}
}

// kbmPolicy is eagerPolicy steering the colour of Boommobile's mana to red in
// two seeds of three (red pays Furnace Whelp and the exhaust ability).
func kbmPolicy(seed uint64, gs **GorgeSide, st *kbmState, n *[kbmN]int) Policy {
	inner := eagerPolicy(seed, map[string]int{})
	manaCode, _ := chooseCode("mana")
	colorCode, _ := chooseCode("color")
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		*gs = c.G
		kbmObserve(c, st, n)
		kbmCountWithheld(c, n)
		if a, ok := kbmSpend(seed, c, st); ok {
			return a
		}
		if seed%3 != 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActChoose+CanonKind(manaCode) || k == ActChoose+CanonKind(colorCode) {
					if o, ok := c.G.GorgeOption(a); ok && o.ManaSymbol == "R" {
						return a
					}
				}
			}
		}
		return inner.Pick(c)
	})
}

// kbmSpend steers a priority decision where the actor holds Boommobile's
// units: toward a paid ability (Furnace Whelp's pump, Boommobile's exhaust),
// or, on the other seeds, toward a spell the units must not pay.
func kbmSpend(seed uint64, c *PickCtx, st *kbmState) (Canon, bool) {
	if c.D.Kind != DPriority || kbmUnits(c.G.E.G, int(c.D.Player)) == 0 {
		return 0, false
	}
	st.picks++
	wantSpell := (seed+uint64(st.picks))%2 == 0
	for _, a := range c.D.Options {
		o, ok := c.G.GorgeOption(a)
		if !ok {
			continue
		}
		ob := c.G.E.G.Obj(o.Obj)
		if ob == nil || ob.Face() == nil {
			continue
		}
		switch k := a.Kind(); {
		case k == ActActivate && !wantSpell && (ob.Face().Name == "Furnace Whelp" || ob.Face().Name == "Boommobile"):
			return a, true
		case k == ActCast && wantSpell:
			return a, true
		}
	}
	return 0, false
}

func kbmRun(t *testing.T, seeds uint64, mutate bool) (games, divs int, fires [kbmN]int) {
	t.Helper()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("boommobileact")
	if err != nil {
		t.Fatal(err)
	}
	md := [2][]cardset.Entry{kbmDeck, kbmDeck}
	decks := kcastDeck(t, reg, md)
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kbm-a", "kbm-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		var st kbmState
		res := Lockstep(m, seed, k, kbmPolicy(seed, &gs, &st, &fires), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if res.Div != nil {
			divs++
			if mutate {
				t.Logf("mutation diverged: seed %d at %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
				return
			}
			if divs <= 6 {
				t.Errorf("seed %d: diverged at %d: %s %s ref=%v ker=%v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions)
			}
		}
	}
	return games, divs, fires
}

func TestBoommobileActManaLockstep(t *testing.T) {
	t.Parallel()
	games, divs, fires := kbmRun(t, 768, false)
	t.Logf("%d games, %d divergent; %v %v", games, divs, kbmNames, fires)
	if fires[kbmSpellPaid] != 0 {
		t.Errorf("gorge paid a spell from the activated-only units %d times", fires[kbmSpellPaid])
	}
	for i, c := range fires {
		if i != kbmSpellPaid && i != kbmSpellCast && c < 5 {
			t.Errorf("%s: %d times, want at least 5", kbmNames[i], c)
		}
	}
}

// TestBoommobileActManaMutation: with dirac.DebugActOnlyRestrSpell a spell
// may spend the activated-only units, which must diverge from gorge.
func TestBoommobileActManaMutation(t *testing.T) {
	old := dirac.DebugActOnlyRestrSpell
	dirac.DebugActOnlyRestrSpell = true
	defer func() { dirac.DebugActOnlyRestrSpell = old }()
	if _, divs, _ := kbmRun(t, 192, true); divs == 0 {
		t.Fatal("mutation did not diverge")
	}
}
