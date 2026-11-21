package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// The Phantom creatures' self-damage replacement beside an amount changer or a
// prevention record: CR 616.1, 616.1e. Unbreathing Horde prevents damage dealt to
// it and removes a +1/+1 counter; gorge collects the line in its object scan
// beside Thor's plus-one, Angrath's Marauders' doubling and Arachnogenesis's
// combat-damage prevention Effect (the Effect first), asks the order of the
// candidates and applies each pick as made, so the counter goes only when the
// Horde's line is picked or is left over when the ask ends. The kernel poses the
// same ask in combat (dirac/repl_selfph.go, spCombatCands, spMergeAmounts) and for
// a parked noncombat event (parkNC, spMergeNC).

// sphDecks: the Horde's seat (Zombies for its counters), a Thor seat, a Marauders
// seat and an Arachnogenesis seat; the red seats hold Starstorms and Fiery
// Confluences for the noncombat damage.
func sphDecks(record bool) [][]string {
	art := []any{4, "Silver Myr", 4, "Containment Construct", 3, "Steel Overseer", 3, "Millikin"}
	d := [][]string{
		spsrMix(append(art, 5, "Unbreathing Horde", 5, "Cryptbreaker", 8, "Swamp", 6, "Forest")...),
		spsrMix(append(art, 4, "Thor, Asgard's Avenger", 4, "Humble Defector", 4, "Starstorm", 3, "Fiery Confluence", 11, "Mountain")...),
		spsrMix(append(art, 4, "Angrath's Marauders", 4, "Humble Defector", 4, "Starstorm", 3, "Fiery Confluence", 11, "Mountain")...),
		spsrMix(append(art, 5, "Arachnogenesis", 4, "Humble Defector", 4, "Starstorm", 4, "Elvish Mystic", 11, "Forest")...),
	}
	// the amount changers and the prevention record are separate tables: a Phantom
	// beside both is refused at the start
	if record {
		d[1] = spsrMix(append(art, 4, "Humble Defector", 4, "Ranging Raptors", 4, "Elvish Mystic", 14, "Mountain")...)
		d[2] = d[1]
	} else {
		d[3] = spsrMix(append(art, 4, "Humble Defector", 4, "Starstorm", 4, "Elvish Mystic", 14, "Mountain")...)
	}
	return d
}

var sphSeeds = []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24}

// sphRun plays sphSeeds and counts the asks that name Unbreathing Horde beside the
// other card, combat and noncombat.
func sphRun(t *testing.T, names ...string) (div, stop string, combat, noncombat int) {
	t.Helper()
	for _, n := range names {
		d, st, c, nc := spsrPlayAsks(t, sphDecks(n == "Arachnogenesis"), sphSeeds, "Unbreathing Horde", n)
		if div == "" {
			div = d
		}
		if stop == "" {
			stop = st
		}
		combat, noncombat = combat+c, noncombat+nc
	}
	return
}

// TestFourSeatSelfPhBesideAmountsLockstep: pods holding Unbreathing Horde beside
// Thor and Angrath's Marauders play in lockstep, and games reached the order ask on
// combat damage and on noncombat damage.
func TestFourSeatSelfPhBesideAmountsLockstep(t *testing.T) {
	div, stop, combat, noncombat := sphRun(t, "Thor, Asgard's Avenger", "Angrath's Marauders")
	if div != "" || stop != "" {
		t.Fatalf("%s %s", div, stop)
	}
	if combat == 0 || noncombat == 0 {
		t.Fatalf("asks reached: combat %d, noncombat %d", combat, noncombat)
	}
}

// TestFourSeatSelfPhBesideRecordLockstep: pods holding Unbreathing Horde beside
// Arachnogenesis play in lockstep, and a game reached the combat order ask.
func TestFourSeatSelfPhBesideRecordLockstep(t *testing.T) {
	div, stop, combat, _ := sphRun(t, "Arachnogenesis")
	if div != "" || stop != "" {
		t.Fatalf("%s %s", div, stop)
	}
	if combat == 0 {
		t.Fatal("no game reached the Arachnogenesis beside Unbreathing Horde order ask")
	}
}

// TestFourSeatSelfPhMutation: with the Phantom's line left out of the ask, each
// pairing (Thor's plus-one, Marauders' doubling, Arachnogenesis's record) diverges
// or stops the kernel loud.
func TestFourSeatSelfPhMutation(t *testing.T) {
	dirac.DebugSelfPhOff = true
	defer func() { dirac.DebugSelfPhOff = false }()
	for _, n := range []string{"Thor, Asgard's Avenger", "Angrath's Marauders", "Arachnogenesis"} {
		if div, stop, _, _ := sphRun(t, n); div == "" && stop == "" {
			t.Fatalf("the mutation neither diverged nor stopped the kernel beside %s", n)
		}
	}
}

// The SHIELD counter (CR 122.1c) beside a Phantom's line or a static prevention:
// gorge appends the counter's prevention after the printed matches, so the order ask
// lists the Phantom (or Losheel, or Temple Altisaur) first and the shielded
// permanent last; the kernel poses it the same way (spShieldLate: replShieldCand,
// parkShieldLate) and spends the counter only when it is picked or left over.

