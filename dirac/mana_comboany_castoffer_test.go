package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// comboCastScen builds the std-msh pair 817 board at the diverging priority
// decision (seed 12867060247504712102): two Forests, a Los Diablos Missile
// Base ({R} or {G}), a Castle Doom ({C}; any colour only for artifacts) and
// a Baxter Building ({T}: Add {C}; "{4}, {T}: Add four mana in any
// combination of colors") beside Wiccan, Rising Magician ({4}{U}) in the
// hand, the floating pool empty. The only {U} the board can make is
// Baxter's combination activation (its {4} fee paid from the other four
// sources), with Surveillance Room's "{1}, {T}: one mana of any color" the
// fifth mana the chain then funds, so the cast offer prices through the
// filter gaps census (CR 605.3a, 601.2g-h).
func comboCastScen(t *testing.T) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Baxter Building", Count: 1},
		{Name: "Wiccan, Rising Magician", Count: 1},
		{Name: "Castle Doom", Count: 1},
		{Name: "Los Diablos Missile Base", Count: 1},
		{Name: "Surveillance Room", Count: 1},
		{Name: "Forest", Count: 10},
	}
	set := cardsettest.Synth(t, "std-msh")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	// the lockstep ran the full surface (lowering, no autopay)
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	ld := sc.perm(id["Los Diablos Missile Base"], 0)
	sc.perm(id["Castle Doom"], 0)
	sc.perm(id["Baxter Building"], 0)
	sc.perm(id["Surveillance Room"], 0)
	sc.s.perms[sc.slot(ld)].flags &^= pfTapped // its ETB-tapped replacement fired; the game's copy was untapped
	wiccan := sc.hand(id["Wiccan, Rising Magician"], 0)
	return sc, wiccan
}

func comboCastOffered(sc *scen, wiccan Obj) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && Obj(a.Source()) == wiccan && a.Mode()&3 == 0 {
			return true
		}
	}
	return false
}

// TestComboAnyFundsCastOffer: gorge's planner prices a plain cast whose only
// coloured pips a combination filter funds, so the kernel's priority list
// carries it (CR 605.3a, 601.2g-h; std-msh pair 817's Wiccan cast beside
// Baxter Building, the ticket's Dark Deed and Super-Skrull games the same
// shape over black).
func TestComboAnyFundsCastOffer(t *testing.T) {
	sc, wiccan := comboCastScen(t)
	if !comboCastOffered(sc, wiccan) {
		d := sc.s.Step()
		t.Fatalf("Wiccan's {4}{U} not offered over Baxter's combination filter; opts %v", d.Options[:d.N])
	}
}

// TestComboAnyFundsCastPay pins the payment the offer's script funds: the
// Forests, the Missile Base and Castle Doom pay Baxter's {4} fee (CR 605.3b),
// the combination's four join the pool, Surveillance Room's {C} is the fifth
// mana, and the {4}{U} cost takes one {U} and four generic -- the board is
// spent to the last mana.
func TestComboAnyFundsCastPay(t *testing.T) {
	sc, wiccan := comboCastScen(t)
	d := sc.s.Step()
	var cast Action
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && Obj(a.Source()) == wiccan && a.Mode()&3 == 0 {
			cast = a
			break
		}
	}
	if cast == 0 {
		t.Fatalf("Wiccan not offered: %v", d.Options[:d.N])
	}
	sc.s.Apply(cast) // the build is pending: the announcement and payment steps follow
	for i := 0; i < 12 && sc.s.objs[wiccan].zone == kop.ZoneHand; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			break
		}
		if d.Kind != DkPriority {
			sc.s.Apply(d.Options[0])
		}
	}
	// the payment is part of the build: a paid cast is on the stack (the
	// pool drained), an unpaid one was reversed to the hand (CR 733.1)
	if z := sc.s.objs[wiccan].zone; z != kop.ZoneStack {
		t.Fatalf("Wiccan in zone %d after the cast, want the stack (pushed and paid)", z)
	}
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("pool %v after the payment, want empty: the script left mana floating", pool)
	}
}

// TestFeeOrderArmIsPlainCastOnly: gorge arms its feeOrder run only for a cast
// of no mode at all (potentialPrefixScript, o.Mode == ""), so the kernel's arm
// takes the plain cast and refuses every cast mode: free, face-down, warp
// recast. The precondition is the plain cast of the same board arming it.
func TestFeeOrderArmIsPlainCastOnly(t *testing.T) {
	sc, wiccan := comboCastScen(t)
	s, k := sc.s, sc.k
	gaps := s.scriptFilt(k, 0)
	arm := func(a Action) bool {
		fs := filtSearch{gaps: gaps, nodes: potentialScriptNodes} // the plain run spent its budget
		return s.feeOrderFallback(k, 0, a, &fs)
	}
	if !arm(MakeAction(AkCast, TCode(wiccan), 0, 0)) {
		t.Fatal("precondition: the plain cast beside Baxter's combination filter must arm the fee-order run")
	}
	for _, mode := range []uint8{castFree, castFaceDown, castWarpRecast} {
		if arm(MakeAction(AkCast, TCode(wiccan), 0, mode)) {
			t.Errorf("cast mode %#x armed the fee-order run, gorge's build leaves feeOrder unset for it", mode)
		}
	}
}
