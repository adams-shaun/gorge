package setdata_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

func restrictCard(name string, r limited.Rarity, w int) setdata.SheetCard {
	return setdata.SheetCard{UUID: name, Name: name, Rarity: r, Weight: w}
}

func restrictFixture() setdata.Spec {
	card := restrictCard
	return setdata.Spec{Code: "TST", Config: setdata.Config{
		TotalWeight: 3,
		Variants: []setdata.Variant{
			{Weight: 2, Contents: map[string]int{"common": 2, "rare": 1}},
			{Weight: 1, Contents: map[string]int{"common": 2, "mythic": 1}},
		},
		Sheets: map[string]setdata.Sheet{
			"common": {TotalWeight: 6, Cards: []setdata.SheetCard{card("c1", 'C', 1), card("c2", 'C', 2), card("c3", 'C', 3)}},
			"rare":   {TotalWeight: 2, Cards: []setdata.SheetCard{card("r1", 'R', 1), card("r2", 'R', 1)}},
			"mythic": {TotalWeight: 1, Cards: []setdata.SheetCard{card("m1", 'M', 1)}},
			"bonus":  {TotalWeight: 18, Cards: []setdata.SheetCard{card("r3", 'R', 9), card("c4", 'C', 9)}},
		},
	}}
}

func allowSet(names ...string) func(string) bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return func(n string) bool { return m[n] }
}

// TestRestrictBorrowsSameRarity: a sheet with no allowed entry deals
// uniformly from the allowed entries of its rarities on the other sheets;
// one that cannot fill even so is emptied, and an allow-list that empties
// every variant is refused.
func TestRestrictBorrowsSameRarity(t *testing.T) {
	spec := restrictFixture()
	got, fills, err := setdata.Restrict(spec, allowSet("c1", "c3", "r3", "c4"))
	if err != nil {
		t.Fatal(err)
	}
	cards := func(name string) string {
		var b []string
		for _, c := range got.Config.Sheets[name].Cards {
			b = append(b, c.Name+":"+string(rune('0'+c.Weight)))
		}
		return strings.Join(b, ",")
	}
	if c := cards("common"); c != "c1:1,c3:3" {
		t.Errorf("common = %s", c)
	}
	if c := cards("rare"); c != "r3:1" {
		t.Errorf("rare = %s, want the bonus sheet's allowed rare at weight 1", c)
	}
	if c := cards("mythic"); c != "" {
		t.Errorf("mythic = %s, want empty (no allowed mythic anywhere)", c)
	}
	byName := map[string]setdata.SheetFill{}
	for _, f := range fills {
		byName[f.Sheet] = f
	}
	if len(fills) != 4 {
		t.Errorf("%d fills, want one per sheet", len(fills))
	}
	if f := byName["rare"]; f.Borrowed != 1 || f.Dead || f.Rarities != "R" {
		t.Errorf("rare fill %+v", f)
	}
	if f := byName["mythic"]; !f.Dead {
		t.Errorf("mythic fill %+v, want dead", f)
	}
	if f := byName["common"]; f.Allowed != 2 || f.AllowedWeight != 4 || f.Weight != 6 || f.Need != 2 {
		t.Errorf("common fill %+v", f)
	}
	if spec.Config.Sheets["rare"].Cards[0].Name != "r1" || spec.Config.Sheets["rare"].TotalWeight != 2 {
		t.Error("Restrict changed its input")
	}
	if _, _, err := setdata.Restrict(spec, allowSet("m1")); err == nil {
		t.Error("an allow-list that empties every variant was accepted")
	}
}

// TestRestrictAllowAllIsIdentity: an allow-all func returns the spec
// unchanged, with nothing borrowed or dead.
func TestRestrictAllowAllIsIdentity(t *testing.T) {
	spec := restrictFixture()
	got, fills, err := setdata.Restrict(spec, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, spec) {
		t.Errorf("allow-all changed the spec:\n got %+v\nwant %+v", got, spec)
	}
	for _, f := range fills {
		if f.Borrowed != 0 || f.Dead || f.Allowed != f.Cards || f.AllowedWeight != f.Weight {
			t.Errorf("allow-all fill %+v", f)
		}
	}
}
