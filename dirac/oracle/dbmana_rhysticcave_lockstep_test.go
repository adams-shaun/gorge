package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Rhystic Cave is a CR 605.1a mana ability whose DB$ Mana carries
// UnlessCost$ 1 | UnlessPayer$ Player. Gorge's payment planner defers the
// source (its head asks a colour), so only a potential play's prefix search
// ever activates it, and that search ends at the opponent's unless ask (any
// ask of another player dead-ends a line, rules potential_script.go visit):
// gorge never offers a play only the Cave funds, and its autopay never taps
// it. The kernel must offer the same plays, so the Cave is no census unit and
// no prefix gap (dirac askColUsable, askColUnlessDead). The Cave's own colour
// and unless asks are therefore unreachable through the oracle's canonical
// surface (no pure mana ability is a priority option and gorge's search never
// finishes an activation); dirac's TestManaUnlessCaveAsksInSeatOrderAndOpponentPays
// and TestManaUnlessPayerTapAnotherPickFinishes pin them in the kernel. This
// is the pool-level guard for the offer side: a Forest + Cave board, a
// {1}{G} creature only the Cave could fund, and a Spree spell whose {1} modes
// only the Cave could pay.
var rhysticCaveDecks = [2][]cardset.Entry{
	{{Name: "Forest", Count: 1}, {Name: "Rhystic Cave", Count: 34}, {Name: "Grizzly Bears", Count: 3}, {Name: "Trash the Town", Count: 2}},
	{{Name: "Plains", Count: 10}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10}},
}

// rhysticCaveSet builds the kernel table the decks need: Rhystic Cave is no
// synthetic pool's card (Prophecy), so the table is a test artifact generated
// from the corpus, never committed.
func rhysticCaveSet(t *testing.T, cards [2][]string) cardset.Set {
	t.Helper()
	var names []string
	for _, d := range cards {
		names = append(names, d...)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "dbmana-rhysticcave", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "dbmana-rhysticcave.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	return cardset.Raw(cardset.NewDataSet(pd))
}

// rhysticCavePolicy plays the Spree spell's {1} trample mode and then Done; any
// other decision is castActivatePlayedPolicy's.
func rhysticCavePolicy(seed uint64, played map[string]int) Policy {
	rnd := castActivatePlayedPolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DModes {
			for _, o := range c.D.Options {
				if o.Kind() == ActMode && o.Mode() == 1 {
					return o
				}
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActDone {
					return o
				}
			}
		}
		return rnd.Pick(c)
	})
}

// TestDbManaRhysticCaveLockstep: four seeds over the Cave fixture, alternating
// the gorge autopay surface (seed parity), 0 divergent. The fixture's opening
// board is a Forest, a Cave and a creature, with a {1}{G} creature and a Spree
// spell in hand: the {1}{G} creature is payable only with the Cave's mana, so
// gorge withholds it (its search dead-ends at the unless ask) and the kernel
// must too (it offered the cast before the Cave left the census and the gap
// mask: "options" divergence at the first priority with the Forest and Cave
// untapped, ref [pass], kernel [pass cast cast]). The Spree spell is cast,
// asked its modes on both sides, and aborted when its {1} cannot be paid: no
// Cave-funded mode is ever paid.
func TestDbManaRhysticCaveLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	var cs [2][]*cards.Card
	var names [2][]string
	for s, d := range rhysticCaveDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
				names[s] = append(names[s], e.Name)
			}
		}
	}
	set := rhysticCaveSet(t, names)
	tbl, _, _, err := set.Matchup(rhysticCaveDecks[0], rhysticCaveDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	at := map[string][]int{}
	for i, n := range names[0] {
		at[n] = append(at[n], i)
	}
	m0, err := NewMatchup(reg, [2]string{"rhystic-a", "rhystic-b"}, cs, 0)
	if err != nil {
		t.Fatal(err)
	}
	m0.Start = &rules.StartSpec{Seats: []rules.StartSeat{
		{Battlefield: []int{at["Forest"][0], at["Rhystic Cave"][0], at["Grizzly Bears"][0]}, Hand: []int{at["Grizzly Bears"][1], at["Trash the Town"][0]}},
		{Battlefield: []int{0, 1, 2}, Hand: []int{3, 4, 5, 6}},
	}}
	played := map[string]int{}
	cavePlayed, caveUses := 0, 0
	for seed := uint64(1); seed <= 4; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m := *m0
		k, err := SetSide(set, tbl, cfg)(&m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			if cov := CoverageOf(g, &m); cov != nil {
				if cc := cov.Cards["Rhystic Cave"]; cc != nil {
					cavePlayed += cc.Land
					for _, n := range cc.Used {
						caveUses += n
					}
				}
			}
		}
		res := Lockstep(&m, seed, k, rhysticCavePolicy(seed, played), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if cavePlayed == 0 {
		t.Fatal("the fixture never played a second Rhystic Cave: the Cave is not on the board beside the Forest")
	}
	if played["Trash the Town"] == 0 {
		t.Fatal("the fixture never cast the Spree spell: its {1} modes are not compared")
	}
	if n := played["Grizzly Bears"]; n != 0 {
		t.Fatalf("Grizzly Bears was cast %d times: only the Cave could pay its {1}{G}, and gorge's search never offers such a play", n)
	}
	t.Logf("4 games, %d Cave land plays, %d Spree casts, %d Cave ability uses, 0 divergent", cavePlayed, played["Trash the Town"], caveUses)
}
