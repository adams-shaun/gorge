package mtgish

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func charsCard(t *testing.T, js string) *Card {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(js), &m); err != nil {
		t.Fatal(err)
	}
	c, ok := decodeCard(m)
	if !ok {
		t.Fatalf("not a card: %s", js)
	}
	return c
}

func TestCompileCharsLowering(t *testing.T) {
	i32 := func(n int32) *int32 { return &n }
	tl := `"Typeline":{"Supertypes":[],"Cardtypes":["Creature"],"Subtypes":["Zzz"]}`
	rows := []struct {
		name string
		js   string
		want *ir.Card
	}{
		{"hybrid and phyrexian cost",
			`{"_OracleCard":"Card","Name":"A",` + tl + `,"ManaCost":[{"_ManaSymbol":"ManaCostGeneric","args":2},{"_ManaSymbol":"ManaCostWU"},{"_ManaSymbol":"ManaCostBP"},{"_ManaSymbol":"ManaCostRWP"},{"_ManaSymbol":"ManaCost2G"},{"_ManaSymbol":"ManaCostX"},{"_ManaSymbol":"ManaCostS"},{"_ManaSymbol":"ManaCostC"}],"CardPT":{"Power":1,"Toughness":2},"ColorIndicator":["Blue","Red"],"Loyalty":{"_LoyaltyNumber":"Integer","args":3},"Defense":4}`,
			&ir.Card{Name: "A", Layout: ir.LayoutNormal, Faces: []*ir.Face{{
				Name: "A", Types: []string{"Creature"}, Subtypes: []string{"Zzz"}, HasManaCost: true,
				ManaCost: []ir.ManaSym{
					{Kind: ir.ManaGeneric, N: 2}, {Kind: ir.ManaHybrid, Colors: ir.ColorW | ir.ColorU},
					{Kind: ir.ManaPhyrexian, Colors: ir.ColorB}, {Kind: ir.ManaHybridPhyrexian, Colors: ir.ColorR | ir.ColorW},
					{Kind: ir.ManaTwoHybrid, Colors: ir.ColorG, N: 2}, {Kind: ir.ManaX}, {Kind: ir.ManaSnow}, {Kind: ir.ManaColorless}},
				ColorIndicator: ir.ColorU | ir.ColorR,
				Power:          &ir.PT{N: 1}, Toughness: &ir.PT{N: 2}, Loyalty: i32(3), Defense: i32(4),
			}}}},
		{"star pt",
			`{"_OracleCard":"Card","Name":"B",` + tl + `,"Rules":[{"_Rule":"CDA_Power","args":{"_GameNumber":"Zzz"}}],"CardPT":{"Power":0,"Toughness":1}}`,
			&ir.Card{Name: "B", Layout: ir.LayoutNormal, Faces: []*ir.Face{{
				Name: "B", Types: []string{"Creature"}, Subtypes: []string{"Zzz"},
				Power: &ir.PT{Star: true}, Toughness: &ir.PT{N: 1},
				Refusals: []ir.Refusal{{Key: "GameNumber.Zzz", Path: "Rule.CDA_Power/GameNumber.Zzz"}},
			}}}},
		{"refused layout",
			`{"_OracleCard":"Scheme","Name":"C",` + tl + `,"Rules":[]}`,
			&ir.Card{Name: "C", Faces: []*ir.Face{{Name: "C", Refusals: []ir.Refusal{{Key: "OracleCard.Scheme", Path: "OracleCard.Scheme"}}}}}},
		{"split card",
			`{"_OracleCard":"Split","Cards":[{"Name":"D1",` + tl + `,"ManaCost":[{"_ManaSymbol":"ManaCostW"}],"Rules":[{"_Rule":"Flash"}]},{"Name":"D2",` + tl + `}]}`,
			&ir.Card{Name: "D1", Layout: ir.LayoutSplit, Faces: []*ir.Face{
				{Name: "D1", Types: []string{"Creature"}, Subtypes: []string{"Zzz"}, HasManaCost: true,
					ManaCost:  []ir.ManaSym{{Kind: ir.ManaColored, Colors: ir.ColorW}},
					Abilities: []*ir.Ability{{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: "Flash"}}}},
				{Name: "D2", Types: []string{"Creature"}, Subtypes: []string{"Zzz"}},
			}}},
		{"refused mana symbol",
			`{"_OracleCard":"Card","Name":"E",` + tl + `,"ManaCost":[{"_ManaSymbol":"ManaCostW"},{"_ManaSymbol":"ManaCostCW"}],"Loyalty":{"_LoyaltyNumber":"LoyaltyX"}}`,
			&ir.Card{Name: "E", Layout: ir.LayoutNormal, Faces: []*ir.Face{{
				Name: "E", Types: []string{"Creature"}, Subtypes: []string{"Zzz"},
				Refusals: []ir.Refusal{{Key: "ManaSymbol.ManaCostCW", Path: "ManaSymbol.ManaCostCW"}, {Key: "LoyaltyNumber.LoyaltyX", Path: "LoyaltyNumber.LoyaltyX"}},
			}}}},
	}
	for _, r := range rows {
		got := Compile(charsCard(t, r.js))
		if !ir.Equal(got, r.want) {
			g, _ := json.Marshal(got)
			w, _ := json.Marshal(r.want)
			t.Errorf("%s:\n got %s\nwant %s", r.name, g, w)
		}
	}
}

