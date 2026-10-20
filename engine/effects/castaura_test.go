package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func TestCastAuraLogic(t *testing.T) {
	face := func(logic, api, valid string) *cards.Face {
		f := &cards.Face{Abilities: []*cards.SA{{Kind: "SP", API: api, Params: map[string]string{"ValidTgts": valid}}}}
		if logic != "" {
			f.SVars = map[string]string{"AttachAILogic": logic}
		}
		return f
	}
	if sa := face("Pump", "Attach", "Creature").SpellAbility(); sa == nil || sa.API != "Attach" {
		t.Fatalf("fixture precondition: spell ability = %+v, want an Attach SA", sa)
	}
	for _, tc := range []struct {
		name string
		f    *cards.Face
		want string
	}{
		{"creature aura", face("Pump", "Attach", "Creature"), "Pump"},
		{"creature you control", face("Pump", "Attach", "Creature.YouCtrl"), "Pump"},
		{"enchant land", face("Pump", "Attach", "Land"), ""},
		{"no hint", face("", "Attach", "Creature"), ""},
		{"not an attach", face("Pump", "Pump", "Creature"), ""},
		{"nil face", nil, ""},
	} {
		if got := CastAuraLogic(tc.f); got != tc.want {
			t.Errorf("%s: CastAuraLogic = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := AttachAILogic(nil); got != "" {
		t.Errorf("AttachAILogic(nil) = %q, want empty", got)
	}
}
