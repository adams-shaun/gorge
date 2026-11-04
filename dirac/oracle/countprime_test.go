package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Count$ heads lockstep: Count$Delirium as a modal
// spell's CharmNum$ (Let's Play a Game), Count$InOwnMainPhase (Dose of
// Dawnglow's blight check), Count$ValidLibrary (Ugin's -11), Count$IsPrime
// (Zimone's end-step trigger) and Count$UnlockedDoors (Rampaging Soulrager)
// against Rooms and mana creatures. The counters read gorge's state.

var kcnt8Decks = [2][]cardset.Entry{
	{
		{Name: "Zimone, All-Questioning", Count: 4}, {Name: "Dose of Dawnglow", Count: 4}, {Name: "Ugin, Eye of the Storms", Count: 2},
		{Name: "Mind Stone", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Let's Play a Game", Count: 2}, {Name: "Divination", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Rampaging Soulrager", Count: 4}, {Name: "Bottomless Pool", Count: 3}, {Name: "Meat Locker", Count: 3},
		{Name: "Surgical Suite", Count: 3}, {Name: "Let's Play a Game", Count: 4}, {Name: "Duress", Count: 3},
		{Name: "Raise Dead", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 3}, {Name: "Plains", Count: 2},
	},
}

type kcnt8Stats struct {
	games, divs, stalls int

	game         int
	seenStack    map[[2]int]bool // (game, obj) a stack object already classified
	seenPrimo    map[[2]int]bool
	seenPump     map[[2]int]bool // (game, turn) Soulrager pumped
	evSeen       int
	evGame       int
	letsMulti    int // Let's Play a Game cast with 2+ modes chosen
	letsCast     int
	lpModes      map[[2]int]int
	letsDelirium int // Let's Play a Game cast with delirium (4+ card types in its controller's graveyard)
	doseIn       int // Dose of Dawnglow cast in its controller's own main phase
	doseOut      int
	primo        int // Primo tokens made
	pumped       int // game-turns with a Rampaging Soulrager at power 4+
	ugin11       int // library searches (Ugin's -11 is the only searcher)
}

func (st *kcnt8Stats) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	for _, id := range g.Stack {
		key := [2]int{st.game, int(id)}
		if st.seenStack[key] {
			continue
		}
		st.seenStack[key] = true
		o := g.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Let's Play a Game":
			if kcnt8Types(g, o.Controller) >= 4 {
				st.letsDelirium++
			}
			st.letsCast++
		case "Dose of Dawnglow":
			if g.Step.IsMain() && g.Active == o.Controller {
				st.doseIn++
			} else {
				st.doseOut++
			}
		}
	}
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Primo, the Indivisible":
				if key := [2]int{st.game, int(id)}; !st.seenPrimo[key] {
					st.seenPrimo[key] = true
					st.primo++
				}
			case "Rampaging Soulrager":
				if ref.E.Power(id) >= 4 {
					if key := [2]int{st.game, int(g.Turn)}; !st.seenPump[key] {
						st.seenPump[key] = true
						st.pumped++
					}
				}
			}
		}
	}
	evs := ref.E.L.Events
	if st.evGame != st.game {
		st.evGame, st.evSeen = st.game, 0
	}
	for ; st.evSeen < len(evs); st.evSeen++ {
		switch e := evs[st.evSeen]; e.Kind {
		case events.SearchedLibrary:
			st.ugin11++
		case events.ModeChosen:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Let's Play a Game" {
				key := [2]int{st.game, int(e.Obj)}
				if st.lpModes[key]++; st.lpModes[key] == 1 && strings.Contains(e.Text, ",") {
					st.letsMulti++ // the announcement's csv names 2+ modes
				}
			}
		}
	}
	return nil
}

