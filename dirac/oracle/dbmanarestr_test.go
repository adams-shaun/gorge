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

// The lockstep (CR 106.6, 605.1a): a resolving DB$ Mana
// with RestrictValid$ Spell.<spec> floats restricted mana. Abstract Paintmage
// adds {U}{R} at the beginning of its controller's first main phase, for
// instants and sorceries only (two fixed colours of one restriction);
// Desolation of Smaug adds four mana in any combination for Dragon spells
// (Produced$ Combo Any, one pick per unit). The kernel's restricted pool names
// two restrictions per table, so Paintmage plays itself and Desolation. The
// mechanism is counted from gorge's log: a restricted ManaAdd waits for the
// next spells pushed, which it funds when the spell fits the restriction,
// refuses when it does not, and is lost unspent when the pool empties first.
// Ashling, Rimebound (Produced$ Any, RestrictValid$ Spell.cmcGE4) stays
// refused by kgen, see lower_manaeffect.go manaChoiceEffect.

var kdbrPaint = []cardset.Entry{
	{Name: "Abstract Paintmage", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 3},
	{Name: "Divination", Count: 3}, {Name: "Counterspell", Count: 3}, {Name: "Hill Giant", Count: 3},
	{Name: "Air Elemental", Count: 3}, {Name: "Goblin Chieftain", Count: 2}, {Name: "Prodigal Sorcerer", Count: 2},
	{Name: "Wind Drake", Count: 1}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
}

var kdbrDecks = [][2][]cardset.Entry{{
	kdbrPaint,
	{
		{Name: "Abstract Paintmage", Count: 3}, {Name: "Dragon Moose", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Air Elemental", Count: 2}, {Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Furnace Whelp", Count: 3}, {Name: "Divination", Count: 3}, {Name: "Goblin Chieftain", Count: 2}, {Name: "Wind Drake", Count: 1},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 7},
	},
}, {
	{
		{Name: "Desolation of Smaug", Count: 4}, {Name: "Rorix Bladewing", Count: 4}, {Name: "Dragon Moose", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Goblin Chieftain", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Mountain", Count: 17},
	},
	kdbrPaint,
}}

const (
	kdbrPaid     = iota // a spell the restriction admits is cast while restricted units float
	kdbrEmptied         // restricted units emptied with no admitted spell cast
	kdbrRefused         // a spell the restriction rejects is cast while they float
	kdbrDesoPaid        // Desolation's colour-choice units: a Dragon spell is cast while they float
	kdbrActGate         // a decision where restricted units float beside an untapped Furnace Whelp and an untapped land
	kdbrN
)

var kdbrNames = [kdbrN]string{"admitted spell cast", "emptied unspent", "rejected spell cast", "Dragon cast over Desolation units", "restricted pool beside a mana-cost activation"}

// kdbrFits reports whether the restriction of carrier src admits the spell
// pushed (CR 106.6; the spec each carrier's RestrictValid$ names).
func kdbrFits(src string, g *GorgeSide, spell state.ObjID) bool {
	ob := g.E.G.Obj(spell)
	if ob == nil || ob.Face() == nil {
		return false
	}
	f := ob.Face()
	switch src {
	case "Abstract Paintmage":
		for _, t := range f.Types {
			if t == "Instant" || t == "Sorcery" {
				return true
			}
		}
	case "Desolation of Smaug":
		return f.Name == "Rorix Bladewing" || f.Name == "Dragon Moose"
	}
	return false
}

// kdbrScan walks gorge's log: a restricted positive ManaAdd names its source
// (the " @id" suffix) and waits for a spell push; the pool emptying
// (ManaClear, TurnChange) drops what waits.
func kdbrScan(g *GorgeSide, n *[kdbrN]int) {
	if g == nil {
		return
	}
	type unit struct {
		src  string
		paid bool
	}
	var pending []unit
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.ManaAdd:
			if ev.Amount <= 0 {
				continue
			}
			if _, src, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text)); ok && src != 0 {
				if ob := g.E.G.Obj(src); ob != nil && ob.Face() != nil {
					switch name := ob.Face().Name; name {
					case "Abstract Paintmage", "Desolation of Smaug":
						pending = append(pending, unit{src: name})
					}
				}
			}
		case events.ManaClear, events.TurnChange:
			for _, u := range pending {
				if !u.paid {
					n[kdbrEmptied]++
				}
			}
			pending = pending[:0]
		case events.PutOnStack:
			for j := range pending {
				u := &pending[j]
				switch {
				case !kdbrFits(u.src, g, ev.Obj):
					n[kdbrRefused]++ // a spell the restriction rejects, cast while the units float
				case !u.paid:
					u.paid = true
					n[kdbrPaid]++
					if u.src == "Desolation of Smaug" {
						n[kdbrDesoPaid]++
					}
				}
			}
		}
	}
}

// kdbrCountAct counts a decision where the actor holds restricted mana and
// controls a Furnace Whelp ({R}: +1/+0) with an untapped land to pay it: gorge's
// planner refuses the pool (PaymentPlanPoolAccepted) so it drops the activation
// the kernel's untapped source could pay.
func kdbrCountAct(c *PickCtx, n *[kdbrN]int) {
	g := c.G.E.G
	p := state.PlayerID(c.D.Player)
	if int(p) >= len(g.Players) || len(g.Players[p].RestrictedMana) == 0 {
		return
	}
	whelp, land := false, false
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.Zone != state.ZBattlefield || ob.Controller != p || ob.Face() == nil {
			continue
		}
		if ob.Face().Name == "Furnace Whelp" {
			whelp = true
		} else if ob.Face().IsLand() && !ob.Tapped {
			land = true
		}
	}
	if whelp && land {
		n[kdbrActGate]++
	}
}

func kdbrSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("dbmanarestr")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestDbmanarestrLockstep(t *testing.T) {
	t.Parallel()
	reg, set := kdbrSetup(t)
	var fires [kdbrN]int
	games, divs := 0, 0
	for mi, md := range kdbrDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 192; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kdbr-a", "kdbr-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			inner := eagerPolicy(seed, map[string]int{})
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					gs = c.G
					kdbrCountAct(c, &fires)
				}
				return inner.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			kdbrScan(gs, &fires)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: kernel stall %s", mi, seed, res.Stall)
			}
			if res.Div != nil {
				divs++
				if divs <= 6 {
					t.Errorf("matchup %d seed %d: diverged at %d: %s %s ref=%v ker=%v", mi, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions)
				}
			}
		}
	}
	t.Logf("%d games, %d divergent; restricted units %v %v", games, divs, kdbrNames, fires)
	for i, c := range fires {
		if c < 5 {
			t.Errorf("%s: %d times, want at least 5", kdbrNames[i], c)
		}
	}
}

// TestDbmanarestrMutation: with dirac.DebugDBManaRestrPlain a resolving
// colour choice's restricted units go to the plain pool, which must diverge
// from gorge (a creature spell paid with Desolation's mana).
func TestDbmanarestrMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := kdbrSetup(t)
	old := dirac.DebugDBManaRestrPlain
	dirac.DebugDBManaRestrPlain = true
	defer func() { dirac.DebugDBManaRestrPlain = old }()
	for mi, md := range kdbrDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 64; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kdbr-a", "kdbr-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			if res.Div != nil {
				t.Logf("mutation diverged: matchup %d seed %d at %d: %s %s", mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				return
			}
		}
	}
	t.Fatal("mutation did not diverge")
}
