package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// grantMixedScen is grantCopyScen's table with a Pretender that holds both
// kinds of as-enters grant in one override record: the Ghoul's ability 0 is its
// activated ability, ability 1 (added here) a triggered ability with a program
// (an AddTriggers$ grant, Phantasmal Image's shape).
func grantMixedScen(t *testing.T) (*scen, *prep, Slot, cgrantRef) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef{}, tbl.Cards...)
	ghoul := tbl.Cards[v.HungryGhoul]
	ghoul.Abilities = append(append([]kop.Ability{}, ghoul.Abilities...), v.Table.Cards[v.JeskaiStudent].Abilities[0])
	tbl.Cards[v.HungryGhoul] = ghoul
	if ghoul.Abilities[0].Kind != kop.AbActivated || ghoul.Abilities[1].Kind != kop.AbTriggered || len(ghoul.Abilities[1].Prog) == 0 {
		t.Fatal("precondition: the Ghoul needs an activated ability then a triggered one with a program")
	}
	tbl.Overrides = append(append([]kop.Override{}, tbl.Overrides...), kop.Override{Grant: [2]uint8{1, 2}})
	sc := newScenTable(t, &tbl, Config{})
	k := genPrepOf(sc)
	k.gen.anyGrant = true // the walk grantedAbilitiesN reads only with a grant in the table
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	return sc, k, sl, cgrantRef{uint8(len(tbl.Overrides)), v.HungryGhoul}
}

// grantInstances counts the instances of each ability of card c that sl is
// granted, as the kernel lists them (grantedAbilitiesN).
func grantInstances(s *State, k *prep, sl Slot, c CardID) (n [2]int) {
	s.grantedAbilitiesN(k, sl, func(gc CardID, ab uint8, times int) {
		if gc == c && int(ab) < len(n) {
			n[ab] += times
		}
	})
	return n
}

// TestCopyGrantMixedRecordDedupesOnce: a copy of a copy of an exception that
// grants an activated ability and a trigger has the activated ability once
// (gorge hasAbilityLine, CR 707.3) and the trigger twice (foldCopyExcept
// appends it each time), though one override record holds both grants.
func TestCopyGrantMixedRecordDedupesOnce(t *testing.T) {
	sc, k, sl, g := grantMixedScen(t)
	s := sc.s
	if k.copyGrantOnce(g) {
		t.Fatal("precondition: the record's trigger grant is not kept once")
	}
	s.copyGrantPut(k, sl, g)
	if got := grantInstances(s, k, sl, v.HungryGhoul); got != [2]int{1, 1} {
		t.Fatalf("after the first application the copy has %v instances of the Ghoul's abilities, want [1 1]", got)
	}
	s.copyGrantPut(k, sl, g) // the copy of the copy re-applies the record (copyGrantsAdd)
	if got := grantInstances(s, k, sl, v.HungryGhoul); got != [2]int{1, 2} {
		t.Fatalf("after the second application the copy has %v instances of the Ghoul's abilities, want [1 2]: activated once, trigger twice", got)
	}
}

// TestCopyGrantAppendedActivatedKeepsRepeats: a record of one activated
// ability put then appended (a Clone's GainThisAbility$ on a copy of a copy,
// copyGrantAppend) is two instances, as gorge appends it each time; only the
// mixed record's activated ability stays one.
func TestCopyGrantAppendedActivatedKeepsRepeats(t *testing.T) {
	sc, k, sl, ov := grantCopyScen(t, false)
	s := sc.s
	g := cgrantRef{ov, v.HungryGhoul}
	if !k.copyGrantOnce(g) || k.t.Cards[v.HungryGhoul].Abilities[0].Kind != kop.AbActivated {
		t.Fatal("precondition: the record holds one activated ability and no appended trigger")
	}
	k.gen.anyGrant = true // the walk grantedAbilitiesN reads only with a grant in the table
	if got := grantInstances(s, k, sl, v.HungryGhoul); got[0] != 1 {
		t.Fatalf("precondition: the scenario's one put shows %v instances, want 1 of ability 0", got)
	}
	s.copyGrantAppend(k, sl, g) // the copy of the copy appends it again
	if got := grantInstances(s, k, sl, v.HungryGhoul); got[0] != 2 {
		t.Fatalf("an activated grant put then appended shows %v instances, want 2 of ability 0", got)
	}
}
