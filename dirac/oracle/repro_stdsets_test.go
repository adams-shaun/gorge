package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Games from the Standard sets' synthetic pools (or, for a card no synthetic
// pool holds, the table built from the repro's own decks) that once
// diverged at the end of their script, on the autopay surface (the full one
// where a case says so): both engines now agree there, and for 200 decisions
// after.
func TestStdSetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "custom")
}

// replayStdRepro replays std pool's repro name on the full surface or on
// autopay, and 200 decisions past its script: the divergence, or nil.
func replayStdRepro(t *testing.T, reg *cards.Registry, pool, name string, full bool) *Divergence {
	t.Helper()
	return replayStdReproResult(t, reg, pool, name, full).Div
}

// replayStdReproResult is replayStdRepro's whole lockstep result.
func replayStdReproResult(t *testing.T, reg *cards.Registry, pool, name string, full bool) Result {
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
	set := reproSet(t, pool, r)
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = !full
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

// TestTapXOwnAnswerMutation: with the answers that tap the play's own {T}
// source kept, the Supportive Parents search spends its nodes on them and
// Guy in the Chair's Web Support is not offered where gorge offers it.
func TestTapXOwnAnswerMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugNoTapXOwnSkip = true
	defer func() { dirac.DebugNoTapXOwnSkip = false }()
	if div := replayStdRepro(t, reg, "std-spm", "tap_two_answer_skips_the_plays_own_tap_source", true); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}

// TestFiltTrigSettleMutation: with a step's Taps trigger left unstacked, the
// filter search prices Kraven, Proud Predator under Spider-Gwen's loot and
// offers the cast gorge does not.
func TestFiltTrigSettleMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugFiltNoTrigSettle = true
	defer func() { dirac.DebugFiltNoTrigSettle = false }()
	if div := replayStdRepro(t, reg, "std-spm", "tap_two_trigger_stacks_before_the_play_is_priced", true); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}

// TestAskYesFirstMutation: with a yes-or-no election tried in the kernel's
// order, the prefix search declines Spider-Gwen's loot where gorge's discards
// and draws.
func TestAskYesFirstMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugAskNoYesFirst = true
	defer func() { dirac.DebugAskNoYesFirst = false }()
	if div := replayStdRepro(t, reg, "std-spm", "prefix_trigger_if_you_do_cost_answers_yes_first", true); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}

// TestRestrManaOpenMutation: with a restricted mana source left out of the
// window's untapped-source gate, Grow Extra Arms's menu drops the creature
// Interdimensional Web Watch's exile-cast mana may fund.
func TestRestrManaOpenMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugNoRestrManaOpen = true
	defer func() { dirac.DebugNoRestrManaOpen = false }()
	if div := replayStdRepro(t, reg, "std-spm", "target_menu_keeps_candidates_an_exile_cast_source_may_fund", true); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}
