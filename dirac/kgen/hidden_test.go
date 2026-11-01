package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// khiddenLower lowers a synthetic enchantment whose end-step trigger runs the given ChangeZone
// node and returns its -lir dump (header line dropped) and refusal reasons.
func khiddenLower(t *testing.T, node string) (string, []string) {
	t.Helper()
	return khiddenLowerSV(t, map[string]string{"Trig": node})
}

// khiddenLowerSV is khiddenLower over several SVars (the trigger runs "Trig").
func khiddenLowerSV(t *testing.T, svars map[string]string) (string, []string) {
	t.Helper()
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "1 B",
		Types:    []string{"Enchantment"},
		SVars:    svars,
		Triggers: []cards.Trigger{{
			Mode:   "Phase",
			Params: map[string]string{"Phase": "End of Turn", "ValidPlayer": "You", "Execute": "Trig"},
			Effect: cards.ResolveSVar(svars, "Trig"),
		}},
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

// TestHiddenDefinedLowers: Hidden$ True beside a Defined$ is inert in gorge
// (the hidden pick needs cz.Defined == ""), so the move lowers exactly as
// without it; without Defined$ or with any other value it still refuses.
func TestHiddenDefinedLowers(t *testing.T) {
	const base = "DB$ ChangeZone | Defined$ Self | Origin$ Battlefield | Destination$ Hand"
	plain, rs := khiddenLower(t, base)
	if len(rs) > 0 || plain == "" {
		t.Fatalf("baseline move did not lower: %v lir=%q", rs, plain)
	}
	hidden, rs := khiddenLower(t, base+" | Hidden$ True")
	if len(rs) > 0 {
		t.Fatalf("Hidden$ True with Defined$ refused: %v", rs)
	}
	if hidden != plain {
		t.Errorf("Hidden$ True changed the lowering:\n%s\nvs\n%s", hidden, plain)
	}
	_, rs = khiddenLower(t, "DB$ ChangeZone | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Hand | Hidden$ True")
	if !strings.Contains(strings.Join(rs, ";"), "Hidden$ without ChangeType$") {
		t.Errorf("targeted Hidden$ move: want the Hidden$ refusal, got %v", rs)
	}
	_, rs = khiddenLower(t, base+" | Hidden$ False")
	if !strings.Contains(strings.Join(rs, ";"), "Hidden$ without ChangeType$") {
		t.Errorf("Hidden$ False: want the Hidden$ refusal, got %v", rs)
	}
	// GainControl$ ChosenPlayer without the armed one-opponent ChoosePlayer
	// (and any other non-True value) names a controller the kernel's move
	// cannot give; it refuses, never lowers as "your control" or as none
	for _, gc := range []string{"ChosenPlayer", "Opponent", "False"} {
		_, rs = khiddenLower(t, base+" | Hidden$ True | GainControl$ "+gc)
		if !strings.Contains(strings.Join(rs, ";"), "GainControl$") {
			t.Errorf("GainControl$ %s: want a GainControl$ refusal, got %v", gc, rs)
		}
	}
}

// khiddenZuko lowers Zuko, Conflicted's exile / ChoosePlayer / MoveToPlay
// chain on a synthetic enchantment with the given MoveToPlay node.
func khiddenZuko(t *testing.T, move string) (string, []string) {
	t.Helper()
	return khiddenLowerSV(t, map[string]string{
		"Trig":  "DB$ ChangeZone | Defined$ Self | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | SubAbility$ Opp",
		"Opp":   "DB$ ChoosePlayer | Defined$ You | Choices$ Opponent | SubAbility$ Move",
		"Move":  move + " | SubAbility$ Clean",
		"Clean": "DB$ Cleanup | ClearRemembered$ True | ClearChosenPlayer$ True",
	})
}

// TestHiddenZukoLowers: GainControl$ ChosenPlayer on a Defined$ Remembered
// move to the battlefield, after the one-opponent ChoosePlayer, lowers to a
// Return carrying UnderOpponent (kop.RetUnderOpponent); every other shape
// refuses with a GainControl$ reason.
func TestHiddenZukoLowers(t *testing.T) {
	const mv = "DB$ ChangeZone | Hidden$ True | Origin$ All | Destination$ Battlefield | Defined$ Remembered | GainControl$ "
	lir, rs := khiddenZuko(t, mv+"ChosenPlayer")
	if len(rs) > 0 || !strings.Contains(lir, "Return obj=Remembered C=1 CS=UnderOpponent") {
		t.Fatalf("Zuko's MoveToPlay: refusals %v lir=%s", rs, lir)
	}
	if lir2, rs := khiddenZuko(t, mv+"Player.Chosen"); len(rs) > 0 || lir2 != lir {
		t.Errorf("Player.Chosen: refusals %v lir=%s", rs, lir2)
	}
	for _, bad := range []string{
		mv + "Opponent",
		mv + "ChosenPlayer | WithCountersType$ P1P1",
		mv + "ChosenPlayer | Attacking$ True",
		mv + "ChosenPlayer | Transformed$ True",
		"DB$ ChangeZone | Hidden$ True | Origin$ All | Destination$ Battlefield | ChangeType$ Creature | GainControl$ ChosenPlayer",
		"DB$ ChangeZone | Hidden$ True | Origin$ All | Destination$ Hand | Defined$ Remembered | GainControl$ ChosenPlayer",
		"DB$ ChangeZone | Hidden$ True | Origin$ All | Destination$ Battlefield | Defined$ Self | GainControl$ ChosenPlayer",
	} {
		if _, rs := khiddenZuko(t, bad); len(rs) == 0 {
			t.Errorf("%q lowered; want a refusal", bad)
		}
	}
	// no ChoosePlayer armed: ChosenPlayer is unresolved
	_, rs = khiddenLower(t, mv+"ChosenPlayer")
	if !strings.Contains(strings.Join(rs, ";"), "GainControl$") {
		t.Errorf("ChosenPlayer without ChoosePlayer: want a GainControl$ refusal, got %v", rs)
	}
}
