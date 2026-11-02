package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Games that once diverged at the end of their script: both engines now agree
// there, and for 200 decisions after.
func TestStdMkmReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-mkm")
	for _, name := range []string{
		"deathtouch_wither_hit",                // CR 704.5h: a counter-form hit marks no damage, and the mark alone is lethal
		"dealt_once_each_damage",               // EachDamage lands on one referent twice: one trigger with the batch's total
		"wither_from_departed_source",          // a dying Shady Informant's trigger keeps the wither Massacre Girl gave it
		"potential_once_each_turn_mana",        // Gravestone Strider's {1} mana ability counts toward a trigger's own cost
		"hold_out_ends_with_first_strike_step", // the regular damage step is a new window with no StepChange
		"dealt_damage_new_object",              // CR 400.7: a returned creature has dealt no damage this turn
		"entering_is_not_leaving",              // Doorkeeper Thrull leaves "leave your graveyard" lines alone
		"spell_gained_deathtouch_lifelink",     // Judith's mode on the spell that triggered her
		"trigger_limit_look_back",              // CR 603.10a: Homicide Investigator's own death reads the limit it spent
	} {
		t.Run(name, func(t *testing.T) {
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
			rc.Config.GorgeAutopaySurface = true
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
		})
	}
}
