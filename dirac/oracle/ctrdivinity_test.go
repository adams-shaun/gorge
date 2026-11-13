package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated
// ctrdivinity pool, whose carriers put named counters with no inherent rules
// meaning (CR 122.1): DIVINITY (Myojin of Night's Reach) and IMPOSTOR
// (Illicit Masquerade). The kernel keeps each kind in a permanent's sparse
// counter slots and projects it as ProjObj.CountersX[k - CtrCharge]; gorge
// keeps the bare name string.
//
// Myojin's K:etbCounter puts its divinity counter only when it is cast from
// hand, its counters_GE1_DIVINITY static grants indestructible while it has
// one, and its activated ability's cost removes one. Illicit Masquerade's
// enters trigger puts an impostor counter on each creature you control and
// its dies trigger reads Creature.YouCtrl+counters_GE1_IMPOSTOR. Cheap
// black and green creatures and removal (Murder, Doom Blade) put creatures
// on the battlefield and kill the marked ones; Stitcher's Supplier
// self-mills so the dies trigger can return a creature card.

var kctrnamed3Decks = [2][]cardset.Entry{
	// the B/G deck: Myojin and Illicit Masquerade with Llanowar Elves for
	// the eight mana, cheap black creatures and black removal
	{
		{Name: "Myojin of Night's Reach", Count: 2}, {Name: "Illicit Masquerade", Count: 2},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Vampire Neonate", Count: 4}, {Name: "Murder", Count: 2},
		{Name: "Swamp", Count: 14}, {Name: "Forest", Count: 8},
	},
	// the B/G deck: the same named-counter carriers beside Stitcher's
	// Supplier (self-mill) and both removal spells so the impostor-marked
	// creatures die and the dies trigger fires
	{
		{Name: "Myojin of Night's Reach", Count: 2}, {Name: "Illicit Masquerade", Count: 2},
		{Name: "Stitcher's Supplier", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Murder", Count: 2}, {Name: "Doom Blade", Count: 2},
		{Name: "Swamp", Count: 12}, {Name: "Forest", Count: 8},
	},
}

func kctrnamed3Setup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ctrdivinity")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func kctrnamed3DecksOf(t *testing.T, reg *cards.Registry) [2][]*cards.Card {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range kctrnamed3Decks {
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
	return decks
}

// kctrnamed3Seen counts the events the minimums read, across games.
type kctrnamed3Seen struct {
	myojinDivinity int // a Myojin entered the battlefield with a divinity counter (a hand cast)
	myojinActivate int // a Myojin activated ability was put on the stack
	masqueradeETB  int // an Illicit Masquerade entered the battlefield
	impostorDies   int // an Illicit Masquerade dies trigger was put on the stack
}

// kctrnamed3Watch is one game's hook state: the battlefield presence at the
// previous decision and the previous stack, so an entry is counted once.
type kctrnamed3Watch struct {
	on     [dirac.MaxObj]bool
	stack  [dirac.MaxStack]dirac.ProjStack
	nStack int
}

func (w *kctrnamed3Watch) observe(s *dirac.State, seen *kctrnamed3Seen) {
	tbl := s.Table()
	var p dirac.Projection
	s.Project(&p)
	for i := 0; i < int(p.NObj); i++ {
		o := &p.Objs[i]
		on := o.Zone == kop.ZoneBattlefield
		if on && !w.on[i] {
			switch kopCardNameLower(tbl, o.Card) {
			case "myojin of night's reach":
				// The etbCounter replacement fires only for a hand cast,
				// so a Myojin that appears with the counter was cast from
				// hand (the pool has no reanimation).
				if o.CountersX[int(kop.CtrDivinity)-int(kop.CtrCharge)] > 0 {
					seen.myojinDivinity++
				}
			case "illicit masquerade":
				seen.masqueradeETB++
			}
		}
		w.on[i] = on
	}
	for i := int(p.NObj); i < len(w.on); i++ {
		w.on[i] = false
	}
	for i := 0; i < int(p.NStack); i++ {
		e := p.Stack[i]
		fresh := i >= w.nStack || w.stack[i] != e
		if !fresh {
			continue
		}
		switch e.Kind {
		case 1: // an activated ability
			if kopCardNameLower(tbl, e.Card) == "myojin of night's reach" {
				seen.myojinActivate++
			}
		case 2: // a triggered ability
			if kopCardNameLower(tbl, e.Card) != "illicit masquerade" || int(e.Card) >= len(tbl.Cards) {
				continue
			}
			abs := tbl.Cards[e.Card].Abilities
			// The enters trigger puts the impostor counters; the dies
			// trigger's event is TrigDies.
			if int(e.Ability) < len(abs) && abs[e.Ability].Trig.Event == kop.TrigDies {
				seen.impostorDies++
			}
		}
	}
	w.nStack = int(p.NStack)
	copy(w.stack[:], p.Stack[:p.NStack])
}

// kctrnamed3Run plays seeds 1..seeds of the matchup through tbl; it returns
// the games played, whether any diverged (the first is logged) and the
// stalls, and adds the observed events to seen.
func kctrnamed3Run(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, seeds uint64, seen *kctrnamed3Seen, stopOnDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	decks := kctrnamed3DecksOf(t, reg)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kctrnamed3-a", "kctrnamed3-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var w kctrnamed3Watch
		prev := OccupancyHook
		OccupancyHook = func(s *dirac.State) { w.observe(s, seen) }
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		OccupancyHook = prev
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div = true
			if !stopOnDiv {
				t.Logf("seed %d: diverged at decision %d: %s %s", seed, d.At, d.What, d.Field)
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestNamedCountersDivinityLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set := kctrnamed3Setup(t)
	tbl, _, _, err := set.Matchup(kctrnamed3Decks[0], kctrnamed3Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var seen kctrnamed3Seen
	games, div, stalls := kctrnamed3Run(t, reg, set, tbl, 300, &seen, true)
	t.Logf("%d games, diverged %v, stalls %d, seen %+v", games, div, stalls, seen)
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	for _, m := range []struct {
		what      string
		got, want int
	}{
		{"Myojin casts from hand that entered with DIVINITY", seen.myojinDivinity, 10},
		{"Myojin divinity activations", seen.myojinActivate, 3},
		{"Illicit Masquerade ETBs", seen.masqueradeETB, 10},
		{"impostor death triggers", seen.impostorDies, 5},
	} {
		if m.got < m.want {
			t.Errorf("%s: %d, want at least %d", m.what, m.got, m.want)
		}
	}
}

// TestNamedCountersDivinityMutation: the kernel's counter placement rewritten to drop
// DIVINITY counters makes the lockstep diverge within 128 seeds (gorge's
// Myojin enters with the counter, the kernel's does not, so gorge's offers
// the SubCounter activation and grants indestructible).
func TestNamedCountersDivinityMutation(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set := kctrnamed3Setup(t)
	tbl, _, _, err := set.Matchup(kctrnamed3Decks[0], kctrnamed3Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugCtrXDropDivinity = true
	defer func() { dirac.DebugCtrXDropDivinity = false }()
	var seen kctrnamed3Seen
	_, div, _ := kctrnamed3Run(t, reg, set, tbl, 128, &seen, false)
	if !div {
		t.Fatalf("divinity counters dropped: no divergence in 128 seeds")
	}
}
