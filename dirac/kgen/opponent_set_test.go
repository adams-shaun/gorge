package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOpponentSetAnySeats: the eight supported cards whose plural opponent
// reference was once read as the one opponent of a two-player game lower at
// any seat count: Fblthp, Impossibly Lost and Master of Barbs take the
// batch-level DamageAll latch, Kardur, Doomscourge and Maximum Carnage carry
// the "not the static's controller" requirement flag, Ensign Pavel Chekov
// lowers the per-opponent land-choice loop, Teysa, Opulent Oligarch repeats
// the investigate token per opponent who lost life, and Mindbreak Trap and
// Ertai's Scorn count the opponents whose own spells-cast tally reaches the
// threshold.
func TestOpponentSetAnySeats(t *testing.T) {
	names := []string{
		"Fblthp, Impossibly Lost", "Master of Barbs",
		"Kardur, Doomscourge", "Maximum Carnage",
		"Ensign Pavel Chekov", "Teysa, Opulent Oligarch",
		"Mindbreak Trap", "Ertai's Scorn",
	}
	lw := lowerNamedPool(t, names)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		if _, ok := byName[c.Name]; !ok && !c.Token {
			byName[c.Name] = c
		}
	}
	for _, n := range names {
		c := byName[n]
		if c == nil {
			t.Fatalf("%s did not lower", n)
		}
		if c.TwoSeat {
			t.Errorf("%s is two-seat only: %v", n, c.TwoSeatWhy)
		}
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", n, c.Refusals)
		}
	}

	// DamageAll over the opponents is the batch-level DamageDone latch: one
	// trigger per damage batch (gorge damageBatchKey's all key).
	for _, n := range []string{"Fblthp, Impossibly Lost", "Master of Barbs"} {
		c := byName[n]
		ok := false
		for _, a := range c.Abilities {
			if a.Trigger == nil || a.Trigger.Event != "DamageDone" {
				continue
			}
			if a.Trigger.Batch && !a.Trigger.PerRef {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s: no batch-level DamageDone trigger", n)
		}
	}

	// MustAttack$ Player.Other keeps the requirement and carries the
	// "not the static's controller" flag.
	for _, n := range []string{"Kardur, Doomscourge", "Maximum Carnage"} {
		c := byName[n]
		found, flagged := 0, 0
		for _, e := range c.Emblems {
			for _, st := range e.Statics {
				if st.Kind != "MustAttack" {
					continue
				}
				found++
				if containsString(st.Flags, "MustAttackOther") {
					flagged++
				}
			}
		}
		if found == 0 || flagged == 0 {
			t.Errorf("%s: %d MustAttack statics, %d flagged MustAttackOther", n, found, flagged)
		}
	}

	// Chekov: the loop body is the per-opponent ExileLandChoose.
	{
		c := byName["Ensign Pavel Chekov"]
		ok := false
		for _, a := range c.Abilities {
			if a.Trigger == nil || a.Trigger.Event != "ETB" {
				continue
			}
			for i, in := range a.Prog {
				if in.Op != "RepeatEach" || in.Obj.Kind != rEachOpp {
					continue
				}
				if i+1 < len(a.Prog) && a.Prog[i+1].Op == "ExileLandChoose" {
					ok = true
				}
			}
		}
		if !ok {
			t.Error("Ensign Pavel Chekov: no per-opponent ExileLandChoose loop")
		}
	}

	// Teysa: the investigate amount is the opponents-who-lost-life count,
	// repeated into one CreateToken per opponent.
	{
		c := byName["Teysa, Opulent Oligarch"]
		ok := false
		for _, a := range c.Abilities {
			for i, in := range a.Prog {
				if in.Op != "Repeat" || in.A.Kind != vOppsLostLife {
					continue
				}
				if i+1 < len(a.Prog) && a.Prog[i+1].Op == "CreateToken" && a.Prog[i+1].A.isConst() && a.Prog[i+1].A.N == 1 {
					ok = true
				}
			}
		}
		if !ok {
			t.Error("Teysa, Opulent Oligarch: no Repeat over the opponents who lost life")
		}
	}

	// Mindbreak Trap and Ertai's Scorn: the group count compares each
	// opponent's own spells-cast tally against the GE<n> bound, not one
	// opponent's.
	for n, bound := range map[string]int{"Mindbreak Trap": 3, "Ertai's Scorn": 2} {
		c := byName[n]
		ok := false
		for _, st := range c.Statics {
			if st.Cond.Kind != cValue || st.Cond.A.Kind != vOppExtreme {
				continue
			}
			a := st.Cond.A
			if a.N&kop.OppExtCountCmp != 0 && a.N&0xff == kop.OppExtSpellsCast &&
				st.Cond.Op == opGE && len(a.Sub) == 1 && a.Sub[0].isConst() && a.Sub[0].N == bound {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s: no seat-exact opponents' spells-cast group count with a GE%d bound", n, bound)
		}
	}
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
