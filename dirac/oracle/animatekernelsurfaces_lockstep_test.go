package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Animate kernel-surface lockstep pools (dirac/kgen/lower_animatereplace.go,
// lower_landwalkgrant.go, lower_zone.go animate, lower_gainsof.go): one pool per
// class, each carrying the class's fixture cards against opposing decks that
// reach them.
//   - replacements: Geth, Thane of Contracts grants the "if this leaves the
//     battlefield, exile it instead" line (no Destination$, so a bounce exiles
//     too) to the reanimated creature; the opposing deck kills and bounces
//     creatures. Llanowar Greenwidow is in the pool's table but not its deck:
//     the bot activates its graveyard ability twice before either resolves,
//     which is two replacement effects on one death (gorge asks the CR 616.1
//     order, the kernel stalls: the unmodelled replleave class); the unit test
//     TestAnimateReturnedSelfOverride (dirac) covers the return-then-animate
//     path and the two-record count.
//   - landwalk: Mystic Decree removes flying and islandwalk from every creature.
//   - atEOT set: Depthshaker Titan animates any number of noncreature artifacts
//     and sacrifices them together at the next end step.
//   - gainsof: Territory Forge borrows the activated abilities of the card it
//     exiled (an untyped Card.ExiledWithSource lender). Mirran Safehouse
//     (graveyard lands) stays refused: its lent mana abilities diverge from
//     gorge on which source taps.

func animateReplacementsDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Geth, Thane of Contracts", Count: 10},
		{Name: "Typhoid Rats", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Swamp", Count: 14}, {Name: "Forest", Count: 6},
	}
	b := []cardset.Entry{
		{Name: "Unsummon", Count: 6}, {Name: "Vapor Snag", Count: 4}, {Name: "Doom Blade", Count: 4},
		{Name: "Wind Drake", Count: 6}, {Name: "Island", Count: 12}, {Name: "Swamp", Count: 8},
	}
	return [2][]cardset.Entry{a, b}
}

func animateLandwalkDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Mystic Decree", Count: 6}, {Name: "Bull Hippo", Count: 6},
		{Name: "Lurking Crocodile", Count: 4}, {Name: "Zodiac Horse", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 10},
	}
	b := []cardset.Entry{
		{Name: "Savannah Lions", Count: 10}, {Name: "Elite Vanguard", Count: 8},
		{Name: "Wind Drake", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 10},
	}
	return [2][]cardset.Entry{a, b}
}

func animateAtEOTSetDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Depthshaker Titan", Count: 8}, {Name: "Mind Stone", Count: 6},
		{Name: "Fellwar Stone", Count: 4}, {Name: "Darksteel Relic", Count: 4},
		{Name: "Mountain", Count: 18},
	}
	b := []cardset.Entry{
		{Name: "Savannah Lions", Count: 8}, {Name: "Elite Vanguard", Count: 6},
		{Name: "Wind Drake", Count: 6}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 10},
	}
	return [2][]cardset.Entry{a, b}
}

func animateGainsOfDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Territory Forge", Count: 8}, {Name: "Mind Stone", Count: 6},
		{Name: "Darksteel Relic", Count: 4}, {Name: "Mountain", Count: 14}, {Name: "Island", Count: 8},
	}
	b := []cardset.Entry{
		{Name: "Savannah Lions", Count: 8}, {Name: "Elite Vanguard", Count: 8},
		{Name: "Mountain", Count: 12}, {Name: "Plains", Count: 12},
	}
	return [2][]cardset.Entry{a, b}
}

func TestAnimateKernelSurfacesPoolsPlayable(t *testing.T) {
	t.Parallel()
	for name, decks := range map[string][2][]cardset.Entry{
		"animatereplacements": animateReplacementsDecks(),
		"animatelandwalk":     animateLandwalkDecks(),
		"animateateotset":     animateAtEOTSetDecks(),
		"animategainsof":      animateGainsOfDecks(),
	} {
		_, set, gd := lockstepPool(t, name, decks)
		for s, d := range gd {
			if !SetDeckSupported(set, d) {
				t.Errorf("%s seat %d: deck not playable in the pool", name, s)
			}
		}
	}
}

func TestAnimateReplacementsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "animatereplacements", animateReplacementsDecks(), 200, "ANIMATE_REPLACEMENTS_SEEDS")
}

func TestAnimateLandwalkLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "animatelandwalk", animateLandwalkDecks(), 200, "ANIMATE_LANDWALK_SEEDS")
}

func TestAnimateAtEOTSetLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "animateateotset", animateAtEOTSetDecks(), 200, "ANIMATE_ATEOT_SET_SEEDS")
}

func TestAnimateGainsOfLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "animategainsof", animateGainsOfDecks(), 200, "ANIMATE_GAINSOF_SEEDS")
}

// TestAnimateKernelSurfacesLockstepMutation: with the override's ExileOnLeave
// (Geth, Greenwidow) or RemoveLandwalk (Mystic Decree) cleared some game
// diverges, so each pool reaches the surface it names.
func TestAnimateKernelSurfacesLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name  string
		decks [2][]cardset.Entry
		fn    func(*kop.Override)
	}{
		{"animatereplacements", animateReplacementsDecks(), func(o *kop.Override) { o.ExileOnLeave = false }},
		{"animatelandwalk", animateLandwalkDecks(), func(o *kop.Override) { o.RemoveLandwalk = 0 }},
	} {
		n := lockstepRun(t, m.name, m.decks, 200, false, func(tbl *kop.Table) {
			ovs := append([]kop.Override(nil), tbl.Overrides...)
			for i := range ovs {
				m.fn(&ovs[i])
			}
			tbl.Overrides = ovs
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged with the surface cleared: the pool never reaches it", m.name)
		}
	}
}
