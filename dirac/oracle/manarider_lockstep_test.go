package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The rider mana sources gorge defers (dirac/mana_rider.go): the depletion
// lands and Gemstone Mine remove their last counter and sacrifice
// themselves, Grove of the Burnwillows gives each opponent life. Lockstep
// pins that the kernel offers no priority activation for them (gorge's
// priority vocabulary lists only "activate"), while autopay and the prefix
// search still tap them for the spells in the decks.
var (
	manaRiderDeckA = []cardset.Entry{
		{Name: "Peat Bog", Count: 6}, {Name: "Remote Farm", Count: 6}, {Name: "Gemstone Mine", Count: 4},
		{Name: "Swamp", Count: 2}, {Name: "Plains", Count: 2},
		{Name: "Doom Blade", Count: 4}, {Name: "Duress", Count: 4}, {Name: "Black Knight", Count: 4},
		{Name: "Raise the Alarm", Count: 4}, {Name: "Pacifism", Count: 4},
	}
	manaRiderDeckB = []cardset.Entry{
		{Name: "Grove of the Burnwillows", Count: 4}, {Name: "Hickory Woodlot", Count: 4},
		{Name: "Sandstone Needle", Count: 4}, {Name: "Saprazzan Skerry", Count: 4},
		{Name: "Mountain", Count: 2}, {Name: "Forest", Count: 2},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Giant Growth", Count: 4}, {Name: "Divination", Count: 2}, {Name: "Unsummon", Count: 2},
	}
	manaRiderMatchups = []fixtureDecks{{manaRiderDeckA, manaRiderDeckB}, {manaRiderDeckB, manaRiderDeckA}}
	manaRiderWatch    = map[string]bool{
		"Doom Blade": true, "Duress": true, "Black Knight": true, "Raise the Alarm": true,
		"Pacifism": true, "Lightning Bolt": true, "Shock": true, "Grizzly Bears": true,
		"Giant Growth": true, "Divination": true, "Unsummon": true,
	}
)

// TestManaRiderActivationLockstep: 120 seeds on the manarider pool. gorge
// never lists a deferred rider source as a priority play, so the kernel must
// not either; every divergence or capacity stall fails. The scripted floor
// shows the players actually paid spells with the rider sources (gorge's
// autopay defers every one of them, so a play they pay is scripted).
func TestManaRiderActivationLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "manarider", manaRiderMatchups, manaRiderWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, %d scripted, casts %v", run.games, run.divs, run.stalls, run.scripted, run.casts)
	if run.divs != 0 {
		t.Errorf("%d divergences: %s", run.divs, run.first)
	}
	if run.stalls != 0 {
		t.Errorf("%d kernel capacity stalls", run.stalls)
	}
	if run.scripted < 20 {
		t.Errorf("only %d scripted plays: the rider sources did not fund enough spells", run.scripted)
	}
}

// TestManaRiderActivationLockstepMutation: remove Peat Bog's depletion rider
// and the run must diverge. The mutation is a no-op if the table's Peat Bog
// ability carries no rider, so the test asserts it saw one first.
func TestManaRiderActivationLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	sawRider := false
	mutate := func(tbl *kop.Table) {
		mutateCard(t, tbl, "Peat Bog", func(a *kop.Ability, in *kop.Instr) {
			if a.ManaRider == kop.ManaRiderDeplete {
				sawRider = true
			}
			a.ManaRider = kop.ManaRiderNone
		})
	}
	run := runFixtureLockstep(t, "manarider", manaRiderMatchups, manaRiderWatch, 120, false, mutate)
	if !sawRider {
		t.Fatal("Peat Bog's mana ability carries no deplete rider: the mutation is a no-op")
	}
	run.requireDivergence(t, "Peat Bog loses its deplete rider")
}
