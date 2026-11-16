package oracle

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// CR 205.4e: a legendary instant or sorcery is cast only while its caster
// controls a legendary creature or planeswalker. The kernel read that lock
// (the spell ability's cast restriction, prepSpellCond) only when the table
// held no CantBeCast static, because the two probes were joined with ||; a
// pool holding one left Jaya's Immolating Inferno offered to a player whose
// opponent alone controlled the legend (decision 558 of the recording).
//
// The game replays on the table its own decks need (pool "custom": tail
// buckets reshuffle whenever the table-source key moves, so the pool that
// dealt the recording holds its cards only sometimes), and the matchup table
// is built with a table-only CantBeCast witness — the way a dealt pool's
// table holds a static no deck plays. A deck-only table has no such static,
// so the || short-circuit is never taken and the game passes without the
// fix; the witness restores the probe the bug skips.
func TestMBCJayaSImmolatingInfernoReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "legendary_spell_lock_with_cantcast_static.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set := printedDelveSet(t, [2][]string{append(slices.Clone(r.Cards[0]), cantCastWitness), r.Cards[1]})
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), append(entries(r.Cards[1]), cardset.Entry{Name: cantCastWitness, Count: 1}))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
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
	stallCheck(t, r, res, false)
}

// cantCastWitness is the one-card CantBeCast static the matchup table holds:
// Eidolon of Rhetoric's single "each player can't cast more than one spell
// each turn" line. It is in neither deck, so it never enters the game —
// tableHasStatic (StatCantBeCast) is the only thing that sees it.
const cantCastWitness = "Eidolon of Rhetoric"
