package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Duration$ UntilUntaps and Duration$ Permanent Pump lockstep (T14a):
// Zelyon Sword's "target creature gets +2/+0 for as long as CARDNAME remains
// tapped", its six corpus siblings and the five Couriers (+2/+2 and a
// keyword), and Subversive Acolyte's charm modes (+1/+1 lifelink and +3/+2
// trample, Duration$ Permanent with a P/T part). Riding the Dilu Horse is not
// in the pool: its KW$ Horsemanship has no grant surface in the kernel (a
// separate ticket), so the card as a whole stays refused. The kernel's
// records must live exactly while gorge's grants do: the host-bound one from
// its host's untap or departure, the permanent ones forever. The pool is
// built in-process (kgen.SynthUnionTable, the focus_synthwith shape) over the
// newly lowered cards, so the run proves the lowering too: a card the fix
// does not reach leaves the pool refused and the deal fails.
//
// Subversive Acolyte's Animate charm modes carry "Types$ Human Cleric": the
// lowering splits it into the two types (gorge animateGrant's word split),
// so the kernel's granted subtype is one the gorge side can name and the
// projections agree.

var kpumpDecks = [2][]cardset.Entry{
	{
		{Name: "Zelyon Sword", Count: 3}, {Name: "Endoskeleton", Count: 3},
		{Name: "Ashnod's Battle Gear", Count: 2}, {Name: "Flowstone Armor", Count: 2},
		{Name: "Tawnos's Weaponry", Count: 2}, {Name: "Spirit Shield", Count: 2},
		{Name: "Everglove Courier", Count: 1}, {Name: "Frightshroud Courier", Count: 1},
		{Name: "Flamestick Courier", Count: 1}, {Name: "Pearlspear Courier", Count: 1},
		{Name: "Ghosthelm Courier", Count: 1},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Scathe Zombies", Count: 4},
		{Name: "Elite Vanguard", Count: 2}, {Name: "Fugitive Wizard", Count: 1},
		{Name: "Goblin Piker", Count: 1},
		{Name: "Forest", Count: 4}, {Name: "Plains", Count: 2},
		{Name: "Island", Count: 1}, {Name: "Swamp", Count: 1}, {Name: "Mountain", Count: 1},
	},
	{
		{Name: "Subversive Acolyte", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Scathe Zombies", Count: 4},
		{Name: "Shock", Count: 6},
		{Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 6},
	},
}

func runPump(t *testing.T, seeds int) (st struct {
	games, divs, stalls int
	pumped, pumpedLater int // decisions with a creature off its printed P/T; ... on a later turn than first seen
}) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, d := range kpumpDecks {
		for _, e := range d {
			names = append(names, e.Name)
		}
	}
	d, err := kgen.SynthUnionTable(names, cardsDir, "pumpuntiltaps")
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.Raw(cardset.NewDataSet(d))
	// precondition: every focus card the fix lowers actually plays here; a
	// refusal means the fix did not reach it
	for _, n := range []string{
		"Zelyon Sword", "Endoskeleton", "Ashnod's Battle Gear", "Flowstone Armor",
		"Tawnos's Weaponry", "Spirit Shield", "Everglove Courier", "Frightshroud Courier",
		"Flamestick Courier", "Pearlspear Courier", "Ghosthelm Courier",
		"Subversive Acolyte",
	} {
		i, ok := set.Lookup(n)
		if !ok || set.Status(i) == cardset.StatusNotLowered {
			t.Fatalf("the pool does not lower %s", n)
		}
	}
	var decks [2][]*cards.Card
	for s, dk := range kpumpDecks {
		for _, e := range dk {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kpumpDecks[0], kpumpDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpump-a", "kpump-b"}, decks, 1)
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
		first := map[state.ObjID]int32{}
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				for _, id := range ref.E.G.Zone(state.ZBattlefield, 0) {
					o := ref.E.G.Obj(id)
					if o == nil || o.Face() == nil || !ref.E.IsCreature(id) {
						continue
					}
					if ref.E.Power(id) != int32(o.Face().Power()) || ref.E.Toughness(id) != int32(o.Face().Toughness()) {
						st.pumped++
						if f, ok := first[id]; !ok {
							first[id] = ref.E.G.Turn
						} else if ref.E.G.Turn > f {
							st.pumpedLater++
						}
					}
				}
				return nil
			}})
		CloseSide(k)
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if dv := res.Div; dv != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, dv.At, dv.What, dv.Field, dv.Diff, dv.Detail, dv.RefOptions, dv.KerOptions)
		}
	}
	return
}

// TestPumpHostBoundLockstep: the lowered pool plays clean on both surfaces.
func TestPumpHostBoundLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runPump(t, 4)
	t.Logf("%+v", st)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.games != 4 {
		t.Fatalf("%d games, want 4", st.games)
	}
	// the pumps are reached: a creature stands off its printed P/T, and one
	// does across a turn boundary
	if st.pumped < 5 || st.pumpedLater < 1 {
		t.Errorf("pumped %d decisions (%d later), want at least 5 and 1", st.pumped, st.pumpedLater)
	}
}
