package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// condand pool, whose carriers carry two static gates in one condition
// (kop.CondAnd, re-evaluated whenever the static is read; CR 611.3a):
//
//   - Momo, Friendly Flier: Condition$ PlayerTurn + CheckSVar$ X EQ0
//     (the first non-Lemur creature spell with flying each turn), a
//     ReduceCost static (CR 601.2f);
//   - Of One Mind: IsPresent$ Human + CheckSVar$ non-Human, ReduceCost;
//   - Incisor Steed: CheckSVar$ HighestOppPoison GE3 + Condition$ Metalcraft,
//     AddPower;
//   - Sardian Cliffstomper: IsPresent$ Mountain GE4 + Condition$ PlayerTurn,
//     AddPower X.
//
// Momo's deck holds other non-Lemur flyers (Stoic Sphinx, Wind Drake,
// Ornithopter) so its EQ0 gate flips: the first such cast of a turn is
// reduced, later ones are not. The scan below counts both. Prologue to
// Phyresis (each opponent gets a poison counter, then draw) gives the
// opponent poison counters so Incisor Steed's HighestOppPoison >= 3 child
// also reaches its true arm, not just false.

var kcondandDecks = [][2][]cardset.Entry{{
	{
		{Name: "Momo, Friendly Flier", Count: 4}, {Name: "Stoic Sphinx", Count: 3},
		{Name: "Wind Drake", Count: 2}, {Name: "Ornithopter", Count: 2},
		{Name: "Incisor Steed", Count: 2}, {Name: "Of One Mind", Count: 3},
		{Name: "Prologue to Phyresis", Count: 3}, {Name: "Elite Vanguard", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 7}, {Name: "Universal Automaton", Count: 3},
	},
	{
		{Name: "Sardian Cliffstomper", Count: 3}, {Name: "Gray Ogre", Count: 6},
		{Name: "Hill Giant", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: 13}, {Name: "Swamp", Count: 6},
	},
}, {
	{
		{Name: "Momo, Friendly Flier", Count: 4}, {Name: "Stoic Sphinx", Count: 3},
		{Name: "Wind Drake", Count: 3}, {Name: "Ornithopter", Count: 2},
		{Name: "Incisor Steed", Count: 2}, {Name: "Of One Mind", Count: 3},
		{Name: "Prologue to Phyresis", Count: 3}, {Name: "Elite Vanguard", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 6}, {Name: "Universal Automaton", Count: 3},
	},
	{
		{Name: "Sardian Cliffstomper", Count: 3}, {Name: "Gray Ogre", Count: 6},
		{Name: "Hill Giant", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: 13}, {Name: "Swamp", Count: 6},
	},
}}

// kcondandCarriers names the cards whose casts the test counts (each must
// be played at least kcondandMinPlays times).
var kcondandCarriers = []string{
	"Incisor Steed", "Momo, Friendly Flier",
	"Of One Mind", "Sardian Cliffstomper",
}

const kcondandMinPlays = 8

// kcondandFlyers is every non-Lemur flyer in the pool: Momo's CheckSVar
// counts these creature spells.
var kcondandFlyers = map[string]bool{
	"Stoic Sphinx": true, "Wind Drake": true, "Ornithopter": true,
}

// kcondandHumans and kcondandNonHumans are the creature cards of the
// white-blue deck that Of One Mind's IsPresent$ (Human) and CheckSVar$
// (nonHuman) children count.
var kcondandHumans = map[string]bool{"Elite Vanguard": true}
var kcondandNonHumans = map[string]bool{
	"Momo, Friendly Flier": true, "Stoic Sphinx": true, "Wind Drake": true,
	"Ornithopter": true, "Incisor Steed": true,
}

// kcondandLive counts, at one decision, the precondition for Of One Mind's
// conjunct-only Human discount: Of One Mind in the active player's hand
// with a Human and a non-Human in play (mindPre), and the subcase where
// the only Human is a changeling, which has every creature type (CR
// 702.73a), next to some other non-Human (changelingPre). This counts the
// precondition, not the discount actually applied; the Of One Mind casts
// and the 0-divergence lockstep are what prove the cast path.
func kcondandLive(g *GorgeSide) (mind, changeling int) {
	if g == nil {
		return 0, 0
	}
	var human, nonHuman, hand, auto [8]bool
	for i := range g.E.G.Objs {
		o := &g.E.G.Objs[i]
		if o.Face() == nil || int(o.Controller) >= len(human) {
			continue
		}
		n := o.Face().Name
		switch {
		case o.Zone == state.ZBattlefield:
			human[o.Controller] = human[o.Controller] || kcondandHumans[n]
			nonHuman[o.Controller] = nonHuman[o.Controller] || kcondandNonHumans[n]
			auto[o.Controller] = auto[o.Controller] || n == "Universal Automaton"
		case o.Zone == state.ZHand && n == "Of One Mind":
			hand[o.Controller] = true
		}
	}
	a := g.E.G.Active
	if int(a) >= len(human) || !hand[a] {
		return 0, 0
	}
	if (human[a] || auto[a]) && (nonHuman[a] || auto[a]) {
		mind = 1
	}
	if !human[a] && auto[a] && nonHuman[a] {
		changeling = 1
	}
	return mind, changeling
}

