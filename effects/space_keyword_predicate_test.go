package effects

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func TestSpaceBearingKeywordPredicates(t *testing.T) {
	g, ids := board(t)
	add := func(owner state.PlayerID, script string) state.ObjID {
		c, diags := cards.ParseBytes("space-keyword.txt", []byte(script))
		if len(diags) != 0 {
			t.Fatalf("parse card: %v", diags)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, owner)
		o.Zone = state.ZBattlefield
		g.SetZone(state.ZBattlefield, owner, append(g.Zone(state.ZBattlefield, owner), o.ID))
		return o.ID
	}
	firstStrike := add(0, "Name:First Striker\nTypes:Creature Human\nK:First Strike\nOracle:x\n")
	doctor := add(0, "Name:Doctor's Friend\nTypes:Creature Human\nK:Doctor's companion\nOracle:x\n")
	bear := ids["myBear"]
	if g.Obj(firstStrike).Zone != state.ZBattlefield || !g.Obj(firstStrike).Face().HasKeyword("First Strike") {
		t.Fatal("First Strike precondition: expected a battlefield creature with First Strike")
	}
	if g.Obj(doctor).Zone != state.ZBattlefield || !g.Obj(doctor).Face().HasKeyword("Doctor's companion") {
		t.Fatal("Doctor's companion precondition: expected a battlefield creature with the keyword")
	}
	if g.Obj(bear).Zone != state.ZBattlefield || g.Obj(bear).Face().HasKeyword("First Strike") {
		t.Fatal("negative precondition: expected a battlefield creature without First Strike")
	}
	if !MatchesSpec(g, "Creature", firstStrike, 0) || !MatchesSpec(g, "Creature", doctor, 0) {
		t.Fatal("test precondition: expected both keyword carriers to match the Creature base")
	}

	cases := []struct {
		spec string
		id   state.ObjID
		want bool
	}{
		{"Creature.withFirst Strike", firstStrike, true},
		{"Creature.withoutFirst Strike", firstStrike, false},
		{"Creature.withoutFirst Strike", bear, true},
		{"Creature.withDoctor's companion", doctor, true},
		{"Creature.withoutDoctor's companion", doctor, false},
		{"Creature.!withFirst Strike", firstStrike, false},
		{"Creature.!withFirst Strike", bear, true},
		{"Creature.!withoutFirst Strike", firstStrike, true},
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			_, predicate, _ := strings.Cut(tc.spec, ".")
			positive, negated := strings.CutPrefix(predicate, "!")
			if kp, ok := keywordPredicateFor(positive); !ok {
				t.Fatalf("keyword classifier rejected %q", positive)
			} else if got, ok := matchPositive(g, positive, g.Obj(tc.id), SpecContext{}); !ok || (got != tc.want) != negated {
				t.Errorf("direct matchPositive(%q) = %v, %v; keyword=%q; negated=%v", positive, got, ok, kp.keyword, negated)
			}
			if got := MatchesSpec(g, tc.spec, tc.id, 0); got != tc.want {
				t.Errorf("MatchesSpec(%q) = %v, want %v", tc.spec, got, tc.want)
			}
			if unknown := UnknownPredicates(tc.spec); len(unknown) != 0 {
				t.Errorf("UnknownPredicates(%q) = %v, want no unknown predicates", tc.spec, unknown)
			}
			if strings.Contains(tc.spec, ".!with") && !SpecReadsKeywords(tc.spec) {
				t.Errorf("SpecReadsKeywords(%q) = false, want keyword dependency", tc.spec)
			}
		})
	}
}

func TestUnsupportedSpaceBearingKeywordPredicateFailsClosed(t *testing.T) {
	g, ids := board(t)
	bear := g.Obj(ids["myBear"])
	if bear.Zone != state.ZBattlefield || !slices.Contains(bear.Face().Types, "Creature") {
		t.Fatal("test precondition: expected myBear to be a battlefield creature")
	}
	for _, spec := range []string{
		"Creature.withAt the beginning of your upkeep",
		"Creature.!withAt the beginning of your upkeep",
	} {
		if got := MatchesSpec(g, spec, ids["myBear"], 0); got {
			t.Errorf("MatchesSpec(%q) = true, want unsupported wording to fail closed", spec)
		}
		unknown := UnknownPredicates(spec)
		if len(unknown) != 1 || unknown[0] != spec[len("Creature."):] {
			t.Errorf("UnknownPredicates(%q) = %v, want original token", spec, unknown)
		}
	}
}
