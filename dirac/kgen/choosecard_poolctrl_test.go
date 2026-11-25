package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// ccpLower lowers one synthetic card script and returns its LIR chunk and
// refusal reasons.
func ccpLower(t *testing.T, src string) (string, []string) {
	t.Helper()
	card, diags := cards.ParseBytes("ccp_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "ChooseCard Pool Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, name, card, false)
	var rs []string
	for _, r := range lc.Refusals {
		rs = append(rs, r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, lc)
	return cardLIRChunks(sb.String())[name], rs
}

const ccpHead = "Name:ChooseCard Pool Probe\nManaCost:1 U\nTypes:Sorcery\n"

// TestChooseCardControlledByChosenPod: a mandatory ChooseCard's
// ControlledByPlayer$ Chooser restricts the pool to the chooser's own
// objects (gorge controlledByChoicePlayerChooser). The kernel matches the
// pool from the resolving controller's seat, except for the chosen-player
// chooser (RefEachOpponent), whose pool it also matches from the resolving
// controller, so the restriction is spelled in that base: YouCtrl (ctrl=1 in the
// LIR) for the resolving controller's own pick, OppCtrl (ctrl=2) for the opponent's. A pod of two
// is the only pod dirac holds, so the chosen opponent is the one opponent.
func TestChooseCardControlledByChosenPod(t *testing.T) {
	const ch = "A:SP$ ChooseCard | Mandatory$ True | Choices$ Creature | ControlledByPlayer$ Chooser"
	cases := []struct {
		name, extra, chooser, prop string
	}{
		{"no Defined$", "", "You", "ctrl=1"},
		{"Defined$ You", " | Defined$ You", "You", "ctrl=1"},
		{"Defined$ Opponent", " | Defined$ Opponent", "EachOpp", "ctrl=2"},
		{"graveyard", " | Defined$ Opponent | ChoiceZone$ Graveyard", "EachOpp", "ctrl=2"},
	}
	for _, c := range cases {
		lir, rs := ccpLower(t, ccpHead+ch+c.extra+"\n")
		if len(rs) != 0 {
			t.Errorf("%s: refused: %v", c.name, rs)
			continue
		}
		if !strings.Contains(lir, "ChooseCard") || !strings.Contains(lir, c.chooser) {
			t.Errorf("%s: no ChooseCard by %s:\n%s", c.name, c.chooser, lir)
		}
		if !strings.Contains(lir, c.prop) {
			t.Errorf("%s: pool is not restricted by %s (the LIR ctrl field):\n%s", c.name, c.prop, lir)
		}
	}

	// the chosen-player chain: the armed ChoosePlayer's pick answers, from its own pool
	svars := map[string]string{
		"Trig":       "DB$ ChoosePlayer | Defined$ You | Choices$ Opponent | SubAbility$ OppPickOne",
		"OppPickOne": "DB$ ChooseCard | Defined$ ChosenPlayer | Mandatory$ True | Choices$ Creature | ControlledByPlayer$ Chooser | SubAbility$ DBCleanup",
		"DBCleanup":  "DB$ Cleanup | ClearChosenPlayer$ True",
	}
	lir, rs := kcpPhase(t, svars)
	if len(rs) != 0 {
		t.Fatalf("Defined$ ChosenPlayer chain refused: %v", rs)
	}
	if !strings.Contains(lir, "EachOpp") || !strings.Contains(lir, "ctrl=2") {
		t.Errorf("chosen-player chain: chooser or pool restriction missing:\n%s", lir)
	}

	// near misses keep their by-name refusals
	for _, v := range []string{"Remembered", "Left", "Opponent"} {
		_, rs := ccpLower(t, ccpHead+"A:SP$ ChooseCard | Mandatory$ True | Choices$ Creature | ControlledByPlayer$ "+v+"\n")
		if !strings.Contains(strings.Join(rs, ";"), "mandatory ChooseCard ControlledByPlayer$") {
			t.Errorf("ControlledByPlayer$ %s: refusals %v, want the mandatory refusal", v, rs)
		}
	}
	for _, d := range []string{"Player", "TargetedController", "ChosenPlayer"} {
		_, rs := ccpLower(t, ccpHead+ch+" | Defined$ "+d+"\n")
		if len(rs) == 0 {
			t.Errorf("Defined$ %s with ControlledByPlayer$ Chooser lowered, want a refusal", d)
		}
	}
}

// TestChooseCardOptionalPools: an optional ChooseCard picks up to one object
// of its zone's filter pool (gorge choiceBounds: no Mandatory$ is Min 0), in
// any zone the mandatory pick reads, with ControlledByPlayer$ Chooser as the
// chooser's own objects.
func TestChooseCardOptionalPools(t *testing.T) {
	for _, z := range []string{"Hand", "Graveyard", "Exile"} {
		lir, rs := ccpLower(t, ccpHead+"A:SP$ ChooseCard | ChoiceZone$ "+z+" | Choices$ Creature.YouOwn | ChoiceTitle$ You may choose a creature card\n")
		if len(rs) != 0 {
			t.Errorf("optional ChooseCard from %s refused: %v", z, rs)
		}
		if !strings.Contains(lir, "Optional") {
			t.Errorf("optional ChooseCard from %s lost its Done:\n%s", z, lir)
		}
	}
	// an optional counted pick (Min 0, Amount$ 2) is an up-to-N Done pick
	lir, rs := ccpLower(t, ccpHead+"A:SP$ ChooseCard | Choices$ Creature | Amount$ 2\n")
	if len(rs) != 0 || !strings.Contains(lir, "AmountOpt") {
		t.Errorf("optional ChooseCard Amount$ 2: refusals %v, want an AmountOpt lowering:\n%s", rs, lir)
	}
	// near misses stay refused
	for _, extra := range []string{" | ChoiceZone$ Hand,Battlefield", " | ForgetOtherRemembered$ True"} {
		_, rs := ccpLower(t, ccpHead+"A:SP$ ChooseCard | Choices$ Creature"+extra+"\n")
		if len(rs) == 0 {
			t.Errorf("optional ChooseCard%s lowered, want a refusal", extra)
		}
	}
}
