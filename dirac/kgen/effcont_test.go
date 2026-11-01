package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// effcontForms: the Effect Continuous pool's lowering forms
// (continuous): the multi-static until-your-next-turn emblem, the no-maximum-
// hand-size and extra-land-play grants, the hidden-keyword cant-block grant,
// and the shapes that stay refused.
var effcontForms = []formCase{
	{card: "Kardur, Doomscourge", form: "Effect MustAttack multi-static, UntilYourNextTurn (gorge effEffect MustAttack, absentRememberObjects)", lir: "emblem[0] trigger=0 statics=2 until=next"},
	{card: "Enter the Infinite", form: "Effect SetMaxHandSize$ Unlimited, UntilYourNextTurn (gorge effEffect static), no maximum hand size", lir: "emblem[0] trigger=0 statics=1 until=next"},
	{card: "Explore", form: "Effect AdjustLandPlays$ 1, this turn (gorge effEffect static), extra land play", lir: "emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Summer Bloom", form: "Effect AdjustLandPlays$ 3, this turn (gorge effEffect static), three extra land plays", lir: "emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Mournwillow", form: "Effect AddHiddenKeyword$ cant-block, this turn (gorge effEffect static), hidden keyword", lir: "emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Gruul Charm", form: "GainControl without a target stays refused (AllValid$ unread)", refused: "A[0] SP$ Charm: SP$ Charm: DB$ GainControl: unread params [AllValid]"},
	// Coward is a creature type (gorge CreatureTypeWords)
	// no pool card carries; it lowers to the pool subtype table
	{card: "Pyrophobia", form: "Coward filter, a creature type outside the pool"},
}

func TestEffcontForms(t *testing.T) { checkForms(t, effcontForms) }

func TestEffcontEmblemFields(t *testing.T) {
	cardsDir := ratchetCards(t)
	names := []string{"Kardur, Doomscourge", "Enter the Infinite", "Explore", "Summer Bloom", "Mournwillow"}
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	card := func(name string) *lCard {
		t.Helper()
		c := byName[name]
		if c == nil {
			t.Fatalf("%s: not lowered", name)
		}
		return c
	}
	oneEmblem := func(name string) lEmblem {
		t.Helper()
		c := card(name)
		if len(c.Emblems) != 1 {
			t.Fatalf("%s: %d emblems, want 1", name, len(c.Emblems))
		}
		return c.Emblems[0]
	}

	ke := oneEmblem("Kardur, Doomscourge")
	if !ke.UntilNextTurn || ke.UntilEOT || len(ke.Statics) != 2 {
		t.Fatalf("Kardur emblem = %+v, want two statics and UntilNextTurn", ke)
	}
	for i, st := range ke.Statics {
		if st.Kind != "MustAttack" || st.F == nil {
			t.Errorf("Kardur static %d = %+v, want MustAttack with a filter", i, st)
		}
	}

	ke = oneEmblem("Enter the Infinite")
	if !ke.UntilNextTurn || ke.UntilEOT || len(ke.Statics) != 1 || ke.Statics[0].Kind != "NoMaxHandSize" {
		t.Fatalf("Enter the Infinite emblem = %+v, want NoMaxHandSize UntilNextTurn", ke)
	}

	for _, tc := range []struct {
		name string
		n    int
	}{
		{"Explore", 1},
		{"Summer Bloom", 3},
	} {
		ke = oneEmblem(tc.name)
		if !ke.UntilEOT || ke.UntilNextTurn || len(ke.Statics) != 1 {
			t.Fatalf("%s emblem = %+v, want one EOT static", tc.name, ke)
		}
		st := ke.Statics[0]
		if st.Kind != "ExtraLandPlay" || !st.Amount.isConst() || st.Amount.N != tc.n {
			t.Errorf("%s static = %+v, want ExtraLandPlay %d", tc.name, st, tc.n)
		}
	}

	ke = oneEmblem("Mournwillow")
	if !ke.UntilEOT || ke.UntilNextTurn || len(ke.Statics) != 1 {
		t.Fatalf("Mournwillow emblem = %+v, want one EOT static", ke)
	}
	if st := ke.Statics[0]; st.Kind != "CantBlock" || st.F == nil {
		t.Errorf("Mournwillow static = %+v, want CantBlock with a filter", st)
	}
}

func TestEffcontDuration(t *testing.T) {
	for _, tc := range []struct {
		d      string
		eot    bool
		next   bool
		refuse bool
	}{
		{"", true, false, false},
		{"EOT", true, false, false},
		{"UntilYourNextEndStep", true, false, false},
		{"ThisTurn", true, false, false},
		{"UntilYourNextTurn", false, true, false},
		{"UntilTheEndOfYourNextTurn", false, false, true},
		{"Permanent", false, false, true},
	} {
		p := newParams(map[string]string{})
		if tc.d != "" {
			p.m["Duration"] = tc.d
		}
		eot, next, err := effectEmblemDuration(p)
		if tc.refuse {
			if err == nil {
				t.Errorf("Duration %q: no refusal", tc.d)
			}
			continue
		}
		if err != nil {
			t.Errorf("Duration %q: %v", tc.d, err)
			continue
		}
		if eot != tc.eot || next != tc.next {
			t.Errorf("Duration %q = eot %v next %v, want %v %v", tc.d, eot, next, tc.eot, tc.next)
		}
	}
}

func TestEffcontContinuousKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    map[string]string
		want string
	}{
		{
			name: "hidden",
			p:    map[string]string{"Mode": "Continuous", "Affected": "Creature.powerLE2", "AffectedZone": "Battlefield", "AddHiddenKeyword": "CARDNAME can't block."},
		},
		{
			name: "land",
			p:    map[string]string{"Mode": "Continuous", "Affected": "You", "AdjustLandPlays": "1"},
		},
		{
			name: "maxhand",
			p:    map[string]string{"Mode": "Continuous", "Affected": "You", "AffectedZone": "Battlefield", "SetMaxHandSize": "Unlimited"},
		},
		{
			name: "hidden+land",
			p:    map[string]string{"Mode": "Continuous", "Affected": "You", "AddHiddenKeyword": "CARDNAME can't block.", "AdjustLandPlays": "1"},
			want: "beside AddHiddenKeyword",
		},
		{
			name: "land+zone",
			p:    map[string]string{"Mode": "Continuous", "Affected": "You", "AffectedZone": "Battlefield", "AdjustLandPlays": "1"},
			want: "AffectedZone",
		},
		{
			name: "unknown",
			p:    map[string]string{"Mode": "Continuous", "Affected": "You", "SetMaxHandSize": "Unlimited", "Foo": "x"},
			want: "Foo",
		},
	} {
		err := effectContinuousKeys(tc.p)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want refusal containing %q", tc.name, err, tc.want)
		}
	}
}
