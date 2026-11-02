package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The selfcond lockstep: constructed matchups over the generated
// selfcond pool, whose carriers are layer-4 override statics whose
// condition counts the permanents the static itself rewrites (the layer
// walk's re-entry guard, dirac/static_reentryguard.go):
//
//   - Grond, the Gatebreaker: Condition$ PlayerTurn + IsPresent$
//     Army.YouCtrl, AddType$ Artifact & Creature. Counting Armies reads
//     every permanent's subtypes, Grond's own among them, through the walk
//     that is evaluating Grond's static (it overflowed the stack beside a
//     changeling). The Armies are Amass tokens (Lazotep Reaver, Mordor
//     Muster, Grim Initiate) and changelings (Universal Automaton,
//     Woodland Changeling: every creature type, CR 702.73a).
//   - Rusted Relic: Metalcraft (IsPresent$ Artifact.YouCtrl GE3), a 5/5
//     artifact creature Golem; the artifact count reads Relic's own types.
//
// The scan counts, at every decision, the carriers on the battlefield
// with their gate holding (gorge's derived types make them creatures) and
// failing, so both arms of each re-entered condition are reached.

var kselfcondDecks = [][2][]cardset.Entry{{
	{
		{Name: "Grond, the Gatebreaker", Count: 4}, {Name: "Lazotep Reaver", Count: 4},
		{Name: "Mordor Muster", Count: 3}, {Name: "Grim Initiate", Count: 3},
		{Name: "Universal Automaton", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Gray Ogre", Count: 2},
		{Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Rusted Relic", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Universal Automaton", Count: 3}, {Name: "Woodland Changeling", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Grond, the Gatebreaker", Count: 2},
		{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 3},
	},
}, {
	{
		{Name: "Grond, the Gatebreaker", Count: 4}, {Name: "Woodland Changeling", Count: 4},
		{Name: "Universal Automaton", Count: 4}, {Name: "Lazotep Reaver", Count: 3},
		{Name: "Rusted Relic", Count: 3}, {Name: "Ornithopter", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Mordor Muster", Count: 4}, {Name: "Grim Initiate", Count: 4},
		{Name: "Grond, the Gatebreaker", Count: 3}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 8},
	},
}}

// kselfcondCarriers are the cards whose gate the scan counts: each must be
// seen on the battlefield with its gate holding and failing at least
// kselfcondMinSeen decisions apiece.
var kselfcondCarriers = []string{"Grond, the Gatebreaker", "Rusted Relic"}

const kselfcondMinSeen = 20

// kselfcondScan counts, on gorge's board, each carrier with its gate
// holding (a creature) and failing; chg counts Grond animated while its
// controller controls a changeling and no Army token (the shape that
// overflowed).
func kselfcondScan(g *GorgeSide, on, off map[string]int, chg *int) {
	for i := range g.E.G.Objs {
		o := &g.E.G.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil {
			continue
		}
		n := o.Face().Name
		if n != "Grond, the Gatebreaker" && n != "Rusted Relic" {
			continue
		}
		if !g.E.IsCreature(o.ID) {
			off[n]++
			continue
		}
		on[n]++
		if n != "Grond, the Gatebreaker" {
			continue
		}
		changeling, token := false, false
		for j := range g.E.G.Objs {
			a := &g.E.G.Objs[j]
			if a.Zone != state.ZBattlefield || a.Controller != o.Controller || a.Face() == nil {
				continue
			}
			switch an := a.Face().Name; {
			case an == "Universal Automaton" || an == "Woodland Changeling":
				changeling = true
			case strings.Contains(an, "Army"):
				token = true
			}
		}
		if changeling && !token {
			*chg++
		}
	}
}

func kselfcondSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("selfcond")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestSelfcondLockstep(t *testing.T) {
	t.Parallel()

	reg, set := kselfcondSetup(t)
	games, chg := 0, 0
	played := map[string]int{}
	on, off := map[string]int{}, map[string]int{}
	stalls := map[string]int{}
	for mi, md := range kselfcondDecks {
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
		for seed := uint64(1); seed <= 128; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kselfcond-a", "kselfcond-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			inner := eagerPolicy(seed, played)
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					kselfcondScan(c.G, on, off, &chg)
				}
				return inner.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls[res.Stall]++
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
	t.Logf("%d games, plays %v, gate on %v off %v, Grond on beside a changeling only %d, kernel stalls %v",
		games, strings.Join(plays, " "), on, off, chg, stalls)
	for _, n := range kselfcondCarriers {
		if on[n] < kselfcondMinSeen || off[n] < kselfcondMinSeen {
			t.Errorf("%s: gate held at %d and failed at %d decisions, want at least %d each", n, on[n], off[n], kselfcondMinSeen)
		}
	}
	if chg < kselfcondMinSeen {
		t.Errorf("Grond animated by a changeling alone at %d decisions, want at least %d", chg, kselfcondMinSeen)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}
