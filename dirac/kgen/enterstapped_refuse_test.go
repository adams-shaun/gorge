package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// enterstappedRefuseTapRepl builds a self ETB Tap replacement with the given Tap-node
// params and returns the lowering's refusal reasons.
func enterstappedRefuseTapRepl(t *testing.T, validCard string, with *cards.SA) []refusal {
	t.Helper()
	rp := map[string]string{
		"Destination":       "Battlefield",
		"ReplaceWith":       "LandTapped",
		"ReplacementResult": "Updated",
		"ValidCard":         validCard,
	}
	if validCard != "Card.Self" {
		rp["ActiveZones"] = "Battlefield"
	}
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "no cost",
		Types:    []string{"Land"},
		Repls:    []cards.Repl{{Event: "Moved", Params: rp, With: with}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false).Refusals
}

func enterstappedRefuseTap(ps map[string]string) *cards.SA {
	m := map[string]string{"Defined": "Self", "ETB": "True"}
	for k, v := range ps {
		m[k] = v
	}
	return &cards.SA{Kind: "DB", API: "Tap", Params: m}
}

// TestEntersTappedRefusals: every conditional-enters-tapped shape the lowering does
// not read exactly is refused with a named reason.
func TestEntersTappedRefusals(t *testing.T) {
	cases := []struct {
		name      string
		validCard string
		with      *cards.SA
		want      string
	}{
		{
			name:      "no compare",
			validCard: "Card.Self",
			with:      enterstappedRefuseTap(map[string]string{"ConditionPresent": "Land.YouCtrl"}),
			want:      "ConditionPresent without ConditionCompare",
		},
		{
			name:      "chained",
			validCard: "Card.Self",
			with: func() *cards.SA {
				s := enterstappedRefuseTap(map[string]string{"ConditionPresent": "Land.YouCtrl", "ConditionCompare": "GT2"})
				s.Sub = enterstappedRefuseTap(map[string]string{"ConditionPresent": "Land.YouCtrl", "ConditionCompare": "GT2"})
				return s
			}(),
			want: "ConditionPresent on a chained ETB replacement",
		},
		{
			name:      "condition zone",
			validCard: "Card.Self",
			with: enterstappedRefuseTap(map[string]string{
				"ConditionPresent": "Land.YouCtrl", "ConditionCompare": "GT2",
				"ConditionZone": "Battlefield",
			}),
			want: "Tap unread params [ConditionZone]",
		},
		{
			name:      "condition defined",
			validCard: "Card.Self",
			with: enterstappedRefuseTap(map[string]string{
				"ConditionPresent": "Land.YouCtrl", "ConditionCompare": "GT2",
				"ConditionDefined": "You",
			}),
			want: "Tap unread params [ConditionDefined]",
		},
		{
			name:      "condition present2",
			validCard: "Card.Self",
			with: enterstappedRefuseTap(map[string]string{
				"ConditionPresent": "Land.YouCtrl", "ConditionCompare": "GT2",
				"ConditionPresent2": "Creature.YouCtrl",
			}),
			want: "Tap unread params [ConditionPresent2]",
		},
		{
			name:      "condition check svar",
			validCard: "Card.Self",
			with: enterstappedRefuseTap(map[string]string{
				"ConditionPresent": "Land.YouCtrl", "ConditionCompare": "GT2",
				"ConditionCheckSVar": "X",
			}),
			want: "ConditionCheckSVar beside another ETB replacement condition",
		},
		{
			name:      "other object",
			validCard: "Permanent.YouCtrl",
			with: enterstappedRefuseTap(map[string]string{
				"Defined":          "ReplacedCard",
				"ConditionPresent": "Land.YouCtrl", "ConditionCompare": "GT2",
			}),
			want: "ConditionPresent on an others-enter replacement",
		},
		{
			name:      "unreadable spec",
			validCard: "Card.Self",
			with:      enterstappedRefuseTap(map[string]string{"ConditionPresent": "Bogus.Whatever", "ConditionCompare": "EQ0"}),
			want:      "filter",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rs := enterstappedRefuseTapRepl(t, c.validCard, c.with)
			for _, r := range rs {
				if strings.Contains(r.Reason, c.want) {
					return
				}
			}
			t.Fatalf("want a refusal containing %q, got %v", c.want, rs)
		})
	}
}

