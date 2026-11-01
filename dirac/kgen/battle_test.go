package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func kbattleRefusals(t *testing.T, front, back *cards.Face, mode string, token bool) []refusal {
	t.Helper()
	card := &cards.Card{Faces: []*cards.Face{front, back}, AlternateMode: mode}
	p := &pool{byName: map[string]*cards.Card{"Synth": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synth", card, token).Refusals
}

func kbattleFaces(defense string) (*cards.Face, *cards.Face) {
	return &cards.Face{Name: "Synth", ManaCost: "2 R", Types: []string{"Battle", "Siege"}, Defense: defense},
		&cards.Face{Name: "Synth Back", ManaCost: "no cost", Types: []string{"Creature", "Dragon"}, PT: "3/3"}
}

// TestBattleRefusals: battlesAccepted false keeps every Battle refused as
// "battle (CR 310)"; with the gate open each shape the lowering does not
// read is refused under its named reason.
func TestBattleRefusals(t *testing.T) {
	has := func(rs []refusal, want string) bool {
		for _, r := range rs {
			if strings.HasPrefix(r.Reason, want) {
				return true
			}
		}
		return false
	}
	f, b := kbattleFaces("4")
	old := battlesAccepted
	battlesAccepted = false
	if !has(kbattleRefusals(t, f, b, "DoubleFaced", false), "battle (CR 310)") {
		t.Fatalf("gate closed: a battle must stay refused")
	}
	battlesAccepted = true
	defer func() { battlesAccepted = old }()

	f, b = kbattleFaces("4")
	if rs := kbattleRefusals(t, f, b, "DoubleFaced", false); has(rs, "battle") {
		t.Errorf("clean Siege refused: %v", rs)
	}
	cases := []struct {
		name string
		mk   func() (*cards.Face, *cards.Face, string, bool)
		want string
	}{
		{"modal layout", func() (*cards.Face, *cards.Face, string, bool) {
			f, b := kbattleFaces("4")
			return f, b, "Modal", false
		}, "battle layout"},
		{"X defense", func() (*cards.Face, *cards.Face, string, bool) {
			f, b := kbattleFaces("X")
			return f, b, "DoubleFaced", false
		}, "battle defense"},
		{"zero defense", func() (*cards.Face, *cards.Face, string, bool) {
			f, b := kbattleFaces("0")
			return f, b, "DoubleFaced", false
		}, "battle defense"},
		{"huge defense", func() (*cards.Face, *cards.Face, string, bool) {
			f, b := kbattleFaces("128")
			return f, b, "DoubleFaced", false
		}, "battle defense"},
		{"back is a battle", func() (*cards.Face, *cards.Face, string, bool) {
			f, b := kbattleFaces("4")
			b.Types = []string{"Battle", "Siege"}
			b.PT = ""
			return f, b, "DoubleFaced", false
		}, "battle back face is a battle"},
		{"enters replacement", func() (*cards.Face, *cards.Face, string, bool) {
			f, b := kbattleFaces("4")
			f.Repls = []cards.Repl{{Event: "Moved", Params: map[string]string{"Destination": "Battlefield", "ValidCard": "Card.Self"}}}
			return f, b, "DoubleFaced", false
		}, "battle with an enters replacement"},
		{"token", func() (*cards.Face, *cards.Face, string, bool) {
			f, b := kbattleFaces("4")
			return f, b, "DoubleFaced", true
		}, "battle token or copy"},
	}
	for _, c := range cases {
		f, b, mode, tok := c.mk()
		if rs := kbattleRefusals(t, f, b, mode, tok); !has(rs, c.want) {
			t.Errorf("%s: refusals %v, want %q", c.name, rs, c.want)
		}
	}
}
