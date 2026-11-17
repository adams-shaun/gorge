package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// skipWithoutRepoCorpus skips a test whose git repository root cannot be
// resolved (an exported tree with no .git, as scripts/pr-precheck.sh's fresh
// step leaves): there is no corpus to play, and testutil.CorpusDirOrSkip
// fails rather than skips there.
func skipWithoutRepoCorpus(t *testing.T) {
	t.Helper()
	if _, ok := testutil.SharedCorpusDir("."); !ok {
		t.Skip("no git repository root, so no corpus to play")
	}
}

// The granted-protection-forms lockstep: a Continuous static's AddKeyword$
// Protection from red & Protection from blue (Sword of Fire and Ice), a
// Pump's KW$ Protection from each color until end of turn (Eldritch
// Immunity) and an Animate's Keywords$ Protection:Player.Opponent (Figure of
// Fable) lowered onto kop.Static.ProtFrom and kop.Override.ProtFrom, with
// coloured creatures, burn and removal on the other seat to test the
// protection against (blocking, damage and targeting; CR 702.16). The
// expected answer at every decision is gorge's.
var kprotectionformsDecks = [2][]cardset.Entry{
	{
		{Name: "Sword of Fire and Ice", Count: 2}, {Name: "Eldritch Immunity", Count: 4},
		{Name: "Figure of Fable", Count: 2}, {Name: "Savannah Lions", Count: 4},
		{Name: "Silver Knight", Count: 2}, {Name: "Plains", Count: 26},
	},
	{
		{Name: "Hill Giant", Count: 5}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Terror", Count: 3}, {Name: "Mountain", Count: 8},
		{Name: "Forest", Count: 3}, {Name: "Swamp", Count: 8},
	},
}

func TestProtectionFormsLockstep(t *testing.T) {
	t.Parallel()
	skipWithoutRepoCorpus(t)
	lockstepSet(t, "protectionforms", kprotectionformsDecks, 60, "KPROTECTIONFORMS_SEEDS")
}

// TestProtectionFormsLockstepMutation: with every override record's granted
// protection dropped, some seed diverges, so the pool resolves the granted
// protection and the kernel prices it.
func TestProtectionFormsLockstepMutation(t *testing.T) {
	t.Parallel()
	skipWithoutRepoCorpus(t)
	n := lockstepRun(t, "protectionforms", kprotectionformsDecks, 60, false, func(tbl *kop.Table) {
		ovs := append([]kop.Override(nil), tbl.Overrides...)
		hit := 0
		for i := range ovs {
			if ovs[i].ProtFrom != 0 {
				ovs[i].ProtFrom = 0
				hit++
			}
		}
		tbl.Overrides = ovs
		if hit == 0 {
			t.Errorf("no Override record grants protection in the table")
		}
	})
	if n == 0 {
		t.Errorf("no seed diverged: the pool never resolves the granted protection")
	}
}

// kprotectionformsOvDecks: the pool's only protection is Eldritch Immunity's
// Pump grant -- no printed CardDef.ProtFrom and no Continuous static rides
// beside it -- so every opponent Shock and Bolt reads the targetableGen
// shortcut that the override-armed anyProtGrant guards (legal_gen.go), with coloured
// creatures to block and kill the granted creatures against (CR 702.16).
var kprotectionformsOvDecks = [2][]cardset.Entry{
	{
		{Name: "Eldritch Immunity", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 16}, {Name: "Wastes", Count: 16}, // Immunity costs {C}: Wastes pays it
	},
	{
		{Name: "Hill Giant", Count: 5}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Terror", Count: 3}, {Name: "Mountain", Count: 8},
		{Name: "Forest", Count: 3}, {Name: "Swamp", Count: 8},
	},
}

// TestProtectionFormsOverrideOnlyLockstep plays the override-only pool.
func TestProtectionFormsOverrideOnlyLockstep(t *testing.T) {
	t.Parallel()
	skipWithoutRepoCorpus(t)
	lockstepSet(t, "protectionformsov", kprotectionformsOvDecks, 600, "KPROTECTIONFORMSOV_SEEDS")
}

// TestProtectionFormsOverrideOnlyLockstepMutation: with the override record's
// granted protection dropped, some seed diverges, so the pool resolves the
// override-only grant and the kernel prices it. The grant lasts until end of
// turn, so it is up only in the window the resolving controller's turn gives
// it, and more seeds than the plain pool need to price it.
func TestProtectionFormsOverrideOnlyLockstepMutation(t *testing.T) {
	t.Parallel()
	skipWithoutRepoCorpus(t)
	n := lockstepRun(t, "protectionformsov", kprotectionformsOvDecks, 600, false, func(tbl *kop.Table) {
		ovs := append([]kop.Override(nil), tbl.Overrides...)
		hit := 0
		for i := range ovs {
			if ovs[i].ProtFrom != 0 {
				ovs[i].ProtFrom = 0
				hit++
			}
		}
		tbl.Overrides = ovs
		if hit == 0 {
			t.Errorf("no Override record grants protection in the table")
		}
	})
	if n == 0 {
		t.Errorf("no seed diverged: the pool never resolves the override-only granted protection")
	}
}
