package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The T17c replacement-mode locksteps: the newly ported modes behave like
// gorge over full games — the printed phase skip (Eon Hub's upkeep skip,
// Symbiotic Deployment's draw skip, Infernal Sovereign's), the damage-only
// life floor (Ali from Cairo, Worship), the DamageDone Updated rewrites
// (Benevolent Unicorn's minus, Divine Presence's set), and the widened
// printed-counter scope (Root Sliver's Sliver spells of every controller).
// Each pool holds ONE copy of every line-carrying card and the two decks'
// lines never co-match one event: gorge poses CR 616.1's order ask whenever
// two or more replacements match one event (damage replacement_choice.go,
// life's non-commuting floors, posePhaseOrderChoice, replChoiceCounter), and
// the kernel composes the new kinds' matches in scan order instead of asking
// (repl_dmgminus.go, repl_beginphase.go, repl_uncounterable.go's pose cap).
// Each pool has a mutation: with the kind dropped from the kernel's table
// some seed diverges, so the lockstep tests the mode, not the cards around it.

var kphaseSkipDecks = [2][]cardset.Entry{
	{
		{Name: "Eon Hub", Count: 1}, {Name: "Symbiotic Deployment", Count: 1},
		{Name: "Divination", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Island", Count: 12}, {Name: "Plains", Count: 12},
	},
	{
		{Name: "Infernal Sovereign", Count: 1},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Grizzly Bears", Count: 9},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
	},
}

var klifeFloorDecks = [2][]cardset.Entry{
	{
		{Name: "Ali from Cairo", Count: 1}, {Name: "Lightning Bolt", Count: 8},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 11},
	},
	{
		{Name: "Worship", Count: 1}, {Name: "Shock", Count: 8},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Forest", Count: 12}, {Name: "Plains", Count: 11},
	},
}

var kdmgMinusDecks = [2][]cardset.Entry{
	{
		{Name: "Benevolent Unicorn", Count: 1},
		{Name: "Grizzly Bears", Count: 10}, {Name: "Plains", Count: 15}, {Name: "Forest", Count: 14},
	},
	{
		{Name: "Lightning Bolt", Count: 8}, {Name: "Shock", Count: 8},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
	},
}

var kdmgSetDecks = [2][]cardset.Entry{
	{
		{Name: "Divine Presence", Count: 1},
		{Name: "Grizzly Bears", Count: 10}, {Name: "Plains", Count: 15}, {Name: "Forest", Count: 14},
	},
	{
		{Name: "Lightning Bolt", Count: 6}, {Name: "Lava Axe", Count: 6},
		{Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 24},
	},
}

var kuncAnyCtrlDecks = [2][]cardset.Entry{
	{
		{Name: "Root Sliver", Count: 1}, {Name: "Allosaurus Shepherd", Count: 1},
		{Name: "Grizzly Bears", Count: 10}, {Name: "Forest", Count: 14}, {Name: "Plains", Count: 14},
	},
	{
		{Name: "Counterspell", Count: 6}, {Name: "Cancel", Count: 4},
		{Name: "Root Sliver", Count: 1}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Island", Count: 16}, {Name: "Forest", Count: 8},
	},
}

func TestPhaseSkipLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "phase-skip", kphaseSkipDecks, 400, "KPHASESKIP_SEEDS")
}

func TestPhaseSkipLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "phase-skip", kphaseSkipDecks, 100, false, func(tbl *kop.Table) {
		replKindMutation(tbl, kop.ReplBeginPhase)
	})
	if n == 0 {
		t.Fatal("no seed diverged with the phase skip dropped")
	}
}

func TestLifeFloorLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "life-floor", klifeFloorDecks, 400, "KLIFEFLOOR_SEEDS")
}

func TestLifeFloorLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "life-floor", klifeFloorDecks, 100, false, func(tbl *kop.Table) {
		replKindMutation(tbl, kop.ReplLifeReduced)
	})
	if n == 0 {
		t.Fatal("no seed diverged with the life floor dropped")
	}
}

func TestDmgMinusLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "dmg-minus", kdmgMinusDecks, 400, "KDmgMINUS_SEEDS")
}

func TestDmgMinusLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "dmg-minus", kdmgMinusDecks, 100, false, func(tbl *kop.Table) {
		replKindMutation(tbl, kop.ReplDamageMinus)
	})
	if n == 0 {
		t.Fatal("no seed diverged with the damage minus dropped")
	}
}

func TestDmgSetLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "dmg-set", kdmgSetDecks, 400, "KDmgSET_SEEDS")
}

func TestDmgSetLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "dmg-set", kdmgSetDecks, 100, false, func(tbl *kop.Table) {
		replKindMutation(tbl, kop.ReplDamageSet)
	})
	if n == 0 {
		t.Fatal("no seed diverged with the damage set dropped")
	}
}

func TestUncAnyCtrlLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "unc-any-ctrl", kuncAnyCtrlDecks, 400, "KUNCANYCTRL_SEEDS")
}

func TestUncAnyCtrlLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "unc-any-ctrl", kuncAnyCtrlDecks, 100, false, func(tbl *kop.Table) {
		replKindMutation(tbl, kop.ReplSpellsCantBeCountered)
	})
	if n == 0 {
		t.Fatal("no seed diverged with the uncounterable scope dropped")
	}
}

// replKindMutation zeroes every replacement of one kind (the mutation hook
// TestBloodletterLockstepMutation hand-rolls).
func replKindMutation(tbl *kop.Table, kind kop.ReplKind) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			if abs[j].Kind == kop.AbReplacement && abs[j].Repl.Kind == kind {
				abs[j].Repl.Kind = kop.ReplNone
			}
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
}