// kcondandMomoScan counts, from gorge's event log, the first and every
// later non-Lemur flyer cast on its caster's turn while that player
// controls Momo (the EQ0 gate's true and false arms).
func kcondandMomoScan(g *GorgeSide) (first, later int) {
	if g == nil {
		return 0, 0
	}
	var seen [8]bool
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TurnChange:
			seen = [8]bool{}
		case events.PutOnStack:
			if int(ev.Player) >= len(seen) || ev.Player != g.E.G.Active {
				continue
			}
			ob := g.E.G.Obj(ev.Obj)
			if ob == nil || ob.Face() == nil || !kcondandFlyers[ob.Face().Name] {
				continue
			}
			momo := false
			for i := range g.E.G.Objs {
				m := &g.E.G.Objs[i]
				if m.Zone == state.ZBattlefield && m.Controller == ev.Player &&
					m.Face() != nil && m.Face().Name == "Momo, Friendly Flier" {
					momo = true
					break
				}
			}
			if !momo {
				continue
			}
			if seen[ev.Player] {
				later++
			} else {
				first++
				seen[ev.Player] = true
			}
		}
	}
	return first, later
}

// kcondandPoisoned reports whether either player finished the game with
// three or more poison counters, i.e. Incisor Steed's HighestOppPoison
// child reached its true arm at some point.
func kcondandPoisoned(g *GorgeSide) bool {
	if g == nil {
		return false
	}
	for p := range g.E.G.Players {
		if g.E.G.Players[p].Counter("POISON") >= 3 {
			return true
		}
	}
	return false
}

func kcondandSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("condand")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestCondandLockstep(t *testing.T) {
	t.Parallel()

	reg, set := kcondandSetup(t)
	games, first, later, poisoned, mindPre, changelingPre := 0, 0, 0, 0, 0, 0
	played := map[string]int{}
	stalls := map[string]int{}
	for mi, md := range kcondandDecks {
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
		// A subtype only a conjunct names (Human in Of One Mind's IsPresent$) must reach the matchup's subtype table, or the
		// conjunct's filter becomes NoSubtype and never matches a changeling.
		for _, want := range []string{"Human"} {
			found := false
			for _, n := range tbl.Subtypes {
				found = found || n == want
			}
			if !found {
				t.Errorf("matchup %d: subtype %s dropped from the table %v", mi, want, tbl.Subtypes)
			}
		}
		for seed := uint64(1); seed <= 256; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcondand-a", "kcondand-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			inner := eagerPolicy(seed, played)
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					gs = c.G
					m, ch := kcondandLive(gs)
					mindPre += m
					changelingPre += ch
				}
				return inner.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls[res.Stall]++
			}
			f, l := kcondandMomoScan(gs)
			first += f
			later += l
			if kcondandPoisoned(gs) {
				poisoned++
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	}
	names := make([]string, 0, len(played))
	for n := range played {
		names = append(names, n)
	}
	sort.Strings(names)
	var plays []string
	for _, n := range names {
		plays = append(plays, fmt.Sprintf("%s:%d", n, played[n]))
	}
	t.Logf("%d games, carrier plays %v, kernel stalls %v, Momo gate first %d later %d, poisoned games %d, Of One Mind discount preconditions %d (changeling-only Human %d)",
		games, strings.Join(plays, " "), stalls, first, later, poisoned, mindPre, changelingPre)
	for _, n := range kcondandCarriers {
		if played[n] < kcondandMinPlays {
			t.Errorf("carrier %s played %d times, want at least %d", n, played[n], kcondandMinPlays)
		}
	}
	if first == 0 || later == 0 {
		t.Errorf("Momo's EQ0 gate did not flip both ways (first %d, later %d)", first, later)
	}
	if poisoned == 0 {
		t.Error("no game reached three poison counters, so Incisor Steed's HighestOppPoison child never held")
	}
	if mindPre < kcondandMinPlays || changelingPre < kcondandMinPlays {
		t.Errorf("conjunct-only Human preconditions barely reached: Of One Mind %d, changeling-only %d (want at least %d each)",
			mindPre, changelingPre, kcondandMinPlays)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}
