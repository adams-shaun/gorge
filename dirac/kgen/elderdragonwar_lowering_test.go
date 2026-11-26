package kgen

import (
	"strings"
	"testing"
)

// The Elder Dragon War's shape (K:Read ahead beside a three-chapter Saga),
// rewritten under a synthetic name: the printed script is Forge's and lives
// only in .cards.
const readAheadSaga = `Name:Read Ahead Dragon Saga
ManaCost:2 R R
Types:Enchantment Saga
K:Read ahead
K:Chapter:3:ChapOne,ChapTwo,ChapThree
SVar:ChapOne:DB$ DamageAll | NumDmg$ 2 | ValidCards$ Creature | ValidPlayers$ Opponent | ValidDescription$ each creature and each opponent. | SpellDescription$ Two damage to each creature and each opponent.
SVar:ChapTwo:DB$ Discard | Mode$ TgtChoose | AnyNumber$ True | Optional$ True | RememberDiscarded$ True | SubAbility$ ChapTwoDraw | SpellDescription$ Discard any number, then draw that many.
SVar:ChapTwoDraw:DB$ Draw | NumCards$ Y | Defined$ You | SubAbility$ ChapTwoEnd
SVar:ChapTwoEnd:DB$ Cleanup | ClearRemembered$ True
SVar:Y:Count$RememberedSize
SVar:ChapThree:DB$ Token | TokenScript$ r_4_4_dragon_flying | SpellDescription$ A 4/4 red Dragon with flying.
`

// TestElderDragonWarLowering: K:Read ahead lowers (no refusal) to one enters
// program that opens with the as-enters number ask over 1..the final chapter,
// beside the three chapter triggers; a face the ask cannot be read from
// refuses by name.
func TestElderDragonWarLowering(t *testing.T) {
	c := lowerScriptCard(t, "Read Ahead Dragon Saga", readAheadSaga)
	for _, rf := range c.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	if c.SagaN != 3 {
		t.Fatalf("SagaN = %d, want 3", c.SagaN)
	}
	n := 0
	for _, r := range c.Repls {
		if r.Kind != "EntersWith" {
			continue
		}
		n++
		if len(r.Prog) != 1 || r.Prog[0].Op != "ChooseNumber" || !r.Prog[0].B.isConst() || r.Prog[0].B.N != 1 || r.Prog[0].C != 3 {
			t.Errorf("enters program = %+v, want one ChooseNumber over 1..3", r.Prog)
		}
		if r.S == "Updated" {
			t.Error("the ask must be the stack pre-ask (gorge asks a cast Saga at its resolution), not an R: Updated line")
		}
	}
	if n != 1 {
		t.Errorf("%d enters programs, want 1", n)
	}
	chapters := 0
	for _, ab := range c.Abilities {
		if ab.Name == "Chapter" {
			chapters++
		}
	}
	if chapters != 3 {
		t.Errorf("%d chapter triggers, want 3", chapters)
	}
}

// TestReadAheadRefusals: read ahead without chapters, or beside an enters
// choice of the face's own, refuses by name rather than dropping the keyword.
func TestReadAheadRefusals(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"no chapters", strings.Replace(readAheadSaga, "K:Chapter:3:ChapOne,ChapTwo,ChapThree\n", "", 1), "without chapters"},
		{"own number choice", readAheadSaga + `K:ETBReplacement:Other:ChooseN
SVar:ChooseN:DB$ ChooseNumber | Defined$ You | Min$ 1 | Max$ 3 | SpellDescription$ choose a number
`, "ChooseNumber"},
	} {
		c := lowerScriptCard(t, "Read Ahead Dragon Saga", tc.src)
		var all []string
		for _, rf := range c.Refusals {
			all = append(all, rf.Reason)
		}
		if !strings.Contains(strings.Join(all, "\n"), tc.want) {
			t.Errorf("%s: refusals %v, want one containing %q", tc.name, all, tc.want)
		}
	}
}