// kcnt8Types counts the card types among p's graveyard.
func kcnt8Types(g *state.Game, p state.PlayerID) int {
	var land, cre, ins, sor, art, enc bool
	for _, id := range g.Zone(state.ZGraveyard, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil {
			f := o.Face()
			land, cre, ins, sor = land || f.IsLand(), cre || f.IsCreature(), ins || f.IsInstant(), sor || f.IsSorcery()
			art, enc = art || f.IsArtifact(), enc || f.IsEnchantment()
		}
	}
	n := 0
	for _, b := range [...]bool{land, cre, ins, sor, art, enc} {
		if b {
			n++
		}
	}
	return n
}

// kcnt8Policy casts, activates, unlocks and attacks whenever offered (Dose
// of Dawnglow only one offer in six, so it lands in both phases), else
// random.
func kcnt8Policy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	coin := rand.New(rand.NewPCG(seed, 0x636e7438)) // "cnt8"
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		var all []Canon
		for _, a := range c.D.Options {
			dose := false
			if a.Kind() == ActCast && c.G != nil && int(a.Src()) < len(c.G.id.obj) {
				if ob := c.G.E.G.Obj(c.G.id.obj[a.Src()]); ob != nil && ob.Face() != nil && ob.Face().Name == "Dose of Dawnglow" {
					dose = true
				}
			}
			all = append(all, a)
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActSpecial, ActAttack, ActPlayLand:
				if dose && coin.IntN(6) != 0 {
					continue
				}
				buf = append(buf, a)
			}
		}
		d := *c.D
		d.Options = all
		if len(buf) > 0 {
			d.Options = buf
		}
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runCountprime plays the matchup over seeds 1..seeds; mut sets DebugPrimeOdd.
func runCountprime(t *testing.T, seeds int, mut bool) *kcnt8Stats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("countprime")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugPrimeOdd
	dirac.DebugPrimeOdd = mut
	defer func() { dirac.DebugPrimeOdd = old }()
	st := &kcnt8Stats{seenStack: map[[2]int]bool{}, seenPrimo: map[[2]int]bool{}, seenPump: map[[2]int]bool{}, lpModes: map[[2]int]int{}}
	var decks [2][]*cards.Card
	for s, d := range kcnt8Decks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kcnt8Decks[0], kcnt8Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcnt8-a", "kcnt8-b"}, decks, 1)
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
		st.game++
		res := LockstepLed(m, seed, g, k, kcnt8Policy(seed), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25, Check: st.check})
		CloseSide(k)
		st.games++
		if d := res.Div; d != nil {
			st.divs++
			if !mut {
				lo := max(d.At-10, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			if !mut {
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[max(len(res.Actions)-14, 0):]))
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
	}
	return st
}

func TestCountHeadsDeliriumPrimeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runCountprime(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls; Let's Play a Game %d of %d casts chose 2+ modes (%d with delirium); Dose of Dawnglow %d in / %d out of its main phase; %d Primo tokens; %d Soulrager pumped game-turns; %d library searches (Ugin -11)",
		st.games, st.divs, st.stalls, st.letsMulti, st.letsCast, st.letsDelirium, st.doseIn, st.doseOut, st.primo, st.pumped, st.ugin11)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.doseIn < 10 || st.doseOut < 10 {
		t.Errorf("Dose of Dawnglow %d in / %d out of its main phase, want >= 10 each", st.doseIn, st.doseOut)
	}
	if st.primo < 5 {
		t.Errorf("%d Primo tokens, want >= 5", st.primo)
	}
	if st.pumped < 10 {
		t.Errorf("%d pumped Soulrager game-turns, want >= 10", st.pumped)
	}
	if st.letsMulti < 3 {
		t.Errorf("%d multi-mode Let's Play a Game, want >= 3", st.letsMulti)
	}
}

// TestCountHeadsDeliriumPrimeMutation reads "prime" as "odd": Zimone's end-step trigger then
// fires on odd non-primes (1, 9) and misses 2, so the games must diverge.
func TestCountHeadsDeliriumPrimeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runCountprime(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left no divergence in %d games", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
