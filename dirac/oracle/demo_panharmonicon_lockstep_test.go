package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The demo Panharmonicon pool (dirac/cardset/pools/fixtures/demopanharmonicon):
//
//   - Echoes of Eternity's static scopes its echo with ValidZone$
//     Battlefield,Stack, so a triggered ability of a colorless permanent on the
//     battlefield OR of a colorless spell on the stack triggers an additional
//     time. Eldrazi Repurposer's self-cast trigger is the stack half; Eldrazi
//     Skyspawner's enters and Matter Reshaper's dies triggers are the
//     battlefield half.
//   - Panoptic Projektor's static names ValidMode$ TurnFaceUp and scopes the
//     turned object with ValidTurned$, so it doubles a turn-up trigger of any
//     permanent you control: Growing Dread's watcher (a +1/+1 counter), Dog
//     Walker's two Dogs, Mistfire Weaver's hexproof and Icefeather Aven's
//     bounce. Its activated ability is an Effect-delivered ReduceCost beside
//     ForgetOnCast$, ended by the next face-down creature spell.
//
// PanharmLog counts every echo, and its Turned/Stack fields prove the two new
// reads decided a placement; a green game is not one where no new shape fired.
const demoPanharmoniconPool = "demopanharmonicon"

// demoPanharmoniconTurnDeck is the turn-face-up half: Panoptic, the turn-up
// watcher and four Morph/Disguise carriers, with Priest of Urabrask floating the
// face-down cast's {3}.
func demoPanharmoniconTurnDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Panoptic Projektor", Count: 3}, {Name: "Growing Dread", Count: 3},
		{Name: "Dog Walker", Count: 4}, {Name: "Mistfire Weaver", Count: 2}, {Name: "Icefeather Aven", Count: 2},
		{Name: "Priest of Urabrask", Count: 4}, {Name: "Sol Ring", Count: 2},
		{Name: "Forest", Count: 5}, {Name: "Island", Count: 5}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5},
	}
}

// demoPanharmoniconEchoDeck is the colorless-spell half: Echoes, a cheap
// self-cast carrier, two battlefield trigger carriers and mana rocks to reach
// Echoes's six mana.
func demoPanharmoniconEchoDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Echoes of Eternity", Count: 4}, {Name: "Eldrazi Repurposer", Count: 4},
		{Name: "Eldrazi Skyspawner", Count: 4}, {Name: "Matter Reshaper", Count: 2},
		{Name: "Sol Ring", Count: 4}, {Name: "Mind Stone", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Wastes", Count: 4}, {Name: "Mountain", Count: 4},
	}
}

var demoPanharmoniconDecks = [][2][]cardset.Entry{
	{demoPanharmoniconTurnDeck(), demoPanharmoniconTurnDeck()},
	{demoPanharmoniconEchoDeck(), demoPanharmoniconEchoDeck()},
}

// demoPanharmoniconPolicy casts the two carriers first (Echoes/Panoptic and
// Growing Dread) and activates Panoptic's discount before falling through to
// FaceDownPolicy, which takes a face-down cast/turn-up and otherwise plays the
// random base game.
func demoPanharmoniconPolicy(seed uint64) Policy {
	fd := FaceDownPolicy(AffordableTargetPolicy(RandomPolicy(seed)))
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, name := range []string{"Echoes of Eternity", "Panoptic Projektor", "Growing Dread"} {
			for _, a := range c.D.Options {
				if a.Kind() == ActCast {
					if f := optionFace(c, a); f != nil && f.Name == name {
						return a
					}
				}
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate {
				if f := optionFace(c, a); f != nil && f.Name == "Panoptic Projektor" {
					return a
				}
			}
		}
		return fd.Pick(c)
	})
}