func TestCompileRecoversPanic(t *testing.T) {
	withStubs(t, map[string]lowerFunc{"Rule.Boom": func(*Ctx, node) (any, error) { panic("x") }})
	got := Compile(charsCard(t, `{"_OracleCard":"Card","Name":"P","Typeline":{},"Rules":[{"_Rule":"Boom"}]}`))
	if r := got.Faces[0].Refusals; len(r) != 1 || r[0].Key != "error:Rule.Boom" {
		t.Errorf("refusals %v", r)
	}
}

func TestKeywordRulesLowering(t *testing.T) {
	if len(charsKeywordNames) != 21 {
		t.Fatalf("%d keyword rules", len(charsKeywordNames))
	}
	for v, name := range charsKeywordNames {
		c := &Ctx{}
		ab, err := c.Ability(node{"_Rule": v})
		if err != nil {
			t.Errorf("%s: %v", v, err)
			continue
		}
		if !ir.Equal(ab, &ir.Ability{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: name}}) {
			t.Errorf("%s lowered to %+v", v, ab.Keyword)
		}
		_, err = c.Ability(node{"_Rule": v, "args": 1.0})
		re, ok := err.(*RefusalError)
		if !ok || re.Key != "Rule."+v {
			t.Errorf("%s with args: %v", v, err)
		}
	}
}

func TestCharsSliceVariantsRegistered(t *testing.T) {
	for v := range charsKeywordNames {
		if !Registered("Rule." + v) {
			t.Errorf("Rule.%s not registered", v)
		}
	}
	checkFamilyCoverage(t, []string{"ManaSymbol", "OracleCard", "LoyaltyNumber"}, map[string]string{})
}

func TestCompileProofSliceNoPanic(t *testing.T) {
	for _, card := range proofSlice(t) {
		got := Compile(card)
		if got == nil || len(got.Faces) == 0 {
			t.Errorf("%s: no faces", card.Name)
			continue
		}
		for i, f := range got.Faces {
			if f.Name == "" {
				t.Errorf("%s face %d: no name", card.Name, i)
			}
			refused := got.Layout == ir.LayoutNone
			if !refused && len(f.Types) == 0 {
				t.Errorf("%s face %d: no types", card.Name, i)
			}
			// An error: refusal is a lowering that failed without naming the
			// variant it stopped at, which hides the real refusal.
			for _, r := range f.Refusals {
				if strings.HasPrefix(r.Key, "error:") {
					t.Errorf("%s face %d: %s (%s)", card.Name, i, r.Key, r.Path)
				}
			}
		}
	}
}
