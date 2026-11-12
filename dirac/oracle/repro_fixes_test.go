package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Games that once diverged at the end of their script, on the surface they
// diverged on: both engines now agree there, and for 200 decisions after.
func TestStdMkmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-mkm")
}

// std-otj's, on the same terms.
func TestStdOtjReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-otj")
}

// std-blb's, on the same terms.
func TestStdBlbReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-blb")
}

// std-trk's, on the same terms.
func TestStdTrkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-trk")
}

// std-fdn's, on the same terms.
func TestStdFdnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-fdn")
}

// std-eoe's, on the same terms.
func TestStdEoeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-eoe")
}

// std-hob's, on the same terms.
func TestStdHobReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-hob")
}

// std-big's, on the same terms.
func TestStdBigReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-big")
}

// std-tdm's, on the same terms.
func TestStdTdmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-tdm")
}

// Count$AttackersDeclared counts each creature once however many combats it attacked
// in: one creature attacking in three combats of a turn is one attacker, so Case of the
// Gateway Express (three or more) stays unsolved at the end step, as in gorge and Forge.
func TestAttackersDeclaredCountsEachCreatureOnceLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attackers_declared_counts_each_creature_once")
}

// A sorcery-speed activation is offered with the stack empty (CR 602.5d), as a cast is
// (CR 307.1): Ba Sing Se's earthbend, paid after Master Pakku's tap trigger, which
// Great Divide Guide's mana ability stacked, resolves.
func TestSorceryActivationWaitsForPrefixTriggerLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sorcery_activation_waits_for_prefix_trigger")
}

// A token copy of a transforming permanent is double-faced and each face takes the
// copy effect's exceptions (CR 707.8a, 707.9b): Saheeli's copy of Dowsing Device is
// an artifact on its front face and an artifact land once it transforms to Geode Grotto.
func TestTransformedTokenCopyKeepsCopyExceptionsLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "transformed_token_copy_keeps_copy_exceptions")
}

// A Vehicle crewed again after Eaten by Piranhas resolved on it is an artifact creature
// again (CR 613.7): the second crew is a newer effect than the Aura's type removal, so it
// is not folded into the first crew's older position.
func TestCrewAgainUnderTypeRemovingAuraLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "crew_again_under_type_removing_aura")
}

// std-tla's: the first decision of each once diverged.
func TestStdTlaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-tla")
}

type stdRepro struct {
	name    string
	autopay bool
	stall   bool // the replay is expected to end in the declared kernel stall
}

func stdReprosLockstep(t *testing.T, pool string, cases []stdRepro) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			replayRepro(t, reg, set, c.name, c.autopay, c.stall)
		})
	}
}

// std-fin's: each of these once diverged at the end of its script.
func TestStdFinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-fin")
}

// std-sos's, on the same terms.
func TestStdSosReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-sos")
}

// std-ecl's, on the same terms.
func TestStdEclReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-ecl")
}

// std-fra's, on the same terms.
func TestStdFraReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-fra")
}

// std-lci's, on the same terms.
func TestStdLciReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-lci")
}

// std-spm's, on the same terms.
func TestStdSpmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-spm")
}

// std-dft's, on the same terms.
func TestStdDftReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-dft")
}

// std-tmt's: each of these once ended in a kernel stall, which replayRepro alone does not see.
func TestStdTmtReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-tmt")
}

// std-dsk's, on the same terms (this one recorded on the full surface).
func TestStdDskReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-dsk")
}

// The same, on the pools the games came from.
func TestStdPoolReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlobPools(t, "std-msh", "std-fra", "std-fin", "std-tla", "std-mkm", "std-dsk")
}

// The same, for scripts recorded on the full option surface.
func TestStdPoolReprosLockstepFull(t *testing.T) {
	t.Parallel()
	stdReprosGlobPools(t, "std-tla", "std-lci", "std-fin", "std-ecl", "std-mkm", "std-otj", "std-hob", "std-woe", "std-tdm")
}

// replayRepro plays a fixture's script in lockstep and 200 decisions past it.
func replayRepro(t *testing.T, reg *cards.Registry, set cardset.Set, name string, autopay, stall bool) {
	t.Helper()
	res := replayReproRes(t, reg, set, name, autopay)
	if !stall && strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("kernel stalled: %s", res.Stall)
	}
}

// replayReproRes is replayRepro returning the game's result, for a fixture whose
// failure is a stall the game used to end on.
func replayReproRes(t *testing.T, reg *cards.Registry, set cardset.Set, name string, autopay bool) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = autopay
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	return res
}

// A keyword removal that resolves after a grant is the newer effect (CR 613.1f, 613.7):
// The Fire Nation Drill's second removal is not folded into its first, older one, so the
// hexproof Enter the Avatar State gave in between is gone.
func TestKeywordRemovalAfterAGrantLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "keyword_removal_resolved_after_a_grant")
}

// std-woe's, on the same terms.
func TestStdWoeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-woe")
}

// std-meta's: the pool's own decks (Avatar and non-WOE cards), not the WOE
// synthetic pool's. Each fixture holds a Wild Ride in the graveyard that
// Stingcaster Mage granted flashback, beside the card's printed Harmonize;
// the kernel's mode-2 flashback cast was canonicalised as "harmonize" (15)
// where gorge lists "flashback" (2).
func TestStdMetaGrantedFlashbackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "standard-meta")
}
