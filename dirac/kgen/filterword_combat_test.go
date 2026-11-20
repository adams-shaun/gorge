package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterWordsCombatLower: each ported combat/history word is a program
// literal, beside a comparison too, and only in the matcher kinds its
// manifest entry declares.
func TestFilterWordsCombatLower(t *testing.T) {
	cases := []struct {
		expr string
		zone kop.Zone
		want []kop.FLit
	}{
		{"Creature.IsGoaded", kop.ZoneBattlefield, []kop.FLit{{W: kop.FwGoaded}}},
		{"Creature.firstTurnControlled", kop.ZoneBattlefield, []kop.FLit{{W: kop.FwFirstTurnControlled}}},
		{"Creature.!firstTurnControlled", kop.ZoneBattlefield, []kop.FLit{{W: kop.FwFirstTurnControlled, Neg: true}}},
		{"Creature.ThisTurnEnteredFrom_Graveyard", kop.ZoneBattlefield, []kop.FLit{{W: kop.FwEnteredFrom, Arg: int16(kop.ZoneGraveyard)}}},
		{"Card.ThisTurnEnteredFrom_Graveyard", kop.ZoneHand, []kop.FLit{{W: kop.FwEnteredFrom, Arg: int16(kop.ZoneGraveyard)}}},
		{"Creature.!firstTurnControlled+powerGE3", kop.ZoneBattlefield, []kop.FLit{
			{W: kop.FwFirstTurnControlled, Neg: true}, {W: kop.FwPowerCmp, Op: kop.CmpGE, V: kop.Const(3)}}},
	}
	for _, tc := range cases {
		c, err := spillClause(t, tc.expr)
		if err != nil {
			t.Errorf("%s: %v", tc.expr, err)
			continue
		}
		lits, _ := progOf(t, spillBuilder(), c, tc.zone)
		if len(lits) != len(tc.want) {
			t.Errorf("%s: program %+v, want %+v", tc.expr, lits, tc.want)
			continue
		}
		for i := range lits {
			if lits[i] != tc.want[i] {
				t.Errorf("%s: literal %d is %+v, want %+v", tc.expr, i, lits[i], tc.want[i])
			}
		}
	}
}

// TestFilterWordsCombatContexts: IsGoaded declares the permanent context only
// (its static half walks the layers), so a card or no-zone read refuses, as
// does a layer-walk read; firstTurnControlled is a permanent word.
func TestFilterWordsCombatContexts(t *testing.T) {
	for _, tc := range []struct {
		expr string
		zone kop.Zone
		ok   bool
	}{
		{"Creature.IsGoaded", kop.ZoneBattlefield, true},
		{"Creature.IsGoaded", kop.ZoneGraveyard, false},
		{"Creature.IsGoaded", kop.ZoneNone, false},
		{"Creature.firstTurnControlled", kop.ZoneGraveyard, false},
		{"Card.ThisTurnEnteredFrom_Graveyard", kop.ZoneNone, true},
	} {
		c, err := spillClause(t, tc.expr)
		if err != nil {
			t.Fatal(err)
		}
		_, err = spillBuilder().clause(c, tc.zone)
		if (err == nil) != tc.ok {
			t.Errorf("%s in zone %v: err %v, want ok=%v", tc.expr, tc.zone, err, tc.ok)
		}
	}
	// the static (layer walk) read of IsGoaded is refused
	c, _ := spillClause(t, "Creature.IsGoaded")
	cb := spillBuilder()
	cb.inStatic = true
	if _, err := cb.clause(c, kop.ZoneBattlefield); err == nil || !strings.Contains(err.Error(), "filter word FwGoaded in cur") {
		t.Errorf("static IsGoaded: err %v, want a refusal naming the cur context", err)
	}
}

// TestFilterWordsCombatStayRefused: the words gorge reads from state the
// kernel does not keep, and the words gorge itself does not know, stay
// unknown properties.
func TestFilterWordsCombatStayRefused(t *testing.T) {
	for _, w := range []string{
		"attackedThisCombat", "blockedThisTurn", "!blockedThisTurn", "attackedLastTurn", "dealtDamageToYouThisTurn",
		"milledThisTurn", "DefendingPlayerCtrl", "prowled", "Transformed", "Kicked", "kicked 1",
	} {
		if _, err := spillClause(t, "Creature."+w); err == nil || !strings.Contains(err.Error(), "unknown") {
			t.Errorf("%s: err %v, want an unknown property", w, err)
		}
	}
}

// TestFilterWordsNotKicked: !kicked is the negated F2Kicked bit (PredNotF2).
func TestFilterWordsNotKicked(t *testing.T) {
	c, err := spillClause(t, "Card.!kicked")
	if err != nil {
		t.Fatal(err)
	}
	kf, err := spillBuilder().clause(c, kop.ZoneStack)
	if err != nil {
		t.Fatal(err)
	}
	if kf.Pred != kop.PredNotF2 || kop.FilterFlags2(1)<<kf.PredN != kop.F2Kicked {
		t.Errorf("Pred %d PredN %d, want PredNotF2 of F2Kicked", kf.Pred, kf.PredN)
	}
}
