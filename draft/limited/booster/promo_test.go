package booster_test

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

func playSpec(t *testing.T, code string) setdata.Spec {
	t.Helper()
	spec, err := setdata.LoadCollation(code, setdata.Play)
	if err != nil {
		t.Fatalf("LoadCollation(%s): %v", code, err)
	}
	return spec
}

func TestOpenPromoRareOrMythicOfTheSet(t *testing.T) {
	for _, code := range []string{"FDN", "TRK"} {
		spec := playSpec(t, code)
		mythics := 0
		for seed := uint64(0); seed < 200; seed++ {
			c, err := booster.OpenPromo(spec, rand.New(rand.NewPCG(seed, 1)), &booster.IDs{})
			if err != nil {
				t.Fatalf("%s seed %d: %v", code, seed, err)
			}
			if c.Rarity != limited.Rare && c.Rarity != limited.Mythic {
				t.Fatalf("%s seed %d: promo %+v is not R or M", code, seed, c)
			}
			if !strings.EqualFold(c.Set, code) {
				t.Fatalf("%s seed %d: promo %+v is from set %s", code, seed, c, c.Set)
			}
			if c.Rarity == limited.Mythic {
				mythics++
			}
		}
		if mythics == 0 || mythics == 200 {
			t.Errorf("%s: %d of 200 promos were mythic, want a mix", code, mythics)
		}
	}
}

func TestOpenPromoDeterministic(t *testing.T) {
	spec := playSpec(t, "FDN")
	draw := func(seed uint64) limited.CardRef {
		c, err := booster.OpenPromo(spec, rand.New(rand.NewPCG(seed, 9)), &booster.IDs{})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	differ := false
	first := draw(0)
	for seed := uint64(0); seed < 50; seed++ {
		if a, b := draw(seed), draw(seed); a != b {
			t.Fatalf("seed %d drew %+v then %+v", seed, a, b)
		}
		if draw(seed).Name != first.Name {
			differ = true
		}
	}
	if !differ {
		t.Error("50 seeds all drew the same promo")
	}
}

func TestOpenPrereleasePodKeepsBoosters(t *testing.T) {
	spec := playSpec(t, "FDN")
	for _, seed := range []uint64{1, 77} {
		want, err := booster.OpenPod(spec, seed, 4, 6, &booster.IDs{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		out, err := booster.OpenPrereleasePod(spec, seed, 4, 6, 1, &booster.IDs{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 4 {
			t.Fatalf("%d seats, want 4", len(out))
		}
		seen := map[string]bool{}
		maxBooster := 0
		for s := range out {
			if len(out[s]) != 7 {
				t.Fatalf("seat %d has %d packs, want 7", s, len(out[s]))
			}
			if !reflect.DeepEqual(out[s][:6], want[s]) {
				t.Fatalf("seat %d boosters differ from OpenPod", s)
			}
			for _, p := range want[s] {
				maxBooster += len(p)
			}
		}
		for s := range out {
			if len(out[s][6]) != 1 {
				t.Fatalf("seat %d promo pack has %d cards", s, len(out[s][6]))
			}
			for _, p := range out[s] {
				for _, c := range p {
					if seen[c.ID] {
						t.Fatalf("ID %s repeats", c.ID)
					}
					seen[c.ID] = true
				}
			}
		}
		if len(seen) != maxBooster+4 {
			t.Fatalf("%d IDs, want %d", len(seen), maxBooster+4)
		}
		if got, want := out[0][6][0].ID, "c"+itoa(maxBooster+1); got != want {
			t.Errorf("first promo ID %s, want %s (after every booster)", got, want)
		}
	}
}

func TestOpenPrereleasePodZeroPromosIsOpenPod(t *testing.T) {
	spec := playSpec(t, "FDN")
	want, err := booster.OpenPod(spec, 5, 4, 6, &booster.IDs{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := booster.OpenPrereleasePod(spec, 5, 4, 6, 0, &booster.IDs{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("promos 0 differs from OpenPod")
	}
	if _, err := booster.OpenPrereleasePod(spec, 5, 4, 6, -1, &booster.IDs{}, nil); err == nil {
		t.Error("promos -1 succeeded")
	}
}

func TestPromoUniverseFallsBackAndDedups(t *testing.T) {
	foreign := func(name, number string, r limited.Rarity) setdata.SheetCard {
		sc := sheetCard(name, number, r, 1)
		sc.Set = "OTH" // none of the sheet's cards carries the spec's TST code
		return sc
	}
	v := []setdata.Variant{{Weight: 1, Contents: map[string]int{"a": 1, "b": 1}}}
	spec := handSpec(2, v, map[string]setdata.Sheet{
		"a": {TotalWeight: 2, Cards: []setdata.SheetCard{foreign("Rex", "20", limited.Rare), foreign("Com", "1", limited.Common)}},
		"b": {TotalWeight: 1, Cards: []setdata.SheetCard{foreign("Rex", "3", limited.Rare)}},
	})
	// The fallback universe is the one rare: every draw is Rex at number 3
	// (a string compare would pick "20").
	for seed := uint64(0); seed < 30; seed++ {
		c, err := booster.OpenPromo(spec, rand.New(rand.NewPCG(seed, 2)), &booster.IDs{})
		if err != nil {
			t.Fatal(err)
		}
		if c.Name != "Rex" || c.Number != "3" {
			t.Fatalf("seed %d drew %+v, want Rex #3", seed, c)
		}
	}
	// A card of the spec's own set wins over the foreign fallback.
	own := handSpec(2, v, map[string]setdata.Sheet{
		"a": {TotalWeight: 2, Cards: []setdata.SheetCard{foreign("Rex", "3", limited.Rare), sheetCard("Mine", "9", limited.Mythic, 1)}},
		"b": {TotalWeight: 1, Cards: []setdata.SheetCard{sheetCard("Com", "1", limited.Common, 1)}},
	})
	for seed := uint64(0); seed < 30; seed++ {
		c, err := booster.OpenPromo(own, rand.New(rand.NewPCG(seed, 2)), &booster.IDs{})
		if err != nil || c.Name != "Mine" {
			t.Fatalf("seed %d drew %+v, %v, want Mine", seed, c, err)
		}
	}
	none := handSpec(1, []setdata.Variant{{Weight: 1, Contents: map[string]int{"a": 1}}}, map[string]setdata.Sheet{
		"a": {TotalWeight: 1, Cards: []setdata.SheetCard{sheetCard("Com", "1", limited.Common, 1)}},
	})
	if _, err := booster.OpenPromo(none, rand.New(rand.NewPCG(1, 2)), &booster.IDs{}); err == nil {
		t.Error("a spec with no rare or mythic opened a promo")
	}
}
