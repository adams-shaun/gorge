package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCAttackedBattleLeaveReturnReprosLockstep replays the custom game in
// which seat 0 attacks its own Invasion of Ixalan, seat 1 flashes Cast Out on
// it, and then Disenchants the Cast Out before combat damage. The battle
// returns as a new object (CR 400.7) that the attacker is not attacking
// (CR 506.4c), so the attacker deals it no combat damage.
func TestMBCAttackedBattleLeaveReturnReprosLockstep(t *testing.T) {
	t.Parallel()
	// The custom pool's table is built from the corpus, so skip before globSet
	// reaches for it (a clone with no .cards must skip, not fail).
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", "attacked_battle_leave_return.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	set := globSet(t, r.Pool, r)
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
	rc.MaxDecisions = len(r.Actions) + 200
	var battle state.ObjID
	var battleGen uint32
	var haveBattle, reached bool
	rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		for i := range g.Objs {
			a := &g.Objs[i]
			if !a.IsAttacking || a.AttackingBattle == 0 {
				continue
			}
			o := g.Obj(a.AttackingBattle)
			if o == nil || o.Zone != state.ZBattlefield {
				continue
			}
			if !haveBattle || battle != a.AttackingBattle {
				battle, battleGen, haveBattle = a.AttackingBattle, o.ZoneGen, true
				continue
			}
			if o.ZoneGen != battleGen {
				// The return must happen before the combat damage step,
				// or the attacker's damage is dealt to the old object.
				reached = g.Step < state.StepCombatDamage
			}
		}
		return nil
	}
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
	if !reached {
		t.Fatal("the recording never has the attacked battle leave and return during combat")
	}
}
