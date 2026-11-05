package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestSetStateTurnUpLowering: SetState Mode$ TurnFaceUp lowers to OpExt
// ExtTurnFaceUp over one target (Expose the Culprit's first mode) or a
// Choices$ pick (Hustle // Bustle, Zimone, Ugin's Mastery); every other
// shape refuses with a named reason.
func TestSetStateTurnUpLowering(t *testing.T) {
	cases := []struct {
		name, line, want string // want "" = lowers
	}{
		{name: "target", line: "SP$ SetState | ValidTgts$ Creature.faceDown | TgtPrompt$ x | Mode$ TurnFaceUp"},
		{name: "optional pick", line: "SP$ SetState | Optional$ True | Choices$ Creature.faceDown+YouCtrl | ChoiceTitle$ x | Mode$ TurnFaceUp"},
		{name: "pick", line: "SP$ SetState | Choices$ Permanent.faceDown+YouCtrl | Mode$ TurnFaceUp"},
		{name: "defined", line: "SP$ SetState | Defined$ Self | Mode$ TurnFaceUp", want: "SetState TurnFaceUp Defined$"},
		{name: "optional target", line: "SP$ SetState | Optional$ True | ValidTgts$ Creature.faceDown | Mode$ TurnFaceUp", want: "Optional$ without Choices$"},
		{name: "amount", line: "SP$ SetState | Choices$ Creature.faceDown | Amount$ 2 | Mode$ TurnFaceUp", want: "SetState TurnFaceUp Amount$"},
		{name: "reveal", line: "SP$ SetState | ValidTgts$ Permanent.faceDown | RevealFirst$ True | Mode$ TurnFaceUp", want: "SetState TurnFaceUp RevealFirst$"},
		{name: "remember", line: "SP$ SetState | ValidTgts$ Permanent.faceDown | RememberChanged$ True | Mode$ TurnFaceUp", want: "SetState TurnFaceUp RememberChanged$"},
		{name: "no referent", line: "SP$ SetState | Mode$ TurnFaceUp", want: "without ValidTgts$ or Choices$"},
		{name: "two targets", line: "SP$ SetState | ValidTgts$ Creature.faceDown | TargetMin$ 0 | TargetMax$ 2 | Mode$ TurnFaceUp", want: "SetState TurnFaceUp TargetMin$"},
		{name: "face down", line: "SP$ SetState | ValidTgts$ Creature | Mode$ TurnFaceDown", want: `SetState Mode$ "TurnFaceDown"`},
	}
	for _, c := range cases {
		src := "Name:Synthetic\nManaCost:1 R\nTypes:Instant\nA:" + c.line + "\nOracle:x\n"
		card, diags := cards.ParseBytes("synthetic.txt", []byte(src))
		if card == nil {
			t.Fatalf("%s: unparsable: %v", c.name, diags)
		}
		p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		lc := lowerCard(p, "Synthetic", card, false)
		var got []string
		for _, r := range lc.Refusals {
			got = append(got, r.Reason)
		}
		all := strings.Join(got, " | ")
		if c.want != "" {
			if !strings.Contains(all, c.want) {
				t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
			}
			continue
		}
		if len(got) != 0 {
			t.Errorf("%s: refused: %s", c.name, all)
			continue
		}
		var b strings.Builder
		dumpLIR(&b, lc)
		if !strings.Contains(b.String(), "TurnFaceUp") {
			t.Errorf("%s: listing has no TurnFaceUp instruction:\n%s", c.name, b.String())
		}
	}
}