// demoPanharmoniconGames plays each matchup for seeds 1..hi and returns the
// clean games, the divergent ones and every echo recorded.
func demoPanharmoniconGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs []lockstepDivergence, log []dirac.PanharmRec) {
	t.Helper()
	old := dirac.PanharmLog
	dirac.PanharmLog = &log
	defer func() { dirac.PanharmLog = old }()
	for _, md := range demoPanharmoniconDecks {
		reg, set, gd := lockstepPool(t, demoPanharmoniconPool, md)
		c, _, d := lockstepSeedsMode(t, reg, set, demoPanharmoniconPool, md, gd, 1, hi, mode, nil, nil, demoPanharmoniconPolicy)
		clean += c
		divs = append(divs, d...)
	}
	return clean, divs, log
}

// TestDemoPanharmoniconLockstep: no divergence and no kernel stall over 64
// seeds, and across them an echo of both new shapes: a turn-face-up trigger
// (Panoptic's ValidTurned$) and a spell-on-the-stack trigger (Echoes's
// ValidZone$ Stack).
func TestDemoPanharmoniconLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	clean, divs, log := demoPanharmoniconGames(t, 64, lockstepFailEach)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	turned, stack := 0, 0
	for _, r := range log {
		if r.Turned {
			turned++
		}
		if r.Stack {
			stack++
		}
	}
	if turned == 0 {
		t.Error("no echo doubled a turn-face-up trigger (Panoptic's ValidTurned$ read did not decide a placement)")
	}
	if stack == 0 {
		t.Error("no echo doubled a spell-on-the-stack trigger (Echoes's ValidZone$ Stack read did not decide a placement)")
	}
	t.Logf("%d clean games, %d echoes (%d turn-face-up, %d stack)", clean, len(log), turned, stack)
}

// TestDemoPanharmoniconMutation: with either new read disabled some game must
// diverge from gorge: the stack zone half of Echoes and the TurnFaceUp mode of
// Panoptic each decide an echo.
func TestDemoPanharmoniconMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []struct {
		name string
		sw   *bool
	}{
		{"DebugPanhNoStack", &dirac.DebugPanhNoStack},
		{"DebugPanhNoTurnUp", &dirac.DebugPanhNoTurnUp},
	} {
		*m.sw = true
		_, divs, _ := demoPanharmoniconGames(t, 64, lockstepQuiet)
		*m.sw = false
		if len(divs) == 0 {
			t.Errorf("%s: no divergence in 64 games", m.name)
			continue
		}
		t.Logf("%s diverged in %d games", m.name, len(divs))
	}
}

// demoPanharmoniconPodTurnDeck / demoPanharmoniconPodEchoDeck are the
// four-seat decks: fewer engine copies and more lands than the two-seat decks
// so a random pod does not fill MaxPending with Echoes's doubled cast triggers
// (a declared capacity stall, not a divergence).
func demoPanharmoniconPodTurnDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Panoptic Projektor", Count: 2}, {Name: "Growing Dread", Count: 2},
		{Name: "Dog Walker", Count: 3}, {Name: "Mistfire Weaver", Count: 1}, {Name: "Icefeather Aven", Count: 1},
		{Name: "Priest of Urabrask", Count: 3}, {Name: "Sol Ring", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 7}, {Name: "Mountain", Count: 7},
	}
}

func demoPanharmoniconPodEchoDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Echoes of Eternity", Count: 2}, {Name: "Eldrazi Repurposer", Count: 2},
		{Name: "Eldrazi Skyspawner", Count: 2}, {Name: "Matter Reshaper", Count: 1},
		{Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 8}, {Name: "Wastes", Count: 5}, {Name: "Mountain", Count: 8},
	}
}

// TestFourSeatDemoPanharmoniconLockstep: four seats, gorge leading.
func TestFourSeatDemoPanharmoniconLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := demoDeckNames(demoPanharmoniconPodTurnDeck())
	b := demoDeckNames(demoPanharmoniconPodEchoDeck())
	if n := pod4FixtureLockstep(t, demoPanharmoniconPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}

// demoDeckNames expands a deck's counted entries to the 40-card name list
// pod4FixtureLockstep takes.
func demoDeckNames(d []cardset.Entry) []string {
	var out []string
	for _, e := range d {
		for i := 0; i < e.Count; i++ {
			out = append(out, e.Name)
		}
	}
	if len(out) != 40 {
		panic("demo deck is not 40 cards")
	}
	return out
}
