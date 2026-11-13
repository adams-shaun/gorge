package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The MBC chosen-player-card-6 spellings (dirac/kgen/lower_chooseplayer.go,
// lower_zone.go's ChooseCard chooser, lower_effect.go's Attach destination):
//
//   - ChoosePlayer Choices$ "Player.Opponent+!EnchantedBy" with Random$ True
//     (Maddening Hex) lowers to OpChoosePlayer, and the chain's Attach
//     Defined$ ChosenPlayer and Cleanup ClearChosenPlayer$ lower beside it;
//   - a mandatory ChooseCard Defined$ ChosenPlayer (Plargg and Nassari's
//     OppPickOne, and the Demonic Hordes / Wake to Slaughter / Wormfang Crab
//     class) lowers with the armed one-opponent ChoosePlayer's pick as the
//     chooser: the opponent answers the ask.

// kcpLower lowers a synthetic curse whose one trigger (mode + params) runs
// the named chain and returns the trigger's LIR chunk and refusal reasons.
func kcpLower(t *testing.T, mode string, trigParams map[string]string, svars map[string]string) (string, []string) {
	t.Helper()
	params := map[string]string{"TriggerZones": "Battlefield", "Execute": "Trig"}
	for k, v := range trigParams {
		params[k] = v
	}
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "1 R R",
		Types:    []string{"Enchantment", "Aura", "Curse"},
		Keywords: []string{"Enchant:Player"},
		SVars:    svars,
		Triggers: []cards.Trigger{{Mode: mode, Params: params, Effect: cards.ResolveSVar(svars, "Trig")}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, "Synthetic", card, false)
	var rs []string
	for _, r := range lc.Refusals {
		rs = append(rs, r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, lc)
	return cardLIRChunks(sb.String())["Synthetic"], rs
}

// kcpPhase is kcpLower for a your-upkeep Phase trigger (the Plargg shape).
func kcpPhase(t *testing.T, svars map[string]string) (string, []string) {
	t.Helper()
	return kcpLower(t, "Phase", map[string]string{"Phase": "Upkeep", "ValidPlayer": "You"}, svars)
}

// TestChoosePlayerRandomLowers: the Maddening Hex re-attach chain lowers:
// the random ChoosePlayer emits OpChoosePlayer, the Attach takes the chosen
// player, and the Cleanup's ClearChosenPlayer$ is consumed beside it.
func TestChoosePlayerRandomLowers(t *testing.T) {
	svars := map[string]string{
		"Trig":      "DB$ RollDice | ResultSVar$ X | SubAbility$ DBDamage",
		"DBDamage":  "DB$ DealDamage | Defined$ You | NumDmg$ X | SubAbility$ DBChoose",
		"DBChoose":  "DB$ ChoosePlayer | Defined$ You | Choices$ Player.Opponent+!EnchantedBy | Random$ True | SubAbility$ DBAttach",
		"DBAttach":  "DB$ Attach | Defined$ ChosenPlayer | SubAbility$ DBCleanup",
		"DBCleanup": "DB$ Cleanup | ClearChosenPlayer$ True",
	}
	lir, rs := kcpLower(t, "SpellCast", map[string]string{
		"ValidCard": "Card.nonCreature", "ValidActivatingPlayer": "You",
	}, svars)
	if len(rs) > 0 {
		t.Fatalf("Maddening Hex chain refused: %v", rs)
	}
	if !strings.Contains(lir, "ChoosePlayer") || !strings.Contains(lir, "Random") {
		t.Errorf("no OpChoosePlayer lowering in the chain:\n%s", lir)
	}
	if !strings.Contains(lir, "ChosenPlayer") {
		t.Errorf("the Attach does not read the chosen player:\n%s", lir)
	}
	// the empty-pool shape draws nothing (gorge randomChoices with an empty
	// pool): the kernel op must pose no decision, so the lowering carries no
	// ask flag beyond the random draw
	if n := strings.Count(lir, "ChoosePlayer"); n != 1 {
		t.Errorf("%d ChoosePlayer instructions, want one:\n%s", n, lir)
	}
}

// TestChoosePlayerRandomRefuses: near-misses of the spelling stay refused by
// name, and the chain's ChosenPlayer reads are unresolved without the armed
// ChoosePlayer.
func TestChoosePlayerRandomRefuses(t *testing.T) {
	trig := map[string]string{"ValidCard": "Card.nonCreature", "ValidActivatingPlayer": "You"}
	for _, tc := range []struct{ name, choose, attach, cleanup string }{
		{"no Random$", "DB$ ChoosePlayer | Defined$ You | Choices$ Player.Opponent+!EnchantedBy | SubAbility$ DBAttach", "", ""},
		{"a plain Choices$", "DB$ ChoosePlayer | Defined$ You | Choices$ Player.Opponent | Random$ True | SubAbility$ DBAttach", "", ""},
		{"an Amount$", "DB$ ChoosePlayer | Defined$ You | Choices$ Player.Opponent+!EnchantedBy | Random$ True | Amount$ 2 | SubAbility$ DBAttach", "", ""},
		{"a non-You chooser", "DB$ ChoosePlayer | Defined$ TargetedOrController | Choices$ Player.Opponent+!EnchantedBy | Random$ True | SubAbility$ DBAttach", "", ""},
	} {
		svars := map[string]string{
			"Trig":     "DB$ RollDice | ResultSVar$ X | SubAbility$ DBDamage",
			"DBDamage": "DB$ DealDamage | Defined$ You | NumDmg$ X | SubAbility$ " + tc.name,
			tc.name:    tc.choose,
		}
		_, rs := kcpLower(t, "SpellCast", trig, svars)
		if len(rs) == 0 {
			t.Errorf("%s: lowered, want a refusal", tc.name)
		}
	}
	// the Attach and the Cleanup are unresolved without the armed ChoosePlayer
	svars := map[string]string{
		"Trig":      "DB$ RollDice | ResultSVar$ X | SubAbility$ DBDamage",
		"DBDamage":  "DB$ DealDamage | Defined$ You | NumDmg$ X | SubAbility$ DBAttach",
		"DBAttach":  "DB$ Attach | Defined$ ChosenPlayer | SubAbility$ DBCleanup",
		"DBCleanup": "DB$ Cleanup | ClearChosenPlayer$ True",
	}
	_, rs := kcpLower(t, "SpellCast", trig, svars)
	if !strings.Contains(strings.Join(rs, ";"), "ChosenPlayer") {
		t.Errorf("a ChosenPlayer chain without the ChoosePlayer: want a refusal, got %v", rs)
	}
}

// TestChooseCardChosenPlayerChooserLowers: the Plargg class — a mandatory
// ChooseCard whose Defined$ is the armed one-opponent ChoosePlayer's pick —
// lowers with the opponent answering the ask; without the armed ChoosePlayer
// it still refuses.
func TestChooseCardChosenPlayerChooserLowers(t *testing.T) {
	svars := map[string]string{
		"Trig":           "DB$ DigUntil | Defined$ Player | Valid$ Card.nonLand | FoundDestination$ Exile | RevealedDestination$ Exile | RememberFound$ True | SubAbility$ DBChoosePlayer",
		"DBChoosePlayer": "DB$ ChoosePlayer | Defined$ You | Choices$ Opponent | ChoiceTitle$ Choose an opponent | SubAbility$ OppPickOne",
		"OppPickOne":     "DB$ ChooseCard | Defined$ ChosenPlayer | Amount$ 1 | Mandatory$ True | ChoiceTitle$ Choose a nonland card | Choices$ Card.IsRemembered+nonLand | ChoiceZone$ Exile | SubAbility$ DBCleanup",
		"DBCleanup":      "DB$ Cleanup | ClearRemembered$ True",
	}
	lir, rs := kcpPhase(t, svars)
	if len(rs) > 0 {
		t.Fatalf("the Plargg chain refused: %v", rs)
	}
	// the ask is the opponent's, not the resolving controller's
	if !strings.Contains(lir, "ChooseCard") {
		t.Fatalf("no ChooseCard lowering:\n%s", lir)
	}
	if !strings.Contains(lir, "EachOpp") {
		t.Errorf("the ChooseCard chooser is not the chosen player (EachOpp):\n%s", lir)
	}
	// unresolved without the ChoosePlayer
	delete(svars, "DBChoosePlayer")
	svars["Trig"] = strings.Replace(svars["Trig"], "SubAbility$ DBChoosePlayer", "SubAbility$ OppPickOne", 1)
	_, rs = kcpPhase(t, svars)
	if !strings.Contains(strings.Join(rs, ";"), "ChosenPlayer") {
		t.Errorf("the ChooseCard without the armed ChoosePlayer: want a refusal, got %v", rs)
	}
	// any other mandatory Defined$ still refuses
	svars["OppPickOne"] = "DB$ ChooseCard | Defined$ Targeted | Amount$ 1 | Mandatory$ True | Choices$ Card.IsRemembered+nonLand | ChoiceZone$ Exile"
	_, rs = kcpPhase(t, svars)
	if !strings.Contains(strings.Join(rs, ";"), "ChooseCard Defined$") {
		t.Errorf("a mandatory ChooseCard Defined$ Targeted: want the by-name refusal, got %v", rs)
	}
}
