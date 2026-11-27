package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander counter and animate shapes of ticket
// cli-20261127T171904Z-e493ca42, played decision by decision against gorge
// (gorge leads) through the cedh pod harness: each case builds its kernel
// table with kgen, so a case is RED while kgen still refuses the card, and
// fails on the first divergence once it lowers. The tally is the reached
// check: a game that never put the counters on the animated artifact cannot
// pass it.
//
//   - Rise and Shine: `SP$ Animate | ... | RememberAnimated$ True |
//     SubAbility$ DBPutCounter` over `DB$ PutCounter | Defined$ Remembered |
//     CounterType$ P1P1 | CounterNum$ 4`. The Animate's grant and its
//     remembered set are one op (OpOverride, W = RefRemembered, whose apply
//     tail joins each animated permanent to the resolution's set; gorge
//     effAnimate's rememberAnimated, engine/effects/combatfx.go:763), and the
//     chained PutCounter reads exactly those. Sol Ring is the victim: a
//     noncreature artifact that becomes a 0/0 and lives only because the four
//     +1/+1 counters landed, so a kernel that animates without remembering
//     leaves a dead 0/0 where gorge has a 4/4.
var hostedCmdrRiseAndShineDeck = []cardset.Entry{
	{Name: "Rise and Shine", Count: 8}, {Name: "Sol Ring", Count: 8}, {Name: "Island", Count: 24},
}

// Moira and Teshar's trigger returns a nonland permanent card from the
// graveyard, grants it haste, exiles it at the next end step and promises it
// is exiled instead of leaving anywhere else (`DB$ Pump | KW$ Haste |
// LeaveBattlefield$ Exile | Defined$ Targeted | Duration$ Permanent | AtEOT$
// Exile`). Urza's Bauble and Phyrexian Walker are the historic spells cast and
// the permanent cards returned; Lightning Bolt is what tries to kill the
// returnee, which the promise sends to exile instead of the graveyard.
var hostedCmdrMoiraDeck = []cardset.Entry{
	{Name: "Moira and Teshar", Count: 4}, {Name: "Urza's Bauble", Count: 8},
	{Name: "Phyrexian Walker", Count: 8}, {Name: "Lightning Bolt", Count: 7},
	{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 6},
}

// TestHostedCmdrPumpLeaveExileLockstep plays Moira and Teshar's
// LeaveBattlefield$ Exile against gorge, counting the kernel's promises by
// source card and the objects they exiled instead: a game that never sent a
// promised permanent off the battlefield cannot pass it. Not t.Parallel: the
// promise hook is a package global.
func TestHostedCmdrPumpLeaveExileLockstep(t *testing.T) {
	d := hostedCmdrMoiraDeck
	// the deck holds no other promise carrier (no unearth, no Isareth), so
	// every registration and every exile-instead is Moira's Pump's.
	regs := 0
	hits := map[dirac.Obj]bool{}
	dirac.DebugLeaveExileHook = &struct {
		Reg func(c dirac.CardID)
		Hit func(o dirac.Obj)
	}{
		Reg: func(c dirac.CardID) { regs++ },
		Hit: func(o dirac.Obj) { hits[o] = true },
	}
	defer func() { dirac.DebugLeaveExileHook = nil }()
	got := cedhPodRun(t, "hcc-moirateshar", [][]cardset.Entry{d, d}, []string{"Moira and Teshar"}, 40,
		func(g *state.Game) int { return cedhZoneCount(g, state.ZExile, "Phyrexian Walker") })
	t.Logf("%d exiled Walkers, %d promises, %d exiled instead", got, regs, len(hits))
	if regs == 0 {
		t.Fatal("Moira and Teshar registered no exile-instead promise: its Pump never resolved")
	}
	if len(hits) == 0 {
		t.Fatal("no promised permanent left the battlefield: the exile-instead branch was never read")
	}
}

// TestHostedCmdrCounterShapesLockstep plays each lowered shape.
func TestHostedCmdrCounterShapesLockstep(t *testing.T) {
	t.Parallel()
	t.Run("rise_and_shine_animate_rememberanimated", func(t *testing.T) {
		d := hostedCmdrRiseAndShineDeck
		// an animated Sol Ring: a permanent named Sol Ring on the battlefield
		// holding +1/+1 counters (the chained PutCounter's four), the only way
		// a 0/0 artifact creature survives.
		tally := func(g *state.Game) int {
			n := 0
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					o := g.Obj(id)
					if o == nil || o.Face() == nil || o.Face().Name != "Sol Ring" {
						continue
					}
					if o.Counter("P1P1") >= 4 {
						n++
					}
				}
			}
			return n
		}
		got := cedhPodRun(t, "hcc-riseandshine", [][]cardset.Entry{d, d}, []string{"Rise and Shine"}, 40, tally)
		if got == 0 {
			t.Fatal("no game animated an artifact with +1/+1 counters on it: the RememberAnimated$ chain was never resolved")
		}
		t.Logf("%d animated artifacts with counters", got)
	})
}
