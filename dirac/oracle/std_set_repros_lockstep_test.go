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
	stdReprosGlobStrict(t, "std-tmt",
		"put_counters_from_unheard_kind_order",
		"second_ability_counter_cost_taps_source_for_generic",
		"second_ability_counter_gap_tries_plain_ability_first",
		"entry_body_rider_own_counter_replacement_excluded",
		"attach_triggered_card_moved_skips_ninja",
		// CR 603.10, 608.2h: a triggered cost's X read over the triggering
		// card after it left the battlefield is its last-known power there.
		"trigger_cost_x_reads_departed_lki")
}

// std-dsk's, on the same terms (this one recorded on the full surface).
func TestStdDskReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "std-dsk")
}

// No multi-pool alias replays these pools again: TestStdPoolReprosLockstep and TestStdPoolReprosLockstepFull
// were deleted on 11-22 as duplicates by construction (invariant 7.1). Since 99a3ae2038 (11-11) they globbed by
// pool the same recordings, with the same registry and the same per-pool table, as each set's own test above
// (TestStdMshReprosLockstep, TestStdFraReprosLockstep, ... TestStdTdmReprosLockstep), so each of 12 sets' games
// played twice in every gate, batch and precheck.

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
