package effects

// compound-statics1: the face CantAttack whitelist
// (CantAttackParamsReadableForRules) gained the present-gate family with an
// orphan-compare and unread-spec guard. These unit cases pin the guard's two
// fail directions directly -- the pairing the guards protect against is
// measured at zero corpus rows (orphan PresentCompare) and one
// (Flowering Lumberknot's unimplemented `withSoulbond`), so a corpus-only pin
// would not exercise them.

import "testing"

func TestCantAttackParamsReadablePresentGate(t *testing.T) {
	base := func(kv ...string) map[string]string {
		m := map[string]string{"Mode": "CantAttack", "ValidCard": "Card.Self"}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	cases := []struct {
		name string
		p    map[string]string
		want bool
	}{
		{"bast shape", base("IsPresent", "Creature.YouCtrl", "PresentCompare", "LE2"), true},
		{"isPresent alone", base("IsPresent", "Creature.YouCtrl"), true},
		{"isPresent2", base("IsPresent2", "Creature.YouCtrl", "PresentCompare", "EQ0"), true},
		{"orphan compare", base("PresentCompare", "EQ0"), false},
		{"empty isPresent + EQ0", base("IsPresent", "", "PresentCompare", "EQ0"), false},
		{"whitespace isPresent + EQ0", base("IsPresent", "   ", "PresentCompare", "EQ0"), false},
		{"empty isPresent no compare", base("IsPresent", ""), false},
		{"empty isPresent2 + EQ0", base("IsPresent2", "", "PresentCompare", "EQ0"), false},
		{"unread spec + EQ0", base("IsPresent", "Creature.PairedWith+withSoulbond", "PresentCompare", "EQ0"), false},
		{"unread spec + GE1", base("IsPresent", "Creature.PairedWith+withSoulbond"), false},
		{"readable spec + EQ0", base("IsPresent", "Creature.Other+YouCtrl+powerGE4", "PresentCompare", "EQ0"), true},
		{"other param still fails", base("IsPresent", "Creature.YouCtrl", "Cost", "1"), false},
	}
	for _, c := range cases {
		if got := CantAttackParamsReadableForRules(c.p); got != c.want {
			t.Errorf("%s: got %v, want %v (%v)", c.name, got, c.want, c.p)
		}
	}
}
