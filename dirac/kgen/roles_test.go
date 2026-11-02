package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kroleTokens is the synthetic token registry: a Role (Aura, Enchant) and a
// non-Aura treasure.
func kroleTokens() map[string]*cards.Card {
	return map[string]*cards.Card{
		"role_monster": {Faces: []*cards.Face{{Name: "role_monster",
			Types: []string{"Enchantment", "Aura", "Role"}, Keywords: []string{"Enchant:Creature"}}}},
		"treasure": {Faces: []*cards.Face{{Name: "treasure", Types: []string{"Artifact"}}}},
	}
}

// kroleLower lowers a synthetic sorcery whose root SVar is root.
func kroleLower(svars map[string]string, root string) *lCard {
	sa := cards.ResolveSVar(svars, root)
	if sa == nil {
		panic("root SVar " + root)
	}
	sa.Kind = "SP" // the card's root ability
	face := &cards.Face{Name: "Synthetic", ManaCost: "1 R", Types: []string{"Sorcery"},
		SVars: svars, Abilities: []*cards.SA{sa}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{},
		reg: &cards.Registry{Tokens: kroleTokens()}}
	return lowerCard(p, "Synthetic", card, false)
}

// TestRoleRefusals: every AttachedTo$ shape the kernel does not read exactly
// is a named refusal.
func TestRoleRefusals(t *testing.T) {
	const role = "DB$ Token | TokenScript$ role_monster | TokenOwner$ You"
	cases := []struct {
		name  string
		svars map[string]string
		want  string
	}{
		{"unknown value", map[string]string{"Self": role + " | AttachedTo$ Remembered"},
			`Token AttachedTo "Remembered"`},
		{"triggered new card", map[string]string{"Self": role + " | AttachedTo$ TriggeredNewCardLKICopy"},
			`Token AttachedTo "TriggeredNewCardLKICopy"`},
		{"two chain slots", map[string]string{
			"Self": "DB$ Destroy | ValidTgts$ Creature | SubAbility$ D2",
			"D2":   "DB$ Destroy | ValidTgts$ Creature | SubAbility$ DT",
			"DT":   role + " | AttachedTo$ Targeted",
		}, "Token AttachedTo Targeted over 2 chain target slots"},
		{"this targeted without own filter", map[string]string{
			"Self": "DB$ Destroy | ValidTgts$ Creature | SubAbility$ DT",
			"DT":   role + " | AttachedTo$ ThisTargetedCard",
		}, "Token AttachedTo ThisTargetedCard without its own ValidTgts"},
		{"target slot above two", map[string]string{
			"Self": "DB$ Destroy | ValidTgts$ Creature | SubAbility$ D2",
			"D2":   "DB$ Destroy | ValidTgts$ Creature | SubAbility$ D3",
			"D3":   "DB$ Destroy | ValidTgts$ Creature | SubAbility$ DT",
			"DT":   role + " | AttachedTo$ Targeted | ValidTgts$ Creature",
		}, "Token AttachedTo target slot 3"},
		{"token amount two", map[string]string{"Self": role + " | AttachedTo$ Self | TokenAmount$ 2"},
			`Token AttachedTo with TokenAmount "2"`},
		{"non-aura token", map[string]string{
			"Self": "DB$ Token | TokenScript$ treasure | AttachedTo$ Self",
		}, `Token AttachedTo on a non-Aura token "treasure"`},
		{"aura without attachedto", map[string]string{"Self": role},
			`Aura token "role_monster" without AttachedTo$`},
	}
	for _, c := range cases {
		var got []string
		for _, r := range kroleLower(c.svars, "Self").Refusals {
			got = append(got, r.Reason)
		}
		if all := strings.Join(got, " | "); !strings.Contains(all, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
		}
	}
}

// TestRoleLowersShapes: the three read shapes lower to TokAttach's C bit and
// the destination Ref in Obj2.
func TestRoleLowersShapes(t *testing.T) {
	const role = "DB$ Token | TokenScript$ role_monster | TokenOwner$ You"
	for _, c := range []struct {
		name  string
		svars map[string]string
		ref   lRef
	}{
		{"own target", map[string]string{"Self": role + " | AttachedTo$ Targeted | ValidTgts$ Creature.YouCtrl"},
			lRef{Kind: rTarget, N: 0}},
		{"chain target", map[string]string{
			"Self": "DB$ Destroy | ValidTgts$ Creature | SubAbility$ DT",
			"DT":   role + " | AttachedTo$ Targeted",
		}, lRef{Kind: rTarget, N: 0}},
		{"self", map[string]string{"Self": role + " | AttachedTo$ Self"}, lRef{Kind: rSelf}},
	} {
		lc := kroleLower(c.svars, "Self")
		if len(lc.Refusals) != 0 {
			t.Errorf("%s: refused: %v", c.name, lc.Refusals)
			continue
		}
		var found bool
		for _, in := range lc.Abilities[0].Prog {
			if in.Op == "CreateToken" && in.C&32 != 0 {
				found = true
				if in.Obj2 != c.ref {
					t.Errorf("%s: Obj2 %v, want %v", c.name, in.Obj2, c.ref)
				}
			}
		}
		if !found {
			t.Errorf("%s: no AttachedTo CreateToken", c.name)
		}
	}
}