// enterstappedRefuseCost builds a triggered ability whose Execute$ is an AB$ carrying a
// resolution cost, and returns the lowering's refusal reasons.
func enterstappedRefuseCost(t *testing.T, exec string, svars map[string]string) []refusal {
	t.Helper()
	if svars == nil {
		svars = map[string]string{}
	}
	svars["Body"] = "DB$ Pump | ValidTgts$ Creature.YouCtrl"
	svars["Trig"] = exec
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "1 G",
		Types:    []string{"Enchantment"},
		SVars:    svars,
		Triggers: []cards.Trigger{{
			Mode:   "Phase",
			Params: map[string]string{"Phase": "BeginCombat", "ValidPlayer": "You", "Execute": "Trig"},
			Effect: cards.ResolveSVar(svars, "Trig"),
		}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false).Refusals
}

func enterstappedRefuseWant(t *testing.T, rs []refusal, want string) {
	t.Helper()
	for _, r := range rs {
		if strings.Contains(r.Reason, want) {
			return
		}
	}
	t.Fatalf("want a refusal containing %q, got %v", want, rs)
}

// TestSourceCounterCostRefusals: every B/C resolution-cost shape the lowering does not
// read exactly is refused with a named reason.
func TestSourceCounterCostRefusals(t *testing.T) {
	cases := []struct {
		name  string
		exec  string
		svars map[string]string
		want  string
	}{
		{
			name: "subcounter any kind",
			exec: "AB$ ImmediateTrigger | Cost$ SubCounter<1/Any> | Execute$ Body",
			want: `counter cost "SubCounter<1/Any>": Any without a source or filter`,
		},
		{
			name: "subcounter unknown kind",
			exec: "AB$ ImmediateTrigger | Cost$ SubCounter<1/Bogus> | Execute$ Body",
			want: `resolution SubCounter cost of kind "Bogus"`,
		},
		{
			name: "subcounter beside another part",
			exec: "AB$ ImmediateTrigger | Cost$ SubCounter<1/P1P1> 1 | Execute$ Body",
			want: "resolution SubCounter cost shape",
		},
		{
			name: "tapx beside another part",
			exec: "AB$ ImmediateTrigger | Cost$ tapXType<1/Creature> 1 | Execute$ Body",
			want: "resolution tapXType cost shape",
		},
		{
			// lowers a lone unless tapXType; one
			// beside another part stays refused
			name: "tapx beside mana as an unless cost",
			exec: "AB$ Effect | UnlessCost$ 1 tapXType<1/Creature> | UnlessPayer$ You | Execute$ Body",
			want: "resolution tapXType cost shape",
		},
		{
			name: "cantblockby remembered attacker",
			exec: "AB$ Effect | Cost$ tapXType<1/Creature.Other> | RememberObjects$ TriggeredAttacker | ExileOnMoved$ Battlefield | StaticAbilities$ Unblockable",
			svars: map[string]string{
				"Unblockable": "Mode$ CantBlockBy | ValidAttacker$ Card.IsRemembered",
			},
			want: "Effect CantBlockBy RememberObjects",
		},
		{
			name: "cantblockby remembered attacker LKI copy",
			exec: "AB$ Effect | Cost$ tapXType<1/Creature.Other> | RememberObjects$ TriggeredAttackerLKICopy | ExileOnMoved$ Battlefield | StaticAbilities$ Unblockable",
			svars: map[string]string{
				"Unblockable": "Mode$ CantBlockBy | ValidAttacker$ Card.IsRemembered",
			},
			want: "Effect CantBlockBy RememberObjects LKICopy",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			enterstappedRefuseWant(t, enterstappedRefuseCost(t, c.exec, c.svars), c.want)
		})
	}
}
