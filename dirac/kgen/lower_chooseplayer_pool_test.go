package kgen

import (
	"strings"
	"testing"
)

// The ChoosePlayer pools beyond the one opponent (lower_chooseplayer.go,
// lower_chooseplayerres.go): Random$ True over plain opponents, the
// Choices$-less any-player pool, and the opponents at the highest life total.

// TestChoosePlayerRandomOppLowers: a random pick among plain opponents lowers
// to OpChoosePlayer in its RandomOpp form (kop.ChPlPlain, no Aura exclusion)
// and the chain's ChosenPlayer reader takes the pick, for both Choices$
// spellings; Maddening Hex's pool keeps its own "Random" form.
func TestChoosePlayerRandomOppLowers(t *testing.T) {
	for _, choices := range []string{"Player.Opponent", "Opponent"} {
		svars := map[string]string{
			"Trig":     "DB$ ChoosePlayer | Defined$ You | Choices$ " + choices + " | Random$ True | SubAbility$ DBDamage",
			"DBDamage": "DB$ DealDamage | Defined$ ChosenPlayer | NumDmg$ 2",
		}
		lir, rs := kcpPhase(t, svars)
		if len(rs) > 0 {
			t.Fatalf("Choices$ %s refused: %v", choices, rs)
		}
		if !strings.Contains(lir, "ChoosePlayer obj=You CS=RandomOpp") {
			t.Errorf("Choices$ %s: no RandomOpp ChoosePlayer:\n%s", choices, lir)
		}
		if !strings.Contains(lir, "DealDamage obj=ResChosenPlayer") {
			t.Errorf("Choices$ %s: the damage does not read the random pick:\n%s", choices, lir)
		}
	}
	// near-misses stay refused: Player.Other is not the plain-opponents pool,
	// and a pool without Random$ keeps its ask
	svars := map[string]string{
		"Trig":     "DB$ ChoosePlayer | Defined$ You | Choices$ Player.Other | Random$ True | SubAbility$ DBDamage",
		"DBDamage": "DB$ DealDamage | Defined$ ChosenPlayer | NumDmg$ 2",
	}
	if _, rs := kcpPhase(t, svars); len(rs) == 0 {
		t.Error("Choices$ Player.Other with Random$ lowered; want it refused")
	}
}

// TestChoosePlayerPoolLowers: the opponents-at-the-highest-life pool lowers to
// OpExt ExtChoosePlayerMid with the high-life bit when its X is the opponents'
// greatest life total, and refuses any other X or a chain that never reads
// the choice; the Choices$-less pool lowers to ExtChoosePlayerRes with the
// forget bit (no RememberChosen$) and a RememberChosen$ one without it.
func TestChoosePlayerPoolLowers(t *testing.T) {
	high := func(x string) map[string]string {
		return map[string]string{
			"Trig":     "DB$ ChoosePlayer | Defined$ You | Choices$ Player.Opponent+lifeEQX | SubAbility$ DBDamage",
			"DBDamage": "DB$ DealDamage | Defined$ ChosenPlayer | NumDmg$ 3",
			"X":        x,
		}
	}
	for _, x := range []string{"PlayerCountOpponents$HighestLifeTotal", "Count$OppGreatestLifeTotal"} {
		lir, rs := kcpPhase(t, high(x))
		if len(rs) > 0 {
			t.Fatalf("X %s refused: %v", x, rs)
		}
		if !strings.Contains(lir, "ChoosePlayerMid C=1") {
			t.Errorf("X %s: no high-life ChoosePlayerMid:\n%s", x, lir)
		}
	}
	if _, rs := kcpPhase(t, high("PlayerCountOpponents$LowestLifeTotal")); len(rs) == 0 {
		t.Error("X as the lowest life total lowered; want it refused")
	}
	noRead := high("PlayerCountOpponents$HighestLifeTotal")
	noRead["DBDamage"] = "DB$ DealDamage | Defined$ You | NumDmg$ 3"
	if _, rs := kcpPhase(t, noRead); len(rs) == 0 {
		t.Error("a lifeEQX pool whose chain never reads the choice lowered; want it refused")
	}

	any := func(extra string) map[string]string {
		return map[string]string{
			"Trig":     "DB$ ChoosePlayer | Defined$ You" + extra + " | SubAbility$ DBDamage",
			"DBDamage": "DB$ DealDamage | Defined$ ChosenPlayer | NumDmg$ 3",
		}
	}
	lir, rs := kcpPhase(t, any(""))
	if len(rs) > 0 {
		t.Fatalf("the Choices$-less pool refused: %v", rs)
	}
	if !strings.Contains(lir, "ChoosePlayerRes C=2") {
		t.Errorf("the Choices$-less pool is not the forgetting ChoosePlayerRes:\n%s", lir)
	}
	lir, rs = kcpPhase(t, any(" | RememberChosen$ True"))
	if len(rs) > 0 {
		t.Fatalf("the remembering Choices$-less pool refused: %v", rs)
	}
	if !strings.Contains(lir, "ChoosePlayerRes C=0") && !strings.Contains(lir, "ChoosePlayerRes //") {
		t.Errorf("the RememberChosen$ pool is not the plain ChoosePlayerRes:\n%s", lir)
	}
	// a pool with Choices$ Player and no RememberChosen$ stays refused (its
	// own class, not this one)
	if _, rs := kcpPhase(t, any(" | Choices$ Player")); len(rs) == 0 {
		t.Error("Choices$ Player without RememberChosen$ lowered; want it refused")
	}
}
