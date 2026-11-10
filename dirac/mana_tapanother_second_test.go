package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A second mana ability "{T}, tap an untapped creature you control: Add one
// mana of any color" beside a plain "{T}: Add {C}" first one (Scene of the
// Crime, Holdout Settlement, Survivors' Encampment). The source is a planner
// {C} unit through its first ability and, with a creature to tap, a census gap
// through its second (gorge's relaxed census appends that free any-colour
// unit beside the {C}); the prefix search answers the activation's ability
// ask with the plain ability first, then the tap-another one (CR 605.3b,
// 601.2g).

// sceneScen: p0 controls board (none summoning sick, all untapped) and holds Bubble
// Smuggler ({1}{U}); the std-mkm pool.
func sceneScen(t *testing.T, board ...string) (sc *scen, smuggler Obj, slots []Slot, id map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Bubble Smuggler", Count: 1}, {Name: "Forest", Count: 10}}
	seen := map[string]bool{}
	for _, n := range board {
		if !seen[n] {
			seen[n] = true
			deck = append(deck, cardset.Entry{Name: n, Count: 1})
		}
	}
	set := cardsettest.Synth(t, "std-mkm")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	for _, n := range board {
		x := sc.slot(sc.perm(id[n], 0))
		sc.s.perms[x].flags &^= pfTapped // Scene of the Crime enters tapped
		slots = append(slots, x)
	}
	smuggler = sc.hand(id["Bubble Smuggler"], 0)
	return sc, smuggler, slots, id
}

// sceneIsSecond asserts the precondition every test here reads: the kernel
// sees Scene of the Crime's tap-another ability as a later one than its
// first mana ability.
func sceneIsSecond(t *testing.T, sc *scen, scene CardID) {
	t.Helper()
	if !sc.k.tapMana || !sc.k.tapXSecond(scene) {
		t.Fatalf("Scene of the Crime: tapMana %v, tapXAb %d, manaAb %d: want its second mana ability as the tap-another one",
			sc.k.tapMana, sc.k.tapXAb[scene], sc.k.cards[scene].manaAb)
	}
	a := sc.k.tapXAbOf(scene)
	if a == nil || !kop.TapXManaAb(a) || a.ManaColors != kop.ColAny {
		t.Fatalf("Scene of the Crime's tap-another ability %+v: want the any-colour TapXManaAb shape", a)
	}
}

// TestSecondTapXSourceIsAGap: an untapped Scene of the Crime with an untapped
// creature to tap is a tap-another gap (its any-colour unit in the gaps walk)
// and still a planner source of {C}; a summoning-sick creature is a
// candidate too (CR 302.6 restricts only the creature's own {T} abilities).
func TestSecondTapXSourceIsAGap(t *testing.T) {
	for _, sick := range []bool{false, true} {
		sc, _, sl, id := sceneScen(t, "Scene of the Crime", "Aftermath Analyst")
		sceneIsSecond(t, sc, id["Scene of the Crime"])
		scene, analyst := sl[0], sl[1]
		if sick {
			sc.s.perms[analyst].flags |= pfSick
		}
		if sc.s.perms[analyst].flags&pfTapped != 0 {
			t.Fatal("precondition: the Analyst is tapped")
		}
		if !sc.s.scriptTapX(sc.k, 0).has(scene) {
			t.Fatalf("sick %v: Scene of the Crime with an untapped creature is no tap-another gap", sick)
		}
		var cs [MaxPerm]Slot
		if n := sc.s.tapXCands(sc.k, 0, scene, &cs); n != 1 || cs[0] != analyst {
			t.Fatalf("sick %v: candidates %v, want the Analyst alone (the source's own {T} is the cost's)", sick, cs[:n])
		}
		if !sc.s.canTapForMana(sc.k, scene) {
			t.Fatalf("sick %v: Scene of the Crime is no planner source: its first ability's {C} is", sick)
		}
		var av avail
		sc.s.availTapX(sc.k, 0, &av)
		var none avail
		if av == none {
			t.Fatalf("sick %v: the gaps walk counts no unit for the tap-another gap", sick)
		}
	}
}

// TestSecondTapXSourceNeedsACandidate: with no other creature the second
// ability cannot be paid, so the source is no prefix gap; gorge's window census
// (which does not read the tap part's payability) still leaves it unpriced.
// Tapped, it is neither.
func TestSecondTapXSourceNeedsACandidate(t *testing.T) {
	sc, _, sl, id := sceneScen(t, "Scene of the Crime", "Forest")
	sceneIsSecond(t, sc, id["Scene of the Crime"])
	scene := sl[0]
	if sc.s.scriptTapX(sc.k, 0).has(scene) {
		t.Fatal("Scene of the Crime with no creature to tap is a tap-another gap")
	}
	if !sc.s.tapXCensusGap(sc.k, 0).has(scene) {
		t.Fatal("the window census prices Scene of the Crime's tap-another ability")
	}
	sc, _, sl, _ = sceneScen(t, "Scene of the Crime", "Aftermath Analyst")
	scene = sl[0]
	sc.s.perms[scene].flags |= pfTapped
	if sc.s.scriptTapX(sc.k, 0).has(scene) || sc.s.tapXCensusGap(sc.k, 0).has(scene) {
		t.Fatal("a tapped Scene of the Crime is a tap-another gap")
	}
}

// TestSecondTapXPaysAColouredPip: Bubble Smuggler's {1}{U} beside a Forest,
// Scene of the Crime and Aftermath Analyst is offered only through the
// second ability's {U} (the prefix search tries the Scene's {C} first, which
// leaves the {U} unpaid), and casting it taps the Scene, the Analyst and the
// Forest. Without the Analyst it is not offered.
func TestSecondTapXPaysAColouredPip(t *testing.T) {
	sc, smuggler, _, id := sceneScen(t, "Forest", "Scene of the Crime")
	sceneIsSecond(t, sc, id["Scene of the Crime"])
	d := sc.s.Step()
	if castOffered(&d, smuggler) {
		t.Fatal("Bubble Smuggler offered with a Forest and Scene of the Crime alone: no blue source")
	}
	sc, smuggler, sl, _ := sceneScen(t, "Forest", "Scene of the Crime", "Aftermath Analyst")
	forest, scene, analyst := sl[0], sl[1], sl[2]
	d = sc.s.Step()
	if !castOffered(&d, smuggler) {
		t.Fatal("Bubble Smuggler not offered: Scene of the Crime tapping the Analyst makes its {U}")
	}
	sc.s.Apply(MakeAction(AkCast, uint8(smuggler), 0, 0))
	sc.s.Step()
	if sc.s.sp != 1 {
		t.Fatalf("stack %d after the cast, want 1", sc.s.sp)
	}
	for _, x := range []Slot{forest, scene, analyst} {
		if sc.s.perms[x].flags&pfTapped == 0 {
			t.Fatalf("slot %d untapped after the cast: want the Forest, the Scene and the Analyst tapped", x)
		}
	}
}