// sphPlayPred plays seeds over decks in lockstep and counts the order asks whose
// option names pred accepts, combat and noncombat apart.
func sphPlayPred(t *testing.T, decks [][]string, seeds []uint64, pred func(names []string) bool) (div, stop string, combat, noncombat int) {
	t.Helper()
	return spsrPlayAsksPred(t, decks, seeds, pred)
}

// sphShieldDecks: a Horde seat holding Brotherhood Outcasts, a Marauders seat, a Thor
// seat and a plain seat.
func sphShieldDecks() [][]string {
	art := []any{4, "Silver Myr", 4, "Containment Construct", 3, "Steel Overseer", 3, "Millikin"}
	return [][]string{
		spsrMix(append(art, 4, "Unbreathing Horde", 4, "Cryptbreaker", 5, "Brotherhood Outcast", 5, "Swamp", 4, "Plains")...),
		spsrMix(append(art, 4, "Thor, Asgard's Avenger", 4, "Humble Defector", 4, "Starstorm", 3, "Fiery Confluence", 11, "Mountain")...),
		spsrMix(append(art, 4, "Angrath's Marauders", 4, "Humble Defector", 4, "Starstorm", 3, "Fiery Confluence", 11, "Mountain")...),
		spsrMix(append(art, 4, "Humble Defector", 3, "Blasphemous Act", 4, "Starstorm", 4, "Elvish Mystic", 11, "Mountain")...),
	}
}

// twice: name appears at least twice among the options (a Phantom's line and its
// own shield counter both name the permanent).
func twice(names []string, name string) bool {
	n := 0
	for _, x := range names {
		if x == name {
			n++
		}
	}
	return n >= 2
}

var sphShieldSeeds = []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

// TestFourSeatSelfPhBesideShieldLockstep: pods holding Unbreathing Horde and Brotherhood
// Outcast beside Thor and Angrath's Marauders play in lockstep, and games reached the
// order ask that lists the Horde's line and its shield counter.
func TestFourSeatSelfPhBesideShieldLockstep(t *testing.T) {
	div, stop, combat, noncombat := sphPlayPred(t, sphShieldDecks(), sphShieldSeeds, func(n []string) bool { return twice(n, "Unbreathing Horde") })
	if div != "" || stop != "" {
		t.Fatalf("%s %s", div, stop)
	}
	if combat == 0 || noncombat == 0 {
		t.Fatalf("asks reached: combat %d, noncombat %d", combat, noncombat)
	}
	t.Logf("asks: combat %d noncombat %d", combat, noncombat)
}

// sphLosheelDecks: Losheel's seat (attacking artifact creatures) holding Outcasts,
// Temple Altisaur's seat and two plain attackers.
func sphLosheelDecks() [][]string {
	art := []any{4, "Silver Myr", 4, "Containment Construct", 3, "Steel Overseer", 3, "Millikin"}
	return [][]string{
		spsrMix(append(art, 4, "Losheel, Clockwork Scholar", 5, "Brotherhood Outcast", 4, "Ranging Raptors", 5, "Plains", 4, "Forest")...),
		spsrMix(append(art, 4, "Temple Altisaur", 5, "Brotherhood Outcast", 4, "Ranging Raptors", 5, "Plains", 4, "Forest")...),
		spsrMix(append(art, 4, "Humble Defector", 4, "Starstorm", 4, "Elvish Mystic", 11, "Mountain")...),
		spsrMix(append(art, 4, "Humble Defector", 3, "Blasphemous Act", 4, "Starstorm", 4, "Elvish Mystic", 11, "Mountain")...),
	}
}

// TestFourSeatStaticPrevBesideShieldLockstep: pods holding Losheel and Temple Altisaur
// beside Brotherhood Outcast's shield counters play in lockstep, and games reached
// the order ask of the prevention and the counter.
func TestFourSeatStaticPrevBesideShieldLockstep(t *testing.T) {
	div, stop, combat, noncombat := sphPlayPred(t, sphLosheelDecks(), sphShieldSeeds, func(n []string) bool {
		// the shield counter's candidate names the shielded permanent, last
		return len(n) == 2 && (n[0] == "Losheel, Clockwork Scholar" || n[0] == "Temple Altisaur") && n[1] != n[0] && n[1] != "Temple Altisaur"
	})
	if div != "" || stop != "" {
		t.Fatalf("%s %s", div, stop)
	}
	if combat+noncombat == 0 {
		t.Fatal("no game reached a static prevention beside a shield counter order ask")
	}
	t.Logf("asks: combat %d noncombat %d", combat, noncombat)
}

// TestFourSeatShieldLateMutation: with the shield counter posed before the merged
// candidates (the walk's old position) a game diverges or stops the kernel.
func TestFourSeatShieldLateMutation(t *testing.T) {
	dirac.DebugShieldEarly = true
	defer func() { dirac.DebugShieldEarly = false }()
	for name, decks := range map[string][][]string{"Horde": sphShieldDecks(), "Losheel and Altisaur": sphLosheelDecks()} {
		if div, stop, _, _ := sphPlayPred(t, decks, sphShieldSeeds, func([]string) bool { return false }); div == "" && stop == "" {
			t.Fatalf("the mutation neither diverged nor stopped the kernel for %s beside the shield", name)
		}
	}
}
